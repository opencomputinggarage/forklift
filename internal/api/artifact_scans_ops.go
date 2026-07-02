package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/younsl/o/box/kubernetes/forklift/internal/meta"
	"github.com/younsl/o/box/kubernetes/forklift/internal/repoconfig"
)

type artifactScanAllReq struct {
	RepositoryID   int64  `json:"repository_id,omitempty"`
	ScannerProfile string `json:"scanner_profile,omitempty"`
	Limit          int    `json:"limit,omitempty"`
}

type artifactScanRecomputeReq struct {
	RepositoryID   int64  `json:"repository_id,omitempty"`
	BlobSHA256     string `json:"blob_sha256,omitempty"`
	ScannerProfile string `json:"scanner_profile,omitempty"`
	Limit          int    `json:"limit,omitempty"`
}

type artifactSBOMExportReq struct {
	RepositoryID   int64  `json:"repository_id,omitempty"`
	Path           string `json:"path,omitempty"`
	BlobSHA256     string `json:"blob_sha256,omitempty"`
	ScannerProfile string `json:"scanner_profile,omitempty"`
	Destination    string `json:"destination"`
}

type artifactScanOpsDTO struct {
	Queued     int                      `json:"queued,omitempty"`
	Recomputed int                      `json:"recomputed,omitempty"`
	Skipped    int                      `json:"skipped,omitempty"`
	Jobs       []artifactScanJobDTO     `json:"jobs,omitempty"`
	Verdicts   []artifactScanVerdictDTO `json:"verdicts,omitempty"`
}

type artifactScanVerdictDTO struct {
	RepositoryID   int64  `json:"repository_id"`
	BlobSHA256     string `json:"blob_sha256"`
	ScannerProfile string `json:"scanner_profile"`
	Status         string `json:"status"`
	MaxSeverity    string `json:"max_severity"`
	Reason         string `json:"reason"`
}

type artifactSBOMExportDTO struct {
	ID          int64     `json:"id"`
	SBOMID      int64     `json:"sbom_id"`
	Destination string    `json:"destination"`
	Status      string    `json:"status"`
	NextRunAt   time.Time `json:"next_run_at"`
}

func (h *Handler) scanAllArtifacts(w http.ResponseWriter, r *http.Request) {
	var req artifactScanAllReq
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Limit <= 0 {
		req.Limit = 1000
	}
	refs, err := h.artifactScanRefs(r.Context(), req.RepositoryID, req.ScannerProfile, req.Limit)
	if err != nil {
		mapError(w, err)
		return
	}
	out := artifactScanOpsDTO{Jobs: make([]artifactScanJobDTO, 0, len(refs))}
	seen := map[string]bool{}
	now := time.Now().UTC()
	for _, ref := range refs {
		key := ref.blobSHA256 + "\x00" + ref.profile
		if seen[key] {
			out.Skipped++
			continue
		}
		seen[key] = true
		jobID, err := randomArtifactScanJobID()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		job, err := h.store.EnqueueArtifactScan(r.Context(), jobID, ref.blobSHA256, ref.profile, now)
		if err != nil {
			mapError(w, err)
			return
		}
		out.Queued++
		out.Jobs = append(out.Jobs, artifactScanJobDTO{
			JobID:          job.ID,
			Status:         string(job.Status),
			ScannerProfile: job.ScannerProfile,
			Scanner:        job.Scanner,
			BlobSHA256:     job.BlobSHA256,
		})
	}
	writeJSON(w, http.StatusAccepted, out)
}

func (h *Handler) recomputeArtifactScanVerdicts(w http.ResponseWriter, r *http.Request) {
	var req artifactScanRecomputeReq
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Limit <= 0 {
		req.Limit = 1000
	}
	refs, err := h.artifactScanRefs(r.Context(), req.RepositoryID, req.ScannerProfile, req.Limit)
	if err != nil {
		mapError(w, err)
		return
	}
	out := artifactScanOpsDTO{Verdicts: make([]artifactScanVerdictDTO, 0, len(refs))}
	seen := map[string]bool{}
	now := time.Now().UTC()
	for _, ref := range refs {
		if req.BlobSHA256 != "" && req.BlobSHA256 != ref.blobSHA256 {
			continue
		}
		key := strconv.FormatInt(ref.repositoryID, 10) + "\x00" + ref.blobSHA256 + "\x00" + ref.profile
		if seen[key] {
			out.Skipped++
			continue
		}
		seen[key] = true
		verdict, err := h.store.RecomputeArtifactScanVerdict(r.Context(), ref.repositoryID, ref.blobSHA256, ref.profile, now)
		if errors.Is(err, meta.ErrNotFound) {
			out.Skipped++
			continue
		}
		if err != nil {
			mapError(w, err)
			return
		}
		out.Recomputed++
		out.Verdicts = append(out.Verdicts, artifactScanVerdictDTO{
			RepositoryID:   verdict.RepositoryID,
			BlobSHA256:     verdict.BlobSHA256,
			ScannerProfile: verdict.ScannerProfile,
			Status:         string(verdict.Status),
			MaxSeverity:    string(verdict.MaxSeverity),
			Reason:         verdict.Reason,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) exportArtifactSBOM(w http.ResponseWriter, r *http.Request) {
	var req artifactSBOMExportReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if req.Destination == "" {
		writeError(w, http.StatusBadRequest, "destination required")
		return
	}
	blob := req.BlobSHA256
	profile := req.ScannerProfile
	if req.RepositoryID != 0 {
		repo, err := h.store.GetRepository(r.Context(), req.RepositoryID)
		if err != nil {
			mapError(w, err)
			return
		}
		cfg, _ := repoconfig.Parse(repo.ConfigJSON)
		if profile == "" {
			profile = cfg.ArtifactScan.EffectiveScannerProfile()
		}
		if req.Path != "" {
			artifact, err := h.store.GetArtifact(r.Context(), req.RepositoryID, req.Path)
			if err != nil {
				mapError(w, err)
				return
			}
			blob = artifact.BlobSHA256
		}
	}
	if blob == "" {
		writeError(w, http.StatusBadRequest, "blob_sha256 or repository_id/path required")
		return
	}
	if profile == "" {
		profile = "grype-default"
	}
	sbom, err := h.store.LatestArtifactSBOM(r.Context(), blob, profile)
	if err != nil {
		mapError(w, err)
		return
	}
	export, err := h.store.CreateArtifactSBOMExport(r.Context(), sbom.ID, req.Destination, time.Now().UTC())
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, artifactSBOMExportDTO{
		ID:          export.ID,
		SBOMID:      export.SBOMID,
		Destination: export.Destination,
		Status:      export.Status,
		NextRunAt:   export.NextRunAt,
	})
}

type artifactScanRef struct {
	repositoryID int64
	blobSHA256   string
	profile      string
}

func (h *Handler) artifactScanRefs(ctx context.Context, repositoryID int64, profile string, limit int) ([]artifactScanRef, error) {
	var repos []meta.Repository
	if repositoryID != 0 {
		repo, err := h.store.GetRepository(ctx, repositoryID)
		if err != nil {
			return nil, err
		}
		repos = []meta.Repository{repo}
	} else {
		var err error
		repos, err = h.store.ListRepositories(ctx)
		if err != nil {
			return nil, err
		}
	}
	out := make([]artifactScanRef, 0)
	for _, repo := range repos {
		cfg, err := repoconfig.Parse(repo.ConfigJSON)
		if err != nil {
			return nil, err
		}
		if !cfg.ArtifactScan.Enabled {
			continue
		}
		resolved := cfg.ArtifactScan.EffectiveScannerProfile()
		if profile != "" && profile != resolved {
			continue
		}
		arts, err := h.store.ListRepoArtifactsPage(ctx, repo.ID, "", limit, 0)
		if err != nil {
			return nil, err
		}
		for _, art := range arts {
			if art.BlobSHA256 == "" {
				continue
			}
			out = append(out, artifactScanRef{repositoryID: repo.ID, blobSHA256: art.BlobSHA256, profile: resolved})
			if len(out) >= limit {
				return out, nil
			}
		}
	}
	return out, nil
}
