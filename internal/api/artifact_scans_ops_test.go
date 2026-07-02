package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/younsl/o/box/kubernetes/forklift/internal/artifactscan"
	"github.com/younsl/o/box/kubernetes/forklift/internal/meta"
	"github.com/younsl/o/box/kubernetes/forklift/internal/repoconfig"
)

func TestArtifactScanOpsScanAllAndRecompute(t *testing.T) {
	srv, store, _ := newConsoleServer(t)
	ctx := context.Background()
	repoID := mkProxyRepo(t, srv.URL, "npmjs")
	cfg := repoconfig.Default()
	cfg.ArtifactScan = repoconfig.ArtifactScanPolicyConfig{
		Enabled:        true,
		ScannerProfile: "grype-default",
		Action:         repoconfig.VulnActionBlock,
		Threshold:      repoconfig.SeverityCritical,
	}
	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateRepositoryConfig(ctx, repoID, srv.URL, string(cfgJSON)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.PutArtifact(ctx, meta.Artifact{
		RepoID: repoID, Path: "left-pad/-/left-pad-1.0.0.tgz", BlobSHA256: "sha", Size: 42,
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	ensureArtifactScanProfile(t, store, now)

	var scanOut artifactScanOpsDTO
	postArtifactScanOpsJSON(t, srv.URL+"/artifact-scans/scan-all", artifactScanAllReq{RepositoryID: repoID}, &scanOut)
	if scanOut.Queued != 1 || len(scanOut.Jobs) != 1 || scanOut.Jobs[0].ScannerProfile != "grype-default" {
		t.Fatalf("scan-all response = %+v", scanOut)
	}

	claimNow := time.Now().UTC().Add(time.Second)
	claimed, err := store.ClaimArtifactScanJob(ctx, "worker", artifactScanCaps(now), claimNow.Add(time.Minute), claimNow, 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteArtifactScanJob(ctx, claimed.ID, "worker", artifactscan.Result{
		JobID:           claimed.ID,
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
	verdict, err := store.LatestArtifactScanVerdict(ctx, repoID, "sha", "grype-default")
	if err != nil {
		t.Fatal(err)
	}
	if verdict.Status != artifactscan.VerdictAllow {
		t.Fatalf("initial verdict = %+v", verdict)
	}

	cfg.ArtifactScan.Threshold = repoconfig.SeverityHigh
	cfgJSON, _ = json.Marshal(cfg)
	if err := store.UpdateRepositoryConfig(ctx, repoID, srv.URL, string(cfgJSON)); err != nil {
		t.Fatal(err)
	}
	var recomputeOut artifactScanOpsDTO
	postArtifactScanOpsJSON(t, srv.URL+"/artifact-scans/verdicts/recompute", artifactScanRecomputeReq{RepositoryID: repoID}, &recomputeOut)
	if recomputeOut.Recomputed != 1 || len(recomputeOut.Verdicts) != 1 || recomputeOut.Verdicts[0].Status != string(artifactscan.VerdictBlock) {
		t.Fatalf("recompute response = %+v", recomputeOut)
	}
}

func TestArtifactScanSBOMStoredAndReturned(t *testing.T) {
	srv, store, _ := newConsoleServer(t)
	ctx := context.Background()
	repoID := mkProxyRepo(t, srv.URL, "npmjs")
	cfg := repoconfig.Default()
	cfg.ArtifactScan = repoconfig.ArtifactScanPolicyConfig{
		Enabled:        true,
		ScannerProfile: "grype-default",
		Action:         repoconfig.VulnActionAudit,
		Threshold:      repoconfig.SeverityHigh,
	}
	cfgJSON, _ := json.Marshal(cfg)
	if err := store.UpdateRepositoryConfig(ctx, repoID, srv.URL, string(cfgJSON)); err != nil {
		t.Fatal(err)
	}
	const path = "left-pad/-/left-pad-1.0.0.tgz"
	if _, err := store.PutArtifact(ctx, meta.Artifact{RepoID: repoID, Path: path, BlobSHA256: "sbom-sha", Size: 42}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	ensureArtifactScanProfile(t, store, now)
	job, err := store.EnqueueArtifactScan(ctx, "sbom-job", "sbom-sha", "grype-default", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimArtifactScanJob(ctx, "worker", artifactScanCaps(now), now.Add(time.Minute), now, 3); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CompleteArtifactScanJob(ctx, job.ID, "worker", artifactscan.Result{
		JobID:          job.ID,
		BlobSHA256:     "sbom-sha",
		Scanner:        "grype",
		ScannerVersion: "1.0.0",
		Status:         artifactscan.ReportCompleted,
		MaxSeverity:    artifactscan.SeverityUnknown,
		ScannedAt:      now,
		SBOM: &artifactscan.SBOM{
			Format:           "cyclonedx-json",
			Generator:        "syft",
			GeneratorVersion: "1.0.0",
			ContentDigest:    "sha256:test",
			ContentJSON:      `{"bomFormat":"CycloneDX","components":[]}`,
		},
	}, now); err != nil {
		t.Fatal(err)
	}
	resp := adminDo(t, http.MethodGet, srv.URL+"/repositories/"+itoa(repoID)+"/artifacts/sbom?path="+path, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("sbom status=%d", resp.StatusCode)
	}
	defer resp.Body.Close()
	var out struct {
		BOMFormat  string `json:"bomFormat"`
		Components []any  `json:"components"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.BOMFormat != "CycloneDX" || len(out.Components) != 0 {
		t.Fatalf("sbom response = %+v", out)
	}
	var exportOut artifactSBOMExportDTO
	postArtifactScanOpsJSON(t, srv.URL+"/artifact-scans/sboms/export", artifactSBOMExportReq{
		RepositoryID: repoID,
		Path:         path,
		Destination:  "dependency-track:default",
	}, &exportOut)
	if exportOut.ID == 0 || exportOut.SBOMID == 0 || exportOut.Status != "pending" {
		t.Fatalf("export response = %+v", exportOut)
	}
}

func postArtifactScanOpsJSON(t *testing.T, url string, body any, out any) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	resp := adminDo(t, http.MethodPost, url, string(raw))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("post %s status=%d", url, resp.StatusCode)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatal(err)
		}
	}
}
