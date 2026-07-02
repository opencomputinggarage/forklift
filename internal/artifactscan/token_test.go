package artifactscan

import (
	"testing"
	"time"
)

func TestTokenSigner(t *testing.T) {
	s, err := NewTokenSigner([]byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	token, err := s.Sign(TokenClaims{
		JobID:      "job",
		BlobSHA256: "sha",
		Scanner:    "grype",
		ExpiresAt:  now.Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := s.Verify(token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.JobID != "job" || claims.Scanner != "grype" {
		t.Fatalf("claims = %+v", claims)
	}
	if _, err := s.Verify(token + "x"); err == nil {
		t.Fatal("tampered token accepted")
	}
	s.now = func() time.Time { return now.Add(2 * time.Minute) }
	if _, err := s.Verify(token); err == nil {
		t.Fatal("expired token accepted")
	}
}
