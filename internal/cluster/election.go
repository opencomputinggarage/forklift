// Package cluster implements active/standby high availability via Kubernetes
// Lease-based leader election. Only the elected leader becomes Ready (so the
// Service routes to a single active instance) and runs the background blob
// sweeper, which guarantees a single writer to the shared SQLite database.
package cluster

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	"github.com/younsl/o/box/kubernetes/forklift/internal/config"
)

// Elector runs leader election against a Lease object.
type Elector struct {
	cfg    config.HAConfig
	log    *slog.Logger
	client kubernetes.Interface
}

// New builds an Elector using the in-cluster Kubernetes config.
func New(cfg config.HAConfig, log *slog.Logger) (*Elector, error) {
	restCfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("in-cluster config: %w", err)
	}
	client, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return nil, fmt.Errorf("kubernetes client: %w", err)
	}
	return &Elector{cfg: cfg, log: log, client: client}, nil
}

// LeaderIdentity returns the current Lease holder's identity (the leader pod
// name), or "" when the Lease does not exist or has no holder. Replication
// standbys use this to locate the leader pod via the headless Service.
func (e *Elector) LeaderIdentity(ctx context.Context) (string, error) {
	lease, err := e.client.CoordinationV1().Leases(e.cfg.LeaseNamespace).
		Get(ctx, e.cfg.LeaseName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("get lease: %w", err)
	}
	if lease.Spec.HolderIdentity == nil {
		return "", nil
	}
	return *lease.Spec.HolderIdentity, nil
}

// FencingToken returns the Lease's transition counter, a value that increases
// by one every time leadership changes hands. The leader uses it as a fencing
// token: writes to shared storage carry this token, and a stale ("zombie")
// former leader — paused past its lease and superseded — carries a lower token,
// so the storage layer can reject its writes and prevent split-brain overwrites.
// Returns 0 when the Lease is absent or has no recorded transitions.
func (e *Elector) FencingToken(ctx context.Context) (int64, error) {
	lease, err := e.client.CoordinationV1().Leases(e.cfg.LeaseNamespace).
		Get(ctx, e.cfg.LeaseName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("get lease: %w", err)
	}
	if lease.Spec.LeaseTransitions == nil {
		return 0, nil
	}
	return int64(*lease.Spec.LeaseTransitions), nil
}

// Run contends for leadership until ctx is cancelled. onStarted is invoked with
// a context that is cancelled when leadership is lost; onStopped is invoked when
// this instance stops leading. The election loop re-contends after losing
// leadership so a demoted instance can become leader again later.
func (e *Elector) Run(ctx context.Context, onStarted func(context.Context), onStopped func()) {
	lock := &resourcelock.LeaseLock{
		LeaseMeta:  metav1.ObjectMeta{Name: e.cfg.LeaseName, Namespace: e.cfg.LeaseNamespace},
		Client:     e.client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{Identity: e.cfg.Identity},
	}
	for ctx.Err() == nil {
		leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
			Lock:            lock,
			ReleaseOnCancel: true,
			LeaseDuration:   e.cfg.LeaseDuration,
			RenewDeadline:   e.cfg.RenewDeadline,
			RetryPeriod:     e.cfg.RetryPeriod,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(c context.Context) {
					e.log.Info("acquired leadership", "identity", e.cfg.Identity)
					onStarted(c)
				},
				OnStoppedLeading: func() {
					e.log.Warn("lost leadership", "identity", e.cfg.Identity)
					onStopped()
				},
			},
		})
		// RunOrDie returns when leadership is lost or ctx is cancelled. Back off
		// briefly before re-contending to avoid a tight loop.
		select {
		case <-ctx.Done():
		case <-time.After(e.cfg.RetryPeriod):
		}
	}
}
