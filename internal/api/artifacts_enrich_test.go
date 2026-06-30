package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/younsl/o/box/kubernetes/forklift/internal/artifactscan"
	"github.com/younsl/o/box/kubernetes/forklift/internal/meta"
	"github.com/younsl/o/box/kubernetes/forklift/internal/repo"
)

// TestListArtifactsEnriched exercises the artifact listing's vulnerability and
// license enrichment: an artifact whose coordinate has stored scans is returned
// with the severity, advisory ids and licenses attached.
func TestListArtifactsEnriched(t *testing.T) {
	srv, store, _ := newConsoleServer(t)
	ctx := context.Background()
	id := mkProxyRepo(t, srv.URL, "npmjs")

	const artPath, ver = "left-pad/-/left-pad-1.0.0.tgz", "1.0.0"
	if _, err := store.PutArtifact(ctx, meta.Artifact{
		RepoID: id, Path: artPath, Version: ver, BlobSHA256: "sha", Size: 42,
	}); err != nil {
		t.Fatal(err)
	}

	// Seed scans under the exact coordinate the lister resolves, so the match is
	// independent of per-format path parsing.
	eco, pkg := repo.VulnCoordinate(meta.FormatNPM, artPath)
	if err := store.UpsertVulnScan(ctx, eco, pkg, ver, "high",
		[]string{"CVE-2026-1"}, map[string]int{"high": 1}, 3, nil, "osv"); err != nil {
		t.Fatal(err)
	}
	sys, lpkg := repo.LicenseCoordinate(meta.FormatNPM, artPath)
	if err := store.UpsertLicenseScan(ctx, sys, lpkg, ver, []string{"MIT"}, "deps.dev"); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	job, err := store.EnqueueArtifactScan(ctx, "scan-job-1", "sha", "grype", "", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimArtifactScanJob(ctx, "worker", now.Add(time.Minute), now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteArtifactScanJob(ctx, job.ID, "worker", artifactscan.Result{
		JobID:           job.ID,
		BlobSHA256:      "sha",
		Scanner:         "grype",
		ScannerVersion:  "1.0.0",
		DatabaseBuiltAt: now,
		Status:          artifactscan.StatusCompleted,
		MaxSeverity:     artifactscan.SeverityHigh,
		ScannedAt:       now,
		Findings: []artifactscan.Finding{{
			VulnerabilityID: "CVE-2026-2",
			Severity:        artifactscan.SeverityHigh,
			PackageName:     "left-pad",
		}},
	}, now); err != nil {
		t.Fatal(err)
	}

	resp := adminDo(t, http.MethodGet, srv.URL+"/repositories/"+itoa(id)+"/artifacts", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list artifacts status=%d", resp.StatusCode)
	}
	var out struct {
		Count     int   `json:"count"`
		TotalSize int64 `json:"total_size"`
		Artifacts []struct {
			Path                     string   `json:"path"`
			MaxSeverity              string   `json:"max_severity"`
			VulnIDs                  []string `json:"vuln_ids"`
			Licenses                 []string `json:"licenses"`
			ArtifactScanStatus       string   `json:"artifact_scan_status"`
			ArtifactScanSeverity     string   `json:"artifact_scan_max_severity"`
			ArtifactScanFindingCount int      `json:"artifact_scan_finding_count"`
		} `json:"artifacts"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	resp.Body.Close()
	if out.Count != 1 || out.TotalSize != 42 || len(out.Artifacts) != 1 {
		t.Fatalf("unexpected list: %+v", out)
	}
	a := out.Artifacts[0]
	if a.MaxSeverity != "high" || len(a.VulnIDs) != 1 || len(a.Licenses) != 1 || a.Licenses[0] != "MIT" {
		t.Fatalf("enrichment not attached: %+v", a)
	}
	if a.ArtifactScanStatus != "completed" || a.ArtifactScanSeverity != "high" || a.ArtifactScanFindingCount != 1 {
		t.Fatalf("artifact scan enrichment not attached: %+v", a)
	}

	// Prefix filter that matches nothing returns an empty set, still 200.
	resp = adminDo(t, http.MethodGet,
		srv.URL+"/repositories/"+itoa(id)+"/artifacts?prefix="+url.QueryEscape("does-not-exist"), "")
	json.NewDecoder(resp.Body).Decode(&out)
	resp.Body.Close()
	if len(out.Artifacts) != 0 {
		t.Fatalf("prefix filter returned %d, want 0", len(out.Artifacts))
	}
}

func TestListArtifactsShowsPendingArtifactScanJob(t *testing.T) {
	srv, store, _ := newConsoleServer(t)
	ctx := context.Background()
	id := mkProxyRepo(t, srv.URL, "npm-pending")

	const artPath = "pkg/-/pkg-1.0.0.tgz"
	if _, err := store.PutArtifact(ctx, meta.Artifact{
		RepoID: id, Path: artPath, Version: "1.0.0", BlobSHA256: "queued-sha", Size: 10,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.EnqueueArtifactScan(ctx, "scan-job-queued", "queued-sha", "grype", "", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}

	resp := adminDo(t, http.MethodGet, srv.URL+"/repositories/"+itoa(id)+"/artifacts", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list artifacts status=%d", resp.StatusCode)
	}
	var out struct {
		Artifacts []struct {
			Path                string `json:"path"`
			ArtifactScanStatus  string `json:"artifact_scan_status"`
			ArtifactScanScanner string `json:"artifact_scan_scanner"`
		} `json:"artifacts"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	resp.Body.Close()
	if len(out.Artifacts) != 1 {
		t.Fatalf("artifact count=%d, want 1", len(out.Artifacts))
	}
	if out.Artifacts[0].ArtifactScanStatus != "queued" || out.Artifacts[0].ArtifactScanScanner != "grype" {
		t.Fatalf("pending scan status not attached: %+v", out.Artifacts[0])
	}
}
