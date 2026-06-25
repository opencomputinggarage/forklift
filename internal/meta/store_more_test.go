package meta

import (
	"context"
	"testing"
	"time"
)

func TestSetRepositoryDisabledStore(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	repo, err := s.CreateRepository(ctx, Repository{Name: "r", Format: FormatMaven, Type: TypeHosted})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetRepositoryDisabled(ctx, repo.ID, true); err != nil {
		t.Fatalf("disable: %v", err)
	}
	got, _ := s.GetRepository(ctx, repo.ID)
	if !got.Disabled {
		t.Fatal("expected repository disabled")
	}
	if err := s.SetRepositoryDisabled(ctx, repo.ID, false); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetRepository(ctx, repo.ID)
	if got.Disabled {
		t.Fatal("expected repository enabled")
	}
}

func TestBlobStatsStore(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	repo, err := s.CreateRepository(ctx, Repository{Name: "r", Format: FormatNPM, Type: TypeProxy, UpstreamURL: "https://registry.npmjs.org"})
	if err != nil {
		t.Fatal(err)
	}
	if c, b, err := s.BlobStats(ctx); err != nil || c != 0 || b != 0 {
		t.Fatalf("empty blob stats = (%d,%d,%v)", c, b, err)
	}
	for _, a := range []Artifact{
		{RepoID: repo.ID, Path: "a/-/a-1.tgz", Version: "1", BlobSHA256: "sha-a", Size: 10},
		{RepoID: repo.ID, Path: "b/-/b-1.tgz", Version: "1", BlobSHA256: "sha-b", Size: 25},
	} {
		if _, err := s.PutArtifact(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	c, b, err := s.BlobStats(ctx)
	if err != nil || c != 2 || b != 35 {
		t.Fatalf("blob stats = (%d,%d,%v), want (2,35,nil)", c, b, err)
	}
}

func TestListAllTokensStore(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	u, err := s.CreateUser(ctx, User{Username: "svc"})
	if err != nil {
		t.Fatal(err)
	}
	exp := time.Now().Add(time.Hour)
	if _, err := s.CreateToken(ctx, Token{UserID: u.ID, Name: "ci", Hash: "h1", ScopesJSON: "[]", ExpiresAt: &exp}); err != nil {
		t.Fatal(err)
	}
	toks, err := s.ListAllTokens(ctx)
	if err != nil || len(toks) != 1 || toks[0].Name != "ci" {
		t.Fatalf("list all tokens = %+v err=%v", toks, err)
	}
}

func TestListScanTargetsStore(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	repo, err := s.CreateRepository(ctx, Repository{Name: "r", Format: FormatNPM, Type: TypeProxy, UpstreamURL: "https://registry.npmjs.org"})
	if err != nil {
		t.Fatal(err)
	}
	// Versioned artifacts are scan targets; a versionless one is excluded.
	for _, a := range []Artifact{
		{RepoID: repo.ID, Path: "a/-/a-1.tgz", Version: "1.0.0", BlobSHA256: "sa", Size: 1},
		{RepoID: repo.ID, Path: "b/-/b-2.tgz", Version: "2.0.0", BlobSHA256: "sb", Size: 1},
		{RepoID: repo.ID, Path: "meta.json", Version: "", BlobSHA256: "sc", Size: 1},
	} {
		if _, err := s.PutArtifact(ctx, a); err != nil {
			t.Fatal(err)
		}
	}
	targets, err := s.ListScanTargets(ctx, 100, 0)
	if err != nil || len(targets) != 2 {
		t.Fatalf("scan targets = %d err=%v, want 2", len(targets), err)
	}
	if targets[0].Format != FormatNPM || targets[0].Version == "" {
		t.Fatalf("unexpected target: %+v", targets[0])
	}
}

func TestScannedKeysStore(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	if err := s.UpsertVulnScan(ctx, "npm", "left-pad", "1.0.0", "high",
		[]string{"CVE-1"}, map[string]int{"high": 1}, 5, nil, "osv"); err != nil {
		t.Fatal(err)
	}
	keys, err := s.ScannedKeys(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := keys["npm\x00left-pad\x001.0.0"]; !ok {
		t.Fatalf("scanned keys missing coordinate: %v", keys)
	}
}

func TestPendingApprovalCountByRepoStore(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	if _, err := s.UpsertApprovalDecision(ctx, "npmjs", "left-pad", ApprovalPending, "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertApprovalDecision(ctx, "npmjs", "is-odd", ApprovalPending, "", ""); err != nil {
		t.Fatal(err)
	}
	// A decided one must not be counted as pending.
	if _, err := s.UpsertApprovalDecision(ctx, "npmjs", "axios", ApprovalApproved, "admin", "ok"); err != nil {
		t.Fatal(err)
	}
	counts, err := s.PendingApprovalCountByRepo(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if counts["npmjs"] != 2 {
		t.Fatalf("pending count = %d, want 2", counts["npmjs"])
	}
}
