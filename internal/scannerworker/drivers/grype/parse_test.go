package grype

import (
	"testing"

	"github.com/younsl/o/box/kubernetes/forklift/internal/artifactscan"
)

func TestNormalize(t *testing.T) {
	raw := []byte(`{
	  "descriptor": {"version": "1.2.3"},
	  "db": {"status": {"schemaVersion": "6", "built": "2026-07-01T00:00:00Z"}},
	  "matches": [{
	    "vulnerability": {
	      "id": "CVE-2026-0001",
	      "severity": "High",
	      "dataSource": "https://example.test",
	      "urls": ["https://example.test/CVE-2026-0001"],
	      "fix": {"versions": ["1.0.1"]}
	    },
	    "artifact": {
	      "name": "left-pad",
	      "version": "1.0.0",
	      "type": "npm",
	      "purl": "pkg:npm/left-pad@1.0.0"
	    },
	    "matchDetails": [{"type": "exact-direct-match"}]
	  }]
	}`)
	got, err := Normalize(raw)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if got.ScannerVersion != "1.2.3" {
		t.Fatalf("scanner version = %q", got.ScannerVersion)
	}
	if got.MaxSeverity != artifactscan.SeverityHigh {
		t.Fatalf("max severity = %q", got.MaxSeverity)
	}
	if len(got.Findings) != 1 || got.Findings[0].PackageName != "left-pad" {
		t.Fatalf("findings = %+v", got.Findings)
	}
}

func TestParseDBStatus(t *testing.T) {
	got, err := parseDBStatus([]byte(`{
	  "schemaVersion": "v6.1.7",
	  "built": "2026-06-30T07:34:46Z",
	  "valid": true
	}`))
	if err != nil {
		t.Fatalf("parse db status: %v", err)
	}
	if got.SchemaVersion != "v6.1.7" {
		t.Fatalf("schema version = %q", got.SchemaVersion)
	}
	if got.Built.IsZero() {
		t.Fatal("built time is zero")
	}
}

func TestBestPURLTargetNPM(t *testing.T) {
	got := bestPURLTarget([]artifactscan.Target{{
		Format:  "npm",
		Path:    "axios/-/axios-0.21.1.tgz",
		Version: "0.21.1",
	}})
	if got != "pkg:npm/axios@0.21.1" {
		t.Fatalf("purl = %q", got)
	}
}

func TestBestPURLTargetScopedNPM(t *testing.T) {
	got := bestPURLTarget([]artifactscan.Target{{
		Format:  "npm",
		Path:    "@scope/name/-/name-1.2.3.tgz",
		Version: "1.2.3",
	}})
	if got != "pkg:npm/%40scope/name@1.2.3" {
		t.Fatalf("purl = %q", got)
	}
}

func TestBestPURLTargetSkipsNPMMetadata(t *testing.T) {
	got := bestPURLTarget([]artifactscan.Target{{
		Format: "npm",
		Path:   "axios",
	}})
	if got != "" {
		t.Fatalf("purl = %q, want empty", got)
	}
}

func TestBestPURLTargetOtherEcosystems(t *testing.T) {
	tests := []struct {
		name   string
		target artifactscan.Target
		want   string
	}{
		{
			name: "maven",
			target: artifactscan.Target{
				Format:  "maven",
				Path:    "com/google/guava/guava/31.0/guava-31.0.jar",
				Version: "31.0",
			},
			want: "pkg:maven/com.google.guava/guava@31.0",
		},
		{
			name: "pypi",
			target: artifactscan.Target{
				Format:  "pypi",
				Path:    "packages/ab/cd/requests-2.31.0-py3-none-any.whl",
				Version: "2.31.0",
			},
			want: "pkg:pypi/requests@2.31.0",
		},
		{
			name: "cargo",
			target: artifactscan.Target{
				Format:  "cargo",
				Path:    "api/v1/crates/serde/1.0.197/download",
				Version: "1.0.197",
			},
			want: "pkg:cargo/serde@1.0.197",
		},
		{
			name: "go",
			target: artifactscan.Target{
				Format:  "go",
				Path:    "github.com/gin-gonic/gin/@v/v1.9.0.zip",
				Version: "v1.9.0",
			},
			want: "pkg:golang/github.com/gin-gonic/gin@v1.9.0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bestPURLTarget([]artifactscan.Target{tt.target}); got != tt.want {
				t.Fatalf("purl = %q, want %q", got, tt.want)
			}
		})
	}
}
