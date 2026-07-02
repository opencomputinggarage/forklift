package scannerworker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/younsl/o/box/kubernetes/forklift/internal/artifactscan"
)

// ErrNoJob is returned when the server has no queued scan job.
var ErrNoJob = errors.New("no scan job available")

// Client talks to forklift's internal scanner API.
type Client struct {
	BaseURL     string
	WorkerID    string
	WorkerToken string
	HTTP        *http.Client
}

// ClaimedJob is the worker API response for one claimed scan.
type ClaimedJob struct {
	JobID             string                `json:"job_id"`
	BlobSHA256        string                `json:"blob_sha256"`
	ScannerProfile    string                `json:"scanner_profile"`
	Scanner           string                `json:"scanner"`
	ScannerConfigHash string                `json:"scanner_config_hash"`
	Token             string                `json:"token"`
	Deadline          time.Time             `json:"deadline"`
	Limits            artifactscan.Limits   `json:"limits"`
	StoreSBOM         bool                  `json:"store_sbom,omitempty"`
	Targets           []artifactscan.Target `json:"targets,omitempty"`
}

// Claim claims one scan job.
func (c Client) Claim(ctx context.Context, capabilities []artifactscan.ScannerCapability) (ClaimedJob, error) {
	body, _ := json.Marshal(map[string]any{
		"worker_id":    c.WorkerID,
		"capabilities": capabilities,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url("/internal/scans/claim"), bytes.NewReader(body))
	if err != nil {
		return ClaimedJob{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.WorkerToken)
	resp, err := c.http().Do(req)
	if err != nil {
		return ClaimedJob{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return ClaimedJob{}, ErrNoJob
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ClaimedJob{}, fmt.Errorf("claim scan job: status %d", resp.StatusCode)
	}
	var out ClaimedJob
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ClaimedJob{}, err
	}
	return out, nil
}

// OpenBlob opens the artifact blob stream for a claimed job.
func (c Client) OpenBlob(ctx context.Context, job ClaimedJob) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url("/internal/scans/"+job.JobID+"/blob"), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+job.Token)
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("download scan blob: status %d", resp.StatusCode)
	}
	return resp.Body, nil
}

// Heartbeat extends the lease for a claimed job and refreshes its job token.
func (c Client) Heartbeat(ctx context.Context, job ClaimedJob) (ClaimedJob, error) {
	body, err := json.Marshal(map[string]string{"worker_id": c.WorkerID})
	if err != nil {
		return ClaimedJob{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url("/internal/scans/"+job.JobID+"/heartbeat"), bytes.NewReader(body))
	if err != nil {
		return ClaimedJob{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+job.Token)
	resp, err := c.http().Do(req)
	if err != nil {
		return ClaimedJob{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			return ClaimedJob{}, fmt.Errorf("heartbeat scan job: status %d", resp.StatusCode)
		}
		return ClaimedJob{}, fmt.Errorf("heartbeat scan job: status %d: %s", resp.StatusCode, msg)
	}
	var renewed ClaimedJob
	if err := json.NewDecoder(resp.Body).Decode(&renewed); err != nil {
		return ClaimedJob{}, err
	}
	if renewed.Token == "" {
		return ClaimedJob{}, errors.New("heartbeat response missing token")
	}
	job.Token = renewed.Token
	if !renewed.Deadline.IsZero() {
		job.Deadline = renewed.Deadline
	}
	if renewed.Limits.MaxArtifactBytes > 0 {
		job.Limits = renewed.Limits
	}
	return job, nil
}

// SubmitResult submits a normalized scan result for a claimed job.
func (c Client) SubmitResult(ctx context.Context, job ClaimedJob, result artifactscan.Result) error {
	body, err := json.Marshal(map[string]any{
		"worker_id": c.WorkerID,
		"result":    result,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url("/internal/scans/"+job.JobID+"/result"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+job.Token)
	resp, err := c.http().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			return fmt.Errorf("submit scan result: status %d", resp.StatusCode)
		}
		return fmt.Errorf("submit scan result: status %d: %s", resp.StatusCode, msg)
	}
	return nil
}

func (c Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c Client) url(path string) string {
	return strings.TrimRight(c.BaseURL, "/") + path
}
