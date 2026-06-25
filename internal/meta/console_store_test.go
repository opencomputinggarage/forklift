package meta

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReceiverStore(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	a, err := s.CreateReceiver(ctx, Receiver{Name: "slack", Description: "sec", WebhookURL: "https://h/x", Enabled: true})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if a.ID == 0 || !a.Enabled || a.CreatedAt.IsZero() {
		t.Fatalf("unexpected receiver: %+v", a)
	}
	b, err := s.CreateReceiver(ctx, Receiver{Name: "pager", WebhookURL: "https://h/y", Enabled: false})
	if err != nil {
		t.Fatal(err)
	}

	// Duplicate name -> ErrConflict.
	if _, err := s.CreateReceiver(ctx, Receiver{Name: "slack", WebhookURL: "https://h/z"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("dup name err = %v, want ErrConflict", err)
	}

	if got, err := s.GetReceiver(ctx, a.ID); err != nil || got.Name != "slack" {
		t.Fatalf("get = %+v err=%v", got, err)
	}
	if _, err := s.GetReceiver(ctx, 99999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get missing err = %v, want ErrNotFound", err)
	}

	if all, err := s.ListReceivers(ctx); err != nil || len(all) != 2 {
		t.Fatalf("list = %d err=%v, want 2", len(all), err)
	}
	if en, err := s.ListEnabledReceivers(ctx); err != nil || len(en) != 1 || en[0].Name != "slack" {
		t.Fatalf("enabled = %+v err=%v", en, err)
	}

	// Update: rename + enable b.
	upd, err := s.UpdateReceiver(ctx, Receiver{ID: b.ID, Name: "pager2", Description: "oncall", WebhookURL: "https://h/y2", Enabled: true})
	if err != nil || upd.Name != "pager2" || !upd.Enabled {
		t.Fatalf("update = %+v err=%v", upd, err)
	}
	if en, _ := s.ListEnabledReceivers(ctx); len(en) != 2 {
		t.Fatalf("enabled after update = %d, want 2", len(en))
	}
	// Update missing -> ErrNotFound.
	if _, err := s.UpdateReceiver(ctx, Receiver{ID: 99999, Name: "ghost", WebhookURL: "https://h/q"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update missing err = %v, want ErrNotFound", err)
	}

	if err := s.DeleteReceiver(ctx, a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := s.DeleteReceiver(ctx, a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete missing err = %v, want ErrNotFound", err)
	}
}

func TestLicenseScanStore(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if err := s.UpsertLicenseScan(ctx, "npm", "left-pad", "1.0.0", []string{"MIT"}, "deps.dev"); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := s.GetLicenseScan(ctx, "npm", "left-pad", "1.0.0")
	if err != nil || len(got.Licenses) != 1 || got.Licenses[0] != "MIT" {
		t.Fatalf("get = %+v err=%v", got, err)
	}

	// Upsert again refreshes licenses and defaults the source.
	if err := s.UpsertLicenseScan(ctx, "npm", "left-pad", "1.0.0", nil, ""); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetLicenseScan(ctx, "npm", "left-pad", "1.0.0")
	if len(got.Licenses) != 0 || got.Source != "deps.dev" {
		t.Fatalf("after refresh = %+v", got)
	}

	if _, err := s.GetLicenseScan(ctx, "npm", "nope", "9.9.9"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get missing err = %v, want ErrNotFound", err)
	}

	keys, err := s.ResolvedLicenseKeys(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := keys["npm\x00left-pad\x001.0.0"]; !ok {
		t.Fatalf("resolved keys missing coordinate: %v", keys)
	}

	stale, err := s.ListStaleLicenseScans(ctx, time.Now().Add(time.Hour), 10)
	if err != nil || len(stale) != 1 {
		t.Fatalf("stale = %d err=%v, want 1", len(stale), err)
	}
	if none, _ := s.ListStaleLicenseScans(ctx, time.Now().Add(-time.Hour), 10); len(none) != 0 {
		t.Fatalf("stale before past cutoff = %d, want 0", len(none))
	}
}

func TestLoginTracking(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	u, err := s.CreateUser(ctx, User{Username: "bob", PasswordHash: "h"})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.TouchLastLogin(ctx, u.ID); err != nil {
		t.Fatalf("touch: %v", err)
	}

	// Opt into lockout, then fail up to the threshold.
	if err := s.SetLockoutEnabled(ctx, u.ID, true); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := s.RegisterFailedLogin(ctx, u.ID, 3); err != nil {
			t.Fatalf("register fail: %v", err)
		}
	}
	got, _ := s.GetUserByUsername(ctx, "bob")
	if got.FailedLoginCount != 3 || !got.Locked() {
		t.Fatalf("after 3 fails: count=%d locked=%v", got.FailedLoginCount, got.Locked())
	}

	// Reset clears the count and unlocks.
	if err := s.ResetFailedLogin(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetUserByUsername(ctx, "bob")
	if got.FailedLoginCount != 0 || got.Locked() {
		t.Fatalf("after reset: count=%d locked=%v", got.FailedLoginCount, got.Locked())
	}

	// Disabling lockout also clears any accumulated failures.
	if err := s.RegisterFailedLogin(ctx, u.ID, 3); err != nil {
		t.Fatal(err)
	}
	if err := s.SetLockoutEnabled(ctx, u.ID, false); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetUserByUsername(ctx, "bob")
	if got.FailedLoginCount != 0 || got.Locked() {
		t.Fatalf("after disable lockout: count=%d locked=%v", got.FailedLoginCount, got.Locked())
	}
}
