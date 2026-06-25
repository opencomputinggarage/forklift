package notify

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewDefaultsTimeout(t *testing.T) {
	n := New(0, testLogger())
	if n.client.Timeout != 5*time.Second {
		t.Fatalf("default timeout = %v, want 5s", n.client.Timeout)
	}
	n = New(2*time.Second, testLogger())
	if n.client.Timeout != 2*time.Second {
		t.Fatalf("custom timeout = %v, want 2s", n.client.Timeout)
	}
}

func TestSendTest(t *testing.T) {
	var gotBody atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") == "application/json" {
			gotBody.Store(true)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := New(time.Second, testLogger())
	if err := n.SendTest(context.Background(), "slack", srv.URL); err != nil {
		t.Fatalf("SendTest ok case: %v", err)
	}
	if !gotBody.Load() {
		t.Fatal("expected JSON content-type on delivered request")
	}

	// non-2xx -> error
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	if err := n.SendTest(context.Background(), "slack", bad.URL); err == nil {
		t.Fatal("SendTest non-2xx: expected error")
	}

	// bad URL -> transport error
	if err := n.SendTest(context.Background(), "slack", "http://127.0.0.1:0"); err == nil {
		t.Fatal("SendTest bad url: expected error")
	}

	// nil notifier -> error, not panic
	var nilN *Notifier
	if err := nilN.SendTest(context.Background(), "x", srv.URL); err == nil {
		t.Fatal("nil notifier SendTest: expected error")
	}
}

func TestBuildApprovalSample(t *testing.T) {
	n := New(time.Second, testLogger())
	p := n.BuildApprovalSample("npm-proxy", "")
	if p.Event != "approval.request" || p.Repository != "npm-proxy" || p.Package != "com.example:sample" {
		t.Fatalf("unexpected sample: %+v", p)
	}
	if p.Version != "1.0.0" || p.Timestamp == "" {
		t.Fatalf("sample missing fields: %+v", p)
	}
	// requestedBy carried through when provided.
	if got := n.BuildApprovalSample("r", "alice").RequestedBy; got != "alice" {
		t.Fatalf("requested_by = %q, want alice", got)
	}
}

func TestSendApprovalPayload(t *testing.T) {
	n := New(time.Second, testLogger())
	p := n.BuildApprovalSample("repo", "bob")

	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ok.Close()
	if err := n.SendApprovalPayload(context.Background(), ok.URL, p); err != nil {
		t.Fatalf("SendApprovalPayload ok: %v", err)
	}

	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer bad.Close()
	if err := n.SendApprovalPayload(context.Background(), bad.URL, p); err == nil {
		t.Fatal("SendApprovalPayload non-2xx: expected error")
	}

	var nilN *Notifier
	if err := nilN.SendApprovalPayload(context.Background(), ok.URL, p); err == nil {
		t.Fatal("nil notifier SendApprovalPayload: expected error")
	}
}

func TestNotifyApprovalRequest(t *testing.T) {
	var hits atomic.Int32
	done := make(chan struct{}, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
		done <- struct{}{}
	}))
	defer srv.Close()

	n := New(time.Second, testLogger())
	// Two real targets plus one with an empty URL (skipped) and noise.
	targets := []Target{
		{Name: "a", URL: srv.URL},
		{Name: "b", URL: srv.URL},
		{Name: "empty", URL: ""},
	}
	n.NotifyApprovalRequest(targets, "npm-proxy", "left-pad", "1.0.0", "")

	// Wait for the two async deliveries.
	for range 2 {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for webhook delivery")
		}
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("delivered %d times, want 2 (empty URL skipped)", got)
	}

	// nil notifier and empty targets are no-ops (must not panic).
	var nilN *Notifier
	nilN.NotifyApprovalRequest(targets, "r", "p", "", "")
	n.NotifyApprovalRequest(nil, "r", "p", "", "")
}
