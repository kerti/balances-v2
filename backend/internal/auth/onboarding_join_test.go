package auth

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// seedInvitationAt inserts a pending invitation with an explicit created_at so
// the dedupe (most-recent inviter per Household) and ordering (most-recently-
// invited first) can be asserted deterministically. Returns the invitation id.
func seedInvitationAt(t *testing.T, h *authHarness, householdID, inviterID uuid.UUID, email string, createdAt, expiresAt time.Time) uuid.UUID {
	t.Helper()
	token, err := randomInvitationToken()
	if err != nil {
		t.Fatalf("randomInvitationToken: %v", err)
	}
	var id uuid.UUID
	err = h.pool.QueryRow(context.Background(),
		`INSERT INTO household_invitations
		   (household_id, invited_email, token, created_by, created_at, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id`,
		householdID, email, token, inviterID, createdAt, expiresAt).Scan(&id)
	if err != nil {
		t.Fatalf("seed invitation: %v", err)
	}
	return id
}

// seedMember adds a second user to an existing Household so a same-Household
// double-invite can be attributed to a different inviter.
func seedMember(t *testing.T, h *authHarness, householdID uuid.UUID, name string) db.User {
	t.Helper()
	u, err := h.q.CreateUser(context.Background(), db.CreateUserParams{
		HouseholdID: householdID,
		DisplayName: name,
		Email:       name + "@member.example.com",
		GoogleSub:   "sub-" + name,
		Locale:      "en-GB",
		TimeZone:    "Asia/Jakarta",
	})
	if err != nil {
		t.Fatalf("seed member %s: %v", name, err)
	}
	return u
}

// covers: INV-AUTH-08
func TestOnboardingOptions_PendingInvites(t *testing.T) {
	h := newAuthHarness(t)
	const email = "charlie@example.com"
	now := time.Now()
	exp := now.Add(24 * time.Hour)

	// Three Households invite charlie. Household A double-invites (two inviters).
	hhA := h.user.HouseholdID // Alice
	aliceTwo := seedMember(t, h, hhA, "AliceTwo")
	bob := seedInviterHousehold(t, h, "Bob")
	carol := seedInviterHousehold(t, h, "Carol")

	// A: oldest by Alice, then a more-recent one by AliceTwo (the survivor).
	hintInvite := seedInvitationAt(t, h, hhA, h.user.ID, email, now.Add(-4*time.Hour), exp)
	seedInvitationAt(t, h, hhA, aliceTwo.ID, email, now.Add(-3*time.Hour), exp)
	// C in the middle, B newest.
	seedInvitationAt(t, h, carol.HouseholdID, carol.ID, email, now.Add(-2*time.Hour), exp)
	seedInvitationAt(t, h, bob.HouseholdID, bob.ID, email, now.Add(-1*time.Hour), exp)

	// The handshake's clicked-link hint points at A's *oldest* (deduped-out)
	// invite — the hint must still resolve to Household A's surviving row.
	token := mustBeginHandshakeWithHint(t, h, "sub-charlie", email, "Charlie", exp, &hintInvite)

	rec := h.onboardingRequest(t, "GET", "/onboarding/options", token, "")
	requireStatus(t, rec, http.StatusOK)
	resp := decodeBody[onboardingOptionsResponse](t, rec)

	// One row per distinct Household, most-recently-invited first: B, C, A.
	if len(resp.Invitations) != 3 {
		t.Fatalf("want 3 deduped rows, got %d: %+v", len(resp.Invitations), resp.Invitations)
	}
	gotOrder := []uuid.UUID{
		resp.Invitations[0].HouseholdID,
		resp.Invitations[1].HouseholdID,
		resp.Invitations[2].HouseholdID,
	}
	wantOrder := []uuid.UUID{bob.HouseholdID, carol.HouseholdID, hhA}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Errorf("row %d household: want %s, got %s", i, wantOrder[i], gotOrder[i])
		}
	}

	// Household A's surviving row shows the most-recent inviter (AliceTwo) and
	// is the pre-highlighted hint.
	aRow := resp.Invitations[2]
	if aRow.InviterName != "AliceTwo" {
		t.Errorf("Household A inviter: want most-recent AliceTwo, got %q", aRow.InviterName)
	}
	if !aRow.Hint {
		t.Error("Household A should be pre-highlighted as the clicked-link hint")
	}
	if resp.Invitations[0].Hint || resp.Invitations[1].Hint {
		t.Error("only the hinted Household should be highlighted")
	}
}

// covers: INV-AUTH-07
func TestOnboardingChoice_Join(t *testing.T) {
	t.Run("binds the new user to the inviting household and marks the invite used", func(t *testing.T) {
		h := newAuthHarness(t)
		const email = "joiner@example.com"
		exp := time.Now().Add(24 * time.Hour)
		inviteID := seedInvitationAt(t, h, h.user.HouseholdID, h.user.ID, email, time.Now(), exp)
		token := mustBeginHandshake(t, h, "sub-joiner", email, "Joiner", exp)

		body := `{"join":true,"invitation_id":"` + inviteID.String() + `"}`
		rec := h.onboardingRequest(t, "POST", "/onboarding/choice", token, body)
		requireStatus(t, rec, http.StatusNoContent)

		user, err := h.q.GetUserByGoogleSub(context.Background(), "sub-joiner")
		if err != nil {
			t.Fatalf("GetUserByGoogleSub: %v", err)
		}
		if user.HouseholdID != h.user.HouseholdID {
			t.Errorf("joiner should bind to the inviting household; want %s, got %s",
				h.user.HouseholdID, user.HouseholdID)
		}
		// Session issued + handshake consumed.
		if c := findCookie(rec, sessionCookieName); c == nil || c.Value == "" {
			t.Error("expected a session cookie")
		}
		if _, err := h.q.GetOnboardingHandshake(context.Background(), token); err == nil {
			t.Error("expected handshake deleted after join")
		}
		// Invitation marked used.
		inv, err := h.q.GetInvitationByID(context.Background(), inviteID)
		if err != nil {
			t.Fatalf("GetInvitationByID: %v", err)
		}
		if !inv.UsedAt.Valid {
			t.Error("expected invitation marked used")
		}
		// No welcome email on a join (it greets founders only).
		if n := len(h.mailer.sent()); n != 0 {
			t.Errorf("join should not send the welcome email; got %d", n)
		}
	})

	t.Run("missing invitation_id is 400", func(t *testing.T) {
		h := newAuthHarness(t)
		token := mustBeginHandshake(t, h, "sub-nojoinid", "nojoinid@example.com", "N", time.Now().Add(time.Hour))
		rec := h.onboardingRequest(t, "POST", "/onboarding/choice", token, `{"join":true}`)
		requireStatus(t, rec, http.StatusBadRequest)
	})
}

// covers: INV-AUTH-06
// The commit re-validates the chosen invitation (TOCTOU): a used or expired
// invite is no longer joinable and bounces the SPA to a refreshed gate (409).
func TestOnboardingChoice_Join_StaleInvite(t *testing.T) {
	t.Run("used invite is 409 and creates no user", func(t *testing.T) {
		h := newAuthHarness(t)
		const email = "used@example.com"
		inviteID := seedInvitationAt(t, h, h.user.HouseholdID, h.user.ID, email, time.Now(), time.Now().Add(24*time.Hour))
		if err := h.q.MarkInvitationUsed(context.Background(), inviteID); err != nil {
			t.Fatalf("MarkInvitationUsed: %v", err)
		}
		token := mustBeginHandshake(t, h, "sub-used", email, "U", time.Now().Add(time.Hour))

		rec := h.onboardingRequest(t, "POST", "/onboarding/choice", token, `{"join":true,"invitation_id":"`+inviteID.String()+`"}`)
		requireStatus(t, rec, http.StatusConflict)
		if _, err := h.q.GetUserByGoogleSub(context.Background(), "sub-used"); err == nil {
			t.Error("a stale-invite join must create no user")
		}
		// Handshake survives so the refreshed gate still works.
		if _, err := h.q.GetOnboardingHandshake(context.Background(), token); err != nil {
			t.Errorf("handshake should survive a stale-invite 409: %v", err)
		}
	})

	t.Run("expired invite is 409", func(t *testing.T) {
		h := newAuthHarness(t)
		const email = "exp@example.com"
		inviteID := seedInvitationAt(t, h, h.user.HouseholdID, h.user.ID, email, time.Now().Add(-48*time.Hour), time.Now().Add(-time.Hour))
		token := mustBeginHandshake(t, h, "sub-exp2", email, "E", time.Now().Add(time.Hour))

		rec := h.onboardingRequest(t, "POST", "/onboarding/choice", token, `{"join":true,"invitation_id":"`+inviteID.String()+`"}`)
		requireStatus(t, rec, http.StatusConflict)
	})
}

// covers: INV-AUTH-08
// The commit re-validation is email-scoped: a valid invitation addressed to a
// *different* email cannot be joined even if its id is supplied (forwarded-link
// / id-guessing guard), and it never appears in that identity's gate options.
func TestOnboardingChoice_Join_ForeignEmailRejected(t *testing.T) {
	h := newAuthHarness(t)
	inviteID := seedInvitationAt(t, h, h.user.HouseholdID, h.user.ID, "intended@example.com", time.Now(), time.Now().Add(24*time.Hour))
	// Handshake for a *different* verified email.
	token := mustBeginHandshake(t, h, "sub-imposter", "imposter@example.com", "Imposter", time.Now().Add(time.Hour))

	// The foreign invite is invisible in options…
	optsRec := h.onboardingRequest(t, "GET", "/onboarding/options", token, "")
	requireStatus(t, optsRec, http.StatusOK)
	if opts := decodeBody[onboardingOptionsResponse](t, optsRec); len(opts.Invitations) != 0 {
		t.Errorf("foreign-email invite must not surface; got %d rows", len(opts.Invitations))
	}

	// …and supplying its id directly is rejected.
	rec := h.onboardingRequest(t, "POST", "/onboarding/choice", token, `{"join":true,"invitation_id":"`+inviteID.String()+`"}`)
	requireStatus(t, rec, http.StatusConflict)
	if _, err := h.q.GetUserByGoogleSub(context.Background(), "sub-imposter"); err == nil {
		t.Error("must not bind an identity to an invite addressed to another email")
	}
	inv, err := h.q.GetInvitationByID(context.Background(), inviteID)
	if err != nil {
		t.Fatalf("GetInvitationByID: %v", err)
	}
	if inv.UsedAt.Valid {
		t.Error("a rejected foreign-email join must leave the invitation unconsumed")
	}
}

// seedInviterHousehold creates a fresh Household + founding user (a separate inviter
// in its own Household), for the multi-Household options test.
func seedInviterHousehold(t *testing.T, h *authHarness, name string) db.User {
	t.Helper()
	hh, err := h.q.CreateHousehold(context.Background(), db.CreateHouseholdParams{
		DisplayName:       name + "'s Household",
		ReportingCurrency: "IDR",
	})
	if err != nil {
		t.Fatalf("create household for %s: %v", name, err)
	}
	u, err := h.q.CreateUser(context.Background(), db.CreateUserParams{
		HouseholdID: hh.ID,
		DisplayName: name,
		Email:       name + "@hh.example.com",
		GoogleSub:   "sub-hh-" + name,
		Locale:      "en-GB",
		TimeZone:    "Asia/Jakarta",
	})
	if err != nil {
		t.Fatalf("create user for %s: %v", name, err)
	}
	return u
}
