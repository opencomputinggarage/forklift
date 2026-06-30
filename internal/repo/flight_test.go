package repo

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFlightCoalescesConcurrentCalls(t *testing.T) {
	f := newFlight()
	var calls atomic.Int64
	release := make(chan struct{})

	const waiters = 20
	var wg sync.WaitGroup
	wg.Add(waiters)
	for i := 0; i < waiters; i++ {
		go func() {
			defer wg.Done()
			f.do("same-key", func() fetchOutcome {
				calls.Add(1)
				<-release // hold the in-flight call so the rest coalesce behind it
				return fetchOutcome{kind: fetchStored}
			})
		}()
	}

	// Give the goroutines time to pile up behind the first call, then release.
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("coalesced fn ran %d times, want 1", got)
	}
}

func TestFlightDistinctKeysDoNotCoalesce(t *testing.T) {
	f := newFlight()
	var calls atomic.Int64
	var wg sync.WaitGroup
	for _, k := range []string{"a", "b", "c"} {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			f.do(key, func() fetchOutcome {
				calls.Add(1)
				return fetchOutcome{kind: fetchStored}
			})
		}(k)
	}
	wg.Wait()
	if got := calls.Load(); got != 3 {
		t.Fatalf("distinct keys ran fn %d times, want 3", got)
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		hdr  string
		want time.Duration
	}{
		{"empty falls back to default", "", defaultUpstreamCooldown},
		{"small seconds clamped up to default", "2", defaultUpstreamCooldown},
		{"seconds honored", "60", 60 * time.Second},
		{"capped at max", "100000", maxUpstreamCooldown},
		{"garbage falls back to default", "soon", defaultUpstreamCooldown},
		{"http-date honored", now.Add(90 * time.Second).UTC().Format(http.TimeFormat), 90 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseRetryAfter(tc.hdr, now); got != tc.want {
				t.Fatalf("parseRetryAfter(%q) = %v, want %v", tc.hdr, got, tc.want)
			}
		})
	}
}

func TestRetryAfterSeconds(t *testing.T) {
	if got := retryAfterSeconds(15 * time.Second); got != "15" {
		t.Fatalf("retryAfterSeconds(15s) = %q, want 15", got)
	}
	if got := retryAfterSeconds(100 * time.Millisecond); got != "1" {
		t.Fatalf("retryAfterSeconds(100ms) = %q, want 1 (floor of 1)", got)
	}
}

func TestWriteRetry(t *testing.T) {
	e := &Engine{}
	// 429 with a hint is relayed verbatim.
	rec := httptest.NewRecorder()
	e.writeRetry(rec, http.StatusTooManyRequests, "30")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") != "30" {
		t.Fatalf("Retry-After = %q, want 30", rec.Header().Get("Retry-After"))
	}
	// A non-retry status is normalized to 503.
	rec = httptest.NewRecorder()
	e.writeRetry(rec, http.StatusBadGateway, "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestNegCacheRemaining(t *testing.T) {
	c := newNegCache()
	fixed := time.Date(2026, 6, 30, 12, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return fixed }

	if _, ok := c.remaining("missing"); ok {
		t.Fatal("remaining on missing key should report not live")
	}
	c.set("k", 30*time.Second)
	d, ok := c.remaining("k")
	if !ok || d != 30*time.Second {
		t.Fatalf("remaining = (%v, %v), want (30s, true)", d, ok)
	}
	// Advance past expiry.
	c.now = func() time.Time { return fixed.Add(time.Minute) }
	if _, ok := c.remaining("k"); ok {
		t.Fatal("remaining after expiry should report not live")
	}
}
