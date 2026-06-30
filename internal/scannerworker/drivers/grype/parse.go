package grype

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/younsl/o/box/kubernetes/forklift/internal/artifactscan"
)

type document struct {
	Descriptor struct {
		Version string `json:"version"`
	} `json:"descriptor"`
	DB struct {
		Status struct {
			SchemaVersion string    `json:"schemaVersion"`
			Built         time.Time `json:"built"`
		} `json:"status"`
		Providers map[string]struct {
			Captured string `json:"captured"`
			Input    string `json:"input"`
		} `json:"providers"`
	} `json:"db"`
	Matches []matchDoc `json:"matches"`
}

type dbStatusDoc struct {
	SchemaVersion string    `json:"schemaVersion"`
	Built         time.Time `json:"built"`
}

type matchDoc struct {
	Vulnerability struct {
		ID         string   `json:"id"`
		Severity   string   `json:"severity"`
		DataSource string   `json:"dataSource"`
		URLs       []string `json:"urls"`
		Fix        struct {
			Versions []string `json:"versions"`
		} `json:"fix"`
	} `json:"vulnerability"`
	Artifact struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Type    string `json:"type"`
		PURL    string `json:"purl"`
		FoundBy string `json:"foundBy"`
	} `json:"artifact"`
	MatchDetails []struct {
		Type string `json:"type"`
	} `json:"matchDetails"`
}

// Normalize converts Grype JSON output into the artifactscan result model.
func Normalize(raw []byte) (artifactscan.Result, error) {
	var doc document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return artifactscan.Result{}, err
	}
	res := artifactscan.Result{
		Scanner:               "grype",
		ScannerVersion:        doc.Descriptor.Version,
		DatabaseSchemaVersion: doc.DB.Status.SchemaVersion,
		DatabaseBuiltAt:       doc.DB.Status.Built,
		Status:                artifactscan.StatusCompleted,
	}
	for id, p := range doc.DB.Providers {
		var captured time.Time
		if p.Captured != "" {
			captured, _ = time.Parse(time.RFC3339, p.Captured)
		}
		res.DatabaseProviders = append(res.DatabaseProviders, artifactscan.DBProvider{
			ID:          id,
			CapturedAt:  captured,
			InputDigest: p.Input,
		})
	}
	for _, m := range doc.Matches {
		f := artifactscan.Finding{
			VulnerabilityID: m.Vulnerability.ID,
			Severity:        artifactscan.ParseSeverity(m.Vulnerability.Severity),
			PackageName:     m.Artifact.Name,
			PackageVersion:  m.Artifact.Version,
			PackageType:     m.Artifact.Type,
			PackagePURL:     m.Artifact.PURL,
			FixedVersions:   m.Vulnerability.Fix.Versions,
			Source:          m.Vulnerability.DataSource,
			SourceURL:       firstURL(m.Vulnerability.URLs),
			MatchType:       matchType(m.MatchDetails),
		}
		res.Findings = append(res.Findings, f)
	}
	res.RecomputeSummary()
	return res, nil
}

func parseDBStatus(raw []byte) (dbStatusDoc, error) {
	var doc dbStatusDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return dbStatusDoc{}, err
	}
	return doc, nil
}

func firstURL(urls []string) string {
	if len(urls) == 0 {
		return ""
	}
	return urls[0]
}

func matchType(details []struct {
	Type string `json:"type"`
}) string {
	if len(details) == 0 {
		return ""
	}
	var out []string
	for _, d := range details {
		if d.Type != "" {
			out = append(out, d.Type)
		}
	}
	return strings.Join(out, ",")
}
