package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// bootstrapNewUser sits between handleCallback and the DB; testing it
// directly lets us exercise the founder / invited-member fork without
// faking the OAuth exchange. handleCallback's HTTP-shaped wiring stays
// uncovered until the exchanger-DI refactor (deferred).

// covers: INV-AUTH-05
func TestCreateFounder(t *testing.T) {
	h := newAuthHarness(t)

	claims := &googleClaims{
		Sub:           "google-sub-new",
		Email:         "founder@example.com",
		EmailVerified: true,
		Name:          "Founder",
	}
	user, err := h.h.createFounder(context.Background(), claims, "en-GB")
	if err != nil {
		t.Fatalf("createFounder: %v", err)
	}
	if user.Email != claims.Email {
		t.Errorf("email: want %q, got %q", claims.Email, user.Email)
	}
	if user.GoogleSub != claims.Sub {
		t.Errorf("google_sub: want %q, got %q", claims.Sub, user.GoogleSub)
	}
	if user.HouseholdID == h.user.HouseholdID {
		t.Error("founder should land in a new household, not the harness one")
	}

	household, err := h.q.GetHouseholdByID(context.Background(), user.HouseholdID)
	if err != nil {
		t.Fatalf("GetHouseholdByID: %v", err)
	}
	if !strings.Contains(household.DisplayName, "Founder") {
		t.Errorf("household display_name should derive from claim name; got %q", household.DisplayName)
	}
}

// covers: INV-AUTH-06, INV-AUTH-07, INV-AUTH-08
func TestBootstrapNewUser(t *testing.T) {
	t.Run("no invite token creates a founder", func(t *testing.T) {
		h := newAuthHarness(t)
		claims := &googleClaims{
			Sub:           "google-sub-1",
			Email:         "newbie@example.com",
			EmailVerified: true,
			Name:          "Newbie",
		}
		user, err := h.h.bootstrapNewUser(context.Background(), claims, "", "en-GB")
		if err != nil {
			t.Fatalf("bootstrapNewUser: %v", err)
		}
		if user.HouseholdID == h.user.HouseholdID {
			t.Error("expected new household for invite-less bootstrap")
		}
	})

	t.Run("valid invite joins existing household and marks invite used", func(t *testing.T) {
		h := newAuthHarness(t)
		token := mustSeedInvitation(t, h, "invited@example.com", time.Now().Add(24*time.Hour))

		claims := &googleClaims{
			Sub:           "google-sub-2",
			Email:         "invited@example.com",
			EmailVerified: true,
			Name:          "Invited",
		}
		user, err := h.h.bootstrapNewUser(context.Background(), claims, token, "en-GB")
		if err != nil {
			t.Fatalf("bootstrapNewUser: %v", err)
		}
		if user.HouseholdID != h.user.HouseholdID {
			t.Errorf("invited user should join existing household; got %s, want %s",
				user.HouseholdID, h.user.HouseholdID)
		}

		// Invitation should now be marked used.
		inv, err := h.q.GetInvitationByToken(context.Background(), token)
		if err != nil {
			t.Fatalf("GetInvitationByToken: %v", err)
		}
		if !inv.UsedAt.Valid {
			t.Error("expected invitation to be marked used")
		}
	})

	t.Run("unknown token returns 'invitation not found'", func(t *testing.T) {
		h := newAuthHarness(t)
		claims := &googleClaims{Sub: "x", Email: "x@example.com", EmailVerified: true, Name: "X"}
		_, err := h.h.bootstrapNewUser(context.Background(), claims, "definitely-not-a-token", "en-GB")
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Errorf("err: want 'not found', got %v", err)
		}
	})

	t.Run("expired invite returns 'invitation expired'", func(t *testing.T) {
		h := newAuthHarness(t)
		token := mustSeedInvitation(t, h, "stale@example.com", time.Now().Add(-1*time.Hour))

		claims := &googleClaims{Sub: "s", Email: "stale@example.com", EmailVerified: true, Name: "S"}
		_, err := h.h.bootstrapNewUser(context.Background(), claims, token, "en-GB")
		if err == nil || !strings.Contains(err.Error(), "expired") {
			t.Errorf("err: want 'expired', got %v", err)
		}
	})

	t.Run("used invite returns 'invitation already used'", func(t *testing.T) {
		h := newAuthHarness(t)
		token := mustSeedInvitation(t, h, "twice@example.com", time.Now().Add(24*time.Hour))
		// First use should succeed.
		claims1 := &googleClaims{Sub: "g1", Email: "twice@example.com", EmailVerified: true, Name: "T"}
		if _, err := h.h.bootstrapNewUser(context.Background(), claims1, token, "en-GB"); err != nil {
			t.Fatalf("first bootstrap: %v", err)
		}
		// Second use should fail.
		claims2 := &googleClaims{Sub: "g2", Email: "twice@example.com", EmailVerified: true, Name: "T2"}
		_, err := h.h.bootstrapNewUser(context.Background(), claims2, token, "en-GB")
		if err == nil || !strings.Contains(err.Error(), "already used") {
			t.Errorf("err: want 'already used', got %v", err)
		}
	})

	t.Run("email mismatch returns 'invitation email does not match'", func(t *testing.T) {
		h := newAuthHarness(t)
		token := mustSeedInvitation(t, h, "expected@example.com", time.Now().Add(24*time.Hour))

		claims := &googleClaims{Sub: "g", Email: "imposter@example.com", EmailVerified: true, Name: "I"}
		_, err := h.h.bootstrapNewUser(context.Background(), claims, token, "en-GB")
		if err == nil || !strings.Contains(err.Error(), "does not match") {
			t.Errorf("err: want 'does not match', got %v", err)
		}
		// And the invitation should still be unused.
		inv, err := h.q.GetInvitationByToken(context.Background(), token)
		if err != nil {
			t.Fatalf("GetInvitationByToken: %v", err)
		}
		if inv.UsedAt.Valid {
			t.Error("invitation should not be consumed on email-mismatch rejection")
		}
	})
}

// mustSeedInvitation persists an invitation directly via the queries layer.
// Bypasses handleCreateInvitation so we can control expires_at precisely
// (the handler always sets it to +invitationTTL).
func mustSeedInvitation(t *testing.T, h *authHarness, email string, expiresAt time.Time) string {
	t.Helper()
	token, err := randomInvitationToken()
	if err != nil {
		t.Fatalf("randomInvitationToken: %v", err)
	}
	_, err = h.q.CreateInvitation(context.Background(), db.CreateInvitationParams{
		HouseholdID:  h.user.HouseholdID,
		InvitedEmail: email,
		Token:        token,
		CreatedBy:    h.user.ID,
		ExpiresAt:    pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateInvitation: %v", err)
	}
	return token
}
