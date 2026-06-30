package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/younsl/o/box/kubernetes/forklift/internal/artifactscan"
	"github.com/younsl/o/box/kubernetes/forklift/internal/meta"
	"github.com/younsl/o/box/kubernetes/forklift/internal/storage"
)

// ScanInternal serves token-gated scanner worker endpoints. It is mounted
// outside the user-facing API and never grants direct DB or blob write access.
type ScanInternal struct {
	svc         *artifactscan.Service
	blobs       storage.BlobStore
	workerToken string
	log         *slog.Logger
}

// NewScanInternal creates internal scanner-worker routes. workerToken protects
// job claiming; per-job scan tokens protect blob reads and result submission.
func NewScanInternal(svc *artifactscan.Service, blobs storage.BlobStore, workerToken string, log *slog.Logger) *ScanInternal {
	return &ScanInternal{svc: svc, blobs: blobs, workerToken: workerToken, log: log}
}

// Routes returns routes mounted under /internal/scans.
func (h *ScanInternal) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/claim", h.claim)
	r.Get("/{id}/blob", h.blob)
	r.Post("/{id}/heartbeat", h.heartbeat)
	r.Post("/{id}/result", h.result)
	return r
}

type scanClaimReq struct {
	WorkerID string `json:"worker_id"`
}

type scanClaimResp struct {
	JobID      string                `json:"job_id"`
	BlobSHA256 string                `json:"blob_sha256"`
	Scanner    string                `json:"scanner"`
	Token      string                `json:"token"`
	Targets    []artifactscan.Target `json:"targets,omitempty"`
}

func (h *ScanInternal) claim(w http.ResponseWriter, r *http.Request) {
	if !h.authorizedWorker(r) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req scanClaimReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if strings.TrimSpace(req.WorkerID) == "" {
		writeError(w, http.StatusBadRequest, "worker_id required")
		return
	}
	claimed, err := h.svc.Claim(r.Context(), req.WorkerID)
	if errors.Is(err, meta.ErrNotFound) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, scanClaimResp{
		JobID:      claimed.Job.ID,
		BlobSHA256: claimed.Job.BlobSHA256,
		Scanner:    claimed.Job.Scanner,
		Token:      claimed.Token,
		Targets:    claimed.Targets,
	})
}

func (h *ScanInternal) blob(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorizedScanToken(w, r)
	if !ok {
		return
	}
	if claims.JobID != chi.URLParam(r, "id") {
		writeError(w, http.StatusForbidden, "job token mismatch")
		return
	}
	rc, size, err := h.blobs.Open(r.Context(), claims.BlobSHA256)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "blob not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Artifact-SHA256", claims.BlobSHA256)
	if size >= 0 {
		w.Header().Set("Content-Length", strconvFormatInt(size))
	}
	_, _ = io.Copy(w, rc)
}

type scanHeartbeatReq struct {
	WorkerID string `json:"worker_id"`
}

func (h *ScanInternal) heartbeat(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorizedScanToken(w, r)
	if !ok {
		return
	}
	if claims.JobID != chi.URLParam(r, "id") {
		writeError(w, http.StatusForbidden, "job token mismatch")
		return
	}
	var req scanHeartbeatReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := h.svc.Heartbeat(r.Context(), bearerToken(r), req.WorkerID); err != nil {
		mapError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type scanResultReq struct {
	WorkerID string              `json:"worker_id"`
	Result   artifactscan.Result `json:"result"`
}

func (h *ScanInternal) result(w http.ResponseWriter, r *http.Request) {
	claims, ok := h.authorizedScanToken(w, r)
	if !ok {
		return
	}
	if claims.JobID != chi.URLParam(r, "id") {
		writeError(w, http.StatusForbidden, "job token mismatch")
		return
	}
	var req scanResultReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if _, err := h.svc.Complete(r.Context(), bearerToken(r), req.WorkerID, req.Result); err != nil {
		mapError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ScanInternal) authorizedWorker(r *http.Request) bool {
	if h.workerToken == "" {
		return false
	}
	return bearerToken(r) == h.workerToken
}

func (h *ScanInternal) authorizedScanToken(w http.ResponseWriter, r *http.Request) (artifactscan.TokenClaims, bool) {
	token := bearerToken(r)
	claims, err := h.svc.VerifyToken(token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return artifactscan.TokenClaims{}, false
	}
	return claims, true
}

func bearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
}

func strconvFormatInt(v int64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	n := v
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
