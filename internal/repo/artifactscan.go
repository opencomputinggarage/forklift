package repo

import (
	"context"

	"github.com/younsl/o/box/kubernetes/forklift/internal/meta"
)

func (m *Manager) enqueueArtifactStored(repo meta.Repository, artifactPath string) {
	if m.artifactScanEnqueue == nil {
		return
	}
	art, err := m.store.GetArtifact(context.Background(), repo.ID, artifactPath)
	if err != nil || art.BlobSHA256 == "" {
		return
	}
	m.artifactScanEnqueue(art.BlobSHA256)
}
