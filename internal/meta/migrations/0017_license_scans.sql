-- Resolved license information per package coordinate, keyed by deps.dev system
-- so the same package shared by several proxies is resolved once. licenses is a
-- JSON array of SPDX license expressions (empty when the source reports none),
-- source names the data source (e.g. "deps.dev"), and resolved_at drives
-- periodic re-resolution so license changes on already-cached versions surface.

CREATE TABLE license_scans (
    system      TEXT NOT NULL,
    package     TEXT NOT NULL,
    version     TEXT NOT NULL,
    licenses    TEXT NOT NULL DEFAULT '[]',
    source      TEXT NOT NULL DEFAULT 'deps.dev',
    resolved_at TEXT NOT NULL,
    PRIMARY KEY (system, package, version)
);

CREATE INDEX idx_license_scans_resolved_at ON license_scans(resolved_at);
