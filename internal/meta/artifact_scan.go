package meta

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/younsl/o/box/kubernetes/forklift/internal/artifactscan"
)

// EnqueueArtifactScan creates a queued scan job for one blob and scanner.
func (s *Store) EnqueueArtifactScan(ctx context.Context, id, blobSHA256, scanner, configHash string, now time.Time) (artifactscan.Job, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	_, err := s.h().ExecContext(ctx,
		`INSERT INTO artifact_scan_jobs(id, blob_sha256, scanner, scanner_config_hash, status, next_run_at, created_at)
         VALUES(?, ?, ?, ?, ?, ?, ?)`,
		id, blobSHA256, scanner, configHash, artifactscan.StatusQueued, formatTime(now), formatTime(now))
	if err != nil {
		return artifactscan.Job{}, err
	}
	return s.GetArtifactScanJob(ctx, id)
}

// GetArtifactScanJob returns one artifact scan job by id.
func (s *Store) GetArtifactScanJob(ctx context.Context, id string) (artifactscan.Job, error) {
	return scanArtifactScanJobRow(s.h().QueryRowContext(ctx,
		`SELECT id, blob_sha256, scanner, scanner_config_hash, status, worker_id, attempts,
                lease_until, next_run_at, last_heartbeat_at, error, created_at, started_at, finished_at
           FROM artifact_scan_jobs WHERE id = ?`, id))
}

// LatestArtifactScanJob returns the newest job for a blob/scanner/config.
func (s *Store) LatestArtifactScanJob(ctx context.Context, blobSHA256, scanner, configHash string) (artifactscan.Job, error) {
	return scanArtifactScanJobRow(s.h().QueryRowContext(ctx,
		`SELECT id, blob_sha256, scanner, scanner_config_hash, status, worker_id, attempts,
                lease_until, next_run_at, last_heartbeat_at, error, created_at, started_at, finished_at
           FROM artifact_scan_jobs
          WHERE blob_sha256 = ? AND scanner = ? AND scanner_config_hash = ?
          ORDER BY created_at DESC LIMIT 1`,
		blobSHA256, scanner, configHash))
}

// ClaimArtifactScanJob atomically claims the oldest due queued job for a worker.
func (s *Store) ClaimArtifactScanJob(ctx context.Context, workerID string, leaseUntil, now time.Time) (artifactscan.Job, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tx, err := s.h().BeginTx(ctx, nil)
	if err != nil {
		return artifactscan.Job{}, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx,
		`SELECT id FROM artifact_scan_jobs
          WHERE status = ? AND next_run_at <= ?
          ORDER BY created_at ASC LIMIT 1`,
		artifactscan.StatusQueued, formatTime(now))
	var id string
	if err := row.Scan(&id); errors.Is(err, sql.ErrNoRows) {
		return artifactscan.Job{}, ErrNotFound
	} else if err != nil {
		return artifactscan.Job{}, err
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE artifact_scan_jobs
            SET status = ?, worker_id = ?, attempts = attempts + 1,
                lease_until = ?, last_heartbeat_at = ?, started_at = COALESCE(started_at, ?), error = ''
          WHERE id = ? AND status = ?`,
		artifactscan.StatusRunning, workerID, formatTime(leaseUntil), formatTime(now), formatTime(now), id, artifactscan.StatusQueued)
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
	res, err := s.h().ExecContext(ctx,
		`UPDATE artifact_scan_jobs
            SET lease_until = ?, last_heartbeat_at = ?
          WHERE id = ? AND worker_id = ? AND status = ?`,
		formatTime(leaseUntil), formatTime(now), id, workerID, artifactscan.StatusRunning)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// CompleteArtifactScanJob stores a terminal result and marks the job completed.
func (s *Store) CompleteArtifactScanJob(ctx context.Context, id, workerID string, result artifactscan.Result, now time.Time) (int64, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tx, err := s.h().BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var status string
	if err := tx.QueryRowContext(ctx,
		`SELECT status FROM artifact_scan_jobs WHERE id = ? AND worker_id = ?`, id, workerID).Scan(&status); errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	} else if err != nil {
		return 0, err
	}
	if status != string(artifactscan.StatusRunning) {
		return 0, ErrConflict
	}
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
             job_id, blob_sha256, scanner, scanner_version, scanner_config_hash,
             database_schema_version, database_built_at, database_providers_json,
             status, max_severity, finding_count, raw_result_digest, error, scanned_at, created_at
         ) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, result.BlobSHA256, result.Scanner, result.ScannerVersion, result.ScannerConfigHash,
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
	if _, err := tx.ExecContext(ctx,
		`UPDATE artifact_scan_jobs
            SET status = ?, finished_at = ?, error = ?
          WHERE id = ?`,
		result.Status, formatTime(now), result.Error, id); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return resultID, nil
}

// LatestArtifactScanResult returns the newest result for a blob/scanner/config.
func (s *Store) LatestArtifactScanResult(ctx context.Context, blobSHA256, scanner, configHash string) (artifactscan.Result, error) {
	row := s.h().QueryRowContext(ctx,
		`SELECT id, job_id, blob_sha256, scanner, scanner_version, scanner_config_hash,
                database_schema_version, database_built_at, database_providers_json,
                status, max_severity, raw_result_digest, error, scanned_at
           FROM artifact_scan_results
          WHERE blob_sha256 = ? AND scanner = ? AND scanner_config_hash = ?
          ORDER BY scanned_at DESC, id DESC LIMIT 1`,
		blobSHA256, scanner, configHash)
	resultID, result, err := scanArtifactScanResultRow(row)
	if err != nil {
		return artifactscan.Result{}, err
	}
	findings, err := s.ListArtifactScanFindings(ctx, resultID)
	if err != nil {
		return artifactscan.Result{}, err
	}
	result.Findings = findings
	return result, nil
}

// ListArtifactScanFindings returns normalized findings for one scan result id.
func (s *Store) ListArtifactScanFindings(ctx context.Context, resultID int64) ([]artifactscan.Finding, error) {
	rows, err := s.h().QueryContext(ctx,
		`SELECT vulnerability_id, severity, package_name, package_version, package_type,
                package_purl, fixed_versions, source, source_url, match_type
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

func scanArtifactScanJobRow(row *sql.Row) (artifactscan.Job, error) {
	var j artifactscan.Job
	var status string
	var lease, nextRun, heartbeat, created, started, finished sql.NullString
	err := row.Scan(&j.ID, &j.BlobSHA256, &j.Scanner, &j.ScannerConfigHash, &status, &j.WorkerID,
		&j.Attempts, &lease, &nextRun, &heartbeat, &j.Error, &created, &started, &finished)
	if errors.Is(err, sql.ErrNoRows) {
		return artifactscan.Job{}, ErrNotFound
	}
	if err != nil {
		return artifactscan.Job{}, err
	}
	j.Status = artifactscan.Status(status)
	j.LeaseUntil = parseNullTime(lease)
	j.CreatedAt = parseNullTime(created)
	j.StartedAt = parseNullTime(started)
	j.FinishedAt = parseNullTime(finished)
	return j, nil
}

func scanArtifactScanResultRow(row *sql.Row) (int64, artifactscan.Result, error) {
	var id int64
	var r artifactscan.Result
	var status, severity string
	var dbBuilt, scanned string
	var providersJSON string
	err := row.Scan(&id, &r.JobID, &r.BlobSHA256, &r.Scanner, &r.ScannerVersion, &r.ScannerConfigHash,
		&r.DatabaseSchemaVersion, &dbBuilt, &providersJSON, &status, &severity, &r.RawResultDigest, &r.Error, &scanned)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, artifactscan.Result{}, ErrNotFound
	}
	if err != nil {
		return 0, artifactscan.Result{}, err
	}
	r.Status = artifactscan.Status(status)
	r.MaxSeverity = artifactscan.Severity(severity)
	r.DatabaseBuiltAt = parseTime(dbBuilt)
	r.ScannedAt = parseTime(scanned)
	_ = json.Unmarshal([]byte(providersJSON), &r.DatabaseProviders)
	return id, r, nil
}

func parseNullTime(v sql.NullString) time.Time {
	if !v.Valid || v.String == "" {
		return time.Time{}
	}
	return parseTime(v.String)
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
