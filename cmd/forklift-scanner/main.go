// Command forklift-scanner runs isolated artifact scan jobs for forklift.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/younsl/o/box/kubernetes/forklift/internal/artifactscan"
	"github.com/younsl/o/box/kubernetes/forklift/internal/scannerworker"
	grypedriver "github.com/younsl/o/box/kubernetes/forklift/internal/scannerworker/drivers/grype"
)

func main() {
	var serverURL string
	var workerToken string
	var workerID string
	var workRoot string
	var once bool
	var maxArtifactBytes int64
	flag.StringVar(&serverURL, "server", env("FORKLIFT_SERVER_URL", "http://127.0.0.1:8080"), "forklift server base URL")
	flag.StringVar(&workerToken, "worker-token", env("FORKLIFT_ARTIFACT_SCAN_WORKER_TOKEN", ""), "worker bearer token for claiming scan jobs")
	flag.StringVar(&workerID, "worker-id", env("FORKLIFT_SCANNER_WORKER_ID", hostname()), "scanner worker id")
	flag.StringVar(&workRoot, "work-dir", env("FORKLIFT_SCANNER_WORK_DIR", os.TempDir()), "base directory for per-job workspaces")
	flag.BoolVar(&once, "once", envBool("FORKLIFT_SCANNER_ONCE", false), "process at most one job and exit")
	flag.Int64Var(&maxArtifactBytes, "max-artifact-bytes", envInt64("FORKLIFT_SCANNER_MAX_ARTIFACT_BYTES", 100<<20), "maximum artifact bytes to download")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if workerToken == "" {
		log.Error("worker token is required")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	client := scannerworker.Client{
		BaseURL:     serverURL,
		WorkerID:    workerID,
		WorkerToken: workerToken,
		HTTP:        &http.Client{Timeout: 10 * time.Minute},
	}
	registry, err := scannerworker.NewRegistry(grypedriver.Driver{})
	if err != nil {
		log.Error("build scanner registry", "err", err)
		os.Exit(1)
	}
	for {
		if err := runOnce(ctx, log, client, registry, workRoot, maxArtifactBytes); err != nil {
			if errors.Is(err, scannerworker.ErrNoJob) {
				if once {
					return
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
					continue
				}
			}
			log.Error("scan job failed", "err", err)
			if once {
				os.Exit(1)
			}
		}
		if once {
			return
		}
	}
}

func runOnce(ctx context.Context, log *slog.Logger, client scannerworker.Client, registry *scannerworker.Registry, workRoot string, maxArtifactBytes int64) error {
	capabilities, err := registry.Capabilities(ctx)
	if err != nil {
		return err
	}
	job, err := client.Claim(ctx, capabilities)
	if err != nil {
		return err
	}
	var jobMu sync.Mutex
	runCtx, stopHeartbeat := context.WithCancel(ctx)
	defer stopHeartbeat()
	go heartbeatLoop(runCtx, log, client, &job, &jobMu)

	driver, ok := registry.Get(job.Scanner)
	if !ok {
		return fmt.Errorf("no driver registered for scanner %q", job.Scanner)
	}
	rc, err := client.OpenBlob(ctx, job)
	if err != nil {
		return err
	}
	defer rc.Close()

	root := filepath.Join(workRoot, "forklift-scan-"+job.JobID)
	limit := maxArtifactBytes
	if job.Limits.MaxArtifactBytes > 0 {
		limit = job.Limits.MaxArtifactBytes
	}
	prepared, err := scannerworker.PrepareArtifact(runCtx, root, rc, job.BlobSHA256, scannerworker.WorkspaceLimits{MaxArtifactBytes: limit})
	if err != nil {
		var tooLarge scannerworker.ErrArtifactTooLarge
		if errors.As(err, &tooLarge) {
			result := artifactscan.Result{
				JobID:      job.JobID,
				BlobSHA256: job.BlobSHA256,
				Scanner:    job.Scanner,
				Status:     artifactscan.StatusSkippedTooLarge,
				Error:      tooLarge.Error(),
				ScannedAt:  time.Now().UTC(),
			}
			log.Info("submitting scan result", "job", job.JobID, "scanner", job.Scanner, "status", result.Status)
			jobMu.Lock()
			submitJob := job
			jobMu.Unlock()
			return client.SubmitResult(ctx, submitJob, result)
		}
		return err
	}
	prepared.Targets = job.Targets
	defer prepared.Cleanup()

	result, err := driver.Scan(runCtx, prepared)
	if err != nil && result.Status == "" {
		return err
	}
	result.JobID = job.JobID
	result.BlobSHA256 = job.BlobSHA256
	result.Scanner = job.Scanner
	if result.Status == "" {
		result.Status = artifactscan.StatusCompleted
	}
	if result.ScannedAt.IsZero() {
		result.ScannedAt = time.Now().UTC()
	}
	if job.StoreSBOM {
		generator, ok := driver.(scannerworker.SBOMGenerator)
		if !ok {
			return fmt.Errorf("scanner %q does not support sbom generation", job.Scanner)
		}
		sbom, err := generator.GenerateSBOM(runCtx, prepared)
		if err != nil {
			return err
		}
		result.SBOM = &sbom
	}
	log.Info("submitting scan result", "job", job.JobID, "scanner", job.Scanner, "status", result.Status)
	jobMu.Lock()
	submitJob := job
	jobMu.Unlock()
	return client.SubmitResult(ctx, submitJob, result)
}

func heartbeatLoop(ctx context.Context, log *slog.Logger, client scannerworker.Client, job *scannerworker.ClaimedJob, mu *sync.Mutex) {
	mu.Lock()
	interval := heartbeatInterval(job.Deadline, time.Now().UTC())
	current := *job
	mu.Unlock()
	timer := time.NewTimer(interval)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			renewed, err := client.Heartbeat(ctx, current)
			if err != nil {
				log.Warn("scan heartbeat failed", "job", current.JobID, "err", err)
				return
			}
			mu.Lock()
			job.Token = renewed.Token
			job.Deadline = renewed.Deadline
			job.Limits = renewed.Limits
			current = *job
			mu.Unlock()
			timer.Reset(interval)
		}
	}
}

func heartbeatInterval(deadline, now time.Time) time.Duration {
	if deadline.IsZero() || !deadline.After(now) {
		return 30 * time.Second
	}
	d := deadline.Sub(now) / 3
	if d < 5*time.Second {
		return 5 * time.Second
	}
	if d > 30*time.Second {
		return 30 * time.Second
	}
	return d
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	switch os.Getenv(key) {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	default:
		return def
	}
}

func envInt64(key string, def int64) int64 {
	var v int64
	if _, err := fmt.Sscanf(os.Getenv(key), "%d", &v); err == nil && v > 0 {
		return v
	}
	return def
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "forklift-scanner"
	}
	return h
}
