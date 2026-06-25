package license

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DepsDevResolver resolves licenses via the deps.dev API (https://deps.dev),
// which exposes per-version license metadata for every package system forklift
// proxies (npm, Maven, Cargo, Go, PyPI) behind one schema. The endpoint is
// operator-configured and trusted, so a plain client is used; the caller may
// pass an SSRF-guarded client for hardened deployments.
type DepsDevResolver struct {
	url    string
	client *http.Client
}

// NewDepsDev builds a resolver against baseURL (e.g. https://api.deps.dev). A
// nil client gets a default with a short timeout.
func NewDepsDev(baseURL string, client *http.Client) *DepsDevResolver {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &DepsDevResolver{url: strings.TrimRight(baseURL, "/"), client: client}
}

// Source names the data source, recorded on each resolution it produces.
func (d *DepsDevResolver) Source() string { return "deps.dev" }

// depsDevVersion is the subset of the deps.dev GetVersion response we use.
type depsDevVersion struct {
	Licenses []string `json:"licenses"`
}

// Resolve asks deps.dev for the licenses declared by the exact version. system
// is the deps.dev system name (npm, maven, cargo, go, pypi), pkg the package
// name (Maven uses "group:artifact"), and version the exact version string.
func (d *DepsDevResolver) Resolve(ctx context.Context, system, pkg, version string) (Result, error) {
	if system == "" || pkg == "" || version == "" {
		return Result{}, nil
	}
	url := fmt.Sprintf("%s/v3/systems/%s/packages/%s/versions/%s",
		d.url, escapeSegment(system), escapeSegment(pkg), escapeSegment(version))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{}, err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		// Unknown coordinate to deps.dev: resolved, but no license data.
		return Result{}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("deps.dev query: status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return Result{}, err
	}
	var doc depsDevVersion
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Result{}, err
	}
	out := Result{}
	seen := map[string]bool{}
	for _, l := range doc.Licenses {
		l = strings.TrimSpace(l)
		if l == "" || seen[l] {
			continue
		}
		seen[l] = true
		out.Licenses = append(out.Licenses, l)
	}
	return out, nil
}

// escapeSegment percent-encodes a URL path segment per RFC 3986, encoding every
// character outside the unreserved set. Unlike url.PathEscape it also encodes
// "/" and ":", which appear in Go module paths and Maven coordinates and must
// not be read as path separators by deps.dev.
func escapeSegment(s string) string {
	const upperhex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(upperhex[c>>4])
		b.WriteByte(upperhex[c&0xf])
	}
	return b.String()
}
