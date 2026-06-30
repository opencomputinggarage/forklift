package meta

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/younsl/o/box/kubernetes/forklift/internal/artifactscan"
)

func TestArtifactScanJobLifecycle(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	repo, _ := s.CreateRepository(ctx, Repository{Name: "r", Format: FormatNPM, Type: TypeHosted})
	art, err := s.PutArtifact(ctx, Artifact{RepoID: repo.ID, Path: "a.tgz", BlobSHA256: "abc", Size: 3})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 1, 1, 2, 3, 0, time.UTC)
	job, err := s.EnqueueArtifactScan(ctx, "job-1", art.BlobSHA256, "grype", "cfg", now)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if job.Status != artifactscan.StatusQueued {
		t.Fatalf("status = %s", job.Status)
	}
	latestJob, err := s.LatestArtifactScanJob(ctx, art.BlobSHA256, "grype", "cfg")
	if err != nil {
		t.Fatalf("latest job: %v", err)
	}
	if latestJob.ID != job.ID || latestJob.Status != artifactscan.StatusQueued {
		t.Fatalf("latest job = %+v", latestJob)
	}
	claimed, err := s.ClaimArtifactScanJob(ctx, "worker-1", now.Add(time.Minute), now)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if claimed.ID != job.ID || claimed.Status != artifactscan.StatusRunning || claimed.Attempts != 1 {
		t.Fatalf("claimed = %+v", claimed)
	}
	if err := s.HeartbeatArtifactScanJob(ctx, job.ID, "worker-1", now.Add(2*time.Minute), now.Add(time.Second)); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	res := artifactscan.Result{
		JobID:                 job.ID,
		BlobSHA256:            art.BlobSHA256,
		Scanner:               "grype",
		ScannerVersion:        "1.0.0",
		ScannerConfigHash:     "cfg",
		DatabaseSchemaVersion: "6",
		DatabaseBuiltAt:       now,
		Status:                artifactscan.StatusCompleted,
		MaxSeverity:           artifactscan.SeverityHigh,
		ScannedAt:             now.Add(3 * time.Second),
		Findings: []artifactscan.Finding{{
			VulnerabilityID: "CVE-2026-0001",
			Severity:        artifactscan.SeverityHigh,
			PackageName:     "pkg",
			FixedVersions:   []string{"1.0.1"},
		}},
	}
	resultID, err := s.CompleteArtifactScanJob(ctx, job.ID, "worker-1", res, now.Add(4*time.Second))
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if resultID == 0 {
		t.Fatal("missing result id")
	}
	stored, err := s.LatestArtifactScanResult(ctx, art.BlobSHA256, "grype", "cfg")
	if err != nil {
		t.Fatalf("latest result: %v", err)
	}
	if stored.MaxSeverity != artifactscan.SeverityHigh || len(stored.Findings) != 1 {
		t.Fatalf("stored result = %+v", stored)
	}
}

func TestClaimArtifactScanJobEmpty(t *testing.T) {
	s := openTestStore(t)
	_, err := s.ClaimArtifactScanJob(context.Background(), "worker", time.Now().Add(time.Minute), time.Now())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
