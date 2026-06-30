// Package repo serves the package-format protocols (Maven, npm, Cargo, Go,
// PyPI) over Hosted and Proxy (cached upstream) repositories. The
// Engine holds the shared cache/store logic; per-format files translate
// protocol requests into Engine operations.
package repo

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/younsl/o/box/kubernetes/forklift/internal/meta"
	"github.com/younsl/o/box/kubernetes/forklift/internal/repoconfig"
	"github.com/younsl/o/box/kubernetes/forklift/internal/storage"
	"github.com/younsl/o/box/kubernetes/forklift/internal/version"
)

const (
	// defaultUpstreamCooldown is how long an upstream coordinate is shielded from
	// re-fetching after a 429/503 with no usable Retry-After, so client retries
	// are answered locally instead of forwarded into a rate-limit storm.
	defaultUpstreamCooldown = 15 * time.Second
	// maxUpstreamCooldown caps a Retry-After-derived cooldown so a hostile or
	// buggy upstream cannot park a coordinate for an unbounded time.
	maxUpstreamCooldown = 5 * time.Minute
)

// kind classifies a request target, which selects the cache freshness policy.
type kind int

const (
	kindArtifact kind = iota // immutable artifact bytes
	kindMetadata             // mutable index documents (revalidated on MetadataTTL)
)

// Engine implements the shared repository cache/store logic.
type Engine struct {
	store  *meta.Store
	blobs  storage.BlobStore
	client *http.Client
	// extClient fetches client-supplied URLs (fetchSpec.untrustedURL); its
	// dialer refuses private/loopback/link-local destinations to prevent SSRF.
	extClient *http.Client
	log       *slog.Logger
	neg       *negCache
	// cool shields a coordinate from re-fetching for a short window after the
	// upstream returned 429/503, keyed like neg by repo-relative path.
	cool *negCache
	// flight collapses concurrent identical proxy fetches into one upstream
	// round-trip so a cold cache cannot fan a burst of identical requests out
	// into a matching burst of upstream requests.
	flight    *flight
	userAgent string
	now       func() time.Time
	// onStore, when set, is invoked after a proxy fetch caches an artifact, so
	// the manager can enqueue vulnerability/license scanning for it. Best-effort
	// and must not block the serving path; nil disables it.
	onStore func(repo meta.Repository, path string)

	cacheHits   *prometheus.CounterVec
	cacheMiss   *prometheus.CounterVec
	ageBlocks   *prometheus.CounterVec
	upstreamErr *prometheus.CounterVec
	bytes       *prometheus.CounterVec
}

// NewEngine builds an Engine and registers its metrics.
func NewEngine(store *meta.Store, blobs storage.BlobStore, log *slog.Logger, reg prometheus.Registerer) *Engine {
	e := &Engine{
		store:     store,
		blobs:     blobs,
		client:    &http.Client{Timeout: 60 * time.Second},
		extClient: newPublicOnlyClient(60 * time.Second),
		log:       log,
		neg:       newNegCache(),
		cool:      newNegCache(),
		flight:    newFlight(),
		userAgent: "forklift/" + version.Version,
		now:       time.Now,
		cacheHits: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "forklift", Name: "cache_hits_total", Help: "Proxy cache hits.",
		}, []string{"repo"}),
		cacheMiss: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "forklift", Name: "cache_misses_total", Help: "Proxy cache misses.",
		}, []string{"repo"}),
		ageBlocks: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "forklift", Name: "age_policy_violations_total", Help: "Age policy violations.",
		}, []string{"repo", "action"}),
		upstreamErr: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "forklift", Name: "upstream_errors_total", Help: "Upstream fetch errors.",
		}, []string{"repo"}),
		bytes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "forklift", Name: "bytes_transferred_total",
			Help: "Artifact bytes transferred to/from clients (egress=downloads, ingress=uploads).",
		}, []string{"direction", "format"}),
	}
	reg.MustRegister(e.cacheHits, e.cacheMiss, e.ageBlocks, e.upstreamErr, e.bytes)
	return e
}

// fetchSpec parameterises a GET/HEAD against the engine for one request.
type fetchSpec struct {
	repo        meta.Repository
	cfg         repoconfig.Config
	path        string // repo-relative storage key
	upstreamURL string // full upstream URL (proxy only)
	// untrustedURL marks upstreamURL as client-supplied (not derived from the
	// admin-configured upstream); it is fetched via the SSRF-guarded client.
	untrustedURL bool
	kind         kind
	version      string
	contentType  string
	// extractPublished derives the upstream release time from a proxy response.
	extractPublished func(resp *http.Response) *time.Time
}

// serve handles a GET or HEAD for a repository path.
func (e *Engine) serve(w http.ResponseWriter, r *http.Request, spec fetchSpec) {
	ctx := r.Context()
	key := spec.repo.Name + "/" + spec.path

	if art, err := e.store.GetArtifact(ctx, spec.repo.ID, spec.path); err == nil {
		// Hosted repositories are authoritative and always serve stored artifacts;
		// proxy repositories serve from cache only while the entry is fresh.
		if spec.repo.Type == meta.TypeHosted || e.fresh(art, spec.cfg, spec.kind) {
			if e.ageGate(w, spec, art.PublishedAt) {
				return
			}
			if spec.repo.Type == meta.TypeProxy {
				e.cacheHits.WithLabelValues(spec.repo.Name).Inc()
			}
			_ = e.store.Touch(ctx, spec.repo.ID, spec.path)
			n := e.serveArtifact(w, r, art)
			e.bytes.WithLabelValues("egress", spec.repo.Format).Add(float64(n))
			return
		}
	} else if !errors.Is(err, meta.ErrNotFound) {
		http.Error(w, "metadata error", http.StatusInternalServerError)
		return
	}

	if spec.repo.Type == meta.TypeHosted {
		http.NotFound(w, r)
		return
	}

	// Proxy path.
	if e.neg.has(key) {
		http.NotFound(w, r)
		return
	}
	e.cacheMiss.WithLabelValues(spec.repo.Name).Inc()
	e.fetchAndServe(w, r, spec, key)
}

// fetchKind classifies the result of a coalesced upstream fetch so each waiting
// caller can render its own response.
type fetchKind int

const (
	fetchStored     fetchKind = iota // artifact is now cached; serve it from the store
	fetchNotFound                    // upstream 404 (negative-cached)
	fetchAgeBlocked                  // age policy blocked the artifact
	fetchRetry                       // upstream 429/503; cooled down, ask client to retry
	fetchError                       // any other failure
)

// fetchOutcome is the shared result of one coalesced fetch.
type fetchOutcome struct {
	kind       fetchKind
	status     int    // for fetchRetry: 429 or 503 to relay to clients
	retryAfter string // for fetchRetry: seconds, set as the Retry-After header
}

func (e *Engine) fetchAndServe(w http.ResponseWriter, r *http.Request, spec fetchSpec, key string) {
	// Recently rate-limited: answer locally so client retries do not re-enter the
	// upstream storm. Relay a Retry-After so the build tool backs off correctly.
	if d, ok := e.cool.remaining(key); ok {
		e.writeRetry(w, http.StatusServiceUnavailable, retryAfterSeconds(d))
		return
	}

	// Pass-through repositories stream the body straight to the client and cannot
	// be coalesced (each caller needs its own stream), so they take a direct path.
	if !spec.cfg.Cache.Enabled {
		e.fetchPassthrough(w, r, spec, key)
		return
	}

	// Collapse concurrent identical fetches into one upstream round-trip. The
	// shared fetch is detached from any single client's cancellation so a waiter
	// disconnecting cannot abort the fetch the others depend on.
	outcome := e.flight.do(key, func() fetchOutcome {
		return e.fetchAndStore(context.WithoutCancel(r.Context()), spec, key)
	})

	switch outcome.kind {
	case fetchStored:
		art, err := e.store.GetArtifact(r.Context(), spec.repo.ID, spec.path)
		if err != nil {
			http.Error(w, "cache read failed", http.StatusInternalServerError)
			return
		}
		n := e.serveArtifact(w, r, art)
		e.bytes.WithLabelValues("egress", spec.repo.Format).Add(float64(n))
	case fetchNotFound:
		http.NotFound(w, r)
	case fetchAgeBlocked:
		http.Error(w, "blocked by age policy", http.StatusNotFound)
	case fetchRetry:
		e.writeRetry(w, outcome.status, outcome.retryAfter)
	default:
		http.Error(w, "upstream error", http.StatusBadGateway)
	}
}

// fetchAndStore performs the upstream GET and caches the result. It writes no
// HTTP response; it records side effects (cached artifact, negative entry, or
// cooldown) and returns an outcome each caller renders independently. ctx must
// be detached from individual client cancellation.
func (e *Engine) fetchAndStore(ctx context.Context, spec fetchSpec, key string) fetchOutcome {
	// A concurrent caller we were coalesced behind may have just populated the
	// cache; serve that instead of re-fetching.
	if art, err := e.store.GetArtifact(ctx, spec.repo.ID, spec.path); err == nil && e.fresh(art, spec.cfg, spec.kind) {
		return fetchOutcome{kind: fetchStored}
	}

	resp, err := e.upstreamGet(ctx, spec)
	if err != nil {
		e.upstreamErr.WithLabelValues(spec.repo.Name).Inc()
		e.log.Error("upstream fetch failed", "repo", spec.repo.Name, "url", spec.upstreamURL, "err", err)
		return fetchOutcome{kind: fetchError}
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		e.neg.set(key, spec.cfg.Cache.NegativeTTL.D())
		return fetchOutcome{kind: fetchNotFound}
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable:
		d := parseRetryAfter(resp.Header.Get("Retry-After"), e.now())
		e.cool.set(key, d)
		e.upstreamErr.WithLabelValues(spec.repo.Name).Inc()
		e.log.Warn("upstream rate-limited; cooling down",
			"repo", spec.repo.Name, "url", spec.upstreamURL, "status", resp.StatusCode, "cooldown", d.String())
		return fetchOutcome{kind: fetchRetry, status: resp.StatusCode, retryAfter: retryAfterSeconds(d)}
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		e.upstreamErr.WithLabelValues(spec.repo.Name).Inc()
		e.log.Error("upstream non-2xx", "repo", spec.repo.Name, "url", spec.upstreamURL, "status", resp.StatusCode)
		return fetchOutcome{kind: fetchError}
	}

	var published *time.Time
	if spec.extractPublished != nil {
		published = spec.extractPublished(resp)
	}
	if blocked := e.evalAge(spec, published); blocked {
		return fetchOutcome{kind: fetchAgeBlocked}
	}

	contentType := spec.contentType
	if contentType == "" {
		contentType = resp.Header.Get("Content-Type")
	}
	if _, err := e.storeArtifact(ctx, spec, resp.Body, contentType, published); err != nil {
		e.log.Error("cache write failed", "repo", spec.repo.Name, "path", spec.path, "err", err)
		return fetchOutcome{kind: fetchError}
	}
	e.maybeEvict(ctx, spec)
	if e.onStore != nil {
		e.onStore(spec.repo, spec.path)
	}
	return fetchOutcome{kind: fetchStored}
}

// fetchPassthrough serves a cache-disabled proxy repository, streaming the
// upstream body straight to the client without persisting it.
func (e *Engine) fetchPassthrough(w http.ResponseWriter, r *http.Request, spec fetchSpec, key string) {
	resp, err := e.upstreamGet(r.Context(), spec)
	if err != nil {
		e.upstreamErr.WithLabelValues(spec.repo.Name).Inc()
		e.log.Error("upstream fetch failed", "repo", spec.repo.Name, "url", spec.upstreamURL, "err", err)
		http.Error(w, "upstream unreachable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusNotFound:
		http.NotFound(w, r)
		return
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable:
		d := parseRetryAfter(resp.Header.Get("Retry-After"), e.now())
		e.cool.set(key, d)
		e.upstreamErr.WithLabelValues(spec.repo.Name).Inc()
		e.log.Warn("upstream rate-limited; cooling down",
			"repo", spec.repo.Name, "url", spec.upstreamURL, "status", resp.StatusCode, "cooldown", d.String())
		e.writeRetry(w, resp.StatusCode, retryAfterSeconds(d))
		return
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		e.upstreamErr.WithLabelValues(spec.repo.Name).Inc()
		e.log.Error("upstream non-2xx", "repo", spec.repo.Name, "url", spec.upstreamURL, "status", resp.StatusCode)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}

	var published *time.Time
	if spec.extractPublished != nil {
		published = spec.extractPublished(resp)
	}
	if e.ageGate(w, spec, published) {
		return
	}
	contentType := spec.contentType
	if contentType == "" {
		contentType = resp.Header.Get("Content-Type")
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	n, _ := io.Copy(w, resp.Body)
	e.bytes.WithLabelValues("egress", spec.repo.Format).Add(float64(n))
}

// upstreamGet issues the upstream GET with a descriptive User-Agent. Public
// registries (notably Maven Central) throttle the default Go user-agent harder,
// so identifying forklift materially reduces 429s.
func (e *Engine) upstreamGet(ctx context.Context, spec fetchSpec) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, spec.upstreamURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", e.userAgent)
	client := e.client
	if spec.untrustedURL {
		client = e.extClient
	}
	return client.Do(req)
}

// evalAge applies the age policy without writing a response, returning true when
// the artifact must be blocked. Counters and logs mirror ageGate.
func (e *Engine) evalAge(spec fetchSpec, published *time.Time) bool {
	decision, reason := evaluateAge(spec.cfg.AgePolicy, published, e.now())
	switch decision {
	case ageBlock:
		e.ageBlocks.WithLabelValues(spec.repo.Name, "block").Inc()
		e.log.Warn("age policy blocked artifact", "repo", spec.repo.Name, "path", spec.path, "reason", reason)
		return true
	case ageWarn:
		e.ageBlocks.WithLabelValues(spec.repo.Name, "warn").Inc()
		e.log.Warn("age policy warning", "repo", spec.repo.Name, "path", spec.path, "reason", reason)
		return false
	default:
		return false
	}
}

// writeRetry tells the client to back off, relaying the upstream's status
// (429/503) and a Retry-After hint so build tools wait instead of hammering.
func (e *Engine) writeRetry(w http.ResponseWriter, status int, retryAfter string) {
	if status != http.StatusTooManyRequests && status != http.StatusServiceUnavailable {
		status = http.StatusServiceUnavailable
	}
	if retryAfter != "" {
		w.Header().Set("Retry-After", retryAfter)
	}
	http.Error(w, "upstream rate-limited, retry later", status)
}

// parseRetryAfter interprets an HTTP Retry-After header (delta-seconds or an
// HTTP date), clamped to [defaultUpstreamCooldown, maxUpstreamCooldown]. A
// missing or unparseable value falls back to the default cooldown.
func parseRetryAfter(h string, now time.Time) time.Duration {
	d := defaultUpstreamCooldown
	if h != "" {
		if secs, err := strconv.Atoi(h); err == nil {
			d = time.Duration(secs) * time.Second
		} else if t, err := http.ParseTime(h); err == nil {
			d = t.Sub(now)
		}
	}
	if d < defaultUpstreamCooldown {
		d = defaultUpstreamCooldown
	}
	if d > maxUpstreamCooldown {
		d = maxUpstreamCooldown
	}
	return d
}

// retryAfterSeconds renders a cooldown as a whole-seconds Retry-After value.
func retryAfterSeconds(d time.Duration) string {
	secs := int(d.Round(time.Second) / time.Second)
	if secs < 1 {
		secs = 1
	}
	return strconv.Itoa(secs)
}

// storeArtifact streams body into the blob store and records the artifact.
func (e *Engine) storeArtifact(ctx context.Context, spec fetchSpec, body io.Reader, contentType string, published *time.Time) (meta.Artifact, error) {
	digest, size, err := e.blobs.Put(ctx, body)
	if err != nil {
		return meta.Artifact{}, err
	}
	now := e.now()
	return e.store.PutArtifact(ctx, meta.Artifact{
		RepoID:         spec.repo.ID,
		Path:           spec.path,
		Version:        spec.version,
		BlobSHA256:     digest,
		Size:           size,
		ContentType:    contentType,
		PublishedAt:    published,
		CachedAt:       now,
		LastAccessedAt: now,
	})
}

// put stores an uploaded artifact for a hosted repository.
func (e *Engine) put(ctx context.Context, repo meta.Repository, path, version, contentType string, published *time.Time, body io.Reader) error {
	art, err := e.storeArtifact(ctx, fetchSpec{
		repo: repo, path: path, version: version, contentType: contentType,
	}, body, contentType, published)
	e.neg.clear(repo.Name + "/" + path)
	if err == nil {
		e.bytes.WithLabelValues("ingress", repo.Format).Add(float64(art.Size))
	}
	return err
}

// serveArtifact writes the artifact body and returns the number of bytes copied
// to the client (0 for HEAD or a 304 response).
func (e *Engine) serveArtifact(w http.ResponseWriter, r *http.Request, art meta.Artifact) int64 {
	rc, size, err := e.blobs.Open(r.Context(), art.BlobSHA256)
	if err != nil {
		http.Error(w, "blob missing", http.StatusInternalServerError)
		return 0
	}
	defer rc.Close()
	if art.ContentType != "" {
		w.Header().Set("Content-Type", art.ContentType)
	}
	w.Header().Set("ETag", `"`+art.BlobSHA256+`"`)
	if match := r.Header.Get("If-None-Match"); match != "" && match == `"`+art.BlobSHA256+`"` {
		w.WriteHeader(http.StatusNotModified)
		return 0
	}
	w.Header().Set("Content-Length", itoa(size))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return 0
	}
	n, _ := io.Copy(w, rc)
	return n
}

// ageGate evaluates the age policy and, when blocking, writes a 404 and returns
// true so the caller stops. Warnings are logged and counted but allowed.
func (e *Engine) ageGate(w http.ResponseWriter, spec fetchSpec, published *time.Time) bool {
	decision, reason := evaluateAge(spec.cfg.AgePolicy, published, e.now())
	switch decision {
	case ageBlock:
		e.ageBlocks.WithLabelValues(spec.repo.Name, "block").Inc()
		e.log.Warn("age policy blocked artifact",
			"repo", spec.repo.Name, "path", spec.path, "reason", reason)
		http.Error(w, "blocked by age policy", http.StatusNotFound)
		return true
	case ageWarn:
		e.ageBlocks.WithLabelValues(spec.repo.Name, "warn").Inc()
		e.log.Warn("age policy warning",
			"repo", spec.repo.Name, "path", spec.path, "reason", reason)
		return false
	default:
		return false
	}
}

// fresh reports whether a cached artifact is still fresh under the cache policy.
func (e *Engine) fresh(art meta.Artifact, cfg repoconfig.Config, k kind) bool {
	if !cfg.Cache.Enabled {
		return false
	}
	var ttl time.Duration
	switch k {
	case kindMetadata:
		ttl = cfg.Cache.MetadataTTL.D()
	default:
		ttl = cfg.Cache.ArtifactTTL.D()
	}
	if ttl <= 0 {
		// Artifacts are immutable (ttl 0 = never revalidate); metadata with no TTL
		// is treated as always-revalidate to avoid serving stale indexes.
		return k == kindArtifact
	}
	return e.now().Sub(art.CachedAt) < ttl
}

// maybeEvict trims the repository cache to its configured size cap.
func (e *Engine) maybeEvict(ctx context.Context, spec fetchSpec) {
	max := spec.cfg.Cache.MaxSizeBytes
	if max <= 0 {
		return
	}
	size, err := e.store.RepoSize(ctx, spec.repo.ID)
	if err != nil || size <= max {
		return
	}
	// Evict in small batches until under the cap (bounded to avoid long loops).
	for i := 0; i < 64; i++ {
		if n, err := e.store.EvictLRU(ctx, spec.repo.ID, 16); err != nil || n == 0 {
			break
		}
		if size, err := e.store.RepoSize(ctx, spec.repo.ID); err != nil || size <= max {
			break
		}
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
