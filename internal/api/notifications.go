package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/younsl/o/box/kubernetes/forklift/internal/meta"
	"github.com/younsl/o/box/kubernetes/forklift/internal/notify"
	"github.com/younsl/o/box/kubernetes/forklift/internal/repoconfig"
)

// receiverDTO is the wire shape of a notification receiver. The webhook URL is
// write-only — never returned once stored (it may carry a secret token); a
// boolean reports only whether one is configured.
type receiverDTO struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	WebhookConfigured bool   `json:"webhook_configured"`
	Enabled           bool   `json:"enabled"`
	CreatedBy         string `json:"created_by"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

func toReceiverDTO(r meta.Receiver) receiverDTO {
	return receiverDTO{
		ID: r.ID, Name: r.Name, Description: r.Description,
		WebhookConfigured: r.WebhookURL != "",
		Enabled:           r.Enabled,
		CreatedBy:         r.CreatedBy,
		CreatedAt:         r.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         r.UpdatedAt.Format(time.RFC3339),
	}
}

type receiverReq struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	WebhookURL  string `json:"webhook_url"`
	Enabled     *bool  `json:"enabled"`
}

// validWebhookURL accepts only absolute http(s) URLs.
func validWebhookURL(s string) bool {
	u, err := url.ParseRequestURI(strings.TrimSpace(s))
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// listReceivers returns all notification receivers (oldest first).
func (h *Handler) listReceivers(w http.ResponseWriter, r *http.Request) {
	recs, err := h.store.ListReceivers(r.Context())
	if err != nil {
		mapError(w, err)
		return
	}
	out := make([]receiverDTO, 0, len(recs))
	for _, rec := range recs {
		out = append(out, toReceiverDTO(rec))
	}
	writeJSON(w, http.StatusOK, out)
}

// createReceiver adds a notification receiver.
func (h *Handler) createReceiver(w http.ResponseWriter, r *http.Request) {
	var req receiverReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if !validName(strings.TrimSpace(req.Name)) {
		writeError(w, http.StatusBadRequest, "invalid receiver name: "+nameRuleMsg)
		return
	}
	if !validWebhookURL(req.WebhookURL) {
		writeError(w, http.StatusBadRequest, "webhook_url must be an absolute http(s) URL")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	rec, err := h.store.CreateReceiver(r.Context(), meta.Receiver{
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		WebhookURL:  strings.TrimSpace(req.WebhookURL),
		Enabled:     enabled,
		CreatedBy:   principalName(r),
	})
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toReceiverDTO(rec))
}

// updateReceiver overwrites a receiver's fields.
func (h *Handler) updateReceiver(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req receiverReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if !validName(strings.TrimSpace(req.Name)) {
		writeError(w, http.StatusBadRequest, "invalid receiver name: "+nameRuleMsg)
		return
	}
	// The webhook URL is never shown back, so an edit submits it blank to keep
	// the stored one; a non-empty value replaces it (and must be valid).
	existing, err := h.store.GetReceiver(r.Context(), id)
	if err != nil {
		mapError(w, err)
		return
	}
	webhookURL := existing.WebhookURL
	if v := strings.TrimSpace(req.WebhookURL); v != "" {
		if !validWebhookURL(v) {
			writeError(w, http.StatusBadRequest, "webhook_url must be an absolute http(s) URL")
			return
		}
		webhookURL = v
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	rec, err := h.store.UpdateReceiver(r.Context(), meta.Receiver{
		ID:          id,
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		WebhookURL:  webhookURL,
		Enabled:     enabled,
	})
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toReceiverDTO(rec))
}

// testReceiver sends a test alarm to a receiver's stored webhook URL and reports
// whether delivery succeeded. The URL is write-only, so the test must run
// server-side using the stored value.
func (h *Handler) testReceiver(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	rec, err := h.store.GetReceiver(r.Context(), id)
	if err != nil {
		mapError(w, err)
		return
	}
	if rec.WebhookURL == "" {
		writeError(w, http.StatusBadRequest, "this receiver has no webhook URL configured")
		return
	}
	if h.notifier == nil {
		writeError(w, http.StatusServiceUnavailable, "notifications are not configured")
		return
	}
	if err := h.notifier.SendTest(r.Context(), rec.Name, rec.WebhookURL); err != nil {
		writeError(w, http.StatusBadGateway, "test delivery failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// testWebhook sends a test alarm to a webhook URL supplied in the request body,
// for verifying a URL before the receiver is saved (the create/edit form). The
// stored-receiver test lives at /receivers/{id}/test.
func (h *Handler) testWebhookAdhoc(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WebhookURL string `json:"webhook_url"`
		Name       string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	url := strings.TrimSpace(req.WebhookURL)
	if !validWebhookURL(url) {
		writeError(w, http.StatusBadRequest, "webhook_url must be an absolute http(s) URL")
		return
	}
	if h.notifier == nil {
		writeError(w, http.StatusServiceUnavailable, "notifications are not configured")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = "new receiver"
	}
	if err := h.notifier.SendTest(r.Context(), name, url); err != nil {
		writeError(w, http.StatusBadGateway, "test delivery failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// repoSampleTargets resolves a repository's selected receivers to delivery
// targets, returning per-receiver status (exists / enabled) for the preview and
// the deliverable targets for the send.
func (h *Handler) repoSampleTargets(r *http.Request, id int64) (repoName string, info []sampleReceiverInfo, targets []notify.Target, err error) {
	repo, err := h.store.GetRepository(r.Context(), id)
	if err != nil {
		return "", nil, nil, err
	}
	cfg, perr := repoconfig.Parse(repo.ConfigJSON)
	if perr != nil {
		return "", nil, nil, perr
	}
	all, lerr := h.store.ListReceivers(r.Context())
	if lerr != nil {
		return "", nil, nil, lerr
	}
	byName := make(map[string]meta.Receiver, len(all))
	for _, rec := range all {
		byName[rec.Name] = rec
	}
	for _, name := range cfg.Notify.Receivers {
		rec, ok := byName[name]
		info = append(info, sampleReceiverInfo{Name: name, Exists: ok, Enabled: ok && rec.Enabled})
		if ok && rec.Enabled && rec.WebhookURL != "" {
			targets = append(targets, notify.Target{Name: rec.Name, URL: rec.WebhookURL})
		}
	}
	return repo.Name, info, targets, nil
}

type sampleReceiverInfo struct {
	Name    string `json:"name"`
	Exists  bool   `json:"exists"`
	Enabled bool   `json:"enabled"`
}

// previewRepoSample returns the sample approval payload a repository would send
// and the receivers it would target, without delivering anything.
func (h *Handler) previewRepoSample(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.notifier == nil {
		writeError(w, http.StatusServiceUnavailable, "notifications are not configured")
		return
	}
	repoName, info, _, err := h.repoSampleTargets(r, id)
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"payload":   h.notifier.BuildApprovalSample(repoName, principalName(r)),
		"receivers": info,
	})
}

// sendRepoSample delivers a sample approval alarm to the repository's selected
// enabled receivers, reporting each receiver's result.
func (h *Handler) sendRepoSample(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if h.notifier == nil {
		writeError(w, http.StatusServiceUnavailable, "notifications are not configured")
		return
	}
	repoName, _, targets, err := h.repoSampleTargets(r, id)
	if err != nil {
		mapError(w, err)
		return
	}
	if len(targets) == 0 {
		writeError(w, http.StatusBadRequest, "no enabled receivers are selected for this repository")
		return
	}
	payload := h.notifier.BuildApprovalSample(repoName, principalName(r))
	type result struct {
		Name  string `json:"name"`
		OK    bool   `json:"ok"`
		Error string `json:"error,omitempty"`
	}
	results := make([]result, 0, len(targets))
	for _, t := range targets {
		if serr := h.notifier.SendApprovalPayload(r.Context(), t.URL, payload); serr != nil {
			results = append(results, result{Name: t.Name, OK: false, Error: serr.Error()})
		} else {
			results = append(results, result{Name: t.Name, OK: true})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// deleteReceiver removes a receiver.
func (h *Handler) deleteReceiver(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.store.DeleteReceiver(r.Context(), id); err != nil {
		mapError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
