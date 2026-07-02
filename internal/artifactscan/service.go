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
	EnsureArtifactScannerProfile(ctx context.Context, profile Profile) error
	GetArtifactScannerProfile(ctx context.Context, name string) (Profile, error)
	EnqueueArtifactScan(ctx context.Context, id, blobSHA256, profileName string, now time.Time) (Job, error)
	ClaimArtifactScanJob(ctx context.Context, workerID string, capabilities []ScannerCapability, leaseUntil, now time.Time, maxAttempts int) (Job, error)
	ArtifactScanTargets(ctx context.Context, blobSHA256 string) ([]Target, error)
	HeartbeatArtifactScanJob(ctx context.Context, id, workerID string, leaseUntil, now time.Time) error
	CompleteArtifactScanJob(ctx context.Context, id, workerID string, result Result, now time.Time) (int64, error)
}

// Service coordinates artifact scan jobs without executing scanner tools.
type Service struct {
	store       Store
	signer      *TokenSigner
	profiles    map[string]Profile
	defaultName string
	leaseTTL    time.Duration
	tokenTTL    time.Duration
	maxAttempts int
	now         func() time.Time
}

// ServiceConfig configures the artifact scan service.
type ServiceConfig struct {
	DefaultProfile string
	Profiles       []Profile
	LeaseTTL       time.Duration
	TokenTTL       time.Duration
	TokenKey       []byte
	MaxAttempts    int
}

// ClaimedJob is a job plus its short-lived worker capability token.
type ClaimedJob struct {
	Job      Job
	Token    string
	Deadline time.Time
	Limits   Limits
	Targets  []Target
}

// NewService builds a server-side artifact scan coordinator.
func NewService(store Store, cfg ServiceConfig) (*Service, error) {
	if store == nil {
		return nil, errors.New("artifact scan store required")
	}
	if cfg.DefaultProfile == "" {
		cfg.DefaultProfile = "grype-default"
	}
	if cfg.LeaseTTL <= 0 {
		cfg.LeaseTTL = 10 * time.Minute
	}
	if cfg.TokenTTL <= 0 {
		cfg.TokenTTL = cfg.LeaseTTL
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	profiles := map[string]Profile{}
	for _, p := range cfg.Profiles {
		p = normalizeProfile(p)
		if p.Name == "" {
			return nil, errors.New("artifact scanner profile name required")
		}
		if p.Scanner == "" {
			return nil, errors.New("artifact scanner profile scanner required")
		}
		profiles[p.Name] = p
	}
	if _, ok := profiles[cfg.DefaultProfile]; !ok {
		p := normalizeProfile(Profile{
			Name:       cfg.DefaultProfile,
			Scanner:    "grype",
			Mode:       ModeDeployment,
			ConfigHash: cfg.DefaultProfile + "-v1",
			Limits:     Limits{MaxArtifactBytes: 100 << 20},
		})
		profiles[p.Name] = p
	}
	signer, err := NewTokenSigner(cfg.TokenKey)
	if err != nil {
		return nil, err
	}
	return &Service{
		store:       store,
		signer:      signer,
		profiles:    profiles,
		defaultName: cfg.DefaultProfile,
		leaseTTL:    cfg.LeaseTTL,
		tokenTTL:    cfg.TokenTTL,
		maxAttempts: cfg.MaxAttempts,
		now:         time.Now,
	}, nil
}

// InitProfiles persists configured profiles so repository paths can resolve
// them consistently.
func (s *Service) InitProfiles(ctx context.Context) error {
	now := s.now().UTC()
	for _, p := range s.profiles {
		if p.CreatedAt.IsZero() {
			p.CreatedAt = now
		}
		if p.UpdatedAt.IsZero() {
			p.UpdatedAt = now
		}
		if err := s.store.EnsureArtifactScannerProfile(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

// DefaultProfile returns the configured default profile name.
func (s *Service) DefaultProfile() string {
	if s == nil || s.defaultName == "" {
		return "grype-default"
	}
	return s.defaultName
}

// Enqueue creates a scan job for a profile.
func (s *Service) Enqueue(ctx context.Context, blobSHA256, profileName string) (Job, error) {
	if profileName == "" {
		profileName = s.DefaultProfile()
	}
	id, err := randomID()
	if err != nil {
		return Job{}, err
	}
	return s.store.EnqueueArtifactScan(ctx, id, blobSHA256, profileName, s.now().UTC())
}

// Claim claims the next job matching the worker's capabilities and returns a
// capability token.
func (s *Service) Claim(ctx context.Context, workerID string, capabilities []ScannerCapability) (ClaimedJob, error) {
	if workerID == "" {
		return ClaimedJob{}, errors.New("worker_id required")
	}
	if len(capabilities) == 0 {
		return ClaimedJob{}, errors.New("worker capabilities required")
	}
	now := s.now().UTC()
	deadline := now.Add(s.leaseTTL)
	job, err := s.store.ClaimArtifactScanJob(ctx, workerID, capabilities, deadline, now, s.maxAttempts)
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
	return ClaimedJob{Job: job, Token: token, Deadline: deadline, Limits: job.Limits, Targets: targets}, nil
}

// Heartbeat extends a running job lease after token validation and renews the
// short-lived job token for long-running scans.
func (s *Service) Heartbeat(ctx context.Context, token, workerID string) (ClaimedJob, error) {
	claims, err := s.signer.Verify(token)
	if err != nil {
		return ClaimedJob{}, err
	}
	now := s.now().UTC()
	deadline := now.Add(s.leaseTTL)
	if err := s.store.HeartbeatArtifactScanJob(ctx, claims.JobID, workerID, deadline, now); err != nil {
		return ClaimedJob{}, err
	}
	renewed, err := s.signer.Sign(TokenClaims{
		JobID:      claims.JobID,
		BlobSHA256: claims.BlobSHA256,
		Scanner:    claims.Scanner,
		ExpiresAt:  now.Add(s.tokenTTL),
	})
	if err != nil {
		return ClaimedJob{}, err
	}
	return ClaimedJob{Token: renewed, Deadline: deadline}, nil
}

// Complete validates and stores a worker-submitted report.
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

func normalizeProfile(p Profile) Profile {
	if p.Mode == "" {
		p.Mode = ModeDeployment
	}
	if p.ConfigHash == "" && p.Name != "" {
		p.ConfigHash = p.Name + "-v1"
	}
	if p.Limits.MaxArtifactBytes <= 0 {
		p.Limits.MaxArtifactBytes = 100 << 20
	}
	return p
}

func randomID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
