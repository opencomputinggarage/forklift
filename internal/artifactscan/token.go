package artifactscan

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// TokenClaims are the least-privilege claims granted to a scanner worker for
// one job.
type TokenClaims struct {
	JobID      string    `json:"job_id"`
	BlobSHA256 string    `json:"blob_sha256"`
	Scanner    string    `json:"scanner"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// TokenSigner signs and verifies internal scan capability tokens.
type TokenSigner struct {
	key []byte
	now func() time.Time
}

// NewTokenSigner creates a signer. If key is empty, a process-local key is
// generated; production deployments should pass stable secret material.
func NewTokenSigner(key []byte) (*TokenSigner, error) {
	if len(key) == 0 {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
	}
	return &TokenSigner{key: append([]byte(nil), key...), now: time.Now}, nil
}

// Sign returns a compact bearer token for the given claims.
func (s *TokenSigner) Sign(claims TokenClaims) (string, error) {
	if s == nil || len(s.key) == 0 {
		return "", errors.New("token signer not configured")
	}
	body, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	sig := s.sign(body)
	return base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// Verify validates a bearer token and returns its claims.
func (s *TokenSigner) Verify(token string) (TokenClaims, error) {
	if s == nil || len(s.key) == 0 {
		return TokenClaims{}, errors.New("token signer not configured")
	}
	bodyB64, sigB64, ok := strings.Cut(token, ".")
	if !ok {
		return TokenClaims{}, errors.New("invalid token format")
	}
	body, err := base64.RawURLEncoding.DecodeString(bodyB64)
	if err != nil {
		return TokenClaims{}, fmt.Errorf("decode token body: %w", err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return TokenClaims{}, fmt.Errorf("decode token signature: %w", err)
	}
	if !hmac.Equal(sig, s.sign(body)) {
		return TokenClaims{}, errors.New("invalid token signature")
	}
	var claims TokenClaims
	if err := json.Unmarshal(body, &claims); err != nil {
		return TokenClaims{}, err
	}
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	if !claims.ExpiresAt.IsZero() && now().After(claims.ExpiresAt) {
		return TokenClaims{}, errors.New("token expired")
	}
	return claims, nil
}

func (s *TokenSigner) sign(body []byte) []byte {
	mac := hmac.New(sha256.New, s.key)
	mac.Write(body)
	return mac.Sum(nil)
}
