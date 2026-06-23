package meta

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// LicenseScan is a stored license-resolution result for one package coordinate.
type LicenseScan struct {
	System     string // deps.dev system (npm|maven|cargo|go|pypi)
	Package    string
	Version    string
	Licenses   []string // SPDX license expressions; empty when none reported
	Source     string   // data source that produced the result (e.g. "deps.dev")
	ResolvedAt time.Time
}

// UpsertLicenseScan records (or refreshes) a license-resolution result for a
// coordinate. licenses nil is stored as an empty array.
func (s *Store) UpsertLicenseScan(ctx context.Context, system, pkg, version string, licenses []string, source string) error {
	if licenses == nil {
		licenses = []string{}
	}
	licJSON, err := json.Marshal(licenses)
	if err != nil {
		return err
	}
	if source == "" {
		source = "deps.dev"
	}
	_, err = s.h().ExecContext(ctx,
		`INSERT INTO license_scans(system, package, version, licenses, source, resolved_at)
         VALUES(?, ?, ?, ?, ?, ?)
         ON CONFLICT(system, package, version) DO UPDATE SET
             licenses = excluded.licenses,
             source = excluded.source,
             resolved_at = excluded.resolved_at`,
		system, pkg, version, string(licJSON), source, nowRFC3339())
	return err
}

// GetLicenseScan returns a stored result, or ErrNotFound when the coordinate
// has not been resolved yet.
func (s *Store) GetLicenseScan(ctx context.Context, system, pkg, version string) (LicenseScan, error) {
	return scanLicenseRow(s.h().QueryRowContext(ctx,
		`SELECT system, package, version, licenses, source, resolved_at
           FROM license_scans WHERE system = ? AND package = ? AND version = ?`,
		system, pkg, version))
}

// ListStaleLicenseScans returns up to limit results last resolved before the
// cutoff, oldest first, so a re-resolver can refresh them against fresh data.
func (s *Store) ListStaleLicenseScans(ctx context.Context, before time.Time, limit int) ([]LicenseScan, error) {
	rows, err := s.h().QueryContext(ctx,
		`SELECT system, package, version, licenses, source, resolved_at
           FROM license_scans WHERE resolved_at < ? ORDER BY resolved_at ASC LIMIT ?`,
		before.UTC().Format(time.RFC3339Nano), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LicenseScan
	for rows.Next() {
		v, err := scanLicenseRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ResolvedLicenseKeys returns the set of coordinates that already have a stored
// result, keyed as system\x00package\x00version, so a backfill can enqueue only
// coordinates that have never been resolved.
func (s *Store) ResolvedLicenseKeys(ctx context.Context) (map[string]struct{}, error) {
	rows, err := s.h().QueryContext(ctx, `SELECT system, package, version FROM license_scans`)
	if err != nil {
		return nil, wrap("resolved license keys", err)
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var system, pkg, ver string
		if err := rows.Scan(&system, &pkg, &ver); err != nil {
			return nil, err
		}
		out[system+"\x00"+pkg+"\x00"+ver] = struct{}{}
	}
	return out, rows.Err()
}

func scanLicenseRow(row *sql.Row) (LicenseScan, error) {
	var v LicenseScan
	var licenses, resolved string
	err := row.Scan(&v.System, &v.Package, &v.Version, &licenses, &v.Source, &resolved)
	if errors.Is(err, sql.ErrNoRows) {
		return LicenseScan{}, ErrNotFound
	}
	if err != nil {
		return LicenseScan{}, err
	}
	_ = json.Unmarshal([]byte(licenses), &v.Licenses)
	v.ResolvedAt = parseTime(resolved)
	return v, nil
}

func scanLicenseRows(rows *sql.Rows) (LicenseScan, error) {
	var v LicenseScan
	var licenses, resolved string
	if err := rows.Scan(&v.System, &v.Package, &v.Version, &licenses, &v.Source, &resolved); err != nil {
		return LicenseScan{}, err
	}
	_ = json.Unmarshal([]byte(licenses), &v.Licenses)
	v.ResolvedAt = parseTime(resolved)
	return v, nil
}
