package repo

import (
	"context"
	"time"

	"github.com/younsl/o/box/kubernetes/forklift/internal/meta"
	"github.com/younsl/o/box/kubernetes/forklift/internal/packagecoord"
)

// LicenseCoordinate returns the deps.dev system and package name for an artifact
// path of the given format, for joining stored license results to listed
// artifacts. The package name matches the OSV coordinate (Maven uses
// "group:artifact"); only the system label differs. Returns empty strings when
// the format has no resolvable coordinate.
func LicenseCoordinate(format, artifactPath string) (system, pkg string) {
	c := packagecoord.FromArtifact(format, artifactPath, "")
	if c.DepsDevSystem == "" || c.PackageName == "" {
		return "", ""
	}
	return c.DepsDevSystem, c.PackageName
}

// DepsDevSystem maps a forklift repository format to its deps.dev system name,
// exported so the API can join stored license results to artifacts. Returns ""
// for formats deps.dev does not cover.
func DepsDevSystem(format string) string { return depsDevSystem(format) }

// depsDevSystem maps a forklift repository format to its deps.dev system name.
// Returns "" for unsupported formats (the gate then no-ops).
func depsDevSystem(format string) string {
	return packagecoord.DepsDevSystem(format)
}

// resolveStored enqueues an immediate license resolution for a freshly stored
// artifact, so a hosted upload is resolved right away instead of waiting for
// the periodic backfill. It is a no-op without a resolver, for unsupported
// formats, or for paths that carry no version, and deduplicates like any
// enqueue.
func (m *Manager) resolveStored(repo meta.Repository, artifactPath string) {
	if m.resolver == nil {
		return
	}
	system, pkg := LicenseCoordinate(repo.Format, artifactPath)
	if pkg == "" {
		return
	}
	version := versionForPath(repo.Format, artifactPath)
	if version == "" {
		return
	}
	m.enqueueResolve(system, pkg, version)
}

// resolveJob is a queued license resolution for one package coordinate.
type resolveJob struct {
	system  string
	pkg     string
	version string
}

// enqueueResolve schedules an async license resolution for a coordinate,
// deduplicated within the pending-mark TTL so hot paths do not flood the queue.
// Drops silently when the queue is full (the next request after the mark expires
// re-enqueues).
func (m *Manager) enqueueResolve(system, pkg, version string) {
	if m.resolver == nil || system == "" || pkg == "" || version == "" {
		return
	}
	key := "license\x00" + system + "\x00" + pkg + "\x00" + version
	if m.reqMarks.has(key) {
		return
	}
	m.reqMarks.set(key, pendingMarkTTL)
	select {
	case m.resolveQueue <- resolveJob{system: system, pkg: pkg, version: version}:
	default:
	}
}

// RunLicenseWorker drains the resolve queue, querying the license source and
// storing results. It runs until ctx is cancelled. A no-op without a resolver.
func (m *Manager) RunLicenseWorker(ctx context.Context) {
	if m.resolver == nil {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-m.resolveQueue:
			m.runResolve(ctx, job)
		}
	}
}

func (m *Manager) runResolve(ctx context.Context, job resolveJob) {
	res, err := m.resolver.Resolve(ctx, job.system, job.pkg, job.version)
	if err != nil {
		m.licenseResolves.WithLabelValues("error").Inc()
		m.engine.log.Warn("license resolve failed",
			"system", job.system, "package", job.pkg, "version", job.version, "err", err)
		return
	}
	if len(res.Licenses) == 0 {
		m.licenseResolves.WithLabelValues("unknown").Inc()
	} else {
		m.licenseResolves.WithLabelValues("resolved").Inc()
	}
	if err := m.store.UpsertLicenseScan(ctx, job.system, job.pkg, job.version, res.Licenses, m.resolver.Source()); err != nil {
		m.engine.log.Error("store license result failed",
			"system", job.system, "package", job.pkg, "version", job.version, "err", err)
	}
}

// RunLicenseBackfill resolves already-stored artifacts that have never been
// resolved, so license data exists for packages uploaded (hosted) or cached
// (proxy) before resolution ever covered them. It sweeps once immediately and
// then every interval. Leader-gated by the caller. A no-op without a resolver.
func (m *Manager) RunLicenseBackfill(ctx context.Context, interval time.Duration) {
	if m.resolver == nil {
		return
	}
	m.licenseBackfillOnce(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.licenseBackfillOnce(ctx)
		}
	}
}

// licenseBackfillOnce enqueues a resolution for every stored artifact coordinate
// that has no result yet. Already-resolved coordinates are skipped (the
// re-resolver refreshes those); unsupported formats are ignored.
func (m *Manager) licenseBackfillOnce(ctx context.Context) {
	resolved, err := m.store.ResolvedLicenseKeys(ctx)
	if err != nil {
		m.engine.log.Error("license backfill: load resolved keys failed", "err", err)
		return
	}
	enqueued := 0
	for offset := 0; ; offset += reapBatch {
		targets, err := m.store.ListScanTargets(ctx, reapBatch, offset)
		if err != nil {
			m.engine.log.Error("license backfill: list targets failed", "err", err)
			return
		}
		for _, t := range targets {
			system, pkg := LicenseCoordinate(t.Format, t.Path)
			if pkg == "" {
				continue
			}
			if _, ok := resolved[system+"\x00"+pkg+"\x00"+t.Version]; ok {
				continue
			}
			m.enqueueResolve(system, pkg, t.Version)
			enqueued++
		}
		if len(targets) < reapBatch {
			break
		}
	}
	if enqueued > 0 {
		m.engine.log.Info("license backfill enqueued resolutions for stored artifacts", "count", enqueued)
	}
}

// RunLicenseRescanner periodically re-enqueues resolutions older than ttl so
// license metadata changes on already-cached versions surface. Leader-gated by
// the caller. A no-op without a resolver.
func (m *Manager) RunLicenseRescanner(ctx context.Context, interval, ttl time.Duration) {
	if m.resolver == nil {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cutoff := m.engine.now().Add(-ttl)
			stale, err := m.store.ListStaleLicenseScans(ctx, cutoff, reapBatch)
			if err != nil {
				m.engine.log.Error("license rescan list failed", "err", err)
				continue
			}
			for _, s := range stale {
				m.enqueueResolve(s.System, s.Package, s.Version)
			}
		}
	}
}
