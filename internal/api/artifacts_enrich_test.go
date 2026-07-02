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
		RepoID: id, Path: artPath, BlobSHA256: "sha", Size: 42,
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
	ensureArtifactScanProfile(t, store, now)
	job, err := store.EnqueueArtifactScan(ctx, "scan-job-1", "sha", "grype-default", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimArtifactScanJob(ctx, "worker", artifactScanCaps(now), now.Add(time.Minute), now, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteArtifactScanJob(ctx, job.ID, "worker", artifactscan.Result{
		JobID:           job.ID,
		BlobSHA256:      "sha",
		Scanner:         "grype",
		ScannerVersion:  "1.0.0",
		DatabaseBuiltAt: now,
		Status:          artifactscan.ReportCompleted,
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
			Version                  string   `json:"version"`
			PackageName              string   `json:"package_name"`
			Ecosystem                string   `json:"ecosystem"`
			DepsDevSystem            string   `json:"depsdev_system"`
			PackagePURL              string   `json:"package_purl"`
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
	if a.Version != ver || a.MaxSeverity != "high" || len(a.VulnIDs) != 1 || len(a.Licenses) != 1 || a.Licenses[0] != "MIT" {
		t.Fatalf("enrichment not attached: %+v", a)
	}
	if a.PackageName != "left-pad" || a.Ecosystem != "npm" || a.DepsDevSystem != "npm" || a.PackagePURL != "pkg:npm/left-pad@1.0.0" {
		t.Fatalf("package coordinate not attached: %+v", a)
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
	ensureArtifactScanProfile(t, store, time.Now().UTC())
	if _, err := store.EnqueueArtifactScan(ctx, "scan-job-queued", "queued-sha", "grype-default", time.Now().UTC()); err != nil {
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

func TestScanArtifactEnqueuesJob(t *testing.T) {
	srv, store, _ := newConsoleServer(t)
	ctx := context.Background()
	id := mkProxyRepo(t, srv.URL, "npm-manual-scan")

	const artPath = "pkg/-/pkg-1.0.0.tgz"
	if _, err := store.PutArtifact(ctx, meta.Artifact{
		RepoID: id, Path: artPath, Version: "1.0.0", BlobSHA256: "manual-sha", Size: 10,
	}); err != nil {
		t.Fatal(err)
	}
	ensureArtifactScanProfile(t, store, time.Now().UTC())

	resp := adminDo(t, http.MethodPost,
		srv.URL+"/repositories/"+itoa(id)+"/artifacts/scan?path="+url.QueryEscape(artPath), "")
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("scan artifact status=%d", resp.StatusCode)
	}
	var out struct {
		JobID      string `json:"job_id"`
		Status     string `json:"status"`
		Scanner    string `json:"scanner"`
		BlobSHA256 string `json:"blob_sha256"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	resp.Body.Close()
	if out.JobID == "" || out.Status != "queued" || out.Scanner != "grype" || out.BlobSHA256 != "manual-sha" {
		t.Fatalf("unexpected enqueue response: %+v", out)
	}
	job, err := store.LatestArtifactScanJob(ctx, "manual-sha", "grype-default")
	if err != nil {
		t.Fatalf("latest scan job: %v", err)
	}
	if job.ID != out.JobID || job.Status != artifactscan.JobQueued {
		t.Fatalf("stored job = %+v response=%+v", job, out)
	}

	resp = adminDo(t, http.MethodGet, srv.URL+"/repositories/"+itoa(id)+"/artifacts", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list artifacts status=%d", resp.StatusCode)
	}
	var listed struct {
		Artifacts []struct {
			ArtifactScanStatus string `json:"artifact_scan_status"`
		} `json:"artifacts"`
	}
	json.NewDecoder(resp.Body).Decode(&listed)
	resp.Body.Close()
	if len(listed.Artifacts) != 1 || listed.Artifacts[0].ArtifactScanStatus != "queued" {
		t.Fatalf("manual scan status not visible: %+v", listed.Artifacts)
	}
}

func TestArtifactScanDetailsAndBatch(t *testing.T) {
	srv, store, _ := newConsoleServer(t)
	ctx := context.Background()
	id := mkProxyRepo(t, srv.URL, "npm-scan-details")

	paths := []string{"pkg/-/pkg-1.0.0.tgz", "other/-/other-1.0.0.tgz"}
	for _, p := range paths {
		if _, err := store.PutArtifact(ctx, meta.Artifact{
			RepoID: id, Path: p, Version: "1.0.0", BlobSHA256: p + "-sha", Size: 10,
		}); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	ensureArtifactScanProfile(t, store, now)
	job, err := store.EnqueueArtifactScan(ctx, "scan-detail-job", paths[0]+"-sha", "grype-default", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimArtifactScanJob(ctx, "worker", artifactScanCaps(now), now.Add(time.Minute), now, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteArtifactScanJob(ctx, job.ID, "worker", artifactscan.Result{
		JobID:           job.ID,
		BlobSHA256:      paths[0] + "-sha",
		Scanner:         "grype",
		ScannerVersion:  "1.0.0",
		DatabaseBuiltAt: now,
		Status:          artifactscan.ReportCompleted,
		MaxSeverity:     artifactscan.SeverityHigh,
		ScannedAt:       now,
		Findings: []artifactscan.Finding{{
			VulnerabilityID: "CVE-2026-DETAIL",
			Severity:        artifactscan.SeverityHigh,
			PackageName:     "pkg",
			PackageVersion:  "1.0.0",
			FixedVersions:   []string{"1.0.1"},
		}},
	}, now); err != nil {
		t.Fatal(err)
	}

	resp := adminDo(t, http.MethodGet,
		srv.URL+"/repositories/"+itoa(id)+"/artifacts/scan?path="+url.QueryEscape(paths[0]), "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("scan details status=%d", resp.StatusCode)
	}
	var detail struct {
		Status       string `json:"status"`
		PackageName  string `json:"package_name"`
		Ecosystem    string `json:"ecosystem"`
		PackagePURL  string `json:"package_purl"`
		FindingCount int    `json:"finding_count"`
		Findings     []struct {
			VulnerabilityID string `json:"vulnerability_id"`
			PackageName     string `json:"package_name"`
		} `json:"findings"`
	}
	json.NewDecoder(resp.Body).Decode(&detail)
	resp.Body.Close()
	if detail.Status != "completed" || detail.FindingCount != 1 || len(detail.Findings) != 1 || detail.Findings[0].VulnerabilityID != "CVE-2026-DETAIL" {
		t.Fatalf("detail = %+v", detail)
	}
	if detail.PackageName != "pkg" || detail.Ecosystem != "npm" || detail.PackagePURL != "pkg:npm/pkg@1.0.0" {
		t.Fatalf("detail coordinate = %+v", detail)
	}

	resp = adminDo(t, http.MethodGet,
		srv.URL+"/repositories/"+itoa(id)+"/artifacts/scan?path="+url.QueryEscape(paths[1]), "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unscanned detail status=%d", resp.StatusCode)
	}
	var unscanned struct {
		Status       string `json:"status"`
		PackageName  string `json:"package_name"`
		PackagePURL  string `json:"package_purl"`
		FindingCount int    `json:"finding_count"`
	}
	json.NewDecoder(resp.Body).Decode(&unscanned)
	resp.Body.Close()
	if unscanned.Status != "unscanned" || unscanned.FindingCount != 0 || unscanned.PackageName != "other" || unscanned.PackagePURL != "pkg:npm/other@1.0.0" {
		t.Fatalf("unscanned detail = %+v", unscanned)
	}

	body := `{"paths":["` + paths[0] + `","` + paths[1] + `"]}`
	resp = adminDo(t, http.MethodPost, srv.URL+"/repositories/"+itoa(id)+"/artifacts/scan-batch", body)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("batch status=%d", resp.StatusCode)
	}
	var batch struct {
		Queued int `json:"queued"`
	}
	json.NewDecoder(resp.Body).Decode(&batch)
	resp.Body.Close()
	if batch.Queued != 2 {
		t.Fatalf("queued = %d, want 2", batch.Queued)
	}

	resp = adminDo(t, http.MethodGet, srv.URL+"/repositories/"+itoa(id)+"/artifacts?limit=1&offset=1", "")
	var listed struct {
		Count     int `json:"count"`
		Limit     int `json:"limit"`
		Offset    int `json:"offset"`
		Artifacts []struct {
			Path string `json:"path"`
		} `json:"artifacts"`
	}
	json.NewDecoder(resp.Body).Decode(&listed)
	resp.Body.Close()
	if listed.Count != 2 || listed.Limit != 1 || listed.Offset != 1 || len(listed.Artifacts) != 1 {
		t.Fatalf("paged list = %+v", listed)
	}
}

func ensureArtifactScanProfile(t *testing.T, store *meta.Store, now time.Time) {
	t.Helper()
	if err := store.EnsureArtifactScannerProfile(context.Background(), artifactscan.Profile{
		Name:       "grype-default",
		Scanner:    "grype",
		Mode:       artifactscan.ModeDeployment,
		ConfigHash: "grype-default-v1",
		Limits:     artifactscan.Limits{MaxArtifactBytes: 100 << 20},
		CreatedAt:  now,
		UpdatedAt:  now,
	}); err != nil {
		t.Fatalf("ensure artifact scan profile: %v", err)
	}
}

func artifactScanCaps(now time.Time) []artifactscan.ScannerCapability {
	return []artifactscan.ScannerCapability{{
		Name:                  "grype",
		Version:               "1.0.0",
		DatabaseSchemaVersion: "6",
		DatabaseBuiltAt:       now,
	}}
}
