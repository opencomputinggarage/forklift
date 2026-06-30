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
	job, err := client.Claim(ctx)
	if err != nil {
		return err
	}
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
	prepared, err := scannerworker.PrepareArtifact(ctx, root, rc, job.BlobSHA256, scannerworker.WorkspaceLimits{MaxArtifactBytes: maxArtifactBytes})
	if err != nil {
		return err
	}
	defer prepared.Cleanup()

	result, err := driver.Scan(ctx, prepared)
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
	log.Info("submitting scan result", "job", job.JobID, "scanner", job.Scanner, "status", result.Status)
	return client.SubmitResult(ctx, job, result)
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
