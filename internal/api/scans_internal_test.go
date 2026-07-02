package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/younsl/o/box/kubernetes/forklift/internal/artifactscan"
	"github.com/younsl/o/box/kubernetes/forklift/internal/meta"
	"github.com/younsl/o/box/kubernetes/forklift/internal/storage"
)

func TestScanInternalClaimBlobAndResult(t *testing.T) {
	ctx := context.Background()
	store, err := meta.Open(ctx, filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	blobs, err := storage.NewFSStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	blobSHA, blobSize, err := blobs.Put(ctx, bytes.NewBufferString("artifact bytes"))
	if err != nil {
		t.Fatal(err)
	}
	repo, err := store.CreateRepository(ctx, meta.Repository{
		Name:       "npmjs",
		Format:     meta.FormatNPM,
		Type:       meta.TypeHosted,
		ConfigJSON: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PutArtifact(ctx, meta.Artifact{
		RepoID:     repo.ID,
		Path:       "pkg/-/pkg-1.0.0.tgz",
		Version:    "1.0.0",
		BlobSHA256: blobSHA,
		Size:       blobSize,
	}); err != nil {
		t.Fatal(err)
	}

	svc, err := artifactscan.NewService(store, artifactscan.ServiceConfig{
		DefaultProfile: "grype-default",
		TokenKey:       []byte("0123456789abcdef0123456789abcdef"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.InitProfiles(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Enqueue(ctx, blobSHA, "grype-default"); err != nil {
		t.Fatal(err)
	}

	r := chi.NewRouter()
	r.Mount("/internal/scans", NewScanInternal(svc, blobs, "worker-secret", slog.New(slog.NewTextHandler(io.Discard, nil))).Routes())
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	unauth := postScanJSON(t, srv.URL+"/internal/scans/claim", "", `{"worker_id":"worker-1"}`)
	if unauth.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauth claim status=%d", unauth.StatusCode)
	}
	unauth.Body.Close()

	claimResp := postScanJSON(t, srv.URL+"/internal/scans/claim", "worker-secret", `{"worker_id":"worker-1","capabilities":[{"name":"grype","version":"1.0.0","database_schema_version":"6","database_built_at":"2026-07-01T00:00:00Z"}]}`)
	if claimResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(claimResp.Body)
		t.Fatalf("claim status=%d body=%s", claimResp.StatusCode, body)
	}
	var claim scanClaimResp
	if err := json.NewDecoder(claimResp.Body).Decode(&claim); err != nil {
		t.Fatal(err)
	}
	claimResp.Body.Close()
	if claim.JobID == "" || claim.BlobSHA256 != blobSHA || claim.Token == "" {
		t.Fatalf("bad claim response: %+v", claim)
	}
	if claim.Deadline.IsZero() || claim.Limits.MaxArtifactBytes != 100<<20 {
		t.Fatalf("bad claim lease/limits: %+v", claim)
	}
	if len(claim.Targets) != 1 ||
		claim.Targets[0].Repository != "npmjs" ||
		claim.Targets[0].Format != meta.FormatNPM ||
		claim.Targets[0].Path != "pkg/-/pkg-1.0.0.tgz" ||
		claim.Targets[0].Version != "1.0.0" ||
		claim.Targets[0].PackageName != "pkg" ||
		claim.Targets[0].Ecosystem != "npm" ||
		claim.Targets[0].DepsDevSystem != "npm" ||
		claim.Targets[0].PURL != "pkg:npm/pkg@1.0.0" ||
		claim.Targets[0].Size != blobSize {
		t.Fatalf("bad claim targets: %+v", claim.Targets)
	}

	blobReq, err := http.NewRequest(http.MethodGet, srv.URL+"/internal/scans/"+claim.JobID+"/blob", nil)
	if err != nil {
		t.Fatal(err)
	}
	blobReq.Header.Set("Authorization", "Bearer "+claim.Token)
	blobResp, err := http.DefaultClient.Do(blobReq)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(blobResp.Body)
	blobResp.Body.Close()
	if blobResp.StatusCode != http.StatusOK || string(body) != "artifact bytes" {
		t.Fatalf("blob status=%d body=%q", blobResp.StatusCode, body)
	}
	if got := blobResp.Header.Get("X-Artifact-SHA256"); got != blobSHA {
		t.Fatalf("blob sha header=%q, want %q", got, blobSHA)
	}

	heartbeat := postScanJSON(t, srv.URL+"/internal/scans/"+claim.JobID+"/heartbeat", claim.Token, `{"worker_id":"worker-1"}`)
	if heartbeat.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(heartbeat.Body)
		t.Fatalf("heartbeat status=%d body=%s", heartbeat.StatusCode, body)
	}
	var heartbeatResp scanHeartbeatResp
	if err := json.NewDecoder(heartbeat.Body).Decode(&heartbeatResp); err != nil {
		t.Fatal(err)
	}
	heartbeat.Body.Close()
	if heartbeatResp.Token == "" || heartbeatResp.Deadline.IsZero() {
		t.Fatalf("bad heartbeat response: %+v", heartbeatResp)
	}

	result := scanResultReq{
		WorkerID: "worker-1",
		Result: artifactscan.Result{
			JobID:           claim.JobID,
			BlobSHA256:      blobSHA,
			Scanner:         "grype",
			ScannerVersion:  "1.0.0",
			DatabaseBuiltAt: time.Now().UTC(),
			Status:          artifactscan.ReportCompleted,
			MaxSeverity:     artifactscan.SeverityHigh,
			ScannedAt:       time.Now().UTC(),
			Findings: []artifactscan.Finding{{
				VulnerabilityID: "CVE-2026-0001",
				Severity:        artifactscan.SeverityHigh,
				PackageName:     "pkg",
			}},
		},
	}
	resultBody, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	submit := postScanJSON(t, srv.URL+"/internal/scans/"+claim.JobID+"/result", heartbeatResp.Token, string(resultBody))
	if submit.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(submit.Body)
		t.Fatalf("submit status=%d body=%s", submit.StatusCode, body)
	}
	submit.Body.Close()

	stored, err := store.LatestArtifactScanResult(ctx, blobSHA, "grype-default")
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != artifactscan.ReportCompleted || stored.MaxSeverity != artifactscan.SeverityHigh || len(stored.Findings) != 1 {
		t.Fatalf("stored result = %+v", stored)
	}
}

func postScanJSON(t *testing.T, url, token, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}
