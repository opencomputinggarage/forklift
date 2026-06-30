package artifactscan

import (
	"strings"
	"testing"
	"time"
)

func TestValidateResult(t *testing.T) {
	blob := strings.Repeat("a", 64)
	res := Result{
		JobID:           "job-1",
		BlobSHA256:      blob,
		Scanner:         "grype",
		ScannerVersion:  "1.0.0",
		DatabaseBuiltAt: time.Now(),
		Status:          StatusCompleted,
		Findings: []Finding{{
			VulnerabilityID: "CVE-2026-0001",
			Severity:        SeverityHigh,
			PackageName:     "example",
			SourceURL:       "https://example.test/CVE-2026-0001",
		}},
	}
	if err := ValidateResult(res, "job-1", blob, "grype", DefaultValidationLimits()); err != nil {
		t.Fatalf("valid result rejected: %v", err)
	}
	res.BlobSHA256 = strings.Repeat("b", 64)
	if err := ValidateResult(res, "job-1", blob, "grype", DefaultValidationLimits()); err == nil {
		t.Fatal("blob mismatch accepted")
	}
}

func TestEvaluate(t *testing.T) {
	p := Policy{Enabled: true, Action: PolicyBlock, Threshold: SeverityHigh}
	res := &Result{Status: StatusCompleted, MaxSeverity: SeverityMedium}
	if got := Evaluate(p, res); !got.Allowed {
		t.Fatalf("medium should be allowed below high threshold: %+v", got)
	}
	res.MaxSeverity = SeverityCritical
	if got := Evaluate(p, res); got.Allowed {
		t.Fatalf("critical should be blocked: %+v", got)
	}
}
