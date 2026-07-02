package meta

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/younsl/o/box/kubernetes/forklift/internal/artifactscan"
	"github.com/younsl/o/box/kubernetes/forklift/internal/repoconfig"
)

func testScanProfile(now time.Time) artifactscan.Profile {
	return artifactscan.Profile{
		Name:       "grype-default",
		Scanner:    "grype",
		Mode:       artifactscan.ModeDeployment,
		ConfigHash: "grype-default-v1",
		Limits:     artifactscan.Limits{MaxArtifactBytes: 100 << 20},
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

func testCapabilities(now time.Time) []artifactscan.ScannerCapability {
	return []artifactscan.ScannerCapability{{
		Name:                  "grype",
		Version:               "1.0.0",
		DatabaseSchemaVersion: "6",
		DatabaseBuiltAt:       now,
		SupportedEcosystems:   []string{"npm"},
	}}
}

func TestArtifactScanProfileJobReportVerdictLifecycle(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	cfg := repoconfig.Default()
	cfg.ArtifactScan = repoconfig.ArtifactScanPolicyConfig{
		Enabled:        true,
		ScannerProfile: "grype-default",
		Action:         repoconfig.VulnActionBlock,
		Threshold:      repoconfig.SeverityHigh,
	}
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	repo, _ := s.CreateRepository(ctx, Repository{Name: "r", Format: FormatNPM, Type: TypeHosted, ConfigJSON: string(cfgJSON)})
	cachedAt := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	art, err := s.PutArtifact(ctx, Artifact{
		RepoID:         repo.ID,
		Path:           "axios/-/axios-0.21.1.tgz",
		BlobSHA256:     "abc",
		Size:           3,
		ContentType:    "application/octet-stream",
		CachedAt:       cachedAt,
		LastAccessedAt: cachedAt.Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 1, 1, 2, 3, 0, time.UTC)
	if err := s.EnsureArtifactScannerProfile(ctx, testScanProfile(now)); err != nil {
		t.Fatalf("ensure profile: %v", err)
	}
	job, err := s.EnqueueArtifactScan(ctx, "job-1", art.BlobSHA256, "grype-default", now)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if job.Status != artifactscan.JobQueued || job.ScannerProfile != "grype-default" || job.Scanner != "grype" {
		t.Fatalf("job = %+v", job)
	}
	targets, err := s.ArtifactScanTargets(ctx, art.BlobSHA256)
	if err != nil {
		t.Fatalf("scan targets: %v", err)
	}
	if len(targets) != 1 || targets[0].Repository != "r" || targets[0].PURL != "pkg:npm/axios@0.21.1" {
		t.Fatalf("targets = %+v", targets)
	}
	claimed, err := s.ClaimArtifactScanJob(ctx, "worker-1", testCapabilities(now), now.Add(time.Minute), now, 3)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if claimed.ID != job.ID || claimed.Status != artifactscan.JobRunning || claimed.Attempts != 1 {
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
		DatabaseSchemaVersion: "6",
		DatabaseBuiltAt:       now,
		Status:                artifactscan.ReportCompleted,
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
	stored, err := s.LatestArtifactScanResult(ctx, art.BlobSHA256, "grype-default")
	if err != nil {
		t.Fatalf("latest result: %v", err)
	}
	if stored.ScannerProfile != "grype-default" || stored.MaxSeverity != artifactscan.SeverityHigh || len(stored.Findings) != 1 {
		t.Fatalf("stored result = %+v", stored)
	}
	gotVerdict, err := s.LatestArtifactScanVerdict(ctx, repo.ID, art.BlobSHA256, "grype-default")
	if err != nil {
		t.Fatalf("latest verdict: %v", err)
	}
	if gotVerdict.Status != artifactscan.VerdictBlock || gotVerdict.ResultID != stored.ID {
		t.Fatalf("verdict = %+v", gotVerdict)
	}
}

func TestClaimArtifactScanJobRequiresMatchingCapability(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	repo, _ := s.CreateRepository(ctx, Repository{Name: "r", Format: FormatNPM, Type: TypeHosted})
	art, err := s.PutArtifact(ctx, Artifact{RepoID: repo.ID, Path: "a.tgz", BlobSHA256: "abc", Size: 3})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 1, 1, 2, 3, 0, time.UTC)
	if err := s.EnsureArtifactScannerProfile(ctx, testScanProfile(now)); err != nil {
		t.Fatalf("ensure profile: %v", err)
	}
	if _, err := s.EnqueueArtifactScan(ctx, "job-mismatch", art.BlobSHA256, "grype-default", now); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	_, err = s.ClaimArtifactScanJob(ctx, "worker-1", []artifactscan.ScannerCapability{{Name: "trivy"}}, now.Add(time.Minute), now, 3)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("claim err = %v, want ErrNotFound", err)
	}
}

func TestClaimArtifactScanJobMarksExpiredAtMaxAttemptsDead(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	repo, _ := s.CreateRepository(ctx, Repository{Name: "r", Format: FormatNPM, Type: TypeHosted})
	art, err := s.PutArtifact(ctx, Artifact{RepoID: repo.ID, Path: "a.tgz", BlobSHA256: "abc", Size: 3})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 1, 1, 2, 3, 0, time.UTC)
	if err := s.EnsureArtifactScannerProfile(ctx, testScanProfile(now)); err != nil {
		t.Fatalf("ensure profile: %v", err)
	}
	job, err := s.EnqueueArtifactScan(ctx, "job-dead", art.BlobSHA256, "grype-default", now)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if _, err := s.ClaimArtifactScanJob(ctx, "worker-1", testCapabilities(now), now.Add(time.Minute), now, 1); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if _, err := s.ClaimArtifactScanJob(ctx, "worker-2", testCapabilities(now), now.Add(2*time.Minute), now.Add(2*time.Minute), 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("claim after max attempts err = %v, want ErrNotFound", err)
	}
	stored, err := s.LatestArtifactScanJob(ctx, art.BlobSHA256, "grype-default")
	if err != nil {
		t.Fatalf("latest job: %v", err)
	}
	if stored.ID != job.ID || stored.Status != artifactscan.JobDead || stored.FinishedAt.IsZero() {
		t.Fatalf("stored job = %+v", stored)
	}
}
