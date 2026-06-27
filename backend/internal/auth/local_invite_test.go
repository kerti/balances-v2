package auth

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/kerti/balances-v2/backend/internal/httperr"
)

// TestLocalInviteAccept_HappyPath is the #281 tracer bullet: an invited member
// with no Google account follows their link, sets a password, and lands in the
// app — account bound to invited_email, session minted, NO onboarding gate.
//
// covers: INV-AUTH-18
func TestLocalInviteAccept_HappyPath(t *testing.T) {
	h := newAuthHarness(t)
	const email = "newbie@example.com"
	token := mustSeedInvitation(t, h, email, time.Now().Add(time.Hour))

	// Preview resolves the link without consuming it.
	prev := h.doRaw(t, http.MethodGet, "/auth/local/invite?token="+token, nil, nil)
	requireStatus(t, prev, http.StatusOK)
	pv := decodeBody[localInvitePreviewResp](t, prev)
	if pv.InvitedEmail != email {
		t.Errorf("preview invited_email = %q, want %q", pv.InvitedEmail, email)
	}
	if pv.HouseholdName == "" {
		t.Error("preview household_name is empty")
	}

	// Accept sets the password and mints a session directly.
	acc := h.post(t, "/auth/local/invite/accept", map[string]string{
		"token": token, "password": "a-decent-invitee-passphrase",
	})
	requireStatus(t, acc, http.StatusNoContent)
	if findCookie(acc, sessionCookieName) == nil {
		t.Fatal("accept did not mint a session cookie")
	}
	// No ADR-0038 gate: the invitee never gets a handshake cookie.
	if findCookie(acc, onboardingCookieName) != nil {
		t.Error("accept set an onboarding handshake cookie — should be a single path")
	}

	// The account exists, bound to invited_email in the inviter's Household, with
	// a local credential (reachable, not dormant).
	user, err := h.q.GetUserByEmail(context.Background(), email)
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if user.HouseholdID != h.user.HouseholdID {
		t.Errorf("user household = %s, want inviter's %s", user.HouseholdID, h.user.HouseholdID)
	}
	if user.GoogleSub != nil {
		t.Error("invited local user should have no google_sub")
	}
	if _, err := h.q.GetLocalCredentialByUserID(context.Background(), user.ID); err != nil {
		t.Errorf("expected a local_credentials row: %v", err)
	}
}

// covers: INV-AUTH-18
func TestLocalInviteAccept_SingleUse(t *testing.T) {
	h := newAuthHarness(t)
	const email = "single@example.com"
	token := mustSeedInvitation(t, h, email, time.Now().Add(time.Hour))

	first := h.post(t, "/auth/local/invite/accept", map[string]string{
		"token": token, "password": "the-first-good-passphrase",
	})
	requireStatus(t, first, http.StatusNoContent)

	// A forwarded-after-use link cannot create a second account.
	second := h.post(t, "/auth/local/invite/accept", map[string]string{
		"token": token, "password": "another-good-passphrase-x",
	})
	requireStatus(t, second, http.StatusConflict)
	if code := envelopeCode(t, second); code != string(httperr.CodeInvitationNoLongerValid) {
		t.Errorf("second accept code = %q, want %q", code, httperr.CodeInvitationNoLongerValid)
	}
	// Still exactly one user for that email (the consume + create is atomic).
	if _, err := h.q.GetUserByEmail(context.Background(), email); err != nil {
		t.Fatalf("first account should persist: %v", err)
	}
}

// covers: INV-AUTH-18
func TestLocalInviteAccept_ExpiredRejected(t *testing.T) {
	h := newAuthHarness(t)
	const email = "expired@example.com"
	token := mustSeedInvitation(t, h, email, time.Now().Add(-time.Minute))

	prev := h.doRaw(t, http.MethodGet, "/auth/local/invite?token="+token, nil, nil)
	requireStatus(t, prev, http.StatusConflict)

	acc := h.post(t, "/auth/local/invite/accept", map[string]string{
		"token": token, "password": "a-perfectly-fine-passphrase",
	})
	requireStatus(t, acc, http.StatusConflict)
	if code := envelopeCode(t, acc); code != string(httperr.CodeInvitationNoLongerValid) {
		t.Errorf("expired accept code = %q, want %q", code, httperr.CodeInvitationNoLongerValid)
	}
	if _, err := h.q.GetUserByEmail(context.Background(), email); err == nil {
		t.Error("expired link should not create an account")
	}
}

// covers: INV-AUTH-18
func TestLocalInviteAccept_UnknownTokenRejected(t *testing.T) {
	h := newAuthHarness(t)
	acc := h.post(t, "/auth/local/invite/accept", map[string]string{
		"token": "not-a-real-token", "password": "a-perfectly-fine-passphrase",
	})
	requireStatus(t, acc, http.StatusConflict)
	if code := envelopeCode(t, acc); code != string(httperr.CodeInvitationNoLongerValid) {
		t.Errorf("unknown token code = %q, want %q", code, httperr.CodeInvitationNoLongerValid)
	}
}

func TestLocalInvite_BadInputs(t *testing.T) {
	h := newAuthHarness(t)

	t.Run("preview missing token → 400", func(t *testing.T) {
		rec := h.doRaw(t, http.MethodGet, "/auth/local/invite", nil, nil)
		requireStatus(t, rec, http.StatusBadRequest)
		if code := envelopeCode(t, rec); code != string(httperr.CodeValidation) {
			t.Errorf("code = %q, want %q", code, httperr.CodeValidation)
		}
	})

	t.Run("accept invalid json → 400", func(t *testing.T) {
		rec := h.post(t, "/auth/local/invite/accept", "{not-json")
		requireStatus(t, rec, http.StatusBadRequest)
		if code := envelopeCode(t, rec); code != string(httperr.CodeInvalidJSONBody) {
			t.Errorf("code = %q, want %q", code, httperr.CodeInvalidJSONBody)
		}
	})

	t.Run("accept missing token → 400", func(t *testing.T) {
		rec := h.post(t, "/auth/local/invite/accept", map[string]string{"password": "a-fine-passphrase-here"})
		requireStatus(t, rec, http.StatusBadRequest)
		if code := envelopeCode(t, rec); code != string(httperr.CodeValidation) {
			t.Errorf("code = %q, want %q", code, httperr.CodeValidation)
		}
	})
}

func TestLocalInviteAccept_WeakPasswordRejected(t *testing.T) {
	h := newAuthHarness(t)
	const email = "weakpw@example.com"
	token := mustSeedInvitation(t, h, email, time.Now().Add(time.Hour))

	acc := h.post(t, "/auth/local/invite/accept", map[string]string{
		"token": token, "password": "short",
	})
	requireStatus(t, acc, http.StatusBadRequest)
	if code := envelopeCode(t, acc); code != string(httperr.CodeWeakPassword) {
		t.Errorf("weak password code = %q, want %q", code, httperr.CodeWeakPassword)
	}
	// A rejected weak password must not have consumed the link.
	prev := h.doRaw(t, http.MethodGet, "/auth/local/invite?token="+token, nil, nil)
	requireStatus(t, prev, http.StatusOK)
}

// An invitation addressed to an email that already owns an account cannot mint a
// second one; the clash is a clean 409, and the link stays consumed-safe.
func TestLocalInviteAccept_EmailAlreadyTaken(t *testing.T) {
	h := newAuthHarness(t)
	// h.user already exists; seed an invite to that very address (the create
	// handler forbids self-invite, but a stale invite could still exist).
	token := mustSeedInvitation(t, h, h.user.Email, time.Now().Add(time.Hour))

	acc := h.post(t, "/auth/local/invite/accept", map[string]string{
		"token": token, "password": "a-perfectly-fine-passphrase",
	})
	requireStatus(t, acc, http.StatusConflict)
	if code := envelopeCode(t, acc); code != string(httperr.CodeEmailTaken) {
		t.Errorf("email-taken code = %q, want %q", code, httperr.CodeEmailTaken)
	}
}
