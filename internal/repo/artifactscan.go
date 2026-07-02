package repo

import (
	"context"

	"github.com/younsl/o/box/kubernetes/forklift/internal/meta"
	"github.com/younsl/o/box/kubernetes/forklift/internal/repoconfig"
)

func (m *Manager) enqueueArtifactStored(repo meta.Repository, artifactPath string) {
	if m.artifactScanEnqueue == nil {
		return
	}
	art, err := m.store.GetArtifact(context.Background(), repo.ID, artifactPath)
	if err != nil || art.BlobSHA256 == "" {
		return
	}
	cfg, err := repoconfig.Parse(repo.ConfigJSON)
	if err != nil || !cfg.ArtifactScan.Enabled {
		return
	}
	m.artifactScanEnqueue(art.BlobSHA256, cfg.ArtifactScan.EffectiveScannerProfile())
}
