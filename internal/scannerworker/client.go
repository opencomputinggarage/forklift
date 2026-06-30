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
	JobID      string `json:"job_id"`
	BlobSHA256 string `json:"blob_sha256"`
	Scanner    string `json:"scanner"`
	Token      string `json:"token"`
}

// Claim claims one scan job.
func (c Client) Claim(ctx context.Context) (ClaimedJob, error) {
	body, _ := json.Marshal(map[string]string{"worker_id": c.WorkerID})
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
		return fmt.Errorf("submit scan result: status %d", resp.StatusCode)
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
