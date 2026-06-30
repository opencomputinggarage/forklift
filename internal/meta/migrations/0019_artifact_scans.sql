-- Optional artifact-byte scanning state. These tables are separate from the
-- coordinate-level OSV/deps.dev scans so forklift can keep the lightweight
-- built-in analyzers while adding isolated worker-based scanning.

CREATE TABLE artifact_scan_jobs (
    id                  TEXT PRIMARY KEY,
    blob_sha256         TEXT NOT NULL REFERENCES blobs(sha256),
    scanner             TEXT NOT NULL,
    scanner_config_hash TEXT NOT NULL DEFAULT '',
    status              TEXT NOT NULL,
    worker_id           TEXT NOT NULL DEFAULT '',
    attempts            INTEGER NOT NULL DEFAULT 0,
    lease_until         TEXT,
    next_run_at         TEXT NOT NULL,
    last_heartbeat_at   TEXT,
    error               TEXT NOT NULL DEFAULT '',
    created_at          TEXT NOT NULL,
    started_at          TEXT,
    finished_at         TEXT
);

CREATE INDEX idx_artifact_scan_jobs_claim
    ON artifact_scan_jobs(status, next_run_at, created_at);

CREATE INDEX idx_artifact_scan_jobs_blob
    ON artifact_scan_jobs(blob_sha256, scanner, scanner_config_hash);

CREATE TABLE artifact_scan_results (
    id                      INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id                  TEXT NOT NULL REFERENCES artifact_scan_jobs(id) ON DELETE CASCADE,
    blob_sha256             TEXT NOT NULL REFERENCES blobs(sha256),
    scanner                 TEXT NOT NULL,
    scanner_version         TEXT NOT NULL,
    scanner_config_hash     TEXT NOT NULL DEFAULT '',
    database_schema_version TEXT NOT NULL DEFAULT '',
    database_built_at       TEXT,
    database_providers_json TEXT NOT NULL DEFAULT '[]',
    status                  TEXT NOT NULL,
    max_severity            TEXT NOT NULL DEFAULT 'unknown',
    finding_count           INTEGER NOT NULL DEFAULT 0,
    raw_result_digest       TEXT NOT NULL DEFAULT '',
    error                   TEXT NOT NULL DEFAULT '',
    scanned_at              TEXT NOT NULL,
    created_at              TEXT NOT NULL
);

CREATE INDEX idx_artifact_scan_results_blob
    ON artifact_scan_results(blob_sha256, scanner, scanner_config_hash, scanned_at);

CREATE TABLE artifact_scan_findings (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    result_id         INTEGER NOT NULL REFERENCES artifact_scan_results(id) ON DELETE CASCADE,
    vulnerability_id  TEXT NOT NULL,
    severity          TEXT NOT NULL,
    package_name      TEXT NOT NULL,
    package_version   TEXT NOT NULL DEFAULT '',
    package_type      TEXT NOT NULL DEFAULT '',
    package_purl      TEXT NOT NULL DEFAULT '',
    fixed_versions    TEXT NOT NULL DEFAULT '[]',
    source            TEXT NOT NULL DEFAULT '',
    source_url        TEXT NOT NULL DEFAULT '',
    match_type        TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_artifact_scan_findings_result
    ON artifact_scan_findings(result_id);
