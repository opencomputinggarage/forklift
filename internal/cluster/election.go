// Package cluster implements active/standby high availability via Kubernetes
// Lease-based leader election. Only the elected leader becomes Ready (so the
// Service routes to a single active instance) and runs the background blob
// sweeper, which guarantees a single writer to the shared SQLite database.
package cluster

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
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

	// mu guards the live leadership term state below, mutated from the election
	// loop and read by StepDown.
	mu sync.Mutex
	// leading reports whether this instance currently holds leadership.
	leading bool
	// steppingDown marks the current term as a voluntary hand-off so the loop
	// applies a longer re-contention cooldown after it ends.
	steppingDown bool
	// termCancel cancels the current leadership term (releasing the Lease via
	// ReleaseOnCancel). Nil between terms.
	termCancel context.CancelFunc
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
		// Each term runs under its own cancellable context so StepDown can end
		// just this term (releasing the Lease) without tearing down the process.
		termCtx, cancel := context.WithCancel(ctx)
		e.mu.Lock()
		e.termCancel = cancel
		e.mu.Unlock()

		leaderelection.RunOrDie(termCtx, leaderelection.LeaderElectionConfig{
			Lock:            lock,
			ReleaseOnCancel: true,
			LeaseDuration:   e.cfg.LeaseDuration,
			RenewDeadline:   e.cfg.RenewDeadline,
			RetryPeriod:     e.cfg.RetryPeriod,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(c context.Context) {
					e.mu.Lock()
					e.leading = true
					e.mu.Unlock()
					e.log.Info("acquired leadership", "identity", e.cfg.Identity)
					onStarted(c)
				},
				OnStoppedLeading: func() {
					e.mu.Lock()
					e.leading = false
					e.mu.Unlock()
					e.log.Warn("lost leadership", "identity", e.cfg.Identity)
					onStopped()
				},
			},
		})
		cancel()

		// RunOrDie returns when leadership is lost or the term is cancelled. After
		// a voluntary step-down, pause longer than a standby's acquisition latency
		// (a full LeaseDuration plus a retry tick) so the freed Lease is taken over
		// instead of being re-grabbed by this instance; otherwise back off briefly.
		e.mu.Lock()
		e.termCancel = nil
		backoff := e.cfg.RetryPeriod
		if e.steppingDown {
			e.steppingDown = false
			backoff = e.cfg.LeaseDuration + e.cfg.RetryPeriod
		}
		e.mu.Unlock()

		select {
		case <-ctx.Done():
		case <-time.After(backoff):
		}
	}
}

// StepDown voluntarily releases leadership for a controlled manual failover. It
// cancels the current term so the election loop releases the Lease (via
// ReleaseOnCancel) and then pauses re-contention long enough for a standby to
// acquire it. It is a no-op returning false when this instance is not currently
// the leader. The vacated Lease records a transition, so the new leader's
// fencing token strictly exceeds this one, preserving the single-writer guard.
func (e *Elector) StepDown() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.leading || e.termCancel == nil {
		return false
	}
	e.log.Info("stepping down leadership on request", "identity", e.cfg.Identity)
	e.steppingDown = true
	e.termCancel()
	return true
}
