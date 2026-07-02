package repo

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/younsl/o/box/kubernetes/forklift/internal/meta"
	"github.com/younsl/o/box/kubernetes/forklift/internal/repoconfig"
)

func mkFormatRepo(t *testing.T, store *meta.Store, name, format, typ, upstream string, cfg repoconfig.Config) {
	t.Helper()
	j, _ := cfg.JSON()
	if _, err := store.CreateRepository(t.Context(), meta.Repository{
		Name: name, Format: format, Type: typ, UpstreamURL: upstream, ConfigJSON: j,
	}); err != nil {
		t.Fatalf("create repo: %v", err)
	}
}

// --- Go modules ---

func TestGoProxyFlow(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/@v/list"):
			io.WriteString(w, "v1.0.0\nv1.1.0\n")
		case strings.HasSuffix(r.URL.Path, ".info"):
			io.WriteString(w, `{"Version":"v1.0.0","Time":"2024-01-01T00:00:00Z"}`)
		case strings.HasSuffix(r.URL.Path, ".mod"):
			io.WriteString(w, "module example.com/foo\n")
		case strings.HasSuffix(r.URL.Path, ".zip"):
			io.WriteString(w, "ZIPDATA")
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	m, _, store := newTestManager(t)
	mkFormatRepo(t, store, "goproxy", meta.FormatGo, meta.TypeProxy, upstream.URL, repoconfig.Default())
	h := mux(m)

	for _, tc := range []struct{ path, want string }{
		{"/go/goproxy/example.com/foo/@v/list", "v1.0.0\nv1.1.0\n"},
		{"/go/goproxy/example.com/foo/@v/v1.0.0.info", `"Version":"v1.0.0"`},
		{"/go/goproxy/example.com/foo/@v/v1.0.0.mod", "module example.com/foo"},
		{"/go/goproxy/example.com/foo/@v/v1.0.0.zip", "ZIPDATA"},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), tc.want) {
			t.Fatalf("%s: code=%d body=%q", tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestGoHelpers(t *testing.T) {
	if goKind("m/@v/list") != kindMetadata || goKind("m/@v/v1.0.0.zip") != kindArtifact {
		t.Fatal("goKind misclassified")
	}
	if v := goVersion("example.com/foo/@v/v1.2.3.mod"); v != "v1.2.3" {
		t.Fatalf("goVersion = %q", v)
	}
}

// --- Cargo ---

func TestCargoConfigAndDownload(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/download") {
			io.WriteString(w, "CRATEDATA")
			return
		}
		io.WriteString(w, `{"name":"serde","vers":"1.0.0"}`)
	}))
	defer upstream.Close()

	m, _, store := newTestManager(t)
	mkFormatRepo(t, store, "crates", meta.FormatCargo, meta.TypeProxy, upstream.URL, repoconfig.Default())
	h := mux(m)

	// config.json is synthesised and points back at this repo.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cargo/crates/config.json", nil))
	var cfg map[string]string
	json.Unmarshal(rec.Body.Bytes(), &cfg)
	if !strings.Contains(cfg["dl"], "/cargo/crates/api/v1/crates/") {
		t.Fatalf("config dl = %q", cfg["dl"])
	}

	// Index entry (metadata).
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cargo/crates/se/rd/serde", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "serde") {
		t.Fatalf("index = %d %q", rec.Code, rec.Body.String())
	}

	// Download (artifact).
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cargo/crates/api/v1/crates/serde/1.0.0/download", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "CRATEDATA" {
		t.Fatalf("download = %d %q", rec.Code, rec.Body.String())
	}
}

func TestCargoHelpers(t *testing.T) {
	dl := "se/rd/serde-x/api/v1/crates/serde/1.2.3/download"
	if cargoKind(dl) != kindArtifact {
		t.Fatal("download should be artifact")
	}
	if v := cargoVersion(dl); v != "1.2.3" {
		t.Fatalf("cargoVersion = %q", v)
	}
	// The repo-relative download path arrives with its leading slash stripped
	// (resolveRepo), so "api/v1/crates/..." must classify and version-extract
	// just like the prefixed form above.
	real := "api/v1/crates/serde/1.0.197/download"
	if cargoKind(real) != kindArtifact {
		t.Fatal("leading-slash-stripped download should be artifact")
	}
	if v := cargoVersion(real); v != "1.0.197" {
		t.Fatalf("cargoVersion(real) = %q, want 1.0.197", v)
	}
	if cargoKind("se/rd/serde") != kindMetadata {
		t.Fatal("index should be metadata")
	}
}

// --- npm ---

func TestNpmProxyRewritesTarballURLs(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/-/") {
			io.WriteString(w, "TARBALL")
			return
		}
		// packument
		io.WriteString(w, `{
			"name":"left-pad",
			"dist-tags":{"latest":"1.3.0"},
			"versions":{"1.3.0":{"dist":{"tarball":"`+upstreamURL(r)+`/left-pad/-/left-pad-1.3.0.tgz"}}},
			"time":{"1.3.0":"2020-01-01T00:00:00Z"}
		}`)
	}))
	defer upstream.Close()

	m, _, store := newTestManager(t)
	mkFormatRepo(t, store, "npmproxy", meta.FormatNPM, meta.TypeProxy, upstream.URL, repoconfig.Default())
	h := mux(m)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/npm/npmproxy/left-pad", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("packument = %d", rec.Code)
	}
	var doc map[string]any
	json.Unmarshal(rec.Body.Bytes(), &doc)
	versions := doc["versions"].(map[string]any)
	v := versions["1.3.0"].(map[string]any)
	dist := v["dist"].(map[string]any)
	tarball := dist["tarball"].(string)
	if !strings.Contains(tarball, "/npm/npmproxy/left-pad/-/left-pad-1.3.0.tgz") {
		t.Fatalf("tarball not rewritten: %q", tarball)
	}

	// Tarball fetch via the rewritten path.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/npm/npmproxy/left-pad/-/left-pad-1.3.0.tgz", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "TARBALL" {
		t.Fatalf("tarball = %d %q", rec.Code, rec.Body.String())
	}
}

func TestNpmAgePolicyFiltersVersions(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{
			"name":"pkg",
			"dist-tags":{"latest":"2.0.0"},
			"versions":{
				"1.0.0":{"dist":{"tarball":"http://up/pkg/-/pkg-1.0.0.tgz"}},
				"2.0.0":{"dist":{"tarball":"http://up/pkg/-/pkg-2.0.0.tgz"}}
			},
			"time":{"1.0.0":"2024-01-01T00:00:00Z","2.0.0":"2025-06-09T00:00:00Z"}
		}`)
	}))
	defer upstream.Close()

	cfg := repoconfig.Default()
	cfg.AgePolicy = repoconfig.AgePolicyConfig{Enabled: true, MinAge: repoconfig.Duration(30 * 24 * time.Hour), Action: repoconfig.ActionBlock}
	m, eng, store := newTestManager(t)
	mkFormatRepo(t, store, "p", meta.FormatNPM, meta.TypeProxy, upstream.URL, cfg)
	eng.now = func() time.Time { return time.Date(2025, 6, 10, 0, 0, 0, 0, time.UTC) }
	h := mux(m)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/npm/p/pkg", nil))
	var doc map[string]any
	json.Unmarshal(rec.Body.Bytes(), &doc)
	versions := doc["versions"].(map[string]any)
	if _, ok := versions["2.0.0"]; ok {
		t.Fatal("2.0.0 (1 day old) should be filtered by 30d cooldown")
	}
	if _, ok := versions["1.0.0"]; !ok {
		t.Fatal("1.0.0 (old) should remain")
	}
	tags := doc["dist-tags"].(map[string]any)
	if got := tags["latest"]; got != "1.0.0" {
		t.Fatalf("latest should be remapped to the best allowed version, got %v", got)
	}
}

// TestNpmTarballAgeUsesPackumentTime guards the age-gate fix: the tarball gate
// must derive the publish time from the packument `time` map, not the CDN's
// Last-Modified header. Here the header is recent (within the cooldown) while
// the packument says the version is old, so the tarball must be served.
func TestNpmTarballAgeUsesPackumentTime(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/-/") {
			// The npm CDN bumped the tarball mtime to yesterday; a Last-Modified
			// based gate would wrongly block this under a 30d cooldown.
			w.Header().Set("Last-Modified", time.Date(2025, 6, 9, 0, 0, 0, 0, time.UTC).Format(http.TimeFormat))
			io.WriteString(w, "TARBALLBYTES")
			return
		}
		io.WriteString(w, `{
			"name":"pkg",
			"dist-tags":{"latest":"1.0.0"},
			"versions":{"1.0.0":{"dist":{"tarball":"http://up/pkg/-/pkg-1.0.0.tgz"}}},
			"time":{"1.0.0":"2024-01-01T00:00:00Z"}
		}`)
	}))
	defer upstream.Close()

	cfg := repoconfig.Default()
	cfg.AgePolicy = repoconfig.AgePolicyConfig{Enabled: true, MinAge: repoconfig.Duration(30 * 24 * time.Hour), Action: repoconfig.ActionBlock}
	m, eng, store := newTestManager(t)
	mkFormatRepo(t, store, "p", meta.FormatNPM, meta.TypeProxy, upstream.URL, cfg)
	eng.now = func() time.Time { return time.Date(2025, 6, 10, 0, 0, 0, 0, time.UTC) }
	h := mux(m)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/npm/p/pkg/-/pkg-1.0.0.tgz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("tarball should be allowed (packument time 2024-01-01 predates 30d cooldown); got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "TARBALLBYTES" {
		t.Fatalf("unexpected tarball body: %q", rec.Body.String())
	}
}

// TestNpmTarballAgeFallsBackToLastModified verifies that when the packument has
// no timestamp for the version, the gate falls back to the Last-Modified header.
func TestNpmTarballAgeFallsBackToLastModified(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/-/") {
			w.Header().Set("Last-Modified", time.Date(2025, 6, 9, 0, 0, 0, 0, time.UTC).Format(http.TimeFormat))
			io.WriteString(w, "TARBALLBYTES")
			return
		}
		// Packument omits the `time` entry for 1.0.0, forcing header fallback.
		io.WriteString(w, `{
			"name":"pkg",
			"dist-tags":{"latest":"1.0.0"},
			"versions":{"1.0.0":{"dist":{"tarball":"http://up/pkg/-/pkg-1.0.0.tgz"}}},
			"time":{}
		}`)
	}))
	defer upstream.Close()

	cfg := repoconfig.Default()
	cfg.AgePolicy = repoconfig.AgePolicyConfig{Enabled: true, MinAge: repoconfig.Duration(30 * 24 * time.Hour), Action: repoconfig.ActionBlock}
	m, eng, store := newTestManager(t)
	mkFormatRepo(t, store, "p", meta.FormatNPM, meta.TypeProxy, upstream.URL, cfg)
	eng.now = func() time.Time { return time.Date(2025, 6, 10, 0, 0, 0, 0, time.UTC) }
	h := mux(m)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/npm/p/pkg/-/pkg-1.0.0.tgz", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("tarball should be blocked via Last-Modified fallback (1 day < 30d); got %d", rec.Code)
	}
}

func TestHighestStableVersion(t *testing.T) {
	cases := []struct {
		versions map[string]any
		want     string
	}{
		{map[string]any{"1.0.0": nil, "2.1.0": nil, "2.0.5": nil}, "2.1.0"},
		{map[string]any{"1.0.0": nil, "2.0.0-beta.1": nil}, "1.0.0"},
		{map[string]any{"2.0.0-beta.1": nil, "weird": nil}, ""},
		{map[string]any{"v3.0.0": nil, "2.9.9": nil}, "v3.0.0"},
		{map[string]any{"0.0.10": nil, "0.0.9": nil}, "0.0.10"},
	}
	for _, tc := range cases {
		if got := highestStableVersion(tc.versions); got != tc.want {
			t.Errorf("highestStableVersion(%v) = %q, want %q", tc.versions, got, tc.want)
		}
	}
}

func TestNpmPublishAndInstall(t *testing.T) {
	m, _, store := newTestManager(t)
	mkFormatRepo(t, store, "local", meta.FormatNPM, meta.TypeHosted, "", repoconfig.Default())
	h := mux(m)

	tarball := base64.StdEncoding.EncodeToString([]byte("TGZBYTES"))
	publishDoc := `{
		"name":"mylib",
		"versions":{"1.0.0":{"dist":{"tarball":"http://x/mylib/-/mylib-1.0.0.tgz"}}},
		"_attachments":{"mylib-1.0.0.tgz":{"data":"` + tarball + `"}}
	}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/npm/local/mylib", strings.NewReader(publishDoc)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("publish = %d", rec.Code)
	}

	// Packument is retrievable and attachments are stripped.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/npm/local/mylib", nil))
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "_attachments") {
		t.Fatalf("packument = %d body=%q", rec.Code, rec.Body.String())
	}

	// Tarball is retrievable.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/npm/local/mylib/-/mylib-1.0.0.tgz", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "TGZBYTES" {
		t.Fatalf("tarball = %d %q", rec.Code, rec.Body.String())
	}
	repo, err := store.GetRepositoryByName(t.Context(), "local")
	if err != nil {
		t.Fatal(err)
	}
	art, err := store.GetArtifact(t.Context(), repo.ID, "mylib/-/mylib-1.0.0.tgz")
	if err != nil {
		t.Fatal(err)
	}
	if art.Version != "1.0.0" {
		t.Fatalf("stored tarball version = %q, want 1.0.0", art.Version)
	}
}

// TestNpmScopedPublishInstallEncoding guards the scope-slash decoding fix: npm
// publish PUTs a scoped package with a lowercase %2f, while pnpm fetches it with
// an uppercase %2F (or a literal slash). All spellings must resolve to the one
// artifact. Before the fix the raw request path was the storage key, so publish
// and fetch missed each other on encoding alone and hosted GETs 404'd.
func TestNpmScopedPublishInstallEncoding(t *testing.T) {
	m, _, store := newTestManager(t)
	mkFormatRepo(t, store, "local", meta.FormatNPM, meta.TypeHosted, "", repoconfig.Default())
	h := mux(m)

	tarball := base64.StdEncoding.EncodeToString([]byte("SCOPEDTGZ"))
	publishDoc := `{
		"name":"@scope/name",
		"versions":{"1.0.0":{"dist":{"tarball":"http://x/@scope/name/-/name-1.0.0.tgz"}}},
		"_attachments":{"name-1.0.0.tgz":{"data":"` + tarball + `"}}
	}`
	// npm publish encodes the scope separator as lowercase %2f.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/npm/local/@scope%2fname", strings.NewReader(publishDoc)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("publish = %d", rec.Code)
	}

	// The packument resolves via every scope spelling clients send: lowercase
	// %2f (npm), uppercase %2F (pnpm), a literal slash, and a fully-encoded @.
	for _, p := range []string{"@scope%2fname", "@scope%2Fname", "@scope/name", "%40scope%2fname"} {
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/npm/local/"+p, nil))
		if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), "_attachments") {
			t.Fatalf("packument %q = %d body=%q", p, rec.Code, rec.Body.String())
		}
	}

	// HEAD resolves the same way (npm/pnpm probe with it before download).
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodHead, "/npm/local/@scope%2Fname", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("HEAD packument = %d", rec.Code)
	}

	// The tarball resolves whether the scope slash is literal or encoded.
	for _, p := range []string{"@scope/name/-/name-1.0.0.tgz", "@scope%2fname/-/name-1.0.0.tgz"} {
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/npm/local/"+p, nil))
		if rec.Code != http.StatusOK || rec.Body.String() != "SCOPEDTGZ" {
			t.Fatalf("tarball %q = %d %q", p, rec.Code, rec.Body.String())
		}
	}

	// Both are stored under the decoded canonical key, not the encoded path.
	repo, err := store.GetRepositoryByName(t.Context(), "local")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetArtifact(t.Context(), repo.ID, "@scope/name"); err != nil {
		t.Fatalf("packument not stored under decoded key: %v", err)
	}
	if _, err := store.GetArtifact(t.Context(), repo.ID, "@scope/name/-/name-1.0.0.tgz"); err != nil {
		t.Fatalf("tarball not stored under decoded key: %v", err)
	}
}

// TestNpmEncodedTraversalRejected verifies the decoded-path traversal recheck:
// a percent-encoded "../" (%2e%2e%2f) slips past resolve's raw-path check but
// must be rejected once decoded, on both fetch and publish.
func TestNpmEncodedTraversalRejected(t *testing.T) {
	m, _, store := newTestManager(t)
	mkFormatRepo(t, store, "local", meta.FormatNPM, meta.TypeHosted, "", repoconfig.Default())
	h := mux(m)

	for _, tc := range []struct {
		method, target string
	}{
		{http.MethodGet, "/npm/local/%2e%2e%2fetc%2fpasswd"},
		{http.MethodGet, "/npm/local/@scope%2f%2e%2e%2f%2e%2e"},
		{http.MethodPut, "/npm/local/%2e%2e%2fevil"},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.target, strings.NewReader("{}")))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s %s = %d, want 400", tc.method, tc.target, rec.Code)
		}
	}
}

// TestNpmProxyScopedEncoded verifies a scoped package installed through a proxy
// repo: pnpm requests it with an encoded scope slash, and the cache keys on the
// decoded identity so the second request is a hit rather than a duplicate fetch.
func TestNpmProxyScopedEncoded(t *testing.T) {
	var hits int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		io.WriteString(w, `{"name":"@scope/name","versions":{"1.0.0":{"dist":{"tarball":"http://up/@scope/name/-/name-1.0.0.tgz"}}},"time":{"1.0.0":"2020-01-01T00:00:00Z"}}`)
	}))
	defer upstream.Close()

	m, _, store := newTestManager(t)
	mkFormatRepo(t, store, "np", meta.FormatNPM, meta.TypeProxy, upstream.URL, repoconfig.Default())
	h := mux(m)

	// First request (encoded) misses and fetches upstream; second (uppercase
	// encoding) must be served from the cache under the same decoded key.
	for i, p := range []string{"@scope%2fname", "@scope%2Fname"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/npm/np/"+p, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("iter %d (%q) = %d", i, p, rec.Code)
		}
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("upstream packument hits = %d, want 1 (cached under decoded key)", got)
	}

	repo, err := store.GetRepositoryByName(t.Context(), "np")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetArtifact(t.Context(), repo.ID, "@scope/name"); err != nil {
		t.Fatalf("proxy packument not cached under decoded key: %v", err)
	}
}

// TestNpmGroupRewritesHostedTarballURL guards the hosted-packument rewrite: a
// scoped package published to a hosted member carries an absolute dist.tarball
// pointing at the publish registry, sometimes with a malformed scoped filename
// (as npm/pnpm emit). Served through a group, that URL must be rewritten to the
// group's own path with a filename derived from the stored tarball, so a client
// with only the group's credential can install it in one hop.
func TestNpmGroupRewritesHostedTarballURL(t *testing.T) {
	m, _, store := newTestManager(t)
	mkFormatRepo(t, store, "npm-hosted", meta.FormatNPM, meta.TypeHosted, "", repoconfig.Default())
	groupCfg := repoconfig.Default()
	groupCfg.Group.Members = []string{"npm-hosted"}
	mkFormatRepo(t, store, "npm-public", meta.FormatNPM, meta.TypeGroup, "", groupCfg)
	h := mux(m)

	tarball := base64.StdEncoding.EncodeToString([]byte("SCOPEDTGZ"))
	// The publish doc mirrors pnpm: an absolute tarball URL at the hosted
	// registry, with the scope duplicated into the filename.
	publishDoc := `{
		"name":"@scope/name",
		"versions":{"1.0.0":{"dist":{"tarball":"http://localhost:8080/npm/npm-hosted/@scope/name/-/@scope/name-1.0.0.tgz"}}},
		"_attachments":{"name-1.0.0.tgz":{"data":"` + tarball + `"}}
	}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/npm/npm-hosted/@scope%2fname", strings.NewReader(publishDoc)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("publish = %d", rec.Code)
	}

	// Fetch the packument through the group and read back the rewritten tarball.
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/npm/npm-public/@scope%2Fname", nil)
	req.Host = "reg.example"
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("group packument = %d", rec.Code)
	}
	var doc struct {
		Versions map[string]struct {
			Dist struct {
				Tarball string `json:"tarball"`
			} `json:"dist"`
		} `json:"versions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatalf("packument json: %v", err)
	}
	got := doc.Versions["1.0.0"].Dist.Tarball
	want := "http://reg.example/npm/npm-public/@scope/name/-/name-1.0.0.tgz"
	if got != want {
		t.Fatalf("rewritten tarball = %q, want %q", got, want)
	}

	// That rewritten URL must actually serve the tarball through the group.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/npm/npm-public/@scope/name/-/name-1.0.0.tgz", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "SCOPEDTGZ" {
		t.Fatalf("group tarball = %d %q", rec.Code, rec.Body.String())
	}
}

// upstreamURL reconstructs the test server base from a request.
func upstreamURL(r *http.Request) string {
	return "http://" + r.Host
}

func TestNpmProxyCachedAndMissing(t *testing.T) {
	var hits int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "missing") {
			http.NotFound(w, r)
			return
		}
		atomic.AddInt32(&hits, 1)
		io.WriteString(w, `{"name":"p","versions":{"1.0.0":{"dist":{"tarball":"http://up/p/-/p-1.0.0.tgz"}}},"time":{"1.0.0":"2020-01-01T00:00:00Z"}}`)
	}))
	defer upstream.Close()

	m, _, store := newTestManager(t)
	mkFormatRepo(t, store, "np", meta.FormatNPM, meta.TypeProxy, upstream.URL, repoconfig.Default())
	h := mux(m)

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/npm/np/p", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("iter %d = %d", i, rec.Code)
		}
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("packument hits = %d, want 1 (cached)", hits)
	}

	// Missing packument -> 404.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/npm/np/missing", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing packument = %d", rec.Code)
	}
}

func TestNpmLocalMissing(t *testing.T) {
	m, _, store := newTestManager(t)
	mkFormatRepo(t, store, "nl", meta.FormatNPM, meta.TypeHosted, "", repoconfig.Default())
	rec := httptest.NewRecorder()
	mux(m).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/npm/nl/nopkg", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("local missing = %d", rec.Code)
	}
}

func TestCargoLocalPublish(t *testing.T) {
	m, _, store := newTestManager(t)
	mkFormatRepo(t, store, "cl", meta.FormatCargo, meta.TypeHosted, "", repoconfig.Default())
	h := mux(m)
	path := "/cargo/cl/api/v1/crates/mylib/1.0.0/download"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, path, strings.NewReader("CRATE")))
	if rec.Code != http.StatusCreated {
		t.Fatalf("publish = %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "CRATE" {
		t.Fatalf("download = %d %q", rec.Code, rec.Body.String())
	}
}

func TestGoLocalPut(t *testing.T) {
	m, _, store := newTestManager(t)
	mkFormatRepo(t, store, "gl", meta.FormatGo, meta.TypeHosted, "", repoconfig.Default())
	h := mux(m)
	path := "/go/gl/example.com/m/@v/v1.0.0.zip"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, path, strings.NewReader("Z")))
	if rec.Code != http.StatusCreated {
		t.Fatalf("put = %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "Z" {
		t.Fatalf("get = %d %q", rec.Code, rec.Body.String())
	}
}
