// Package artifactscan defines the server-owned model for artifact-byte
// scanning. It intentionally contains no scanner execution code; scanner tools
// run in the separate scannerworker package/process.
package artifactscan

import "time"

// Status is the lifecycle state for an artifact scan job or result.
type Status string

const (
	StatusQueued          Status = "queued"
	StatusRunning         Status = "running"
	StatusCompleted       Status = "completed"
	StatusFailed          Status = "failed"
	StatusNotApplicable   Status = "not_applicable"
	StatusSkippedTooLarge Status = "skipped_too_large"
	StatusDead            Status = "dead"
	StatusReused          Status = "reused"
)

// Terminal reports whether a status represents a final worker outcome.
func (s Status) Terminal() bool {
	switch s {
	case StatusCompleted, StatusFailed, StatusNotApplicable, StatusSkippedTooLarge, StatusDead, StatusReused:
		return true
	default:
		return false
	}
}

// Severity is an ordered vulnerability severity for artifact scan findings.
type Severity string

const (
	SeverityUnknown    Severity = "unknown"
	SeverityNegligible Severity = "negligible"
	SeverityLow        Severity = "low"
	SeverityMedium     Severity = "medium"
	SeverityHigh       Severity = "high"
	SeverityCritical   Severity = "critical"
)

// SeverityRank returns a comparable rank for policy threshold evaluation.
func SeverityRank(s Severity) int {
	switch s {
	case SeverityCritical:
		return 5
	case SeverityHigh:
		return 4
	case SeverityMedium:
		return 3
	case SeverityLow:
		return 2
	case SeverityNegligible:
		return 1
	default:
		return 0
	}
}

// ParseSeverity normalizes scanner-provided severity labels.
func ParseSeverity(s string) Severity {
	switch s {
	case "critical", "Critical", "CRITICAL":
		return SeverityCritical
	case "high", "High", "HIGH":
		return SeverityHigh
	case "medium", "Medium", "MEDIUM", "moderate", "Moderate", "MODERATE":
		return SeverityMedium
	case "low", "Low", "LOW":
		return SeverityLow
	case "negligible", "Negligible", "NEGLIGIBLE":
		return SeverityNegligible
	default:
		return SeverityUnknown
	}
}

// Job is a scanner unit of work owned by the forklift leader.
type Job struct {
	ID                string
	BlobSHA256        string
	Scanner           string
	ScannerConfigHash string
	Status            Status
	WorkerID          string
	Attempts          int
	LeaseUntil        time.Time
	CreatedAt         time.Time
	StartedAt         time.Time
	FinishedAt        time.Time
	Error             string
}

// DBProvider records provenance for one source imported into the scanner DB.
type DBProvider struct {
	ID          string    `json:"id"`
	CapturedAt  time.Time `json:"captured_at,omitempty"`
	InputDigest string    `json:"input_digest,omitempty"`
}

// Finding is one normalized vulnerability finding returned by an artifact
// scanner. All string fields are untrusted scanner output until validated.
type Finding struct {
	VulnerabilityID string   `json:"vulnerability_id"`
	Severity        Severity `json:"severity"`
	PackageName     string   `json:"package_name"`
	PackageVersion  string   `json:"package_version"`
	PackageType     string   `json:"package_type"`
	PackagePURL     string   `json:"package_purl,omitempty"`
	FixedVersions   []string `json:"fixed_versions,omitempty"`
	Source          string   `json:"source,omitempty"`
	SourceURL       string   `json:"source_url,omitempty"`
	MatchType       string   `json:"match_type,omitempty"`
}

// Result is the worker-submitted, normalized scan result. The server validates
// it before persistence or policy evaluation.
type Result struct {
	JobID                 string       `json:"job_id"`
	BlobSHA256            string       `json:"blob_sha256"`
	Scanner               string       `json:"scanner"`
	ScannerVersion        string       `json:"scanner_version"`
	ScannerConfigHash     string       `json:"scanner_config_hash,omitempty"`
	DatabaseSchemaVersion string       `json:"database_schema_version,omitempty"`
	DatabaseBuiltAt       time.Time    `json:"database_built_at,omitempty"`
	DatabaseProviders     []DBProvider `json:"database_providers,omitempty"`
	Status                Status       `json:"status"`
	MaxSeverity           Severity     `json:"max_severity"`
	Findings              []Finding    `json:"findings,omitempty"`
	RawResultDigest       string       `json:"raw_result_digest,omitempty"`
	Error                 string       `json:"error,omitempty"`
	ScannedAt             time.Time    `json:"scanned_at"`
}

// RecomputeSummary updates MaxSeverity from the findings. Call this after
// normalizing scanner output so policy does not depend on scanner-provided
// summary fields.
func (r *Result) RecomputeSummary() {
	max := SeverityUnknown
	for _, f := range r.Findings {
		if SeverityRank(f.Severity) > SeverityRank(max) {
			max = f.Severity
		}
	}
	r.MaxSeverity = max
}
