-- Optional artifact-byte scanning state. This subsystem follows the
-- scanner-profile -> job -> report -> verdict model. Scanner reports are facts
-- about blob bytes; verdicts are repository-policy decisions computed from
-- reports.

CREATE TABLE artifact_scanner_profiles (
    name                 TEXT PRIMARY KEY,
    scanner              TEXT NOT NULL,
    mode                 TEXT NOT NULL DEFAULT 'deployment',
    config_hash          TEXT NOT NULL,
    runtime_class_name   TEXT NOT NULL DEFAULT '',
    max_artifact_bytes   INTEGER NOT NULL DEFAULT 104857600,
    max_extracted_bytes  INTEGER NOT NULL DEFAULT 0,
    max_files            INTEGER NOT NULL DEFAULT 0,
    store_sbom           INTEGER NOT NULL DEFAULT 0,
    created_at           TEXT NOT NULL,
    updated_at           TEXT NOT NULL
);

CREATE TABLE artifact_scanner_capabilities (
    worker_id                TEXT NOT NULL,
    scanner                  TEXT NOT NULL,
    scanner_version          TEXT NOT NULL DEFAULT '',
    database_schema_version  TEXT NOT NULL DEFAULT '',
    database_built_at        TEXT,
    supports_sbom            INTEGER NOT NULL DEFAULT 0,
    supported_ecosystems     TEXT NOT NULL DEFAULT '[]',
    reported_at              TEXT NOT NULL,
    PRIMARY KEY(worker_id, scanner)
);

CREATE INDEX idx_artifact_scanner_capabilities_scanner
    ON artifact_scanner_capabilities(scanner, reported_at);

CREATE TABLE artifact_scan_jobs (
    id                   TEXT PRIMARY KEY,
    blob_sha256          TEXT NOT NULL REFERENCES blobs(sha256),
    scanner_profile      TEXT NOT NULL,
    scanner              TEXT NOT NULL,
    scanner_config_hash  TEXT NOT NULL,
    status               TEXT NOT NULL,
    worker_id            TEXT NOT NULL DEFAULT '',
    attempts             INTEGER NOT NULL DEFAULT 0,
    lease_until          TEXT,
    next_run_at          TEXT NOT NULL,
    last_heartbeat_at    TEXT,
    error                TEXT NOT NULL DEFAULT '',
    created_at           TEXT NOT NULL,
    started_at           TEXT,
    finished_at          TEXT,
    max_artifact_bytes   INTEGER NOT NULL DEFAULT 104857600,
    max_extracted_bytes  INTEGER NOT NULL DEFAULT 0,
    max_files            INTEGER NOT NULL DEFAULT 0,
    store_sbom           INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_artifact_scan_jobs_claim
    ON artifact_scan_jobs(status, scanner, next_run_at, created_at);

CREATE INDEX idx_artifact_scan_jobs_blob_profile
    ON artifact_scan_jobs(blob_sha256, scanner_profile, scanner_config_hash, created_at);

CREATE TABLE artifact_scan_results (
    id                       INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id                   TEXT NOT NULL REFERENCES artifact_scan_jobs(id) ON DELETE CASCADE,
    blob_sha256              TEXT NOT NULL REFERENCES blobs(sha256),
    scanner_profile          TEXT NOT NULL,
    scanner                  TEXT NOT NULL,
    scanner_version          TEXT NOT NULL,
    scanner_config_hash      TEXT NOT NULL,
    database_schema_version  TEXT NOT NULL DEFAULT '',
    database_built_at        TEXT,
    database_providers_json  TEXT NOT NULL DEFAULT '[]',
    status                   TEXT NOT NULL,
    max_severity             TEXT NOT NULL DEFAULT 'unknown',
    finding_count            INTEGER NOT NULL DEFAULT 0,
    raw_result_digest        TEXT NOT NULL DEFAULT '',
    error                    TEXT NOT NULL DEFAULT '',
    scanned_at               TEXT NOT NULL,
    created_at               TEXT NOT NULL,
    source_result_id         INTEGER
);

CREATE INDEX idx_artifact_scan_results_reuse
    ON artifact_scan_results(blob_sha256, scanner, scanner_config_hash, database_built_at, scanned_at);

CREATE INDEX idx_artifact_scan_results_profile
    ON artifact_scan_results(blob_sha256, scanner_profile, scanned_at);

CREATE TABLE artifact_scan_findings (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    result_id          INTEGER NOT NULL REFERENCES artifact_scan_results(id) ON DELETE CASCADE,
    vulnerability_id   TEXT NOT NULL,
    severity           TEXT NOT NULL,
    package_name       TEXT NOT NULL,
    package_version    TEXT NOT NULL DEFAULT '',
    package_type       TEXT NOT NULL DEFAULT '',
    package_purl       TEXT NOT NULL DEFAULT '',
    fixed_versions     TEXT NOT NULL DEFAULT '[]',
    source             TEXT NOT NULL DEFAULT '',
    source_url         TEXT NOT NULL DEFAULT '',
    match_type         TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_artifact_scan_findings_result
    ON artifact_scan_findings(result_id);

CREATE TABLE artifact_scan_verdicts (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    repository_id     INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    blob_sha256       TEXT NOT NULL REFERENCES blobs(sha256),
    result_id         INTEGER REFERENCES artifact_scan_results(id) ON DELETE SET NULL,
    scanner_profile   TEXT NOT NULL,
    policy_hash       TEXT NOT NULL DEFAULT '',
    status            TEXT NOT NULL,
    reason            TEXT NOT NULL DEFAULT '',
    max_severity      TEXT NOT NULL DEFAULT 'unknown',
    computed_at       TEXT NOT NULL,
    UNIQUE(repository_id, blob_sha256, scanner_profile)
);

CREATE INDEX idx_artifact_scan_verdicts_blob
    ON artifact_scan_verdicts(blob_sha256, scanner_profile);

CREATE TABLE artifact_sboms (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    blob_sha256         TEXT NOT NULL REFERENCES blobs(sha256),
    result_id           INTEGER NOT NULL REFERENCES artifact_scan_results(id) ON DELETE CASCADE,
    format              TEXT NOT NULL,
    generator           TEXT NOT NULL,
    generator_version   TEXT NOT NULL DEFAULT '',
    content_digest      TEXT NOT NULL,
    content_json        TEXT NOT NULL,
    created_at          TEXT NOT NULL
);

CREATE INDEX idx_artifact_sboms_blob
    ON artifact_sboms(blob_sha256, result_id);

CREATE TABLE artifact_scan_exports (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    sbom_id              INTEGER NOT NULL REFERENCES artifact_sboms(id) ON DELETE CASCADE,
    destination          TEXT NOT NULL,
    status               TEXT NOT NULL,
    error                TEXT NOT NULL DEFAULT '',
    attempts             INTEGER NOT NULL DEFAULT 0,
    next_run_at          TEXT NOT NULL,
    created_at           TEXT NOT NULL,
    updated_at           TEXT NOT NULL
);
