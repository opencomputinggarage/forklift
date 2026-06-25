// Package notify delivers outbound alarms to configured receivers (named
// webhook channels). Delivery is best-effort and asynchronous: a failure is
// logged, never surfaced to the request path that triggered it.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Target is a resolved delivery destination — a receiver's display name and its
// webhook URL.
type Target struct {
	Name string
	URL  string
}

// Notifier posts JSON alarms to webhook targets.
type Notifier struct {
	client *http.Client
	log    *slog.Logger
}

// New returns a Notifier; timeout bounds each delivery attempt.
func New(timeout time.Duration, log *slog.Logger) *Notifier {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Notifier{client: &http.Client{Timeout: timeout}, log: log}
}

// ApprovalPayload is the body posted for a package-approval alarm. Text is a
// human-readable summary accepted as-is by Slack/Mattermost incoming webhooks;
// the structured fields let any other consumer route on the raw values.
type ApprovalPayload struct {
	Text        string `json:"text"`
	Event       string `json:"event"`
	Repository  string `json:"repository"`
	Package     string `json:"package"`
	Version     string `json:"version,omitempty"`
	RequestedBy string `json:"requested_by,omitempty"`
	Timestamp   string `json:"timestamp"`
}

// NotifyApprovalRequest fans an alarm out to every target for a package newly
// quarantined pending approval. It returns immediately; deliveries run on
// background goroutines.
func (n *Notifier) NotifyApprovalRequest(targets []Target, repo, pkg, version, requestedBy string) {
	if n == nil || len(targets) == 0 {
		return
	}
	who := requestedBy
	if who == "" {
		who = "anonymous"
	}
	coord := pkg
	if version != "" {
		coord = pkg + "@" + version
	}
	payload := ApprovalPayload{
		Text:        fmt.Sprintf("📦 Package pending approval: %s in repo %q (requested by %s)", coord, repo, who),
		Event:       "approval.request",
		Repository:  repo,
		Package:     pkg,
		Version:     version,
		RequestedBy: requestedBy,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		n.log.Error("notify: marshal approval payload failed", "err", err)
		return
	}
	for _, t := range targets {
		if t.URL == "" {
			continue
		}
		go n.post(t, body)
	}
}

// SendTest delivers a test alarm to a webhook URL synchronously and reports the
// outcome, so the "send test" button can give the admin immediate feedback. A
// transport error or a non-2xx response is returned as an error.
func (n *Notifier) SendTest(ctx context.Context, name, url string) error {
	if n == nil {
		return fmt.Errorf("notifications are not configured")
	}
	payload := map[string]string{
		"text":      fmt.Sprintf("✅ Forklift test notification for receiver %q.", name),
		"event":     "test",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

// BuildApprovalSample returns a representative approval alarm for a repository,
// used to preview the message and to deliver a manual sample. It carries a
// clearly marked placeholder package so a delivered sample is not mistaken for a
// real pending approval.
func (n *Notifier) BuildApprovalSample(repo, requestedBy string) ApprovalPayload {
	who := requestedBy
	if who == "" {
		who = "anonymous"
	}
	return ApprovalPayload{
		Text:        fmt.Sprintf("📦 [SAMPLE] Package pending approval: com.example:sample@1.0.0 in repo %q (requested by %s). This is a forklift notification test.", repo, who),
		Event:       "approval.request",
		Repository:  repo,
		Package:     "com.example:sample",
		Version:     "1.0.0",
		RequestedBy: requestedBy,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}
}

// SendApprovalPayload delivers a prepared approval payload to a webhook URL
// synchronously and reports the outcome. Used for the manual sample send so each
// receiver's result can be reported.
func (n *Notifier) SendApprovalPayload(ctx context.Context, url string, p ApprovalPayload) error {
	if n == nil {
		return fmt.Errorf("notifications are not configured")
	}
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

// post delivers one payload to one target, logging any failure. It runs detached
// from the triggering request, so it uses its own bounded context.
func (n *Notifier) post(t Target, body []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), n.client.Timeout+time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.URL, bytes.NewReader(body))
	if err != nil {
		n.log.Error("notify: build request failed", "receiver", t.Name, "err", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := n.client.Do(req)
	if err != nil {
		n.log.Warn("notify: webhook delivery failed", "receiver", t.Name, "err", err)
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		n.log.Warn("notify: webhook non-2xx response", "receiver", t.Name, "status", resp.StatusCode)
	}
}
