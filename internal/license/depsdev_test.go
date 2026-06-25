package license

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDepsDevResolve(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"versionKey":{"system":"NPM","name":"lodash","version":"4.17.21"},"licenses":["MIT","MIT"]}`))
	}))
	defer srv.Close()

	r := NewDepsDev(srv.URL, srv.Client())
	res, err := r.Resolve(t.Context(), "npm", "lodash", "4.17.21")
	if err != nil {
		t.Fatal(err)
	}
	// Deduplicated to a single MIT.
	if len(res.Licenses) != 1 || res.Licenses[0] != "MIT" {
		t.Fatalf("licenses = %v, want [MIT]", res.Licenses)
	}
	if want := "/v3/systems/npm/packages/lodash/versions/4.17.21"; gotPath != want {
		t.Fatalf("path = %q, want %q", gotPath, want)
	}
}

func TestDepsDevResolveEncodesCoordinates(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		_, _ = w.Write([]byte(`{"licenses":["Apache-2.0"]}`))
	}))
	defer srv.Close()

	r := NewDepsDev(srv.URL, srv.Client())
	// Maven "group:artifact" and a Go-style module path must be percent-encoded,
	// including ":" and "/", so deps.dev sees one path segment per coordinate part.
	if _, err := r.Resolve(t.Context(), "maven", "com.google.guava:guava", "32.0.0"); err != nil {
		t.Fatal(err)
	}
	if want := "/v3/systems/maven/packages/com.google.guava%3Aguava/versions/32.0.0"; gotPath != want {
		t.Fatalf("path = %q, want %q", gotPath, want)
	}
}

func TestDepsDevResolveNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	r := NewDepsDev(srv.URL, srv.Client())
	res, err := r.Resolve(t.Context(), "npm", "ghost", "9.9.9")
	if err != nil {
		t.Fatalf("404 should be a clean empty result, not an error: %v", err)
	}
	if len(res.Licenses) != 0 {
		t.Fatalf("licenses = %v, want empty", res.Licenses)
	}
}
