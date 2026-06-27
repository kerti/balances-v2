package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// createFounder is the founding mechanism behind the gate's founder choice
// (ADR-0038): testing it directly exercises the new-household-and-user fork
// without faking the OAuth exchange. The full gate commit is covered
// end-to-end in onboarding_test.go.

// covers: INV-AUTH-13
func TestCreateFounder(t *testing.T) {
	h := newAuthHarness(t)

	claims := &googleClaims{
		Sub:           "google-sub-new",
		Email:         "founder@example.com",
		EmailVerified: true,
		Name:          "Founder",
	}
	user, err := h.h.createFounder(context.Background(), claims, "en-GB", "")
	if err != nil {
		t.Fatalf("createFounder: %v", err)
	}
	if user.Email != claims.Email {
		t.Errorf("email: want %q, got %q", claims.Email, user.Email)
	}
	if stringOrEmpty(user.GoogleSub) != claims.Sub {
		t.Errorf("google_sub: want %q, got %q", claims.Sub, stringOrEmpty(user.GoogleSub))
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

// mustSeedInvitation persists an invitation directly via the queries layer.
// Bypasses handleCreateInvitation so we can control expires_at precisely
// (the handler always sets it to +invitationTTL).
func mustSeedInvitation(t *testing.T, h *authHarness, email string, expiresAt time.Time) string {
	t.Helper()
	token, tokenHash, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	_, err = h.q.CreateInvitation(context.Background(), db.CreateInvitationParams{
		HouseholdID:  h.user.HouseholdID,
		InvitedEmail: email,
		TokenHash:    tokenHash,
		CreatedBy:    h.user.ID,
		ExpiresAt:    pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateInvitation: %v", err)
	}
	return token
}
