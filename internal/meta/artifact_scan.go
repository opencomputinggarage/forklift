package meta

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/younsl/o/box/kubernetes/forklift/internal/artifactscan"
	"github.com/younsl/o/box/kubernetes/forklift/internal/packagecoord"
	"github.com/younsl/o/box/kubernetes/forklift/internal/repoconfig"
)

// EnsureArtifactScannerProfile inserts or updates one scanner profile.
func (s *Store) EnsureArtifactScannerProfile(ctx context.Context, p artifactscan.Profile) error {
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = now
	}
	_, err := s.h().ExecContext(ctx,
		`INSERT INTO artifact_scanner_profiles(
             name, scanner, mode, config_hash, runtime_class_name,
             max_artifact_bytes, max_extracted_bytes, max_files, store_sbom,
             created_at, updated_at
         ) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
         ON CONFLICT(name) DO UPDATE SET
             scanner = excluded.scanner,
             mode = excluded.mode,
             config_hash = excluded.config_hash,
             runtime_class_name = excluded.runtime_class_name,
             max_artifact_bytes = excluded.max_artifact_bytes,
             max_extracted_bytes = excluded.max_extracted_bytes,
             max_files = excluded.max_files,
             store_sbom = excluded.store_sbom,
             updated_at = excluded.updated_at`,
		p.Name, p.Scanner, p.Mode, p.ConfigHash, p.RuntimeClassName,
		p.Limits.MaxArtifactBytes, p.Limits.MaxExtractedBytes, p.Limits.MaxFiles, boolInt(p.StoreSBOM),
		formatTime(p.CreatedAt), formatTime(p.UpdatedAt))
	return err
}

// GetArtifactScannerProfile returns a scanner profile by name.
func (s *Store) GetArtifactScannerProfile(ctx context.Context, name string) (artifactscan.Profile, error) {
	var p artifactscan.Profile
	var mode string
	var storeSBOM int
	var created, updated string
	err := s.h().QueryRowContext(ctx,
		`SELECT name, scanner, mode, config_hash, runtime_class_name,
                max_artifact_bytes, max_extracted_bytes, max_files, store_sbom,
                created_at, updated_at
           FROM artifact_scanner_profiles WHERE name = ?`, name).Scan(
		&p.Name, &p.Scanner, &mode, &p.ConfigHash, &p.RuntimeClassName,
		&p.Limits.MaxArtifactBytes, &p.Limits.MaxExtractedBytes, &p.Limits.MaxFiles, &storeSBOM,
		&created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return artifactscan.Profile{}, ErrNotFound
	}
	if err != nil {
		return artifactscan.Profile{}, err
	}
	p.Mode = artifactscan.ExecutionMode(mode)
	p.StoreSBOM = storeSBOM != 0
	p.CreatedAt = parseTime(created)
	p.UpdatedAt = parseTime(updated)
	return p, nil
}

// EnqueueArtifactScan creates a queued scan job for one blob and profile.
func (s *Store) EnqueueArtifactScan(ctx context.Context, id, blobSHA256, profileName string, now time.Time) (artifactscan.Job, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	profile, err := s.GetArtifactScannerProfile(ctx, profileName)
	if err != nil {
		return artifactscan.Job{}, err
	}
	_, err = s.h().ExecContext(ctx,
		`INSERT INTO artifact_scan_jobs(
             id, blob_sha256, scanner_profile, scanner, scanner_config_hash, status,
             next_run_at, created_at, max_artifact_bytes, max_extracted_bytes, max_files, store_sbom
         ) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, blobSHA256, profile.Name, profile.Scanner, profile.ConfigHash, artifactscan.JobQueued,
		formatTime(now), formatTime(now),
		profile.Limits.MaxArtifactBytes, profile.Limits.MaxExtractedBytes, profile.Limits.MaxFiles, boolInt(profile.StoreSBOM))
	if err != nil {
		return artifactscan.Job{}, err
	}
	return s.GetArtifactScanJob(ctx, id)
}

// GetArtifactScanJob returns one artifact scan job by id.
func (s *Store) GetArtifactScanJob(ctx context.Context, id string) (artifactscan.Job, error) {
	return scanArtifactScanJobRow(s.h().QueryRowContext(ctx,
		`SELECT id, blob_sha256, scanner_profile, scanner, scanner_config_hash, status,
                worker_id, attempts, lease_until, next_run_at, last_heartbeat_at,
                error, created_at, started_at, finished_at,
                max_artifact_bytes, max_extracted_bytes, max_files, store_sbom
           FROM artifact_scan_jobs WHERE id = ?`, id))
}

// LatestArtifactScanJob returns the newest job for a blob/profile.
func (s *Store) LatestArtifactScanJob(ctx context.Context, blobSHA256, profileName string) (artifactscan.Job, error) {
	return scanArtifactScanJobRow(s.h().QueryRowContext(ctx,
		`SELECT id, blob_sha256, scanner_profile, scanner, scanner_config_hash, status,
                worker_id, attempts, lease_until, next_run_at, last_heartbeat_at,
                error, created_at, started_at, finished_at,
                max_artifact_bytes, max_extracted_bytes, max_files, store_sbom
           FROM artifact_scan_jobs
          WHERE blob_sha256 = ? AND scanner_profile = ?
          ORDER BY created_at DESC LIMIT 1`,
		blobSHA256, profileName))
}

// ArtifactScanTargets returns repository artifact context for a blob.
func (s *Store) ArtifactScanTargets(ctx context.Context, blobSHA256 string) ([]artifactscan.Target, error) {
	rows, err := s.h().QueryContext(ctx,
		`SELECT r.id, r.name, r.format, r.type,
                a.id, a.path, a.version, a.size, a.content_type, a.metadata_json,
                a.published_at, a.cached_at, a.last_accessed_at, a.updated_at
           FROM artifacts a JOIN repositories r ON r.id = a.repo_id
          WHERE a.blob_sha256 = ?
          ORDER BY CASE WHEN a.version != '' THEN 0 ELSE 1 END, r.name, a.path`,
		blobSHA256)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []artifactscan.Target
	for rows.Next() {
		var t artifactscan.Target
		var published sql.NullString
		var cached, accessed, updated string
		if err := rows.Scan(&t.RepositoryID, &t.Repository, &t.Format, &t.Type,
			&t.ArtifactID, &t.Path, &t.Version, &t.Size, &t.ContentType, &t.MetadataJSON,
			&published, &cached, &accessed, &updated); err != nil {
			return nil, err
		}
		if published.Valid && published.String != "" {
			p := parseTime(published.String)
			t.PublishedAt = &p
		}
		t.CachedAt = parseTime(cached)
		t.LastAccessedAt = parseTime(accessed)
		t.UpdatedAt = parseTime(updated)
		c := packagecoord.FromArtifact(t.Format, t.Path, t.Version)
		if t.Version == "" {
			t.Version = c.Version
		}
		t.PackageName = c.PackageName
		t.Ecosystem = c.Ecosystem
		t.DepsDevSystem = c.DepsDevSystem
		t.PURL = c.PURL
		out = append(out, t)
	}
	return out, rows.Err()
}

// ClaimArtifactScanJob atomically claims the oldest due job matching a worker's
// scanner capabilities.
func (s *Store) ClaimArtifactScanJob(ctx context.Context, workerID string, capabilities []artifactscan.ScannerCapability, leaseUntil, now time.Time, maxAttempts int) (artifactscan.Job, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	scanners := capabilityScanners(capabilities)
	if len(scanners) == 0 {
		return artifactscan.Job{}, ErrNotFound
	}
	tx, err := s.h().BeginTx(ctx, nil)
	if err != nil {
		return artifactscan.Job{}, err
	}
	defer tx.Rollback()

	if err := upsertScannerCapabilities(ctx, tx, workerID, capabilities, now); err != nil {
		return artifactscan.Job{}, err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE artifact_scan_jobs
		    SET status = ?, finished_at = ?, error = ?
		  WHERE status = ?
		    AND lease_until IS NOT NULL
		    AND lease_until <= ?
		    AND attempts >= ?`,
		artifactscan.JobDead, formatTime(now), "scan job exceeded max attempts",
		artifactscan.JobRunning, formatTime(now), maxAttempts); err != nil {
		return artifactscan.Job{}, err
	}

	in, args := scannerInClause(scanners)
	args = append(args, artifactscan.JobQueued, formatTime(now), artifactscan.JobRunning, formatTime(now), maxAttempts, artifactscan.JobQueued)
	row := tx.QueryRowContext(ctx,
		fmt.Sprintf(`SELECT id FROM artifact_scan_jobs
	          WHERE scanner IN (%s)
	            AND ((status = ? AND next_run_at <= ?)
	              OR (status = ? AND lease_until IS NOT NULL AND lease_until <= ? AND attempts < ?))
	          ORDER BY CASE WHEN status = ? THEN 0 ELSE 1 END, created_at ASC LIMIT 1`, in),
		args...)
	var id string
	if err := row.Scan(&id); errors.Is(err, sql.ErrNoRows) {
		if err := tx.Commit(); err != nil {
			return artifactscan.Job{}, err
		}
		return artifactscan.Job{}, ErrNotFound
	} else if err != nil {
		return artifactscan.Job{}, err
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE artifact_scan_jobs
	            SET status = ?, worker_id = ?, attempts = attempts + 1,
	                lease_until = ?, last_heartbeat_at = ?, started_at = COALESCE(started_at, ?), error = ''
	          WHERE id = ?
	            AND (status = ? OR (status = ? AND lease_until IS NOT NULL AND lease_until <= ? AND attempts < ?))`,
		artifactscan.JobRunning, workerID, formatTime(leaseUntil), formatTime(now), formatTime(now), id,
		artifactscan.JobQueued, artifactscan.JobRunning, formatTime(now), maxAttempts)
	if err != nil {
		return artifactscan.Job{}, err
	}
	if err := tx.Commit(); err != nil {
		return artifactscan.Job{}, err
	}
	return s.GetArtifactScanJob(ctx, id)
}

// HeartbeatArtifactScanJob extends a running job lease for the current worker.
func (s *Store) HeartbeatArtifactScanJob(ctx context.Context, id, workerID string, leaseUntil, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	res, err := s.h().ExecContext(ctx,
		`UPDATE artifact_scan_jobs
            SET lease_until = ?, last_heartbeat_at = ?
          WHERE id = ? AND worker_id = ? AND status = ? AND lease_until > ?`,
		formatTime(leaseUntil), formatTime(now), id, workerID, artifactscan.JobRunning, formatTime(now))
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		var existing string
		if err := s.h().QueryRowContext(ctx,
			`SELECT status FROM artifact_scan_jobs WHERE id = ? AND worker_id = ?`, id, workerID).Scan(&existing); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if existing == string(artifactscan.JobRunning) {
			return ErrConflict
		}
		return ErrNotFound
	}
	return nil
}

// CompleteArtifactScanJob stores a terminal report and marks the job completed.
func (s *Store) CompleteArtifactScanJob(ctx context.Context, id, workerID string, result artifactscan.Result, now time.Time) (int64, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tx, err := s.h().BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var job artifactscan.Job
	var status, leaseUntil, nextRun, heartbeat, created, started, finished sql.NullString
	var storeSBOM int
	err = tx.QueryRowContext(ctx,
		`SELECT id, blob_sha256, scanner_profile, scanner, scanner_config_hash, status,
                worker_id, attempts, lease_until, next_run_at, last_heartbeat_at,
                error, created_at, started_at, finished_at,
                max_artifact_bytes, max_extracted_bytes, max_files, store_sbom
           FROM artifact_scan_jobs WHERE id = ? AND worker_id = ?`, id, workerID).Scan(
		&job.ID, &job.BlobSHA256, &job.ScannerProfile, &job.Scanner, &job.ScannerConfigHash, &status,
		&job.WorkerID, &job.Attempts, &leaseUntil, &nextRun, &heartbeat,
		&job.Error, &created, &started, &finished,
		&job.Limits.MaxArtifactBytes, &job.Limits.MaxExtractedBytes, &job.Limits.MaxFiles, &storeSBOM)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}
	if status.String != string(artifactscan.JobRunning) {
		return 0, ErrConflict
	}
	if !leaseUntil.Valid || !parseTime(leaseUntil.String).After(now) {
		return 0, ErrConflict
	}
	result.ScannerProfile = job.ScannerProfile
	result.ScannerConfigHash = job.ScannerConfigHash
	job.StoreSBOM = storeSBOM != 0
	providersJSON, err := json.Marshal(result.DatabaseProviders)
	if err != nil {
		return 0, err
	}
	dbBuilt := nullableTime(result.DatabaseBuiltAt)
	scannedAt := result.ScannedAt
	if scannedAt.IsZero() {
		scannedAt = now
	}
	res, err := tx.ExecContext(ctx,
		`INSERT INTO artifact_scan_results(
             job_id, blob_sha256, scanner_profile, scanner, scanner_version, scanner_config_hash,
             database_schema_version, database_built_at, database_providers_json,
             status, max_severity, finding_count, raw_result_digest, error, scanned_at, created_at
         ) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, result.BlobSHA256, result.ScannerProfile, result.Scanner, result.ScannerVersion, result.ScannerConfigHash,
		result.DatabaseSchemaVersion, dbBuilt, string(providersJSON),
		result.Status, result.MaxSeverity, len(result.Findings), result.RawResultDigest,
		result.Error, formatTime(scannedAt), formatTime(now))
	if err != nil {
		return 0, err
	}
	resultID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, f := range result.Findings {
		fixedJSON, err := json.Marshal(f.FixedVersions)
		if err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO artifact_scan_findings(
                 result_id, vulnerability_id, severity, package_name, package_version,
                 package_type, package_purl, fixed_versions, source, source_url, match_type
             ) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			resultID, f.VulnerabilityID, f.Severity, f.PackageName, f.PackageVersion,
			f.PackageType, f.PackagePURL, string(fixedJSON), f.Source, f.SourceURL, f.MatchType); err != nil {
			return 0, err
		}
	}
	if result.SBOM != nil {
		sbom := *result.SBOM
		if sbom.BlobSHA256 == "" {
			sbom.BlobSHA256 = result.BlobSHA256
		}
		sbom.ResultID = resultID
		if sbom.CreatedAt.IsZero() {
			sbom.CreatedAt = now
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO artifact_sboms(
                 blob_sha256, result_id, format, generator, generator_version,
                 content_digest, content_json, created_at
             ) VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
			sbom.BlobSHA256, sbom.ResultID, sbom.Format, sbom.Generator, sbom.GeneratorVersion,
			sbom.ContentDigest, sbom.ContentJSON, formatTime(sbom.CreatedAt)); err != nil {
			return 0, err
		}
	}
	jobStatus := artifactscan.JobCompleted
	if result.Status == artifactscan.ReportFailed {
		jobStatus = artifactscan.JobFailed
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE artifact_scan_jobs
            SET status = ?, finished_at = ?, error = ?
          WHERE id = ?`,
		jobStatus, formatTime(now), result.Error, id); err != nil {
		return 0, err
	}
	if err := recomputeArtifactScanVerdictsTx(ctx, tx, resultID, result, now); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return resultID, nil
}

// LatestArtifactScanResult returns the newest report for a blob/profile.
func (s *Store) LatestArtifactScanResult(ctx context.Context, blobSHA256, profileName string) (artifactscan.Result, error) {
	row := s.h().QueryRowContext(ctx,
		`SELECT id, job_id, blob_sha256, scanner_profile, scanner, scanner_version, scanner_config_hash,
                database_schema_version, database_built_at, database_providers_json, status,
                max_severity, finding_count, raw_result_digest, error, scanned_at, created_at
           FROM artifact_scan_results
          WHERE blob_sha256 = ? AND scanner_profile = ?
          ORDER BY scanned_at DESC, id DESC LIMIT 1`,
		blobSHA256, profileName)
	resultID, result, err := scanArtifactScanResultRow(row)
	if err != nil {
		return artifactscan.Result{}, err
	}
	findings, err := s.ListArtifactScanFindings(ctx, resultID)
	if err != nil {
		return artifactscan.Result{}, err
	}
	result.ID = resultID
	result.Findings = findings
	return result, nil
}

// ListArtifactScanFindings returns normalized findings for one scan result id.
func (s *Store) ListArtifactScanFindings(ctx context.Context, resultID int64) ([]artifactscan.Finding, error) {
	rows, err := s.h().QueryContext(ctx,
		`SELECT vulnerability_id, severity, package_name, package_version,
                package_type, package_purl, fixed_versions, source, source_url, match_type
           FROM artifact_scan_findings WHERE result_id = ? ORDER BY id`, resultID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []artifactscan.Finding
	for rows.Next() {
		var f artifactscan.Finding
		var fixedJSON string
		if err := rows.Scan(&f.VulnerabilityID, &f.Severity, &f.PackageName, &f.PackageVersion,
			&f.PackageType, &f.PackagePURL, &fixedJSON, &f.Source, &f.SourceURL, &f.MatchType); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(fixedJSON), &f.FixedVersions)
		out = append(out, f)
	}
	return out, rows.Err()
}

// LatestArtifactSBOM returns the newest stored SBOM for a blob/profile.
func (s *Store) LatestArtifactSBOM(ctx context.Context, blobSHA256, profileName string) (artifactscan.SBOM, error) {
	var sbom artifactscan.SBOM
	var created string
	err := s.h().QueryRowContext(ctx,
		`SELECT s.id, s.blob_sha256, s.result_id, s.format, s.generator,
                s.generator_version, s.content_digest, s.content_json, s.created_at
           FROM artifact_sboms s
           JOIN artifact_scan_results r ON r.id = s.result_id
          WHERE s.blob_sha256 = ? AND r.scanner_profile = ?
          ORDER BY s.created_at DESC, s.id DESC LIMIT 1`,
		blobSHA256, profileName).Scan(&sbom.ID, &sbom.BlobSHA256, &sbom.ResultID, &sbom.Format, &sbom.Generator,
		&sbom.GeneratorVersion, &sbom.ContentDigest, &sbom.ContentJSON, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return artifactscan.SBOM{}, ErrNotFound
	}
	if err != nil {
		return artifactscan.SBOM{}, err
	}
	sbom.CreatedAt = parseTime(created)
	return sbom, nil
}

// CreateArtifactSBOMExport records an external SBOM export request. Delivery is
// intentionally separate from scan result status.
func (s *Store) CreateArtifactSBOMExport(ctx context.Context, sbomID int64, destination string, now time.Time) (artifactscan.Export, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	res, err := s.h().ExecContext(ctx,
		`INSERT INTO artifact_scan_exports(
             sbom_id, destination, status, error, attempts, next_run_at, created_at, updated_at
         ) VALUES(?, ?, ?, '', 0, ?, ?, ?)`,
		sbomID, destination, "pending", formatTime(now), formatTime(now), formatTime(now))
	if err != nil {
		return artifactscan.Export{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return artifactscan.Export{}, err
	}
	return s.GetArtifactSBOMExport(ctx, id)
}

// GetArtifactSBOMExport returns one stored SBOM export request.
func (s *Store) GetArtifactSBOMExport(ctx context.Context, id int64) (artifactscan.Export, error) {
	var out artifactscan.Export
	var nextRun, created, updated string
	err := s.h().QueryRowContext(ctx,
		`SELECT id, sbom_id, destination, status, error, attempts, next_run_at, created_at, updated_at
           FROM artifact_scan_exports WHERE id = ?`, id).Scan(
		&out.ID, &out.SBOMID, &out.Destination, &out.Status, &out.Error, &out.Attempts,
		&nextRun, &created, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return artifactscan.Export{}, ErrNotFound
	}
	if err != nil {
		return artifactscan.Export{}, err
	}
	out.NextRunAt = parseTime(nextRun)
	out.CreatedAt = parseTime(created)
	out.UpdatedAt = parseTime(updated)
	return out, nil
}

// UpsertArtifactScanVerdict stores a repository-specific verdict.
func (s *Store) UpsertArtifactScanVerdict(ctx context.Context, v artifactscan.Verdict) (artifactscan.Verdict, error) {
	if v.ComputedAt.IsZero() {
		v.ComputedAt = time.Now().UTC()
	}
	_, err := s.h().ExecContext(ctx,
		`INSERT INTO artifact_scan_verdicts(
             repository_id, blob_sha256, result_id, scanner_profile, policy_hash,
             status, reason, max_severity, computed_at
         ) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
         ON CONFLICT(repository_id, blob_sha256, scanner_profile) DO UPDATE SET
             result_id = excluded.result_id,
             policy_hash = excluded.policy_hash,
             status = excluded.status,
             reason = excluded.reason,
             max_severity = excluded.max_severity,
             computed_at = excluded.computed_at`,
		v.RepositoryID, v.BlobSHA256, nullableInt64(v.ResultID), v.ScannerProfile, v.PolicyHash,
		v.Status, v.Reason, v.MaxSeverity, formatTime(v.ComputedAt))
	if err != nil {
		return artifactscan.Verdict{}, err
	}
	return s.LatestArtifactScanVerdict(ctx, v.RepositoryID, v.BlobSHA256, v.ScannerProfile)
}

// LatestArtifactScanVerdict returns the current verdict for repository/blob/profile.
func (s *Store) LatestArtifactScanVerdict(ctx context.Context, repoID int64, blobSHA256, profileName string) (artifactscan.Verdict, error) {
	var v artifactscan.Verdict
	var status, severity, computed string
	var resultID sql.NullInt64
	err := s.h().QueryRowContext(ctx,
		`SELECT id, repository_id, blob_sha256, result_id, scanner_profile, policy_hash,
                status, reason, max_severity, computed_at
           FROM artifact_scan_verdicts
          WHERE repository_id = ? AND blob_sha256 = ? AND scanner_profile = ?`,
		repoID, blobSHA256, profileName).Scan(&v.ID, &v.RepositoryID, &v.BlobSHA256, &resultID,
		&v.ScannerProfile, &v.PolicyHash, &status, &v.Reason, &severity, &computed)
	if errors.Is(err, sql.ErrNoRows) {
		return artifactscan.Verdict{}, ErrNotFound
	}
	if err != nil {
		return artifactscan.Verdict{}, err
	}
	if resultID.Valid {
		v.ResultID = resultID.Int64
	}
	v.Status = artifactscan.VerdictStatus(status)
	v.MaxSeverity = artifactscan.Severity(severity)
	v.ComputedAt = parseTime(computed)
	return v, nil
}

// RecomputeArtifactScanVerdict reapplies one repository policy to the latest
// stored report for a blob/profile. It never creates scanner work.
func (s *Store) RecomputeArtifactScanVerdict(ctx context.Context, repoID int64, blobSHA256, profileName string, now time.Time) (artifactscan.Verdict, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	repo, err := s.GetRepository(ctx, repoID)
	if err != nil {
		return artifactscan.Verdict{}, err
	}
	cfg, err := repoconfig.Parse(repo.ConfigJSON)
	if err != nil {
		return artifactscan.Verdict{}, err
	}
	if !cfg.ArtifactScan.Enabled {
		return artifactscan.Verdict{}, ErrNotFound
	}
	if profileName == "" {
		profileName = cfg.ArtifactScan.EffectiveScannerProfile()
	}
	if profileName != cfg.ArtifactScan.EffectiveScannerProfile() {
		return artifactscan.Verdict{}, ErrNotFound
	}
	var result *artifactscan.Result
	if stored, err := s.LatestArtifactScanResult(ctx, blobSHA256, profileName); err == nil {
		result = &stored
	} else if !errors.Is(err, ErrNotFound) {
		return artifactscan.Verdict{}, err
	}
	policy := artifactscan.Policy{
		Enabled:        cfg.ArtifactScan.Enabled,
		ScannerProfile: profileName,
		Action:         artifactscan.PolicyAction(cfg.ArtifactScan.EffectiveAction()),
		Threshold:      artifactscan.ParseSeverity(cfg.ArtifactScan.EffectiveThreshold()),
		BlockUnscanned: cfg.ArtifactScan.BlockUnscanned,
	}
	verdict := artifactscan.ComputeVerdict(repoID, blobSHA256, policy, result, artifactScanPolicyHash(repo.ConfigJSON))
	verdict.ComputedAt = now
	return s.UpsertArtifactScanVerdict(ctx, verdict)
}

func scanArtifactScanJobRow(row *sql.Row) (artifactscan.Job, error) {
	var j artifactscan.Job
	var status string
	var storeSBOM int
	var lease, nextRun, heartbeat, created, started, finished sql.NullString
	err := row.Scan(&j.ID, &j.BlobSHA256, &j.ScannerProfile, &j.Scanner, &j.ScannerConfigHash, &status,
		&j.WorkerID, &j.Attempts, &lease, &nextRun, &heartbeat,
		&j.Error, &created, &started, &finished,
		&j.Limits.MaxArtifactBytes, &j.Limits.MaxExtractedBytes, &j.Limits.MaxFiles, &storeSBOM)
	if errors.Is(err, sql.ErrNoRows) {
		return artifactscan.Job{}, ErrNotFound
	}
	if err != nil {
		return artifactscan.Job{}, err
	}
	j.Status = artifactscan.JobStatus(status)
	j.LeaseUntil = parseNullTime(lease)
	j.NextRunAt = parseNullTime(nextRun)
	j.LastHeartbeatAt = parseNullTime(heartbeat)
	j.CreatedAt = parseNullTime(created)
	j.StartedAt = parseNullTime(started)
	j.FinishedAt = parseNullTime(finished)
	j.StoreSBOM = storeSBOM != 0
	return j, nil
}

func scanArtifactScanResultRow(row *sql.Row) (int64, artifactscan.Result, error) {
	var id int64
	var r artifactscan.Result
	var status, severity, dbBuilt, providersJSON, scannedAt, created string
	err := row.Scan(&id, &r.JobID, &r.BlobSHA256, &r.ScannerProfile, &r.Scanner, &r.ScannerVersion, &r.ScannerConfigHash,
		&r.DatabaseSchemaVersion, &dbBuilt, &providersJSON, &status,
		&severity, new(int), &r.RawResultDigest, &r.Error, &scannedAt, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, artifactscan.Result{}, ErrNotFound
	}
	if err != nil {
		return 0, artifactscan.Result{}, err
	}
	r.ID = id
	r.Status = artifactscan.ReportStatus(status)
	r.MaxSeverity = artifactscan.Severity(severity)
	r.DatabaseBuiltAt = parseTime(dbBuilt)
	_ = json.Unmarshal([]byte(providersJSON), &r.DatabaseProviders)
	r.ScannedAt = parseTime(scannedAt)
	r.CreatedAt = parseTime(created)
	return id, r, nil
}

func upsertScannerCapabilities(ctx context.Context, tx *sql.Tx, workerID string, caps []artifactscan.ScannerCapability, now time.Time) error {
	for _, c := range caps {
		if c.Name == "" {
			continue
		}
		ecosystems, err := json.Marshal(c.SupportedEcosystems)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO artifact_scanner_capabilities(
                 worker_id, scanner, scanner_version, database_schema_version,
                 database_built_at, supports_sbom, supported_ecosystems, reported_at
             ) VALUES(?, ?, ?, ?, ?, ?, ?, ?)
             ON CONFLICT(worker_id, scanner) DO UPDATE SET
                 scanner_version = excluded.scanner_version,
                 database_schema_version = excluded.database_schema_version,
                 database_built_at = excluded.database_built_at,
                 supports_sbom = excluded.supports_sbom,
                 supported_ecosystems = excluded.supported_ecosystems,
                 reported_at = excluded.reported_at`,
			workerID, c.Name, c.Version, c.DatabaseSchemaVersion, nullableTime(c.DatabaseBuiltAt),
			boolInt(c.SupportsSBOM), string(ecosystems), formatTime(now)); err != nil {
			return err
		}
	}
	return nil
}

func capabilityScanners(caps []artifactscan.ScannerCapability) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, c := range caps {
		if c.Name == "" {
			continue
		}
		if _, ok := seen[c.Name]; ok {
			continue
		}
		seen[c.Name] = struct{}{}
		out = append(out, c.Name)
	}
	return out
}

func scannerInClause(scanners []string) (string, []any) {
	parts := make([]string, 0, len(scanners))
	args := make([]any, 0, len(scanners))
	for _, scanner := range scanners {
		parts = append(parts, "?")
		args = append(args, scanner)
	}
	return strings.Join(parts, ","), args
}

func recomputeArtifactScanVerdictsTx(ctx context.Context, tx *sql.Tx, resultID int64, result artifactscan.Result, now time.Time) error {
	rows, err := tx.QueryContext(ctx,
		`SELECT DISTINCT r.id, r.config_json
           FROM artifacts a JOIN repositories r ON r.id = a.repo_id
          WHERE a.blob_sha256 = ?`,
		result.BlobSHA256)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var repoID int64
		var configJSON string
		if err := rows.Scan(&repoID, &configJSON); err != nil {
			return err
		}
		cfg, err := repoconfig.Parse(configJSON)
		if err != nil {
			return err
		}
		if !cfg.ArtifactScan.Enabled {
			continue
		}
		profile := cfg.ArtifactScan.EffectiveScannerProfile()
		if profile != result.ScannerProfile {
			continue
		}
		policy := artifactscan.Policy{
			Enabled:        cfg.ArtifactScan.Enabled,
			ScannerProfile: profile,
			Action:         artifactscan.PolicyAction(cfg.ArtifactScan.EffectiveAction()),
			Threshold:      artifactscan.ParseSeverity(cfg.ArtifactScan.EffectiveThreshold()),
			BlockUnscanned: cfg.ArtifactScan.BlockUnscanned,
		}
		verdict := artifactscan.ComputeVerdict(repoID, result.BlobSHA256, policy, &result, artifactScanPolicyHash(configJSON))
		verdict.ResultID = resultID
		verdict.ComputedAt = now
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO artifact_scan_verdicts(
                 repository_id, blob_sha256, result_id, scanner_profile, policy_hash,
                 status, reason, max_severity, computed_at
             ) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
             ON CONFLICT(repository_id, blob_sha256, scanner_profile) DO UPDATE SET
                 result_id = excluded.result_id,
                 policy_hash = excluded.policy_hash,
                 status = excluded.status,
                 reason = excluded.reason,
                 max_severity = excluded.max_severity,
                 computed_at = excluded.computed_at`,
			verdict.RepositoryID, verdict.BlobSHA256, nullableInt64(verdict.ResultID), verdict.ScannerProfile, verdict.PolicyHash,
			verdict.Status, verdict.Reason, verdict.MaxSeverity, formatTime(verdict.ComputedAt)); err != nil {
			return err
		}
	}
	return rows.Err()
}

func artifactScanPolicyHash(configJSON string) string {
	sum := sha256.Sum256([]byte(configJSON))
	return fmt.Sprintf("sha256:%x", sum[:])
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullableInt64(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

func nullableTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return formatTime(t)
}

func parseNullTime(s sql.NullString) time.Time {
	if !s.Valid || s.String == "" {
		return time.Time{}
	}
	return parseTime(s.String)
}
