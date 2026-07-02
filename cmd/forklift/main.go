// Command forklift is a lightweight, Kubernetes-native artifact repository.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/go-chi/chi/v5"

	"github.com/younsl/o/box/kubernetes/forklift/internal/api"
	"github.com/younsl/o/box/kubernetes/forklift/internal/artifactscan"
	"github.com/younsl/o/box/kubernetes/forklift/internal/audit"
	"github.com/younsl/o/box/kubernetes/forklift/internal/auth"
	"github.com/younsl/o/box/kubernetes/forklift/internal/cluster"
	"github.com/younsl/o/box/kubernetes/forklift/internal/config"
	"github.com/younsl/o/box/kubernetes/forklift/internal/license"
	"github.com/younsl/o/box/kubernetes/forklift/internal/memlimit"
	"github.com/younsl/o/box/kubernetes/forklift/internal/meta"
	"github.com/younsl/o/box/kubernetes/forklift/internal/metrics"
	"github.com/younsl/o/box/kubernetes/forklift/internal/notify"
	"github.com/younsl/o/box/kubernetes/forklift/internal/objstore"
	"github.com/younsl/o/box/kubernetes/forklift/internal/openapi"
	"github.com/younsl/o/box/kubernetes/forklift/internal/replication"
	"github.com/younsl/o/box/kubernetes/forklift/internal/repo"
	"github.com/younsl/o/box/kubernetes/forklift/internal/server"
	"github.com/younsl/o/box/kubernetes/forklift/internal/storage"
	"github.com/younsl/o/box/kubernetes/forklift/internal/version"
	"github.com/younsl/o/box/kubernetes/forklift/internal/vuln"
	"github.com/younsl/o/box/kubernetes/forklift/internal/webui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}

	showVersion := flag.Bool("version", false, "print version and exit")
	// Vulnerability (OSV) and license (deps.dev) scanning are configured via
	// flags. The Load() values seed the defaults so any FORKLIFT_* env still
	// applies, while flags take precedence. An empty URL disables that scanner.
	flag.StringVar(&cfg.Vuln.OSVURL, "osv-url", cfg.Vuln.OSVURL,
		"OSV API base URL for vulnerability scanning; empty disables it")
	flag.DurationVar(&cfg.Vuln.RescanInterval, "vuln-rescan-interval", cfg.Vuln.RescanInterval,
		"how often stale vulnerability scan results are re-queried")
	flag.DurationVar(&cfg.Vuln.TTL, "vuln-ttl", cfg.Vuln.TTL,
		"age at which a vulnerability scan result becomes stale")
	flag.IntVar(&cfg.Vuln.Workers, "vuln-workers", cfg.Vuln.Workers,
		"number of concurrent vulnerability scan workers draining the queue")
	flag.StringVar(&cfg.License.DepsDevURL, "deps-dev-url", cfg.License.DepsDevURL,
		"deps.dev API base URL for license scanning; empty disables it")
	flag.DurationVar(&cfg.License.RescanInterval, "license-rescan-interval", cfg.License.RescanInterval,
		"how often stale license results are re-queried")
	flag.DurationVar(&cfg.License.TTL, "license-ttl", cfg.License.TTL,
		"age at which a license result becomes stale")
	flag.IntVar(&cfg.License.Workers, "license-workers", cfg.License.Workers,
		"number of concurrent license resolution workers draining the queue")
	flag.BoolVar(&cfg.ArtifactScan.Enabled, "artifact-scan-enabled", cfg.ArtifactScan.Enabled,
		"enable optional isolated artifact-byte scanning")
	flag.StringVar(&cfg.ArtifactScan.DefaultProfile, "artifact-scan-default-profile", cfg.ArtifactScan.DefaultProfile,
		"default artifact scanner profile")
	flag.BoolVar(&cfg.ArtifactScan.StoreSBOM, "artifact-scan-store-sbom", cfg.ArtifactScan.StoreSBOM,
		"store SBOM inventory for the default artifact scanner profile")
	flag.StringVar(&cfg.ArtifactScan.WorkerToken, "artifact-scan-worker-token", cfg.ArtifactScan.WorkerToken,
		"bearer token required by scanner workers to claim jobs")
	flag.Parse()
	if *showVersion {
		fmt.Println("forklift", version.String())
		return
	}

	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config) error {
	log := newLogger(cfg)
	log.Info("starting forklift", "version", version.String(), "data_dir", cfg.DataDir)

	// Keep the GC ahead of the container memory limit so request bursts degrade
	// into extra GC work instead of an OOMKill.
	memlimit.Apply(log)

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := meta.Open(ctx, filepath.Join(cfg.DataDir, "forklift.db"))
	if err != nil {
		return fmt.Errorf("open metadata store: %w", err)
	}
	defer store.Close()

	reg := prometheus.NewRegistry()
	reg.MustRegister(prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	// Build metadata, exposed as a constant gauge=1 (standard exporter pattern).
	buildInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "forklift", Name: "build_info",
		Help: "Build metadata; the value is always 1.",
	}, []string{"version", "commit", "go_version"})
	buildInfo.WithLabelValues(version.Version, version.Commit, runtime.Version()).Set(1)
	reg.MustRegister(buildInfo)

	// Repository inventory and physical storage usage, computed per scrape.
	reg.MustRegister(metrics.NewStorageCollector(store))

	// Blob store backend: local filesystem (default) or S3. In s3 mode blobs are
	// shared directly in the bucket and the metadata database is snapshotted to
	// S3 by metaSync, so the deployment needs no EBS/RWX volume.
	var blobs storage.WalkableStore
	var metaSync *objstore.MetaSync
	switch cfg.Storage.Backend {
	case "s3":
		s3blobs, err := storage.NewS3BlobStore(ctx, toS3Config(cfg.Storage.S3), filepath.Join(cfg.DataDir, "blob-tmp"))
		if err != nil {
			return fmt.Errorf("open s3 blob store: %w", err)
		}
		blobs = s3blobs
		metaKey := path.Join(strings.Trim(cfg.Storage.S3.Prefix, "/"), "meta", "forklift.db")
		metaSync = objstore.NewMetaSync(objstore.MetaOptions{
			Store:      store,
			API:        s3blobs.Client(),
			Bucket:     cfg.Storage.S3.Bucket,
			Key:        metaKey,
			DataDir:    cfg.DataDir,
			Interval:   cfg.Storage.MetaSyncInterval,
			Log:        log,
			Registerer: reg,
		})
		// Restore the latest snapshot before bootstrap/seed so they see existing
		// data. The live SQLite file lives on an ephemeral volume that loses its
		// contents on restart; an empty bucket is a clean no-op.
		if err := metaSync.RestoreOnBoot(ctx); err != nil {
			return fmt.Errorf("restore metadata from s3: %w", err)
		}
		go metaSync.Run(ctx)
		log.Info("storage backend: s3", "bucket", cfg.Storage.S3.Bucket, "prefix", cfg.Storage.S3.Prefix)
	default:
		fsblobs, err := storage.NewFSStore(cfg.DataDir)
		if err != nil {
			return fmt.Errorf("open blob store: %w", err)
		}
		blobs = fsblobs
	}

	// Auth: optional Keycloak OIDC plus local users and PATs.
	var oidcProvider *auth.OIDCProvider
	if cfg.Auth.OIDC.Enabled {
		oidcProvider, err = auth.NewOIDC(ctx, auth.OIDCParams{
			IssuerURL:     cfg.Auth.OIDC.IssuerURL,
			ClientID:      cfg.Auth.OIDC.ClientID,
			ClientSecret:  cfg.Auth.OIDC.ClientSecret,
			RedirectURL:   cfg.Auth.OIDC.RedirectURL,
			UsernameClaim: cfg.Auth.OIDC.UsernameClaim,
			GroupsClaim:   cfg.Auth.OIDC.GroupsClaim,
		})
		if err != nil {
			log.Error("OIDC init failed; continuing without OIDC login", "err", err)
			oidcProvider = nil
		}
	}
	authSvc := auth.NewService(store, log, auth.Options{
		SessionSecret:      []byte(cfg.Auth.SessionSecret),
		SessionTTL:         cfg.Auth.SessionTTL,
		AnonymousRead:      cfg.Auth.AnonymousRead,
		OIDC:               oidcProvider,
		DefaultRole:        cfg.Auth.RBAC.DefaultRole,
		BootstrapAdminUser: cfg.Auth.BootstrapAdminUser,
	})
	if err := authSvc.BootstrapAdmin(ctx, cfg.Auth.BootstrapAdminUser, cfg.Auth.BootstrapAdminPassword); err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}
	// Declarative RBAC: reconcile chart-provided roles, grants, group mappings
	// and local accounts. No-op when no policy file is configured.
	if err := auth.ReconcileRBAC(ctx, store, log, cfg.Auth.RBAC.PolicyFile, cfg.Auth.RBAC.AccountsDir); err != nil {
		return fmt.Errorf("reconcile rbac: %w", err)
	}

	if cfg.SeedDefaultRepos {
		if err := repo.SeedDefaults(ctx, store, log); err != nil {
			return fmt.Errorf("seed default repositories: %w", err)
		}
	}

	// Audit recorder: nil (no-op) when disabled. Closed on shutdown so buffered
	// events flush before the store closes.
	var recorder *audit.Recorder
	if cfg.Audit.Enabled {
		recorder = audit.NewRecorder(store, log, reg)
		defer recorder.Close()
	}

	engine := repo.NewEngine(store, blobs, log, reg)
	manager := repo.NewManager(engine, store, authSvc, recorder, reg)
	manager.SetExternalURL(cfg.ExternalURL)
	if cfg.Vuln.OSVURL != "" {
		manager.SetVulnScanner(vuln.NewOSV(cfg.Vuln.OSVURL, nil))
		log.Info("vulnerability scanning enabled", "osv_url", cfg.Vuln.OSVURL)
	}
	if cfg.License.DepsDevURL != "" {
		manager.SetLicenseResolver(license.NewDepsDev(cfg.License.DepsDevURL, nil))
		log.Info("license resolution enabled", "deps_dev_url", cfg.License.DepsDevURL)
	}
	var artifactScanSvc *artifactscan.Service
	if cfg.ArtifactScan.Enabled {
		tokenKey := []byte(cfg.ArtifactScan.TokenKey)
		if len(tokenKey) == 0 && cfg.Auth.SessionSecret != "" {
			tokenKey = []byte(cfg.Auth.SessionSecret)
		}
		artifactScanSvc, err = artifactscan.NewService(store, artifactscan.ServiceConfig{
			DefaultProfile: cfg.ArtifactScan.DefaultProfile,
			LeaseTTL:       cfg.ArtifactScan.LeaseTTL,
			TokenTTL:       cfg.ArtifactScan.TokenTTL,
			TokenKey:       tokenKey,
			Profiles: []artifactscan.Profile{{
				Name:       cfg.ArtifactScan.DefaultProfile,
				Scanner:    "grype",
				Mode:       artifactscan.ModeDeployment,
				ConfigHash: cfg.ArtifactScan.DefaultProfile + "-v1",
				Limits: artifactscan.Limits{
					MaxArtifactBytes: cfg.ArtifactScan.MaxArtifactBytes,
				},
				StoreSBOM: cfg.ArtifactScan.StoreSBOM,
			}},
			MaxAttempts: cfg.ArtifactScan.MaxAttempts,
		})
		if err != nil {
			return fmt.Errorf("init artifact scanner service: %w", err)
		}
		if err := artifactScanSvc.InitProfiles(ctx); err != nil {
			return fmt.Errorf("init artifact scanner profiles: %w", err)
		}
		manager.SetArtifactScanEnqueuer(func(blobSHA256, scannerProfile string) {
			if _, err := artifactScanSvc.Enqueue(context.Background(), blobSHA256, scannerProfile); err != nil {
				log.Warn("artifact scan enqueue failed", "blob", blobSHA256, "profile", scannerProfile, "err", err)
			}
		})
		log.Info("artifact scanning enabled", "profile", cfg.ArtifactScan.DefaultProfile)
	}

	// Outbound approval alarms: when a package is quarantined pending approval,
	// notify the receivers the repository selected (resolved against the enabled
	// receivers managed in the admin console). Runs off the serving path.
	notifier := notify.New(cfg.Notify.WebhookTimeout, log)
	manager.SetApprovalNotifier(func(repoName, pkg, version, requestedBy string, receivers []string) {
		if len(receivers) == 0 {
			return
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			all, err := store.ListEnabledReceivers(ctx)
			if err != nil {
				log.Warn("notify: list receivers failed", "err", err)
				return
			}
			selected := make(map[string]bool, len(receivers))
			for _, n := range receivers {
				selected[n] = true
			}
			var targets []notify.Target
			for _, rec := range all {
				if selected[rec.Name] {
					targets = append(targets, notify.Target{Name: rec.Name, URL: rec.WebhookURL})
				}
			}
			notifier.NotifyApprovalRequest(targets, repoName, pkg, version, requestedBy)
		}()
	})

	// Pending approvals, computed on scrape (one indexed COUNT). Needs no leader
	// gating and stays accurate on standbys after a snapshot swap.
	reg.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Namespace: "forklift", Name: "approval_pending",
		Help: "Package approval requests currently pending.",
	}, func() float64 {
		gctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		n, err := store.CountApprovals(gctx, "", meta.ApprovalPending)
		if err != nil {
			return 0
		}
		return float64(n)
	}))

	srv := server.New(cfg, log, store, reg)
	apiHandler := api.New(store, authSvc, log, recorder)
	// Back the receiver test and repository sample/preview endpoints.
	apiHandler.SetNotifier(notifier)

	// Public OIDC login endpoints (no auth middleware required).
	if oidcProvider != nil {
		srv.Router().Get("/auth/login", authSvc.HandleLogin)
		srv.Router().Get("/auth/callback", authSvc.HandleCallback)
	}

	// OpenAPI spec and Scalar docs UI (public).
	openapi.Register(srv.Router())

	// Application routes carry the auth middleware so handlers see the principal.
	srv.Router().Group(func(r chi.Router) {
		r.Use(authSvc.Middleware)
		r.Mount("/api/v1", apiHandler.Routes())
		manager.Register(r)
	})

	// The embedded React SPA serves the UI and handles client-side routing for
	// any path not matched above.
	srv.Router().NotFound(webui.Handler())

	// leaderGauge reports whether this instance currently holds leadership.
	// Single-instance deployments are always leader; in HA exactly one pod is 1.
	leaderGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "forklift", Name: "leader",
		Help: "1 if this instance currently holds leadership, else 0.",
	})
	reg.MustRegister(leaderGauge)

	// leaderState mirrors leaderGauge for the admin HA status API (gauges are
	// write-only from here). Single-instance deployments set it true on start.
	var leaderState atomic.Bool

	var elector *cluster.Elector
	if cfg.HA.Enabled {
		elector, err = cluster.New(cfg.HA, log)
		if err != nil {
			return fmt.Errorf("init leader election: %w", err)
		}
	}

	// Admin-only HA status for the management console: backend/mode, this pod's
	// identity and role, the current Lease holder, the s3 fencing token, and the
	// process start time (for uptime display).
	startedAt := time.Now()
	apiHandler.SetHAStatus(func(sctx context.Context) api.HAStatus {
		st := api.HAStatus{
			Enabled:   cfg.HA.Enabled,
			Mode:      haMode(cfg),
			Backend:   cfg.Storage.Backend,
			Identity:  cfg.HA.Identity,
			LeaseName: cfg.HA.LeaseName,
			IsLeader:  leaderState.Load(),
			StartedAt: startedAt.Format(time.RFC3339),
			Version:   version.Version,
		}
		// Where artifacts live: the object-storage bucket/endpoint (s3) or the
		// block-storage data directory (fs).
		if cfg.Storage.Backend == "s3" {
			bucket := cfg.Storage.S3.Bucket
			if cfg.Storage.S3.Prefix != "" {
				bucket += "/" + strings.TrimLeft(cfg.Storage.S3.Prefix, "/")
			}
			if ep := strings.TrimRight(cfg.Storage.S3.Endpoint, "/"); ep != "" {
				st.StorageEndpoint = ep + "/" + bucket
			} else {
				st.StorageEndpoint = "s3://" + bucket
			}
		} else {
			st.StorageEndpoint = cfg.DataDir
		}
		if elector == nil {
			// Single instance is always the leader and serves itself.
			st.IsLeader = true
			st.Leader = cfg.HA.Identity
			st.Role = cluster.RoleLeader
			return st
		}
		if st.IsLeader {
			st.Role = cluster.RoleLeader
		} else {
			st.Role = cluster.RoleStandby
		}
		if leader, err := elector.LeaderIdentity(sctx); err == nil {
			st.Leader = leader
		}
		if cfg.Storage.Backend == "s3" {
			if t, err := elector.FencingToken(sctx); err == nil {
				st.FencingToken = t
			}
		}
		return st
	})

	// Manual failover for the management console: ask this instance to release
	// leadership so a standby takes over. Only wired in HA mode; single-instance
	// has no peer to fail over to.
	if elector != nil {
		apiHandler.SetHAStepDown(elector.StepDown)
	}

	// PV-based replication: the leader serves token-gated snapshot/blob
	// endpoints; the standby pulls them onto its own volume and promotes that
	// copy when it wins the election. The mount sits outside the auth middleware
	// group because it carries its own bearer-token check.
	var replicator *replication.Replicator
	if cfg.Replication.Enabled {
		source := replication.NewSource(store, blobs, cfg.Replication.Token, cfg.DataDir, log)
		srv.Router().Mount("/internal/replication", source.Routes())

		resolver := replication.StaticLeaderURL(cfg.Replication.LeaderURL)
		if cfg.Replication.LeaderURL == "" {
			resolver = replication.LeaseLeaderURL(elector, cfg.HA.Identity,
				cfg.Replication.PeerService, cfg.Replication.PeerPort)
		}
		replicator = replication.New(replication.Options{
			Store:      store,
			Blobs:      blobs,
			DataDir:    cfg.DataDir,
			Token:      cfg.Replication.Token,
			Interval:   cfg.Replication.Interval,
			LeaderURL:  resolver,
			Log:        log,
			Registerer: reg,
		})
		go replicator.Run(ctx)

		// With per-pod volumes a StatefulSet rollout waits on pod readiness, so
		// readiness cannot encode leadership (the standby would block rollouts
		// forever). Every pod is Ready; the main Service instead selects the
		// forklift.io/role=leader pod label patched on (de)promotion.
		srv.SetReady(true)
	}
	if artifactScanSvc != nil {
		scans := api.NewScanInternal(artifactScanSvc, blobs, cfg.ArtifactScan.WorkerToken, log)
		srv.Router().Mount("/internal/scans", scans.Routes())
	}

	// labelRouting keeps every replica Ready and routes the Service to the leader
	// via the forklift.io/role label, instead of gating readiness on leadership.
	// Used by replication (per-pod volumes can't gate readiness on leadership) and
	// by the s3 backend in HA, where both pods must stay Ready while a single
	// writer is enforced by leader routing plus S3 fencing.
	labelRouting := cfg.Replication.Enabled || (metaSync != nil && cfg.HA.Enabled)
	if metaSync != nil && cfg.HA.Enabled {
		// s3 HA: become Ready immediately; the leader label routes traffic.
		srv.SetReady(true)
	}
	setPodRole := func(roleCtx context.Context, role string) {
		if !labelRouting || elector == nil || cfg.Replication.PodName == "" {
			return
		}
		if err := elector.SetPodRole(roleCtx, cfg.Replication.PodNamespace, cfg.Replication.PodName, role); err != nil {
			log.Error("set pod role label", "role", role, "err", err)
		}
	}

	// The blob sweeper and audit retention are gated on leadership. In
	// single-instance mode this process is always the leader; in HA mode a
	// Kubernetes Lease elects exactly one active instance so SQLite has a
	// single writer. With replication enabled, the replicated snapshot is
	// applied before this instance takes traffic.
	startLeading := func(leadCtx context.Context) {
		leaderGauge.Set(1)
		leaderState.Store(true)
		if replicator != nil {
			if err := replicator.Promote(leadCtx); err != nil {
				log.Error("replication: promote failed; serving local data", "err", err)
			}
		}
		// In s3 mode, apply the latest metadata snapshot before serving so the
		// new leader takes traffic on current data. The fencing token (Lease
		// transition count) tags snapshot uploads so a superseded leader cannot
		// overwrite this term's metadata.
		if metaSync != nil {
			fence := int64(0)
			if elector != nil {
				if t, err := elector.FencingToken(leadCtx); err != nil {
					log.Warn("objstore: read fencing token failed; using 0", "err", err)
				} else {
					fence = t
				}
			}
			if err := metaSync.Promote(leadCtx, fence); err != nil {
				log.Error("objstore: promote failed; serving local data", "err", err)
			}
		}
		srv.SetReady(true)
		setPodRole(leadCtx, cluster.RoleLeader)
		// A partitioned former leader may not have removed its own leader
		// label; strip it so the Service routes to this pod only.
		if labelRouting && cfg.Replication.PodName != "" {
			if err := elector.DemotePeers(leadCtx, cfg.Replication.PodNamespace, cfg.Replication.PodName); err != nil {
				log.Error("demote peer leader labels", "err", err)
			}
		}
		go engine.RunSweeper(leadCtx, 5*time.Minute)
		go manager.RunIdleReaper(leadCtx, time.Hour)
		// Vulnerability scan worker pool + backfill (scans already-stored
		// artifacts) + periodic re-scanner (no-ops without a scanner). Multiple
		// workers drain the queue concurrently so freshly cached coordinates are
		// scanned promptly under burst.
		for i := 0; i < max(1, cfg.Vuln.Workers); i++ {
			go manager.RunVulnWorker(leadCtx)
		}
		go manager.RunVulnBackfill(leadCtx, cfg.Vuln.RescanInterval)
		go manager.RunVulnRescanner(leadCtx, cfg.Vuln.RescanInterval, cfg.Vuln.TTL)
		// License resolution worker pool + backfill + periodic re-resolver (no-ops
		// without a resolver).
		for i := 0; i < max(1, cfg.License.Workers); i++ {
			go manager.RunLicenseWorker(leadCtx)
		}
		go manager.RunLicenseBackfill(leadCtx, cfg.License.RescanInterval)
		go manager.RunLicenseRescanner(leadCtx, cfg.License.RescanInterval, cfg.License.TTL)
		if recorder != nil && cfg.Audit.Retention > 0 {
			go recorder.RunRetention(leadCtx, time.Hour, cfg.Audit.Retention)
		}
	}
	stopLeading := func() {
		leaderGauge.Set(0)
		leaderState.Store(false)
		if metaSync != nil {
			// Resume downloading the leader's snapshots on the next sync cycle.
			metaSync.Demote()
		}
		if replicator != nil {
			replicator.Demote()
		}
		if labelRouting {
			// Stay Ready so rollouts proceed; moving the forklift.io/role=leader
			// label is what redirects traffic to the new leader.
			demoteCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			setPodRole(demoteCtx, cluster.RoleStandby)
			return
		}
		// Shared-volume HA without label routing: readiness gates the leader.
		srv.SetReady(false)
	}

	if cfg.HA.Enabled {
		go elector.Run(ctx, startLeading, stopLeading)
	} else {
		startLeading(ctx)
	}

	return srv.Run(ctx, reg)
}

// haMode names the active high-availability/storage topology for the admin
// status view.
func haMode(cfg *config.Config) string {
	switch {
	case cfg.Replication.Enabled:
		return "replication"
	case cfg.Storage.Backend == "s3":
		return "object-storage"
	case cfg.HA.Enabled:
		return "shared-volume"
	default:
		return "single"
	}
}

func toS3Config(c config.S3Config) storage.S3Config {
	return storage.S3Config{
		Bucket:          c.Bucket,
		Prefix:          c.Prefix,
		Region:          c.Region,
		Endpoint:        c.Endpoint,
		ForcePathStyle:  c.ForcePathStyle,
		AccessKeyID:     c.AccessKeyID,
		SecretAccessKey: c.SecretAccessKey,
	}
}

func newLogger(cfg *config.Config) *slog.Logger {
	var level slog.Level
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	var h slog.Handler
	if cfg.LogFormat == "text" {
		h = slog.NewTextHandler(os.Stdout, opts)
	} else {
		h = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(h)
}
