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
