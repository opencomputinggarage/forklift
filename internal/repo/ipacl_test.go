package repo

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/younsl/o/box/kubernetes/forklift/internal/meta"
	"github.com/younsl/o/box/kubernetes/forklift/internal/repoconfig"
)

// doIP issues a request carrying an X-Forwarded-For hop so the IP ACL sees a
// deterministic client IP regardless of the synthetic RemoteAddr.
func doIP(h http.Handler, method, path, xff, body string) *httptest.ResponseRecorder {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	if xff != "" {
		r.Header.Set("X-Forwarded-For", xff)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

// TestIPACLEnforcement covers the per-repository source-IP allow list on a
// hosted repository: an allowed IP is served, a disallowed IP gets 403, and the
// gate applies to writes too.
func TestIPACLEnforcement(t *testing.T) {
	m, _, store := newTestManager(t)
	cfg := repoconfig.Default()
	cfg.IPACL = repoconfig.IPACLConfig{Enabled: true, Allow: []string{"203.0.113.0/24"}}
	mkRepo(t, store, "mvn-acl", meta.TypeHosted, "", cfg)
	h := mux(m)
	path := "/maven/mvn-acl/com/example/app/1.0/app-1.0.jar"

	// Upload from an allowed IP succeeds.
	if rec := doIP(h, http.MethodPut, path, "203.0.113.7", "JARBYTES"); rec.Code != http.StatusCreated {
		t.Fatalf("allowed put = %d, want 201", rec.Code)
	}
	// Download from an allowed IP succeeds.
	if rec := doIP(h, http.MethodGet, path, "203.0.113.9", ""); rec.Code != http.StatusOK {
		t.Fatalf("allowed get = %d, want 200", rec.Code)
	}
	// Download from a disallowed IP is refused before serving.
	if rec := doIP(h, http.MethodGet, path, "198.51.100.4", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("denied get = %d, want 403", rec.Code)
	}
	// Write from a disallowed IP is refused too.
	if rec := doIP(h, http.MethodPut, path, "198.51.100.4", "X"); rec.Code != http.StatusForbidden {
		t.Fatalf("denied put = %d, want 403", rec.Code)
	}
}

// TestIPACLGroupGovernsEntry verifies that on a group repository the group's own
// ACL governs entry, and member ACLs are not re-checked during fan-out.
func TestIPACLGroupGovernsEntry(t *testing.T) {
	m, _, store := newTestManager(t)

	// Member: hosted repo with an ACL that would block the test client outright.
	memCfg := repoconfig.Default()
	memCfg.IPACL = repoconfig.IPACLConfig{Enabled: true, Allow: []string{"10.0.0.0/8"}}
	mkRepo(t, store, "mvn-member", meta.TypeHosted, "", memCfg)
	// Seed an artifact in the member, bypassing the ACL by writing as an allowed IP.
	h := mux(m)
	memPath := "/maven/mvn-member/com/example/app/1.0/app-1.0.jar"
	if rec := doIP(h, http.MethodPut, memPath, "10.1.2.3", "JARBYTES"); rec.Code != http.StatusCreated {
		t.Fatalf("seed member put = %d, want 201", rec.Code)
	}

	// Group: allows the test client; members listed in lookup order.
	grpCfg := repoconfig.Default()
	grpCfg.IPACL = repoconfig.IPACLConfig{Enabled: true, Allow: []string{"203.0.113.0/24"}}
	grpCfg.Group = repoconfig.GroupConfig{Members: []string{"mvn-member"}}
	mkRepo(t, store, "mvn-group", meta.TypeGroup, "", grpCfg)
	grpPath := "/maven/mvn-group/com/example/app/1.0/app-1.0.jar"

	// Allowed by the group ACL: served from the member even though the member's
	// own ACL would block this client (member checks skipped via group).
	if rec := doIP(h, http.MethodGet, grpPath, "203.0.113.5", ""); rec.Code != http.StatusOK {
		t.Fatalf("group allowed get = %d, want 200", rec.Code)
	}
	// Denied by the group ACL: refused at entry, members never tried.
	if rec := doIP(h, http.MethodGet, grpPath, "198.51.100.4", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("group denied get = %d, want 403", rec.Code)
	}
}
