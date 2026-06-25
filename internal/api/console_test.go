package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/younsl/o/box/kubernetes/forklift/internal/auth"
	"github.com/younsl/o/box/kubernetes/forklift/internal/meta"
	"github.com/younsl/o/box/kubernetes/forklift/internal/notify"
	"github.com/younsl/o/box/kubernetes/forklift/internal/repoconfig"
)

// newConsoleServer builds an API server wired with a real notifier, and returns
// the handler so a test can inject HA providers. Mirrors newTestServerWithStore
// but exposes the handler for the management-console endpoints.
func newConsoleServer(t *testing.T) (*httptest.Server, *meta.Store, *Handler) {
	t.Helper()
	store, err := meta.Open(context.Background(), filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	authSvc := auth.NewService(store, log, auth.Options{SessionSecret: []byte("test-secret-test-secret-test-secret")})
	if err := authSvc.BootstrapAdmin(context.Background(), adminUser, adminPass); err != nil {
		t.Fatal(err)
	}
	h := New(store, authSvc, log, nil)
	h.SetNotifier(notify.New(2*time.Second, log))
	srv := httptest.NewServer(authSvc.Middleware(h.Routes()))
	t.Cleanup(srv.Close)
	return srv, store, h
}

// webhookSink is an httptest server that counts the alarms posted to it.
func webhookSink(t *testing.T, status int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestReceiverHandlers(t *testing.T) {
	srv, _, _ := newConsoleServer(t)
	sink, _ := webhookSink(t, http.StatusOK)

	// Create.
	body := fmt.Sprintf(`{"name":"slack","description":"sec chan","webhook_url":%q}`, sink.URL)
	resp := adminDo(t, http.MethodPost, srv.URL+"/notification/receivers", body)
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("create status=%d body=%s", resp.StatusCode, b)
	}
	var created receiverDTO
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if created.ID == 0 || !created.WebhookConfigured || !created.Enabled {
		t.Fatalf("unexpected created receiver: %+v", created)
	}

	// List — URL never leaks; only webhook_configured.
	resp = adminDo(t, http.MethodGet, srv.URL+"/notification/receivers", "")
	var list []receiverDTO
	json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if len(list) != 1 || list[0].Name != "slack" || !list[0].WebhookConfigured {
		t.Fatalf("list = %+v", list)
	}

	// Update: blank webhook keeps the stored one; toggle disabled.
	disabled := false
	upd := map[string]any{"name": "slack", "description": "renamed", "webhook_url": "", "enabled": &disabled}
	ub, _ := json.Marshal(upd)
	resp = adminDo(t, http.MethodPut, srv.URL+"/notification/receivers/"+itoa(created.ID), string(ub))
	var updated receiverDTO
	json.NewDecoder(resp.Body).Decode(&updated)
	resp.Body.Close()
	if updated.Description != "renamed" || updated.Enabled || !updated.WebhookConfigured {
		t.Fatalf("update not applied: %+v", updated)
	}

	// Validation: bad name, bad webhook URL, invalid JSON.
	for _, c := range []string{
		`{"name":"bad name","webhook_url":"https://h.example/x"}`,
		`{"name":"ok","webhook_url":"notaurl"}`,
		`{bad json`,
	} {
		resp = adminDo(t, http.MethodPost, srv.URL+"/notification/receivers", c)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("case %q status=%d, want 400", c, resp.StatusCode)
		}
		resp.Body.Close()
	}

	// Delete.
	resp = adminDo(t, http.MethodDelete, srv.URL+"/notification/receivers/"+itoa(created.ID), "")
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status=%d, want 204", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestReceiverTestAndAdhoc(t *testing.T) {
	srv, store, _ := newConsoleServer(t)
	ctx := context.Background()
	sink, hits := webhookSink(t, http.StatusOK)

	rec, err := store.CreateReceiver(ctx, meta.Receiver{Name: "chan", WebhookURL: sink.URL, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	// Stored-receiver test delivers and reports sent.
	resp := adminDo(t, http.MethodPost, srv.URL+"/notification/receivers/"+itoa(rec.ID)+"/test", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("test status=%d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
	if hits.Load() == 0 {
		t.Fatal("expected webhook delivery")
	}

	// Receiver without a webhook URL -> 400.
	noURL, _ := store.CreateReceiver(ctx, meta.Receiver{Name: "nourl", Enabled: true})
	resp = adminDo(t, http.MethodPost, srv.URL+"/notification/receivers/"+itoa(noURL.ID)+"/test", "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("no-url test status=%d, want 400", resp.StatusCode)
	}
	resp.Body.Close()

	// Ad-hoc URL test.
	resp = adminDo(t, http.MethodPost, srv.URL+"/notification/test", fmt.Sprintf(`{"webhook_url":%q,"name":"probe"}`, sink.URL))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("adhoc status=%d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
	// Ad-hoc with an invalid URL -> 400.
	resp = adminDo(t, http.MethodPost, srv.URL+"/notification/test", `{"webhook_url":"nope"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("adhoc bad url status=%d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestReceiverTestNoNotifier(t *testing.T) {
	// A server without a notifier reports 503 for delivery endpoints.
	srv, store := newTestServerWithStore(t)
	rec, err := store.CreateReceiver(context.Background(), meta.Receiver{Name: "chan", WebhookURL: "https://hooks.example/x", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	resp := adminDo(t, http.MethodPost, srv.URL+"/notification/receivers/"+itoa(rec.ID)+"/test", "")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("no-notifier test status=%d, want 503", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestRepoSample(t *testing.T) {
	srv, store, _ := newConsoleServer(t)
	ctx := context.Background()
	sink, hits := webhookSink(t, http.StatusOK)

	id := mkProxyRepo(t, srv.URL, "npmjs")
	if _, err := store.CreateReceiver(ctx, meta.Receiver{Name: "chan", WebhookURL: sink.URL, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	// Wire the receiver into the repository's notify config.
	cfg := repoconfig.Default()
	cfg.Notify.Receivers = []string{"chan"}
	cfgJSON, err := cfg.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateRepositoryConfig(ctx, id, "https://registry.npmjs.org", cfgJSON); err != nil {
		t.Fatal(err)
	}

	// Preview returns the payload and the resolved receivers without delivering.
	resp := adminDo(t, http.MethodGet, srv.URL+"/repositories/"+itoa(id)+"/notification/sample", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("preview status=%d, want 200", resp.StatusCode)
	}
	var prev struct {
		Payload   map[string]any       `json:"payload"`
		Receivers []sampleReceiverInfo `json:"receivers"`
	}
	json.NewDecoder(resp.Body).Decode(&prev)
	resp.Body.Close()
	if len(prev.Receivers) != 1 || !prev.Receivers[0].Enabled || prev.Payload["event"] != "approval.request" {
		t.Fatalf("unexpected preview: %+v", prev)
	}
	if hits.Load() != 0 {
		t.Fatal("preview must not deliver")
	}

	// Send delivers to the enabled receiver.
	resp = adminDo(t, http.MethodPost, srv.URL+"/repositories/"+itoa(id)+"/notification/sample", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("send status=%d, want 200", resp.StatusCode)
	}
	var sent struct {
		Results []struct {
			Name string `json:"name"`
			OK   bool   `json:"ok"`
		} `json:"results"`
	}
	json.NewDecoder(resp.Body).Decode(&sent)
	resp.Body.Close()
	if len(sent.Results) != 1 || !sent.Results[0].OK {
		t.Fatalf("unexpected send results: %+v", sent.Results)
	}
	if hits.Load() != 1 {
		t.Fatalf("expected 1 delivery, got %d", hits.Load())
	}

	// A repository with no selected receivers -> 400 on send.
	bare := mkProxyRepo(t, srv.URL, "pypi-bare")
	resp = adminDo(t, http.MethodPost, srv.URL+"/repositories/"+itoa(bare)+"/notification/sample", "")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("send no-receivers status=%d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestHAStatusAndStepDown(t *testing.T) {
	srv, _, h := newConsoleServer(t)

	// No provider wired -> single-instance leader.
	resp := adminDo(t, http.MethodGet, srv.URL+"/ha", "")
	var st HAStatus
	json.NewDecoder(resp.Body).Decode(&st)
	resp.Body.Close()
	if st.Mode != "single" || !st.IsLeader {
		t.Fatalf("default HA = %+v, want single leader", st)
	}

	// Injected provider is reflected.
	h.SetHAStatus(func(context.Context) HAStatus {
		return HAStatus{Enabled: true, Mode: "object-storage", Backend: "s3", IsLeader: true, Role: "leader", FencingToken: 7}
	})
	resp = adminDo(t, http.MethodGet, srv.URL+"/ha", "")
	json.NewDecoder(resp.Body).Decode(&st)
	resp.Body.Close()
	if st.Backend != "s3" || st.FencingToken != 7 {
		t.Fatalf("injected HA = %+v", st)
	}

	// Step-down with no provider -> 409.
	resp = adminDo(t, http.MethodPost, srv.URL+"/ha/step-down", "")
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("stepdown no-provider status=%d, want 409", resp.StatusCode)
	}
	resp.Body.Close()

	// Not the leader -> 409.
	h.SetHAStepDown(func() bool { return false })
	resp = adminDo(t, http.MethodPost, srv.URL+"/ha/step-down", "")
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("stepdown non-leader status=%d, want 409", resp.StatusCode)
	}
	resp.Body.Close()

	// Leader steps down -> 200.
	h.SetHAStepDown(func() bool { return true })
	resp = adminDo(t, http.MethodPost, srv.URL+"/ha/step-down", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("stepdown leader status=%d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestSetRepositoryDisabled(t *testing.T) {
	srv, _, _ := newConsoleServer(t)
	id := mkProxyRepo(t, srv.URL, "npmjs")

	// Take offline.
	resp := adminDo(t, http.MethodPost, srv.URL+"/repositories/"+itoa(id)+"/disabled", `{"disabled":true}`)
	var dto struct {
		Disabled bool `json:"disabled"`
	}
	json.NewDecoder(resp.Body).Decode(&dto)
	resp.Body.Close()
	if resp != nil && !dto.Disabled {
		t.Fatalf("expected disabled=true, got %+v", dto)
	}

	// Back online.
	resp = adminDo(t, http.MethodPost, srv.URL+"/repositories/"+itoa(id)+"/disabled", `{"disabled":false}`)
	json.NewDecoder(resp.Body).Decode(&dto)
	resp.Body.Close()
	if dto.Disabled {
		t.Fatal("expected disabled=false")
	}

	// Invalid JSON -> 400; unknown id -> 404.
	resp = adminDo(t, http.MethodPost, srv.URL+"/repositories/"+itoa(id)+"/disabled", `{bad`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad json status=%d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
	resp = adminDo(t, http.MethodPost, srv.URL+"/repositories/99999/disabled", `{"disabled":true}`)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown id status=%d, want 404", resp.StatusCode)
	}
	resp.Body.Close()
}
