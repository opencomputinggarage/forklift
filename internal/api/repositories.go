package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/younsl/o/box/kubernetes/forklift/internal/artifactscan"
	"github.com/younsl/o/box/kubernetes/forklift/internal/audit"
	"github.com/younsl/o/box/kubernetes/forklift/internal/auth"
	"github.com/younsl/o/box/kubernetes/forklift/internal/meta"
	repopkg "github.com/younsl/o/box/kubernetes/forklift/internal/repo"
	"github.com/younsl/o/box/kubernetes/forklift/internal/repoconfig"
)

// repositoryDTO is the JSON shape for a repository.
type repositoryDTO struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	Format      string            `json:"format"`
	Type        string            `json:"type"`
	UpstreamURL string            `json:"upstream_url"`
	Config      repoconfig.Config `json:"config"`
	Disabled    bool              `json:"disabled"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

func toDTO(r meta.Repository) (repositoryDTO, error) {
	cfg, err := repoconfig.Parse(r.ConfigJSON)
	if err != nil {
		return repositoryDTO{}, err
	}
	return repositoryDTO{
		ID:          r.ID,
		Name:        r.Name,
		Format:      r.Format,
		Type:        r.Type,
		UpstreamURL: r.UpstreamURL,
		Config:      cfg,
		Disabled:    r.Disabled,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}, nil
}

// repositoryListItemDTO augments a repository with artifact aggregates for the
// list endpoint (the detail endpoint omits them).
type repositoryListItemDTO struct {
	repositoryDTO
	ArtifactCount int64 `json:"artifact_count"`
	TotalSize     int64 `json:"total_size"`
	// PendingApprovalCount is the number of packages awaiting approval in this
	// repository (0 when none or when approval is not configured).
	PendingApprovalCount int64 `json:"pending_approval_count"`
}

type createRepositoryReq struct {
	Name        string             `json:"name"`
	Format      string             `json:"format"`
	Type        string             `json:"type"`
	UpstreamURL string             `json:"upstream_url"`
	Config      *repoconfig.Config `json:"config"`
}

var validFormats = map[string]bool{
	meta.FormatMaven: true, meta.FormatNPM: true, meta.FormatCargo: true, meta.FormatGo: true,
	meta.FormatPyPI: true,
}

func (h *Handler) listRepositories(w http.ResponseWriter, r *http.Request) {
	repos, err := h.store.ListRepositories(r.Context())
	if err != nil {
		mapError(w, err)
		return
	}
	stats, err := h.store.AllRepoStats(r.Context())
	if err != nil {
		mapError(w, err)
		return
	}
	pending, err := h.store.PendingApprovalCountByRepo(r.Context())
	if err != nil {
		mapError(w, err)
		return
	}
	// Non-admins see only repositories they can read. Admins (Can returns true
	// for every repo) and the authz-disabled case (tests) see all.
	p := auth.FromContext(r.Context())
	out := make([]repositoryListItemDTO, 0, len(repos))
	for _, repo := range repos {
		if h.authz != nil && (p == nil || !p.Can(repo.Name, auth.ActionRead)) {
			continue
		}
		dto, err := toDTO(repo)
		if err != nil {
			mapError(w, err)
			return
		}
		st := stats[repo.ID]
		out = append(out, repositoryListItemDTO{
			repositoryDTO:        dto,
			ArtifactCount:        st.ArtifactCount,
			TotalSize:            st.TotalSize,
			PendingApprovalCount: pending[repo.Name],
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// repositoryNameDTO is the slim shape returned to any authenticated user for
// token-scope autocomplete: names only, no config, upstream URLs or stats.
type repositoryNameDTO struct {
	Name   string `json:"name"`
	Format string `json:"format"`
	Type   string `json:"type"`
}

// listRepositoryNames returns repository names so the token-creation UI can
// autocomplete scope patterns. Unlike listRepositories it is available to every
// authenticated user (scoping a token requires knowing the repository names),
// and deliberately exposes no configuration or upstream details.
func (h *Handler) listRepositoryNames(w http.ResponseWriter, r *http.Request) {
	repos, err := h.store.ListRepositories(r.Context())
	if err != nil {
		mapError(w, err)
		return
	}
	out := make([]repositoryNameDTO, 0, len(repos))
	for _, repo := range repos {
		out = append(out, repositoryNameDTO{Name: repo.Name, Format: repo.Format, Type: repo.Type})
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) createRepository(w http.ResponseWriter, r *http.Request) {
	var req createRepositoryReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if !validName(req.Name) {
		writeError(w, http.StatusBadRequest, "invalid repository name: "+nameRuleMsg)
		return
	}
	if !validFormats[req.Format] {
		writeError(w, http.StatusBadRequest, "invalid format")
		return
	}
	if req.Type != meta.TypeHosted && req.Type != meta.TypeProxy && req.Type != meta.TypeGroup {
		writeError(w, http.StatusBadRequest, "invalid type (hosted|proxy|group)")
		return
	}
	if req.Type == meta.TypeProxy && strings.TrimSpace(req.UpstreamURL) == "" {
		writeError(w, http.StatusBadRequest, "proxy repository requires upstream_url")
		return
	}

	cfg := repoconfig.Default()
	if req.Config != nil {
		cfg = *req.Config
	}
	if err := cfg.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Type == meta.TypeGroup {
		req.UpstreamURL = ""
		if err := repopkg.ValidateGroupMembers(r.Context(), h.store, req.Format, cfg.Group.Members); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	} else if len(cfg.Group.Members) > 0 {
		writeError(w, http.StatusBadRequest, "group members are only valid for group repositories")
		return
	}
	if cfg.Approval.Enabled && req.Type != meta.TypeProxy {
		writeError(w, http.StatusBadRequest, "approval is only valid for proxy repositories")
		return
	}
	cfgJSON, err := cfg.JSON()
	if err != nil {
		mapError(w, err)
		return
	}

	repo, err := h.store.CreateRepository(r.Context(), meta.Repository{
		Name:        req.Name,
		Format:      req.Format,
		Type:        req.Type,
		UpstreamURL: strings.TrimSpace(req.UpstreamURL),
		ConfigJSON:  cfgJSON,
	})
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			writeError(w, http.StatusConflict, "repository name already exists")
			return
		}
		mapError(w, err)
		return
	}
	dto, err := toDTO(repo)
	if err != nil {
		mapError(w, err)
		return
	}
	h.audit(r, repo.Name, meta.EventRepoCreate, http.StatusCreated)
	writeJSON(w, http.StatusCreated, dto)
}

// canReadRepo enforces read access to a single repository for non-admins.
// Admins (Can returns true for every repo) and the authz-disabled case (tests)
// pass. It writes 403 and returns false when denied.
func (h *Handler) canReadRepo(w http.ResponseWriter, r *http.Request, repoName string) bool {
	if h.authz == nil {
		return true
	}
	p := auth.FromContext(r.Context())
	if p == nil || !p.Can(repoName, auth.ActionRead) {
		writeError(w, http.StatusForbidden, "forbidden")
		return false
	}
	return true
}

func (h *Handler) getRepository(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	repo, err := h.store.GetRepository(r.Context(), id)
	if err != nil {
		mapError(w, err)
		return
	}
	if !h.canReadRepo(w, r, repo.Name) {
		return
	}
	dto, err := toDTO(repo)
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto)
}

type updateRepositoryReq struct {
	UpstreamURL string            `json:"upstream_url"`
	Config      repoconfig.Config `json:"config"`
}

func (h *Handler) updateRepository(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req updateRepositoryReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if err := req.Config.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	existing, err := h.store.GetRepository(r.Context(), id)
	if err != nil {
		mapError(w, err)
		return
	}
	if existing.Type == meta.TypeGroup {
		if err := repopkg.ValidateGroupMembers(r.Context(), h.store, existing.Format, req.Config.Group.Members); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	} else if len(req.Config.Group.Members) > 0 {
		writeError(w, http.StatusBadRequest, "group members are only valid for group repositories")
		return
	}
	if req.Config.Approval.Enabled && existing.Type != meta.TypeProxy {
		writeError(w, http.StatusBadRequest, "approval is only valid for proxy repositories")
		return
	}
	// PUT replaces the whole resource, so an omitted upstream_url would otherwise
	// zero out a proxy's upstream and silently break it. Mirror createRepository's
	// guard.
	if existing.Type == meta.TypeProxy && strings.TrimSpace(req.UpstreamURL) == "" {
		writeError(w, http.StatusBadRequest, "proxy repository requires upstream_url")
		return
	}
	cfgJSON, err := req.Config.JSON()
	if err != nil {
		mapError(w, err)
		return
	}
	if err := h.store.UpdateRepositoryConfig(r.Context(), id, strings.TrimSpace(req.UpstreamURL), cfgJSON); err != nil {
		mapError(w, err)
		return
	}
	repo, err := h.store.GetRepository(r.Context(), id)
	if err != nil {
		mapError(w, err)
		return
	}
	dto, err := toDTO(repo)
	if err != nil {
		mapError(w, err)
		return
	}
	h.audit(r, repo.Name, meta.EventRepoUpdate, http.StatusOK)
	writeJSON(w, http.StatusOK, dto)
}

type setDisabledReq struct {
	Disabled bool `json:"disabled"`
}

// setRepositoryDisabled toggles a repository online/offline. A disabled
// repository keeps its config and artifacts but stops serving the package
// protocols (503), so it can be re-enabled later.
func (h *Handler) setRepositoryDisabled(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req setDisabledReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	repo, err := h.store.GetRepository(r.Context(), id)
	if err != nil {
		mapError(w, err)
		return
	}
	if err := h.store.SetRepositoryDisabled(r.Context(), id, req.Disabled); err != nil {
		mapError(w, err)
		return
	}
	h.audit(r, repo.Name, meta.EventRepoUpdate, http.StatusOK)
	repo, err = h.store.GetRepository(r.Context(), id)
	if err != nil {
		mapError(w, err)
		return
	}
	dto, err := toDTO(repo)
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dto)
}

type repoPermissionDTO struct {
	RoleID    int64    `json:"role_id"`
	Role      string   `json:"role"`
	Pattern   string   `json:"repo_pattern"`
	Actions   []string `json:"actions"`
	UserCount int      `json:"user_count"`
}

// repositoryPermissions lists the role permissions that grant access to this
// repository: every permission whose repo pattern matches the repository name,
// with the granting role, the matched pattern, the actions, and how many users
// hold that role. Admin-only (registered under the admin route group).
func (h *Handler) repositoryPermissions(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	repo, err := h.store.GetRepository(r.Context(), id)
	if err != nil {
		mapError(w, err)
		return
	}
	roles, err := h.store.ListRoles(r.Context())
	if err != nil {
		mapError(w, err)
		return
	}
	perms, err := h.store.ListPermissions(r.Context())
	if err != nil {
		mapError(w, err)
		return
	}
	rolesBy, err := h.store.RolesByUser(r.Context())
	if err != nil {
		mapError(w, err)
		return
	}
	roleName := make(map[int64]string, len(roles))
	for _, role := range roles {
		roleName[role.ID] = role.Name
	}
	userCount := map[int64]int{}
	for _, urs := range rolesBy {
		for _, ur := range urs {
			userCount[ur.ID]++
		}
	}
	out := make([]repoPermissionDTO, 0)
	for _, p := range perms {
		if !auth.MatchRepoPattern(p.RepoPattern, repo.Name) {
			continue
		}
		out = append(out, repoPermissionDTO{
			RoleID:    p.RoleID,
			Role:      roleName[p.RoleID],
			Pattern:   p.RepoPattern,
			Actions:   strings.Split(p.Actions, ","),
			UserCount: userCount[p.RoleID],
		})
	}
	writeJSON(w, http.StatusOK, out)
}

type repoTokenDTO struct {
	TokenID    int64      `json:"token_id"`
	Name       string     `json:"name"`
	Owner      string     `json:"owner"`
	Pattern    string     `json:"repo_pattern"`
	Actions    []string   `json:"actions"`
	Unscoped   bool       `json:"unscoped"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

// repositoryTokens lists personal access tokens that can reach this repository:
// tokens with a scope whose pattern matches the repo (scoped grant), plus
// unscoped tokens (which inherit the owner's role access to any repo). The
// effective access of a scoped token is still bounded by its owner's roles.
// Admin-only.
func (h *Handler) repositoryTokens(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	repo, err := h.store.GetRepository(r.Context(), id)
	if err != nil {
		mapError(w, err)
		return
	}
	tokens, err := h.store.ListAllTokens(r.Context())
	if err != nil {
		mapError(w, err)
		return
	}
	users, err := h.store.ListUsers(r.Context())
	if err != nil {
		mapError(w, err)
		return
	}
	owner := make(map[int64]string, len(users))
	for _, u := range users {
		owner[u.ID] = u.Username
	}
	out := make([]repoTokenDTO, 0)
	for _, t := range tokens {
		var scopes []struct {
			RepoPattern string   `json:"repo_pattern"`
			Actions     []string `json:"actions"`
		}
		_ = json.Unmarshal([]byte(t.ScopesJSON), &scopes)
		if len(scopes) == 0 {
			// Unscoped: inherits the owner's role access to any repository.
			out = append(out, repoTokenDTO{
				TokenID: t.ID, Name: t.Name, Owner: owner[t.UserID], Pattern: "*",
				Unscoped: true, ExpiresAt: t.ExpiresAt, LastUsedAt: t.LastUsedAt,
			})
			continue
		}
		matched := ""
		seen := map[string]bool{}
		var actions []string
		for _, sc := range scopes {
			if !auth.MatchRepoPattern(sc.RepoPattern, repo.Name) {
				continue
			}
			matched = sc.RepoPattern
			for _, a := range sc.Actions {
				if !seen[a] {
					seen[a] = true
					actions = append(actions, a)
				}
			}
		}
		if matched == "" {
			continue
		}
		out = append(out, repoTokenDTO{
			TokenID: t.ID, Name: t.Name, Owner: owner[t.UserID], Pattern: matched,
			Actions: actions, ExpiresAt: t.ExpiresAt, LastUsedAt: t.LastUsedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

type artifactDTO struct {
	Path           string     `json:"path"`
	Version        string     `json:"version"`
	Size           int64      `json:"size"`
	ContentType    string     `json:"content_type"`
	PublishedAt    *time.Time `json:"published_at"`
	CachedAt       time.Time  `json:"cached_at"`
	LastAccessedAt time.Time  `json:"last_accessed_at"`
	// Vulnerability scan result for this version, when scanned. MaxSeverity is
	// empty/"none" when clean or not yet scanned. VulnCounts is the per-severity
	// advisory breakdown that powers the segmented severity bar (same shape the
	// approvals view uses).
	MaxSeverity string         `json:"max_severity,omitempty"`
	VulnIDs     []string       `json:"vuln_ids,omitempty"`
	VulnCounts  map[string]int `json:"vuln_counts,omitempty"`
	// Provenance of the scan: the advisory source (e.g. "OSV") and when it ran.
	VulnSource    string     `json:"vuln_source,omitempty"`
	VulnScannedAt *time.Time `json:"vuln_scanned_at,omitempty"`
	// Resolved SPDX license(s) for this version, when resolved. Empty when not
	// yet resolved or when the source reports no license. LicenseSource names the
	// data source (e.g. "deps.dev") and LicenseResolvedAt when it ran.
	Licenses                 []string   `json:"licenses,omitempty"`
	LicenseSource            string     `json:"license_source,omitempty"`
	LicenseResolvedAt        *time.Time `json:"license_resolved_at,omitempty"`
	ArtifactScanStatus       string     `json:"artifact_scan_status,omitempty"`
	ArtifactScanSeverity     string     `json:"artifact_scan_max_severity,omitempty"`
	ArtifactScanScanner      string     `json:"artifact_scan_scanner,omitempty"`
	ArtifactScanScannedAt    *time.Time `json:"artifact_scan_scanned_at,omitempty"`
	ArtifactScanFindingCount int        `json:"artifact_scan_finding_count,omitempty"`
}

type artifactScanJobDTO struct {
	JobID      string `json:"job_id"`
	Status     string `json:"status"`
	Scanner    string `json:"scanner"`
	BlobSHA256 string `json:"blob_sha256"`
}

type artifactScanBatchReq struct {
	Paths []string `json:"paths"`
}

type artifactScanBatchDTO struct {
	Queued int                  `json:"queued"`
	Jobs   []artifactScanJobDTO `json:"jobs"`
}

type artifactScanResultDTO struct {
	Path                  string                   `json:"path"`
	Status                string                   `json:"status"`
	Scanner               string                   `json:"scanner,omitempty"`
	ScannerVersion        string                   `json:"scanner_version,omitempty"`
	DatabaseSchemaVersion string                   `json:"database_schema_version,omitempty"`
	DatabaseBuiltAt       *time.Time               `json:"database_built_at,omitempty"`
	MaxSeverity           string                   `json:"max_severity,omitempty"`
	FindingCount          int                      `json:"finding_count"`
	Error                 string                   `json:"error,omitempty"`
	ScannedAt             *time.Time               `json:"scanned_at,omitempty"`
	Findings              []artifactScanFindingDTO `json:"findings,omitempty"`
}

type artifactScanFindingDTO struct {
	VulnerabilityID string   `json:"vulnerability_id"`
	Severity        string   `json:"severity"`
	PackageName     string   `json:"package_name"`
	PackageVersion  string   `json:"package_version,omitempty"`
	PackageType     string   `json:"package_type,omitempty"`
	PackagePURL     string   `json:"package_purl,omitempty"`
	FixedVersions   []string `json:"fixed_versions,omitempty"`
	Source          string   `json:"source,omitempty"`
	SourceURL       string   `json:"source_url,omitempty"`
	MatchType       string   `json:"match_type,omitempty"`
}

// listArtifacts returns the artifacts stored (hosted or cached) in a repository,
// powering the Nexus-style artifact browser in the UI.
func (h *Handler) listArtifacts(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	repo, err := h.store.GetRepository(r.Context(), id)
	if err != nil {
		mapError(w, err)
		return
	}
	if !h.canReadRepo(w, r, repo.Name) {
		return
	}
	prefix := r.URL.Query().Get("prefix")
	limit := queryInt(r, "limit", 100)
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	offset := queryInt(r, "offset", 0)
	if offset < 0 {
		offset = 0
	}
	arts, err := h.store.ListRepoArtifactsPage(r.Context(), id, prefix, limit, offset)
	if err != nil {
		mapError(w, err)
		return
	}
	count, _ := h.store.CountRepoArtifacts(r.Context(), id, prefix)
	size, _ := h.store.RepoSize(r.Context(), id)
	cfg, _ := repoconfig.Parse(repo.ConfigJSON)
	artifactScanScanner := cfg.ArtifactScan.EffectiveScanner()
	artifactScanConfigHash := cfg.ArtifactScan.ConfigHash
	out := make([]artifactDTO, 0, len(arts))
	for _, a := range arts {
		dto := artifactDTO{
			Path: a.Path, Version: a.Version, Size: a.Size, ContentType: a.ContentType,
			PublishedAt: a.PublishedAt, CachedAt: a.CachedAt, LastAccessedAt: a.LastAccessedAt,
		}
		// Attach the stored vulnerability scan for this coordinate, if any.
		if a.Version != "" {
			if eco, pkg := repopkg.VulnCoordinate(repo.Format, a.Path); pkg != "" {
				if scan, serr := h.store.GetVulnScan(r.Context(), eco, pkg, a.Version); serr == nil {
					dto.MaxSeverity = scan.MaxSeverity
					dto.VulnIDs = scan.VulnIDs
					dto.VulnCounts = scan.SeverityCounts
					dto.VulnSource = scan.Source
					if !scan.ScannedAt.IsZero() {
						scannedAt := scan.ScannedAt
						dto.VulnScannedAt = &scannedAt
					}
				}
			}
			// Attach the stored license resolution for this coordinate, if any.
			if system, pkg := repopkg.LicenseCoordinate(repo.Format, a.Path); pkg != "" {
				if ls, lerr := h.store.GetLicenseScan(r.Context(), system, pkg, a.Version); lerr == nil {
					dto.Licenses = ls.Licenses
					dto.LicenseSource = ls.Source
					if !ls.ResolvedAt.IsZero() {
						resolvedAt := ls.ResolvedAt
						dto.LicenseResolvedAt = &resolvedAt
					}
				}
			}
		}
		if job, jerr := h.store.LatestArtifactScanJob(r.Context(), a.BlobSHA256, artifactScanScanner, artifactScanConfigHash); jerr == nil &&
			(job.Status == artifactscan.StatusQueued || job.Status == artifactscan.StatusRunning) {
			dto.ArtifactScanStatus = string(job.Status)
			dto.ArtifactScanScanner = job.Scanner
		} else if res, serr := h.store.LatestArtifactScanResult(r.Context(), a.BlobSHA256, artifactScanScanner, artifactScanConfigHash); serr == nil {
			dto.ArtifactScanStatus = string(res.Status)
			dto.ArtifactScanSeverity = string(res.MaxSeverity)
			dto.ArtifactScanScanner = res.Scanner
			dto.ArtifactScanFindingCount = len(res.Findings)
			if !res.ScannedAt.IsZero() {
				scannedAt := res.ScannedAt
				dto.ArtifactScanScannedAt = &scannedAt
			}
		} else if job, jerr := h.store.LatestArtifactScanJob(r.Context(), a.BlobSHA256, artifactScanScanner, artifactScanConfigHash); jerr == nil {
			dto.ArtifactScanStatus = string(job.Status)
			dto.ArtifactScanScanner = job.Scanner
		}
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"count": count, "total_size": size, "limit": limit, "offset": offset, "artifacts": out,
	})
}

// getArtifactScan returns the latest artifact-byte scan details for one stored
// artifact, including normalized findings when a completed result exists.
func (h *Handler) getArtifactScan(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	repo, err := h.store.GetRepository(r.Context(), id)
	if err != nil {
		mapError(w, err)
		return
	}
	if !h.canReadRepo(w, r, repo.Name) {
		return
	}
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		writeError(w, http.StatusBadRequest, "path required")
		return
	}
	artifact, err := h.store.GetArtifact(r.Context(), id, path)
	if err != nil {
		mapError(w, err)
		return
	}
	cfg, _ := repoconfig.Parse(repo.ConfigJSON)
	scanner := cfg.ArtifactScan.EffectiveScanner()
	configHash := cfg.ArtifactScan.ConfigHash

	if job, jerr := h.store.LatestArtifactScanJob(r.Context(), artifact.BlobSHA256, scanner, configHash); jerr == nil &&
		(job.Status == artifactscan.StatusQueued || job.Status == artifactscan.StatusRunning) {
		writeJSON(w, http.StatusOK, artifactScanResultDTO{
			Path:    artifact.Path,
			Status:  string(job.Status),
			Scanner: job.Scanner,
		})
		return
	}
	res, err := h.store.LatestArtifactScanResult(r.Context(), artifact.BlobSHA256, scanner, configHash)
	if err != nil {
		mapError(w, err)
		return
	}
	out := artifactScanResultDTO{
		Path:                  artifact.Path,
		Status:                string(res.Status),
		Scanner:               res.Scanner,
		ScannerVersion:        res.ScannerVersion,
		DatabaseSchemaVersion: res.DatabaseSchemaVersion,
		MaxSeverity:           string(res.MaxSeverity),
		FindingCount:          len(res.Findings),
		Error:                 res.Error,
	}
	if !res.DatabaseBuiltAt.IsZero() {
		t := res.DatabaseBuiltAt
		out.DatabaseBuiltAt = &t
	}
	if !res.ScannedAt.IsZero() {
		t := res.ScannedAt
		out.ScannedAt = &t
	}
	for _, f := range res.Findings {
		out.Findings = append(out.Findings, artifactScanFindingDTO{
			VulnerabilityID: stringOrUnknown(f.VulnerabilityID),
			Severity:        string(f.Severity),
			PackageName:     f.PackageName,
			PackageVersion:  f.PackageVersion,
			PackageType:     f.PackageType,
			PackagePURL:     f.PackagePURL,
			FixedVersions:   f.FixedVersions,
			Source:          f.Source,
			SourceURL:       f.SourceURL,
			MatchType:       f.MatchType,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// scanArtifact enqueues a manual artifact-byte scan for one stored artifact.
// This does not execute scanner tools in the API process; an isolated scanner
// worker claims the queued job through /internal/scans.
func (h *Handler) scanArtifact(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	repo, err := h.store.GetRepository(r.Context(), id)
	if err != nil {
		mapError(w, err)
		return
	}
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		writeError(w, http.StatusBadRequest, "path required")
		return
	}
	artifact, err := h.store.GetArtifact(r.Context(), id, path)
	if err != nil {
		mapError(w, err)
		return
	}
	cfg, _ := repoconfig.Parse(repo.ConfigJSON)
	jobID, err := randomArtifactScanJobID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	job, err := h.store.EnqueueArtifactScan(r.Context(), jobID, artifact.BlobSHA256,
		cfg.ArtifactScan.EffectiveScanner(), cfg.ArtifactScan.ConfigHash, time.Now().UTC())
	if err != nil {
		mapError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, artifactScanJobDTO{
		JobID:      job.ID,
		Status:     string(job.Status),
		Scanner:    job.Scanner,
		BlobSHA256: job.BlobSHA256,
	})
}

func (h *Handler) scanArtifactsBatch(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	repo, err := h.store.GetRepository(r.Context(), id)
	if err != nil {
		mapError(w, err)
		return
	}
	var req artifactScanBatchReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	if len(req.Paths) == 0 {
		writeError(w, http.StatusBadRequest, "paths required")
		return
	}
	if len(req.Paths) > 100 {
		writeError(w, http.StatusBadRequest, "too many paths; select at most 100")
		return
	}
	cfg, _ := repoconfig.Parse(repo.ConfigJSON)
	out := artifactScanBatchDTO{Jobs: make([]artifactScanJobDTO, 0, len(req.Paths))}
	seen := map[string]bool{}
	for _, rawPath := range req.Paths {
		path := strings.TrimSpace(rawPath)
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		artifact, err := h.store.GetArtifact(r.Context(), id, path)
		if err != nil {
			mapError(w, err)
			return
		}
		jobID, err := randomArtifactScanJobID()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		job, err := h.store.EnqueueArtifactScan(r.Context(), jobID, artifact.BlobSHA256,
			cfg.ArtifactScan.EffectiveScanner(), cfg.ArtifactScan.ConfigHash, time.Now().UTC())
		if err != nil {
			mapError(w, err)
			return
		}
		out.Queued++
		out.Jobs = append(out.Jobs, artifactScanJobDTO{
			JobID:      job.ID,
			Status:     string(job.Status),
			Scanner:    job.Scanner,
			BlobSHA256: job.BlobSHA256,
		})
	}
	writeJSON(w, http.StatusAccepted, out)
}

// deleteArtifact removes artifacts from a repository (admin only, via the
// RequireAdmin route group). With a "path" query parameter it deletes that one
// artifact; without it, it purges every artifact in the repository (the Danger
// Zone "purge all" action). Blob bytes are reclaimed asynchronously by the
// sweeper once their reference count reaches zero.
func (h *Handler) deleteArtifact(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	repo, err := h.store.GetRepository(r.Context(), id)
	if err != nil {
		mapError(w, err)
		return
	}

	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		// No path: purge the whole repository's artifacts.
		n, err := h.store.PurgeArtifacts(r.Context(), id)
		if err != nil {
			mapError(w, err)
			return
		}
		h.auditArtifact(r, repo.Name, "(all artifacts)", http.StatusOK)
		writeJSON(w, http.StatusOK, map[string]any{"deleted": n})
		return
	}

	if err := h.store.DeleteArtifact(r.Context(), id, path); err != nil {
		mapError(w, err)
		return
	}
	h.auditArtifact(r, repo.Name, path, http.StatusNoContent)
	w.WriteHeader(http.StatusNoContent)
}

// auditArtifact records an artifact deletion in the repository's audit log,
// with the artifact path (or "(all artifacts)") in the path column.
func (h *Handler) auditArtifact(r *http.Request, repoName, path string, status int) {
	h.rec.Record(audit.Event{
		Repo:      repoName,
		Action:    meta.EventDelete,
		Path:      path,
		Username:  principalName(r),
		Method:    r.Method,
		Status:    status,
		ClientIP:  audit.ClientIP(r),
		UserAgent: r.UserAgent(),
	})
}

func randomArtifactScanJobID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func stringOrUnknown(s string) string {
	if strings.TrimSpace(s) == "" {
		return "unknown"
	}
	return s
}

func queryInt(r *http.Request, key string, def int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return n
}

// upstreamHealthTimeout bounds the whole probe handler — the metadata read plus
// the outbound probe — with a deadline the server owns. The metadata DB uses a
// single SQLite connection (SetMaxOpenConns(1)), so a contended connection would
// otherwise block GetRepository indefinitely (the request context has no
// deadline of its own), leaving the client's "checking…" badge spinning. Set
// slightly above the 5s probe client timeout so a genuinely slow upstream still
// resolves via the probe rather than tripping this guard.
const upstreamHealthTimeout = 8 * time.Second

// upstreamHealth probes a proxy repository's upstream with a short timeout. Any
// HTTP response (even 4xx) means the upstream is reachable; only transport
// errors are treated as unreachable.
func (h *Handler) upstreamHealth(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), upstreamHealthTimeout)
	defer cancel()
	repo, err := h.store.GetRepository(ctx, id)
	if err != nil {
		mapError(w, err)
		return
	}
	if repo.Type != meta.TypeProxy || repo.UpstreamURL == "" {
		writeJSON(w, http.StatusOK, map[string]any{"applicable": false})
		return
	}
	writeJSON(w, http.StatusOK, h.probeUpstream(ctx, repo.UpstreamURL))
}

type checkUpstreamReq struct {
	URL string `json:"url"`
}

// checkUpstream probes an arbitrary upstream URL so the New repository form can
// validate connectivity before the repository is created. Admin-only (the route
// group), mirroring upstreamHealth's reachability semantics.
func (h *Handler) checkUpstream(w http.ResponseWriter, r *http.Request) {
	var req checkUpstreamReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	raw := strings.TrimSpace(req.URL)
	if raw == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	if u, err := url.Parse(raw); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"applicable": true, "reachable": false, "error": "url must be http(s) with a host",
		})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), upstreamHealthTimeout)
	defer cancel()
	writeJSON(w, http.StatusOK, h.probeUpstream(ctx, raw))
}

// probeUpstream issues a short GET to rawURL and reports reachability, the HTTP
// status, and latency. Any HTTP response (even 4xx) counts as reachable; only a
// transport error is unreachable.
func (h *Handler) probeUpstream(ctx context.Context, rawURL string) map[string]any {
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return map[string]any{"applicable": true, "reachable": false, "error": "bad upstream url"}
	}
	resp, err := h.client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return map[string]any{"applicable": true, "reachable": false, "latency_ms": latency, "error": err.Error()}
	}
	defer resp.Body.Close()
	return map[string]any{"applicable": true, "reachable": true, "status": resp.StatusCode, "latency_ms": latency}
}

func (h *Handler) deleteRepository(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	// Resolve the name before deletion so the audit entry can reference it.
	repo, err := h.store.GetRepository(r.Context(), id)
	if err != nil {
		mapError(w, err)
		return
	}
	if err := h.store.DeleteRepository(r.Context(), id); err != nil {
		mapError(w, err)
		return
	}
	// Approvals and version denies must not outlive the repo: a recreated
	// same-name repo would silently inherit its trust decisions.
	if err := h.store.DeleteApprovalsForRepo(r.Context(), repo.Name); err != nil {
		h.log.Error("delete approvals for repo failed", "repo", repo.Name, "err", err)
	}
	if err := h.store.DeleteVersionDeniesForRepo(r.Context(), repo.Name); err != nil {
		h.log.Error("delete version denies for repo failed", "repo", repo.Name, "err", err)
	}
	h.audit(r, repo.Name, meta.EventRepoDelete, http.StatusNoContent)
	w.WriteHeader(http.StatusNoContent)
}

// auditLogDTO is the JSON shape for one audit log entry.
type auditLogDTO struct {
	ID        int64     `json:"id"`
	Event     string    `json:"event"`
	Path      string    `json:"path"`
	Username  string    `json:"username"`
	Method    string    `json:"method"`
	Status    int       `json:"status"`
	ClientIP  string    `json:"client_ip"`
	UserAgent string    `json:"user_agent"`
	CreatedAt time.Time `json:"created_at"`
}

// listAuditLogs returns a repository's audit log, newest first, with optional
// event filtering and limit/offset pagination.
func (h *Handler) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	repo, err := h.store.GetRepository(r.Context(), id)
	if err != nil {
		mapError(w, err)
		return
	}

	q := r.URL.Query()
	event := q.Get("event")
	limit := intParam(q.Get("limit"), 100)
	if limit < 1 || limit > 500 {
		limit = 100
	}
	offset := max(intParam(q.Get("offset"), 0), 0)

	logs, err := h.store.ListAuditLogs(r.Context(), repo.Name, event, limit, offset)
	if err != nil {
		mapError(w, err)
		return
	}
	count, err := h.store.CountAuditLogs(r.Context(), repo.Name, event)
	if err != nil {
		mapError(w, err)
		return
	}
	out := make([]auditLogDTO, 0, len(logs))
	for _, l := range logs {
		out = append(out, auditLogDTO{
			ID: l.ID, Event: l.Event, Path: l.Path, Username: l.Username, Method: l.Method,
			Status: l.Status, ClientIP: l.ClientIP, UserAgent: l.UserAgent, CreatedAt: l.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": count, "logs": out})
}

func intParam(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := parseID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

func parseID(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
