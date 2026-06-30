package grype

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/younsl/o/box/kubernetes/forklift/internal/artifactscan"
	"github.com/younsl/o/box/kubernetes/forklift/internal/scannerworker"
)

// Driver runs the Grype CLI and normalizes its JSON output.
type Driver struct {
	Binary string
	Env    []string
}

func (d Driver) Name() string { return "grype" }

func (d Driver) binary() string {
	if d.Binary != "" {
		return d.Binary
	}
	return "grype"
}

// Version returns the Grype CLI version string when available.
func (d Driver) Version(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, d.binary(), "version", "-o", "json")
	cmd.Env = append(os.Environ(), d.Env...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	var doc struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		return strings.TrimSpace(string(out)), nil
	}
	return doc.Version, nil
}

// DBStatus returns local Grype vulnerability database metadata.
func (d Driver) DBStatus(ctx context.Context) (dbStatusDoc, error) {
	cmd := exec.CommandContext(ctx, d.binary(), "db", "status", "-o", "json")
	cmd.Env = append(os.Environ(),
		"GRYPE_DB_AUTO_UPDATE=false",
		"GRYPE_CHECK_FOR_APP_UPDATE=false",
	)
	cmd.Env = append(cmd.Env, d.Env...)
	out, err := cmd.Output()
	if err != nil {
		return dbStatusDoc{}, err
	}
	return parseDBStatus(out)
}

// Scan runs grype against the prepared artifact input directory.
func (d Driver) Scan(ctx context.Context, artifact scannerworker.PreparedArtifact) (artifactscan.Result, error) {
	outPath := filepath.Join(artifact.OutputDir, "grype.json")
	target := "dir:" + artifact.InputDir
	if purl := bestPURLTarget(artifact.Targets); purl != "" {
		target = purl
	}
	cmd := exec.CommandContext(ctx, d.binary(), target, "-o", "json")
	cmd.Env = append(os.Environ(),
		"GRYPE_DB_AUTO_UPDATE=false",
		"GRYPE_CHECK_FOR_APP_UPDATE=false",
	)
	cmd.Env = append(cmd.Env, d.Env...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if writeErr := os.WriteFile(outPath, out, 0o600); writeErr != nil {
		return artifactscan.Result{}, fmt.Errorf("write grype output: %w", writeErr)
	}
	if err != nil {
		return artifactscan.Result{
			BlobSHA256: artifact.BlobSHA256,
			Scanner:    d.Name(),
			Status:     artifactscan.StatusFailed,
			Error:      strings.TrimSpace(stderr.String()),
			ScannedAt:  time.Now().UTC(),
		}, err
	}
	result, err := Normalize(out)
	if err != nil {
		return artifactscan.Result{}, err
	}
	if result.DatabaseBuiltAt.IsZero() || result.DatabaseSchemaVersion == "" {
		status, err := d.DBStatus(ctx)
		if err != nil {
			return artifactscan.Result{}, fmt.Errorf("read grype db status: %w", err)
		}
		if result.DatabaseBuiltAt.IsZero() {
			result.DatabaseBuiltAt = status.Built
		}
		if result.DatabaseSchemaVersion == "" {
			result.DatabaseSchemaVersion = status.SchemaVersion
		}
	}
	result.BlobSHA256 = artifact.BlobSHA256
	result.Scanner = d.Name()
	result.Status = artifactscan.StatusCompleted
	result.RawResultDigest = sha256Hex(out)
	result.ScannedAt = time.Now().UTC()
	result.RecomputeSummary()
	return result, nil
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func bestPURLTarget(targets []artifactscan.Target) string {
	for _, t := range targets {
		if t.PURL != "" {
			return t.PURL
		}
	}
	for _, t := range targets {
		if purl := purlForTarget(t); purl != "" {
			return purl
		}
	}
	return ""
}

func purlForTarget(t artifactscan.Target) string {
	switch t.Format {
	case "npm":
		return npmPURL(t)
	default:
		return ""
	}
}

func npmPURL(t artifactscan.Target) string {
	if t.Version == "" || !strings.HasSuffix(t.Path, ".tgz") {
		return ""
	}
	i := strings.Index(t.Path, "/-/")
	if i <= 0 {
		return ""
	}
	name := t.Path[:i]
	if name == "" {
		return ""
	}
	if strings.HasPrefix(name, "@") {
		parts := strings.SplitN(name, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return ""
		}
		scope := strings.ReplaceAll(url.PathEscape(parts[0]), "@", "%40")
		return "pkg:npm/" + scope + "/" + url.PathEscape(parts[1]) + "@" + url.PathEscape(t.Version)
	}
	return "pkg:npm/" + url.PathEscape(name) + "@" + url.PathEscape(t.Version)
}
