package auth

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// ref is an arbitrary fixed instant the clock-boundary tests pin against, so
// "expired" and "live" are decided by comparison to a known constant rather
// than the wall clock — the whole point of the injectable-now seam (#368).
var ref = time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

func tsAt(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

// TestResetTokenUsable_ExpiryBoundary pins the reset-token expiry edge: a token
// is usable strictly before its expires_at, and the instant it is reached (or
// past) it is not. Used or expiry-less tokens are never usable. Exercises the
// short-TTL / expired-link-rejected half of the emailed-reset contract without
// a DB round-trip, now that the boundary reads an injected clock.
//
// covers: INV-AUTH-19
func TestResetTokenUsable_ExpiryBoundary(t *testing.T) {
	live := db.PasswordResetToken{ExpiresAt: tsAt(ref.Add(time.Minute))}
	tests := []struct {
		name string
		row  db.PasswordResetToken
		now  time.Time
		want bool
	}{
		{"one second before expiry", live, ref.Add(-time.Second), true},
		{"exactly at expiry is not usable", db.PasswordResetToken{ExpiresAt: tsAt(ref)}, ref, false},
		{"past expiry", db.PasswordResetToken{ExpiresAt: tsAt(ref.Add(-time.Minute))}, ref, false},
		{"already used", db.PasswordResetToken{ExpiresAt: tsAt(ref.Add(time.Hour)), UsedAt: tsAt(ref)}, ref, false},
		{"no expiry set", db.PasswordResetToken{}, ref, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := resetTokenUsable(tc.row, tc.now); got != tc.want {
				t.Errorf("resetTokenUsable = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestInviteAcceptable_ExpiryBoundary is the invitation counterpart: an invite
// is acceptable strictly before expiry, and a used or expired one is refused —
// the expiring/single-use half of the invitation-token contract, pinned to an
// injected clock.
//
// covers: INV-AUTH-06
func TestInviteAcceptable_ExpiryBoundary(t *testing.T) {
	tests := []struct {
		name string
		inv  db.HouseholdInvitation
		now  time.Time
		want bool
	}{
		{"one second before expiry", db.HouseholdInvitation{ExpiresAt: tsAt(ref.Add(time.Minute))}, ref.Add(-time.Second), true},
		{"exactly at expiry is not acceptable", db.HouseholdInvitation{ExpiresAt: tsAt(ref)}, ref, false},
		{"past expiry", db.HouseholdInvitation{ExpiresAt: tsAt(ref.Add(-time.Minute))}, ref, false},
		{"already used", db.HouseholdInvitation{ExpiresAt: tsAt(ref.Add(time.Hour)), UsedAt: tsAt(ref)}, ref, false},
		{"no expiry set", db.HouseholdInvitation{}, ref, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := inviteAcceptable(tc.inv, tc.now); got != tc.want {
				t.Errorf("inviteAcceptable = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestWithNow_OverridesClock verifies the Option threads through New so a test
// clock replaces time.Now for every expiry/TTL computation in the package. The
// default path (no Option) keeps the real clock. New never touches the DB during
// construction, so a nil-pool Queries keeps this hermetic (no container).
func TestWithNow_OverridesClock(t *testing.T) {
	q := db.New(nil)
	cfg := Config{
		LocalEnabled: true,
		Mailer:       stubMailerForNew{},
		BackendURL:   "http://localhost:8080",
	}

	h, err := New(context.Background(), q, cfg, WithNow(func() time.Time { return ref }))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := h.now(); !got.Equal(ref) {
		t.Errorf("h.now() = %v, want injected %v", got, ref)
	}

	def, err := New(context.Background(), q, cfg)
	if err != nil {
		t.Fatalf("New (default): %v", err)
	}
	if def.now == nil {
		t.Fatal("default now is nil; want time.Now")
	}
	if delta := time.Since(def.now()); delta < 0 || delta > time.Minute {
		t.Errorf("default now() = %v ago; want ~real time.Now", delta)
	}
}
