package artifactscan

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

// Store is the server-side persistence contract used by Service. meta.Store
// implements this interface.
type Store interface {
	EnqueueArtifactScan(ctx context.Context, id, blobSHA256, scanner, configHash string, now time.Time) (Job, error)
	ClaimArtifactScanJob(ctx context.Context, workerID string, leaseUntil, now time.Time) (Job, error)
	ArtifactScanTargets(ctx context.Context, blobSHA256 string) ([]Target, error)
	HeartbeatArtifactScanJob(ctx context.Context, id, workerID string, leaseUntil, now time.Time) error
	CompleteArtifactScanJob(ctx context.Context, id, workerID string, result Result, now time.Time) (int64, error)
}

// Service coordinates artifact scan jobs without executing scanner tools.
type Service struct {
	store    Store
	signer   *TokenSigner
	scanner  string
	config   string
	leaseTTL time.Duration
	tokenTTL time.Duration
	now      func() time.Time
}

// ServiceConfig configures the artifact scan service.
type ServiceConfig struct {
	Scanner           string
	ScannerConfigHash string
	LeaseTTL          time.Duration
	TokenTTL          time.Duration
	TokenKey          []byte
}

// ClaimedJob is a job plus its short-lived worker capability token.
type ClaimedJob struct {
	Job     Job
	Token   string
	Targets []Target
}

// NewService builds a server-side artifact scan coordinator.
func NewService(store Store, cfg ServiceConfig) (*Service, error) {
	if store == nil {
		return nil, errors.New("artifact scan store required")
	}
	if cfg.Scanner == "" {
		cfg.Scanner = "grype"
	}
	if cfg.LeaseTTL <= 0 {
		cfg.LeaseTTL = 10 * time.Minute
	}
	if cfg.TokenTTL <= 0 {
		cfg.TokenTTL = cfg.LeaseTTL
	}
	signer, err := NewTokenSigner(cfg.TokenKey)
	if err != nil {
		return nil, err
	}
	return &Service{
		store:    store,
		signer:   signer,
		scanner:  cfg.Scanner,
		config:   cfg.ScannerConfigHash,
		leaseTTL: cfg.LeaseTTL,
		tokenTTL: cfg.TokenTTL,
		now:      time.Now,
	}, nil
}

// Enqueue creates a scan job for the configured scanner.
func (s *Service) Enqueue(ctx context.Context, blobSHA256 string) (Job, error) {
	id, err := randomID()
	if err != nil {
		return Job{}, err
	}
	return s.store.EnqueueArtifactScan(ctx, id, blobSHA256, s.scanner, s.config, s.now().UTC())
}

// Claim claims the next queued job and returns a capability token.
func (s *Service) Claim(ctx context.Context, workerID string) (ClaimedJob, error) {
	now := s.now().UTC()
	job, err := s.store.ClaimArtifactScanJob(ctx, workerID, now.Add(s.leaseTTL), now)
	if err != nil {
		return ClaimedJob{}, err
	}
	targets, err := s.store.ArtifactScanTargets(ctx, job.BlobSHA256)
	if err != nil {
		return ClaimedJob{}, err
	}
	token, err := s.signer.Sign(TokenClaims{
		JobID:      job.ID,
		BlobSHA256: job.BlobSHA256,
		Scanner:    job.Scanner,
		ExpiresAt:  now.Add(s.tokenTTL),
	})
	if err != nil {
		return ClaimedJob{}, err
	}
	return ClaimedJob{Job: job, Token: token, Targets: targets}, nil
}

// Heartbeat extends a running job lease after token validation.
func (s *Service) Heartbeat(ctx context.Context, token, workerID string) error {
	claims, err := s.signer.Verify(token)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	return s.store.HeartbeatArtifactScanJob(ctx, claims.JobID, workerID, now.Add(s.leaseTTL), now)
}

// Complete validates and stores a worker-submitted result.
func (s *Service) Complete(ctx context.Context, token, workerID string, result Result) (int64, error) {
	claims, err := s.signer.Verify(token)
	if err != nil {
		return 0, err
	}
	if err := ValidateResult(result, claims.JobID, claims.BlobSHA256, claims.Scanner, DefaultValidationLimits()); err != nil {
		return 0, err
	}
	return s.store.CompleteArtifactScanJob(ctx, claims.JobID, workerID, result, s.now().UTC())
}

// VerifyToken validates a scan token for blob downloads.
func (s *Service) VerifyToken(token string) (TokenClaims, error) {
	return s.signer.Verify(token)
}

func randomID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
