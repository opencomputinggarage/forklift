package scannerworker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

func TestPrepareArtifact(t *testing.T) {
	body := "artifact bytes"
	sum := sha256.Sum256([]byte(body))
	root := t.TempDir()
	prepared, err := PrepareArtifact(context.Background(), root, strings.NewReader(body), hex.EncodeToString(sum[:]), WorkspaceLimits{MaxArtifactBytes: 1024})
	if err != nil {
		t.Fatalf("prepare artifact: %v", err)
	}
	got, err := os.ReadFile(prepared.FilePath)
	if err != nil {
		t.Fatalf("read prepared artifact: %v", err)
	}
	if string(got) != body {
		t.Fatalf("prepared bytes = %q, want %q", got, body)
	}
}

func TestPrepareArtifactRejectsOversize(t *testing.T) {
	body := "artifact bytes"
	sum := sha256.Sum256([]byte(body))
	_, err := PrepareArtifact(context.Background(), t.TempDir(), strings.NewReader(body), hex.EncodeToString(sum[:]), WorkspaceLimits{MaxArtifactBytes: 4})
	if err == nil {
		t.Fatal("oversize artifact accepted")
	}
}
