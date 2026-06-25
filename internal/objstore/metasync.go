// Package objstore syncs the SQLite metadata database to S3 for the
// object-storage HA mode. SQLite cannot run live on S3 (it needs POSIX file
// locking), so the live database stays on a local volume (typically an
// emptyDir) and this package keeps a durable copy in S3: the leader periodically
// uploads a VACUUM INTO snapshot, and every pod restores the latest snapshot on
// boot and applies it on promotion.
//
// It reuses meta.Store.Snapshot / SwapFromSnapshot and mirrors the
// leader/standby control flow of the PV-based replicator. The tradeoff is the
// same: writes within one sync interval can be lost on failover (asynchronous
// replication).
package objstore

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/younsl/o/box/kubernetes/forklift/internal/meta"
	"github.com/younsl/o/box/kubernetes/forklift/internal/storage"
)

// MetaOptions configures a MetaSync.
type MetaOptions struct {
	Store      *meta.Store
	API        storage.S3API
	Bucket     string
	Key        string // object key for the snapshot, e.g. "<prefix>/meta/forklift.db"
	DataDir    string
	Interval   time.Duration
	Log        *slog.Logger
	Registerer prometheus.Registerer
}

// MetaSync uploads the leader's database snapshot to S3 and restores it on
// standbys. Exactly one instance is leader at a time (guaranteed by leader
// election), so there is a single writer to the S3 snapshot object.
type MetaSync struct {
	store    *meta.Store
	api      storage.S3API
	bucket   string
	key      string
	dataDir  string
	interval time.Duration
	log      *slog.Logger

	isLeader atomic.Bool
	// fence is the current leadership term (Lease transition count). Snapshot
	// uploads carry it; a stale leader with a lower fence is refused, preventing
	// split-brain overwrites of a newer leader's metadata.
	fence atomic.Int64

	// syncMu serializes sync cycles with promotion so Promote never races a
	// half-written snapshot download.
	syncMu sync.Mutex
	// snapshotPath is the last snapshot downloaded during this process's standby
	// phase; "" when none. Promote applies it and clears it.
	snapshotPath string

	uploads       *prometheus.CounterVec
	downloads     *prometheus.CounterVec
	lastSyncUnix  prometheus.Gauge
	snapshotBytes prometheus.Gauge
}

// NewMetaSync builds a MetaSync and registers its metrics.
func NewMetaSync(o MetaOptions) *MetaSync {
	m := &MetaSync{
		store:    o.Store,
		api:      o.API,
		bucket:   o.Bucket,
		key:      o.Key,
		dataDir:  o.DataDir,
		interval: o.Interval,
		log:      o.Log,
		uploads: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "forklift",
			Name:      "objstore_meta_uploads_total",
			Help:      "Metadata snapshot uploads to S3 by result.",
		}, []string{"result"}),
		downloads: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "forklift",
			Name:      "objstore_meta_downloads_total",
			Help:      "Metadata snapshot downloads from S3 by result.",
		}, []string{"result"}),
		lastSyncUnix: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "forklift",
			Name:      "objstore_meta_last_sync_timestamp_seconds",
			Help:      "Unix time of the last successful metadata sync cycle.",
		}),
		snapshotBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "forklift",
			Name:      "objstore_meta_snapshot_bytes",
			Help:      "Size of the last metadata snapshot transferred.",
		}),
	}
	if o.Registerer != nil {
		o.Registerer.MustRegister(m.uploads, m.downloads, m.lastSyncUnix, m.snapshotBytes)
	}
	return m
}

func (m *MetaSync) workDir() string { return filepath.Join(m.dataDir, "objstore") }

// RestoreOnBoot downloads the latest snapshot from S3 and swaps it into the
// local database before the process serves traffic. It is required because the
// object-storage mode runs on an ephemeral volume that loses the database on
// restart. An empty bucket (no snapshot yet) is a no-op: the process starts with
// its local (fresh) database and the leader will upload it.
func (m *MetaSync) RestoreOnBoot(ctx context.Context) error {
	dst := filepath.Join(m.workDir(), "restore.db")
	found, _, err := m.download(ctx, dst)
	if err != nil {
		return err
	}
	if !found {
		m.log.Info("objstore: no metadata snapshot in bucket; starting with local database")
		return nil
	}
	if err := m.store.SwapFromSnapshot(ctx, dst); err != nil {
		return fmt.Errorf("apply boot snapshot: %w", err)
	}
	m.log.Info("objstore: restored metadata from S3 snapshot")
	return nil
}

// Run executes the sync loop until ctx is cancelled.
func (m *MetaSync) Run(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.sync(ctx); err != nil && ctx.Err() == nil {
				m.log.Error("objstore: meta sync failed", "err", err)
			}
		}
	}
}

// sync runs one cycle: the leader uploads a fresh snapshot, a standby downloads
// the latest one and records it for promotion.
func (m *MetaSync) sync(ctx context.Context) error {
	m.syncMu.Lock()
	defer m.syncMu.Unlock()

	if m.isLeader.Load() {
		if err := m.upload(ctx); err != nil {
			m.uploads.WithLabelValues("error").Inc()
			return fmt.Errorf("upload snapshot: %w", err)
		}
		m.uploads.WithLabelValues("ok").Inc()
		m.lastSyncUnix.SetToCurrentTime()
		return nil
	}

	dst := filepath.Join(m.workDir(), "forklift.db")
	found, size, err := m.download(ctx, dst)
	if err != nil {
		m.downloads.WithLabelValues("error").Inc()
		return fmt.Errorf("download snapshot: %w", err)
	}
	if !found {
		return nil
	}
	m.snapshotPath = dst
	m.snapshotBytes.Set(float64(size))
	m.downloads.WithLabelValues("ok").Inc()
	m.lastSyncUnix.SetToCurrentTime()
	return nil
}

// upload writes a VACUUM INTO snapshot and puts it at the snapshot key.
func (m *MetaSync) upload(ctx context.Context) error {
	if err := os.MkdirAll(m.workDir(), 0o755); err != nil {
		return fmt.Errorf("create objstore dir: %w", err)
	}
	snap := filepath.Join(m.workDir(), "upload.db")
	if err := m.store.Snapshot(ctx, snap); err != nil {
		return err
	}
	defer os.Remove(snap)

	f, err := os.Open(snap)
	if err != nil {
		return fmt.Errorf("open snapshot: %w", err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}

	put := &s3.PutObjectInput{
		Bucket:        aws.String(m.bucket),
		Key:           aws.String(m.key),
		Body:          f,
		ContentLength: aws.Int64(fi.Size()),
		Metadata:      map[string]string{fenceMetaKey: strconv.FormatInt(m.fence.Load(), 10)},
	}
	// Fencing: never overwrite a snapshot written by a newer leadership term.
	// HEAD the current object; a higher stored fence means this process is a
	// stale ("zombie") leader and must not clobber the newer state. The
	// conditional write (If-Match on the observed ETag, or If-None-Match for a
	// first write) closes the HEAD->PUT race: a concurrent writer makes one side
	// lose with 412, which we treat as "skip this cycle".
	head, err := m.api.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(m.bucket),
		Key:    aws.String(m.key),
	})
	switch {
	case err == nil:
		if stored := parseFence(head.Metadata); stored > m.fence.Load() {
			return fmt.Errorf("fencing: local term %d older than stored %d; refusing to overwrite snapshot", m.fence.Load(), stored)
		}
		put.IfMatch = head.ETag
	case storage.IsNotFound(err):
		put.IfNoneMatch = aws.String("*")
	default:
		return fmt.Errorf("head snapshot: %w", err)
	}
	if _, err := m.api.PutObject(ctx, put); err != nil {
		if storage.IsPreconditionFailed(err) {
			return fmt.Errorf("fencing: snapshot changed under us (concurrent writer); skipping cycle: %w", err)
		}
		return err
	}
	m.snapshotBytes.Set(float64(fi.Size()))
	return nil
}

// fenceMetaKey is the S3 user-metadata key holding the leadership term that
// wrote the snapshot.
const fenceMetaKey = "fence"

// parseFence reads the fencing token from S3 object user-metadata. Keys are
// case-insensitive; a missing or malformed value reads as 0.
func parseFence(md map[string]string) int64 {
	for k, v := range md {
		if strings.EqualFold(k, fenceMetaKey) {
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				return n
			}
		}
	}
	return 0
}

// download fetches the snapshot to dst via a temp file and atomic rename.
// It returns found=false (no error) when the object does not exist yet.
func (m *MetaSync) download(ctx context.Context, dst string) (bool, int64, error) {
	out, err := m.api.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(m.bucket),
		Key:    aws.String(m.key),
	})
	if err != nil {
		if storage.IsNotFound(err) {
			return false, 0, nil
		}
		return false, 0, err
	}
	defer out.Body.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return false, 0, fmt.Errorf("create objstore dir: %w", err)
	}
	tmp := dst + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return false, 0, fmt.Errorf("create temp snapshot: %w", err)
	}
	n, err := io.Copy(f, out.Body)
	if err != nil {
		f.Close()
		os.Remove(tmp)
		return false, 0, fmt.Errorf("download snapshot: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return false, 0, fmt.Errorf("sync snapshot: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return false, 0, fmt.Errorf("close snapshot: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return false, 0, fmt.Errorf("commit snapshot: %w", err)
	}
	return true, n, nil
}

// Promote is called when this instance acquires leadership, before it reports
// Ready. If a newer snapshot was downloaded during the standby phase it replaces
// the local database; otherwise the local data (e.g. from RestoreOnBoot or a
// re-elected former leader) is served as-is.
func (m *MetaSync) Promote(ctx context.Context, fence int64) error {
	m.isLeader.Store(true)
	m.fence.Store(fence)
	m.syncMu.Lock()
	defer m.syncMu.Unlock()
	path := m.snapshotPath
	m.snapshotPath = ""
	if path == "" {
		m.log.Info("objstore: promoting with local data (no newer snapshot)")
		return nil
	}
	if err := m.store.SwapFromSnapshot(ctx, path); err != nil {
		return fmt.Errorf("apply snapshot on promote: %w", err)
	}
	m.log.Info("objstore: promoted with S3 snapshot")
	return nil
}

// Demote is called when leadership is lost; the download loop resumes.
func (m *MetaSync) Demote() {
	m.isLeader.Store(false)
}
