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

func TestValidateResultRejectsUntrustedFields(t *testing.T) {
	blob := strings.Repeat("a", 64)
	base := Result{
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
	tests := []struct {
		name   string
		mutate func(*Result)
	}{
		{
			name: "job mismatch",
			mutate: func(r *Result) {
				r.JobID = "job-2"
			},
		},
		{
			name: "scanner mismatch",
			mutate: func(r *Result) {
				r.Scanner = "trivy"
			},
		},
		{
			name: "running status",
			mutate: func(r *Result) {
				r.Status = StatusRunning
			},
		},
		{
			name: "missing db metadata",
			mutate: func(r *Result) {
				r.DatabaseBuiltAt = time.Time{}
			},
		},
		{
			name: "javascript url",
			mutate: func(r *Result) {
				r.Findings[0].SourceURL = "javascript:alert(1)"
			},
		},
		{
			name: "too many fixed versions",
			mutate: func(r *Result) {
				r.Findings[0].FixedVersions = make([]string, DefaultValidationLimits().MaxFixedVersions+1)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := base
			res.Findings = append([]Finding(nil), base.Findings...)
			tt.mutate(&res)
			if err := ValidateResult(res, "job-1", blob, "grype", DefaultValidationLimits()); err == nil {
				t.Fatal("invalid result accepted")
			}
		})
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

func TestEvaluateUnscannedAndDisabled(t *testing.T) {
	if got := Evaluate(Policy{}, nil); !got.Allowed {
		t.Fatalf("disabled policy should allow: %+v", got)
	}
	p := Policy{Enabled: true, Action: PolicyBlock, Threshold: SeverityHigh, BlockUnscanned: true}
	if got := Evaluate(p, nil); got.Allowed {
		t.Fatalf("blockUnscanned policy should block missing result: %+v", got)
	}
	p.Action = PolicyAudit
	if got := Evaluate(p, nil); !got.Allowed {
		t.Fatalf("audit policy should allow missing result: %+v", got)
	}
}

func TestRecomputeSummary(t *testing.T) {
	res := Result{Findings: []Finding{
		{Severity: SeverityLow},
		{Severity: SeverityCritical},
		{Severity: SeverityMedium},
	}}
	res.RecomputeSummary()
	if res.MaxSeverity != SeverityCritical {
		t.Fatalf("max severity = %q, want critical", res.MaxSeverity)
	}
}
