package scannerworker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/younsl/o/box/kubernetes/forklift/internal/artifactscan"
)

// WorkspaceLimits bounds the worker's local preparation step. Kubernetes
// resource limits and emptyDir sizeLimit should also enforce these boundaries.
type WorkspaceLimits struct {
	MaxArtifactBytes int64
}

// PreparedArtifact is the local filesystem view passed to scanner drivers.
type PreparedArtifact struct {
	Root       string
	InputDir   string
	WorkDir    string
	OutputDir  string
	BlobSHA256 string
	FilePath   string
	Size       int64
	Targets    []artifactscan.Target
}

// PrepareArtifact writes one blob into a private workspace and verifies its
// digest. It does not extract archives; extraction/cataloging belongs inside a
// scanner driver running under the worker security boundary.
func PrepareArtifact(ctx context.Context, root string, blob io.Reader, expectedSHA256 string, limits WorkspaceLimits) (PreparedArtifact, error) {
	if expectedSHA256 == "" {
		return PreparedArtifact{}, errors.New("expected sha256 required")
	}
	if limits.MaxArtifactBytes <= 0 {
		return PreparedArtifact{}, errors.New("max artifact bytes must be positive")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return PreparedArtifact{}, fmt.Errorf("create workspace root: %w", err)
	}
	prepared := PreparedArtifact{
		Root:       root,
		InputDir:   filepath.Join(root, "input"),
		WorkDir:    filepath.Join(root, "work"),
		OutputDir:  filepath.Join(root, "output"),
		BlobSHA256: expectedSHA256,
	}
	for _, dir := range []string{prepared.InputDir, prepared.WorkDir, prepared.OutputDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return PreparedArtifact{}, fmt.Errorf("create workspace dir: %w", err)
		}
	}
	prepared.FilePath = filepath.Join(prepared.InputDir, "artifact")
	f, err := os.OpenFile(prepared.FilePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return PreparedArtifact{}, fmt.Errorf("create input artifact: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	limited := &limitReader{r: blob, n: limits.MaxArtifactBytes + 1}
	n, err := copyWithContext(ctx, io.MultiWriter(f, h), limited)
	if err != nil {
		return PreparedArtifact{}, fmt.Errorf("write input artifact: %w", err)
	}
	if n > limits.MaxArtifactBytes {
		return PreparedArtifact{}, fmt.Errorf("artifact exceeds max size %d", limits.MaxArtifactBytes)
	}
	digest := hex.EncodeToString(h.Sum(nil))
	if digest != expectedSHA256 {
		return PreparedArtifact{}, fmt.Errorf("artifact digest mismatch")
	}
	if err := f.Sync(); err != nil {
		return PreparedArtifact{}, fmt.Errorf("sync input artifact: %w", err)
	}
	prepared.Size = n
	return prepared, nil
}

// Cleanup removes the prepared workspace.
func (p PreparedArtifact) Cleanup() error {
	if p.Root == "" {
		return nil
	}
	return os.RemoveAll(p.Root)
}

type limitReader struct {
	r io.Reader
	n int64
}

func (l *limitReader) Read(p []byte) (int, error) {
	if l.n <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > l.n {
		p = p[:l.n]
	}
	n, err := l.r.Read(p)
	l.n -= int64(n)
	return n, err
}

func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				return written, ew
			}
			if nr != nw {
				return written, io.ErrShortWrite
			}
		}
		if er != nil {
			if er == io.EOF {
				return written, nil
			}
			return written, er
		}
	}
}
