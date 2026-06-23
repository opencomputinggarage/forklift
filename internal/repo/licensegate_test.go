package repo

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/younsl/o/box/kubernetes/forklift/internal/license"
	"github.com/younsl/o/box/kubernetes/forklift/internal/meta"
	"github.com/younsl/o/box/kubernetes/forklift/internal/repoconfig"
)

type fakeResolver struct{}

func (fakeResolver) Resolve(context.Context, string, string, string) (license.Result, error) {
	return license.Result{}, nil
}
func (fakeResolver) Source() string { return "fake" }

func licenseCfg(action string, deny, allow []string) repoconfig.Config {
	cfg := repoconfig.Default()
	cfg.License = repoconfig.LicensePolicyConfig{Enabled: true, Action: action, Deny: deny, Allow: allow}
	return cfg
}

func TestLicenseGate(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "tarball-bytes")
	}))
	defer upstream.Close()

	m, _, store := newTestManager(t)
	m.SetLicenseResolver(fakeResolver{}) // activates the gate; async Resolve unused here
	mkFormatRepo(t, store, "npmjs", meta.FormatNPM, meta.TypeProxy, upstream.URL,
		licenseCfg(repoconfig.VulnActionBlock, []string{"GPL-3.0"}, nil))
	h := mux(m)

	tarball := "/npm/npmjs/copyleft/-/copyleft-1.0.0.tgz"
	get := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tarball, nil))
		return rec
	}
	ctx := t.Context()

	// Denied license (case-insensitive match) -> 403.
	if err := store.UpsertLicenseScan(ctx, "npm", "copyleft", "1.0.0", []string{"gpl-3.0"}, "deps.dev"); err != nil {
		t.Fatal(err)
	}
	if rec := get(); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "license policy") {
		t.Fatalf("denied license: code=%d body=%q", rec.Code, rec.Body.String())
	}

	// Permissive license, not denied -> served.
	if err := store.UpsertLicenseScan(ctx, "npm", "copyleft", "1.0.0", []string{"MIT"}, "deps.dev"); err != nil {
		t.Fatal(err)
	}
	if rec := get(); rec.Code != http.StatusOK {
		t.Fatalf("permissive license should serve: code=%d", rec.Code)
	}
}

func TestLicenseGateAllowListAndAudit(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "tarball-bytes")
	}))
	defer upstream.Close()

	m, _, store := newTestManager(t)
	m.SetLicenseResolver(fakeResolver{})
	ctx := t.Context()

	// Allow-list mode: a license outside the allow list is blocked.
	mkFormatRepo(t, store, "npmjs", meta.FormatNPM, meta.TypeProxy, upstream.URL,
		licenseCfg(repoconfig.VulnActionBlock, nil, []string{"MIT", "Apache-2.0"}))
	if err := store.UpsertLicenseScan(ctx, "npm", "weird", "1.0.0", []string{"BSD-3-Clause"}, "deps.dev"); err != nil {
		t.Fatal(err)
	}
	h := mux(m)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/npm/npmjs/weird/-/weird-1.0.0.tgz", nil))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("license outside allow list should block: code=%d", rec.Code)
	}

	// Allowed license -> served.
	if err := store.UpsertLicenseScan(ctx, "npm", "ok", "1.0.0", []string{"MIT"}, "deps.dev"); err != nil {
		t.Fatal(err)
	}
	h = mux(m)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/npm/npmjs/ok/-/ok-1.0.0.tgz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("allowed license should serve: code=%d", rec.Code)
	}

	// Audit mode never blocks even on a denied license.
	mkFormatRepo(t, store, "npm-audit", meta.FormatNPM, meta.TypeProxy, upstream.URL,
		licenseCfg(repoconfig.VulnActionAudit, []string{"GPL-3.0"}, nil))
	if err := store.UpsertLicenseScan(ctx, "npm", "copyleft", "2.0.0", []string{"GPL-3.0"}, "deps.dev"); err != nil {
		t.Fatal(err)
	}
	h = mux(m)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/npm/npm-audit/copyleft/-/copyleft-2.0.0.tgz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("audit mode must serve: code=%d", rec.Code)
	}
}

func TestLicenseGateBlockUnresolved(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "tarball-bytes")
	}))
	defer upstream.Close()

	m, _, store := newTestManager(t)
	m.SetLicenseResolver(fakeResolver{})
	cfg := licenseCfg(repoconfig.VulnActionBlock, []string{"GPL-3.0"}, nil)
	cfg.License.BlockUnresolved = true
	mkFormatRepo(t, store, "npmjs", meta.FormatNPM, meta.TypeProxy, upstream.URL, cfg)
	h := mux(m)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/npm/npmjs/unknown/-/unknown-1.0.0.tgz", nil))
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "pending license resolution") {
		t.Fatalf("block_unresolved: code=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestLicenseGateDisabledWithoutResolver(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "tarball-bytes")
	}))
	defer upstream.Close()

	// No resolver set: even a stored denied license does not block (feature off).
	m, _, store := newTestManager(t)
	mkFormatRepo(t, store, "npmjs", meta.FormatNPM, meta.TypeProxy, upstream.URL,
		licenseCfg(repoconfig.VulnActionBlock, []string{"GPL-3.0"}, nil))
	if err := store.UpsertLicenseScan(t.Context(), "npm", "copyleft", "1.0.0", []string{"GPL-3.0"}, "deps.dev"); err != nil {
		t.Fatal(err)
	}
	h := mux(m)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/npm/npmjs/copyleft/-/copyleft-1.0.0.tgz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("gate must be off without a resolver: code=%d", rec.Code)
	}
}

func TestLicenseViolation(t *testing.T) {
	cases := []struct {
		name     string
		licenses []string
		deny     []string
		allow    []string
		want     bool
	}{
		{"clean", []string{"MIT"}, []string{"GPL-3.0"}, nil, false},
		{"denied", []string{"GPL-3.0"}, []string{"GPL-3.0"}, nil, true},
		{"denied case-insensitive", []string{"gpl-3.0"}, []string{"GPL-3.0"}, nil, true},
		{"allow ok", []string{"MIT"}, nil, []string{"MIT", "Apache-2.0"}, false},
		{"allow miss", []string{"BSD-3-Clause"}, nil, []string{"MIT"}, true},
		{"allow all present", []string{"MIT", "Apache-2.0"}, nil, []string{"MIT", "Apache-2.0"}, false},
		{"allow one missing", []string{"MIT", "GPL-3.0"}, nil, []string{"MIT"}, true},
		{"empty licenses never violate", nil, []string{"GPL-3.0"}, []string{"MIT"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, _ := licenseViolation(c.licenses, c.deny, c.allow)
			if got != c.want {
				t.Fatalf("licenseViolation(%v, deny=%v, allow=%v) = %v, want %v",
					c.licenses, c.deny, c.allow, got, c.want)
			}
		})
	}
}

// recordingResolver records the coordinates it resolves, for backfill/upload tests.
type recordingResolver struct{ calls [][3]string }

func (r *recordingResolver) Resolve(_ context.Context, system, pkg, ver string) (license.Result, error) {
	r.calls = append(r.calls, [3]string{system, pkg, ver})
	return license.Result{Licenses: []string{"MIT"}}, nil
}
func (r *recordingResolver) Source() string { return "rec" }

// TestHostedUploadTriggersResolve verifies that publishing to a hosted
// repository enqueues an immediate license resolution for the uploaded
// coordinate.
func TestHostedUploadTriggersResolve(t *testing.T) {
	m, _, store := newTestManager(t)
	rec := &recordingResolver{}
	m.SetLicenseResolver(rec)
	mkFormatRepo(t, store, "mvn", meta.FormatMaven, meta.TypeHosted, "", repoconfig.Default())
	h := mux(m)
	ctx := t.Context()

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPut,
		"/maven/mvn/com/example/app/1.2.3/app-1.2.3.jar", strings.NewReader("JARDATA")))
	if w.Code != http.StatusCreated {
		t.Fatalf("upload code=%d", w.Code)
	}

	// Drain the queue synchronously so the assertion doesn't race the worker.
	for {
		select {
		case job := <-m.resolveQueue:
			m.runResolve(ctx, job)
			continue
		default:
		}
		break
	}
	if len(rec.calls) != 1 || rec.calls[0] != [3]string{"maven", "com.example:app", "1.2.3"} {
		t.Fatalf("resolve calls = %v, want [[maven com.example:app 1.2.3]]", rec.calls)
	}
	if _, err := store.GetLicenseScan(ctx, "maven", "com.example:app", "1.2.3"); err != nil {
		t.Fatalf("license result not stored: %v", err)
	}
}
