package artifactscan

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ValidationLimits bounds untrusted scanner results before persistence.
type ValidationLimits struct {
	MaxFindings       int
	MaxStringLength   int
	MaxFixedVersions  int
	RequireDBMetadata bool
}

// DefaultValidationLimits returns conservative limits for scanner result input.
func DefaultValidationLimits() ValidationLimits {
	return ValidationLimits{
		MaxFindings:       10000,
		MaxStringLength:   4096,
		MaxFixedVersions:  64,
		RequireDBMetadata: true,
	}
}

// ValidateResult validates a worker-submitted result for one expected job and
// blob. It treats all fields as untrusted scanner output.
func ValidateResult(r Result, expectedJobID, expectedBlobSHA256, expectedScanner string, limits ValidationLimits) error {
	if limits.MaxFindings == 0 {
		limits = DefaultValidationLimits()
	}
	if r.JobID != expectedJobID {
		return fmt.Errorf("job id mismatch")
	}
	if r.BlobSHA256 != expectedBlobSHA256 {
		return fmt.Errorf("blob sha256 mismatch")
	}
	if !validSHA256(r.BlobSHA256) {
		return fmt.Errorf("invalid blob sha256")
	}
	if r.Scanner == "" || r.Scanner != expectedScanner {
		return fmt.Errorf("scanner mismatch")
	}
	if !r.Status.Terminal() || r.Status == StatusReused {
		return fmt.Errorf("invalid terminal status %q", r.Status)
	}
	if r.Status == StatusCompleted && len(r.Findings) > limits.MaxFindings {
		return fmt.Errorf("too many findings: %d", len(r.Findings))
	}
	if limits.RequireDBMetadata && r.Status == StatusCompleted && r.DatabaseBuiltAt.IsZero() {
		return errors.New("missing scanner database built time")
	}
	if err := validateString(r.ScannerVersion, "scanner_version", limits.MaxStringLength, true); err != nil {
		return err
	}
	if err := validateString(r.DatabaseSchemaVersion, "database_schema_version", limits.MaxStringLength, false); err != nil {
		return err
	}
	if err := validateString(r.RawResultDigest, "raw_result_digest", limits.MaxStringLength, false); err != nil {
		return err
	}
	if err := validateString(r.Error, "error", limits.MaxStringLength, false); err != nil {
		return err
	}
	for i, f := range r.Findings {
		if err := validateFinding(f, i, limits); err != nil {
			return err
		}
	}
	return nil
}

func validateFinding(f Finding, idx int, limits ValidationLimits) error {
	prefix := fmt.Sprintf("finding[%d]", idx)
	if err := validateString(f.VulnerabilityID, prefix+".vulnerability_id", limits.MaxStringLength, true); err != nil {
		return err
	}
	if SeverityRank(f.Severity) == 0 && f.Severity != SeverityUnknown {
		return fmt.Errorf("%s.severity invalid", prefix)
	}
	if err := validateString(f.PackageName, prefix+".package_name", limits.MaxStringLength, true); err != nil {
		return err
	}
	if err := validateString(f.PackageVersion, prefix+".package_version", limits.MaxStringLength, false); err != nil {
		return err
	}
	if err := validateString(f.PackageType, prefix+".package_type", limits.MaxStringLength, false); err != nil {
		return err
	}
	if err := validateString(f.PackagePURL, prefix+".package_purl", limits.MaxStringLength, false); err != nil {
		return err
	}
	if len(f.FixedVersions) > limits.MaxFixedVersions {
		return fmt.Errorf("%s.fixed_versions too many values", prefix)
	}
	for j, v := range f.FixedVersions {
		if err := validateString(v, fmt.Sprintf("%s.fixed_versions[%d]", prefix, j), limits.MaxStringLength, false); err != nil {
			return err
		}
	}
	if err := validateString(f.Source, prefix+".source", limits.MaxStringLength, false); err != nil {
		return err
	}
	if err := validateString(f.MatchType, prefix+".match_type", limits.MaxStringLength, false); err != nil {
		return err
	}
	if err := validateURL(f.SourceURL, prefix+".source_url", limits.MaxStringLength); err != nil {
		return err
	}
	return nil
}

func validateString(s, field string, max int, required bool) error {
	if required && strings.TrimSpace(s) == "" {
		return fmt.Errorf("%s required", field)
	}
	if max > 0 && len(s) > max {
		return fmt.Errorf("%s too long", field)
	}
	return nil
}

func validateURL(raw, field string, max int) error {
	if err := validateString(raw, field, max, false); err != nil {
		return err
	}
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s invalid", field)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%s scheme must be http or https", field)
	}
	return nil
}

func validSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
