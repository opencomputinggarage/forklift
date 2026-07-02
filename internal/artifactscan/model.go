// Package artifactscan defines the server-owned model for artifact-byte
// scanning. It intentionally contains no scanner execution code; scanner tools
// run in the separate scannerworker package/process.
package artifactscan

import "time"

// JobStatus is the lifecycle state of one execution lease.
type JobStatus string

const (
	JobQueued    JobStatus = "queued"
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
	JobDead      JobStatus = "dead"
)

// ReportStatus is the scanner fact state persisted for one blob/profile.
type ReportStatus string

const (
	ReportCompleted       ReportStatus = "completed"
	ReportFailed          ReportStatus = "failed"
	ReportNotApplicable   ReportStatus = "not_applicable"
	ReportSkippedTooLarge ReportStatus = "skipped_too_large"
	ReportReused          ReportStatus = "reused"
)

// Status is kept as the JSON-facing report status type used by scanner drivers.
// Job state is represented by JobStatus and verdict state by VerdictStatus.
type Status = ReportStatus

const (
	StatusCompleted       = ReportCompleted
	StatusFailed          = ReportFailed
	StatusNotApplicable   = ReportNotApplicable
	StatusSkippedTooLarge = ReportSkippedTooLarge
	StatusReused          = ReportReused
)

// Terminal reports whether a report status represents a final worker outcome.
func (s ReportStatus) Terminal() bool {
	switch s {
	case ReportCompleted, ReportFailed, ReportNotApplicable, ReportSkippedTooLarge, ReportReused:
		return true
	default:
		return false
	}
}

// VerdictStatus is a repository policy decision computed from a report.
type VerdictStatus string

const (
	VerdictAllow              VerdictStatus = "allow"
	VerdictAudit              VerdictStatus = "audit"
	VerdictWarn               VerdictStatus = "warn"
	VerdictBlock              VerdictStatus = "block"
	VerdictPending            VerdictStatus = "pending"
	VerdictStale              VerdictStatus = "stale"
	VerdictScannerUnavailable VerdictStatus = "scanner_unavailable"
)

// ExecutionMode selects how a profile runs scanner code.
type ExecutionMode string

const (
	ModeDeployment ExecutionMode = "deployment"
	ModeJob        ExecutionMode = "job"
)

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

// Limits are server-owned scan limits returned with each claim.
type Limits struct {
	MaxArtifactBytes  int64 `json:"max_artifact_bytes,omitempty"`
	MaxExtractedBytes int64 `json:"max_extracted_bytes,omitempty"`
	MaxFiles          int64 `json:"max_files,omitempty"`
}

// Profile resolves repository policy into scanner execution settings.
type Profile struct {
	Name             string        `json:"name"`
	Scanner          string        `json:"scanner"`
	Mode             ExecutionMode `json:"mode"`
	ConfigHash       string        `json:"config_hash"`
	RuntimeClassName string        `json:"runtime_class_name,omitempty"`
	Limits           Limits        `json:"limits"`
	StoreSBOM        bool          `json:"store_sbom,omitempty"`
	CreatedAt        time.Time     `json:"created_at,omitempty"`
	UpdatedAt        time.Time     `json:"updated_at,omitempty"`
}

// ScannerCapability describes one worker-side scanner implementation. The
// server uses this for worker matching, health display, and freshness checks.
type ScannerCapability struct {
	Name                  string    `json:"name"`
	Version               string    `json:"version,omitempty"`
	DatabaseSchemaVersion string    `json:"database_schema_version,omitempty"`
	DatabaseBuiltAt       time.Time `json:"database_built_at,omitempty"`
	SupportsSBOM          bool      `json:"supports_sbom,omitempty"`
	SupportedEcosystems   []string  `json:"supported_ecosystems,omitempty"`
	ReportedAt            time.Time `json:"reported_at,omitempty"`
}

// WorkerCapability is the latest capability report from one worker process.
type WorkerCapability struct {
	WorkerID     string              `json:"worker_id"`
	Capabilities []ScannerCapability `json:"capabilities"`
	ReportedAt   time.Time           `json:"reported_at"`
}

// Job is a scanner unit of work owned by the forklift leader.
type Job struct {
	ID                string
	BlobSHA256        string
	ScannerProfile    string
	Scanner           string
	ScannerConfigHash string
	Status            JobStatus
	WorkerID          string
	Attempts          int
	LeaseUntil        time.Time
	NextRunAt         time.Time
	LastHeartbeatAt   time.Time
	CreatedAt         time.Time
	StartedAt         time.Time
	FinishedAt        time.Time
	Error             string
	Limits            Limits
	StoreSBOM         bool
}

// Target describes the repository artifact that caused a blob scan. A blob may
// be referenced by more than one artifact; workers treat this as best-effort
// package coordinate context and still verify the blob digest separately.
type Target struct {
	Repository     string     `json:"repository,omitempty"`
	RepositoryID   int64      `json:"repository_id,omitempty"`
	Format         string     `json:"format,omitempty"`
	Type           string     `json:"type,omitempty"`
	ArtifactID     int64      `json:"artifact_id,omitempty"`
	Path           string     `json:"path,omitempty"`
	Version        string     `json:"version,omitempty"`
	Size           int64      `json:"size,omitempty"`
	ContentType    string     `json:"content_type,omitempty"`
	MetadataJSON   string     `json:"metadata_json,omitempty"`
	PublishedAt    *time.Time `json:"published_at,omitempty"`
	CachedAt       time.Time  `json:"cached_at,omitempty"`
	LastAccessedAt time.Time  `json:"last_accessed_at,omitempty"`
	UpdatedAt      time.Time  `json:"updated_at,omitempty"`
	PackageName    string     `json:"package_name,omitempty"`
	Ecosystem      string     `json:"ecosystem,omitempty"`
	DepsDevSystem  string     `json:"depsdev_system,omitempty"`
	PURL           string     `json:"purl,omitempty"`
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

// Result is the worker-submitted, normalized scan report. The server validates
// it before persistence or policy evaluation.
type Result struct {
	ID                    int64        `json:"id,omitempty"`
	JobID                 string       `json:"job_id"`
	BlobSHA256            string       `json:"blob_sha256"`
	ScannerProfile        string       `json:"scanner_profile,omitempty"`
	Scanner               string       `json:"scanner"`
	ScannerVersion        string       `json:"scanner_version"`
	ScannerConfigHash     string       `json:"scanner_config_hash,omitempty"`
	DatabaseSchemaVersion string       `json:"database_schema_version,omitempty"`
	DatabaseBuiltAt       time.Time    `json:"database_built_at,omitempty"`
	DatabaseProviders     []DBProvider `json:"database_providers,omitempty"`
	Status                ReportStatus `json:"status"`
	MaxSeverity           Severity     `json:"max_severity"`
	Findings              []Finding    `json:"findings,omitempty"`
	SBOM                  *SBOM        `json:"sbom,omitempty"`
	RawResultDigest       string       `json:"raw_result_digest,omitempty"`
	Error                 string       `json:"error,omitempty"`
	ScannedAt             time.Time    `json:"scanned_at"`
	CreatedAt             time.Time    `json:"created_at,omitempty"`
}

// Verdict is a repository-specific policy decision derived from a report.
type Verdict struct {
	ID             int64         `json:"id,omitempty"`
	RepositoryID   int64         `json:"repository_id"`
	BlobSHA256     string        `json:"blob_sha256"`
	ResultID       int64         `json:"result_id,omitempty"`
	ScannerProfile string        `json:"scanner_profile"`
	PolicyHash     string        `json:"policy_hash,omitempty"`
	Status         VerdictStatus `json:"status"`
	Reason         string        `json:"reason,omitempty"`
	MaxSeverity    Severity      `json:"max_severity,omitempty"`
	ComputedAt     time.Time     `json:"computed_at"`
}

// SBOM is optional inventory data produced by an SBOM-capable profile.
type SBOM struct {
	ID               int64     `json:"id,omitempty"`
	BlobSHA256       string    `json:"blob_sha256"`
	ResultID         int64     `json:"result_id"`
	Format           string    `json:"format"`
	Generator        string    `json:"generator"`
	GeneratorVersion string    `json:"generator_version"`
	ContentDigest    string    `json:"content_digest"`
	ContentJSON      string    `json:"content_json,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// Export tracks asynchronous delivery of a stored SBOM to an external system.
type Export struct {
	ID          int64     `json:"id,omitempty"`
	SBOMID      int64     `json:"sbom_id"`
	Destination string    `json:"destination"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	Attempts    int       `json:"attempts,omitempty"`
	NextRunAt   time.Time `json:"next_run_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
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
