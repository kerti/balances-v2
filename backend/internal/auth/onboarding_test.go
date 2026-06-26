package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// mustBeginHandshake records an onboarding handshake directly via the queries
// layer with the given expiry, returning the opaque token (the cookie value).
// Bypasses the OAuth callback so tests control expires_at precisely.
func mustBeginHandshake(t *testing.T, h *authHarness, sub, email, name string, expiresAt time.Time) string {
	t.Helper()
	return mustBeginHandshakeWithHint(t, h, sub, email, name, expiresAt, nil)
}

// mustBeginHandshakeWithHint is mustBeginHandshake with an optional clicked-link
// pre-selection hint (hint_invitation_id).
func mustBeginHandshakeWithHint(t *testing.T, h *authHarness, sub, email, name string, expiresAt time.Time, hint *uuid.UUID) string {
	t.Helper()
	token, err := randomSessionID()
	if err != nil {
		t.Fatalf("randomSessionID: %v", err)
	}
	_, err = h.q.CreateOnboardingHandshake(context.Background(), db.CreateOnboardingHandshakeParams{
		ID:               token,
		GoogleSub:        &sub,
		Email:            email,
		DisplayName:      name,
		SeedLocale:       "en-GB",
		HintInvitationID: hint,
		ExpiresAt:        pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		t.Fatalf("CreateOnboardingHandshake: %v", err)
	}
	return token
}

// onboardingRequest issues a request to a gate endpoint carrying the handshake
// cookie (omitted when token is "").
func (h *authHarness) onboardingRequest(t *testing.T, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	var req *http.Request
	if reader != nil {
		req = httptest.NewRequest(method, path, reader)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		req.AddCookie(&http.Cookie{Name: onboardingCookieName, Value: token})
	}
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

// covers: INV-AUTH-12
func TestOnboardingOptions(t *testing.T) {
	t.Run("valid handshake returns the founder option", func(t *testing.T) {
		h := newAuthHarness(t)
		token := mustBeginHandshake(t, h, "sub-opt", "founder@example.com", "Dana", time.Now().Add(onboardingHandshakeTTL))

		rec := h.onboardingRequest(t, "GET", "/onboarding/options", token, "")
		requireStatus(t, rec, http.StatusOK)

		resp := decodeBody[onboardingOptionsResponse](t, rec)
		if resp.Email != "founder@example.com" {
			t.Errorf("email: got %q", resp.Email)
		}
		if resp.DisplayName != "Dana" {
			t.Errorf("display_name: got %q", resp.DisplayName)
		}
		if resp.SuggestedName != "Dana's Household" {
			t.Errorf("suggested name: got %q", resp.SuggestedName)
		}
		if len(resp.Invitations) != 0 {
			t.Errorf("invitations: want empty in the founder slice, got %d", len(resp.Invitations))
		}
	})

	t.Run("missing handshake cookie is 401", func(t *testing.T) {
		h := newAuthHarness(t)
		rec := h.onboardingRequest(t, "GET", "/onboarding/options", "", "")
		requireStatus(t, rec, http.StatusUnauthorized)
	})

	t.Run("expired handshake is 401", func(t *testing.T) {
		h := newAuthHarness(t)
		token := mustBeginHandshake(t, h, "sub-exp", "stale@example.com", "Stale", time.Now().Add(-time.Minute))
		rec := h.onboardingRequest(t, "GET", "/onboarding/options", token, "")
		requireStatus(t, rec, http.StatusUnauthorized)
	})

	t.Run("unknown token is 401", func(t *testing.T) {
		h := newAuthHarness(t)
		rec := h.onboardingRequest(t, "GET", "/onboarding/options", "not-a-real-token", "")
		requireStatus(t, rec, http.StatusUnauthorized)
	})
}

// covers: INV-AUTH-13
func TestOnboardingChoice_Found(t *testing.T) {
	t.Run("founds a household, issues a session, and consumes the handshake", func(t *testing.T) {
		h := newAuthHarness(t)
		token := mustBeginHandshake(t, h, "sub-found", "found@example.com", "Erin", time.Now().Add(onboardingHandshakeTTL))

		rec := h.onboardingRequest(t, "POST", "/onboarding/choice", token, `{"found":true}`)
		requireStatus(t, rec, http.StatusNoContent)

		// Account now exists in a brand-new household.
		user, err := h.q.GetUserByGoogleSub(context.Background(), "sub-found")
		if err != nil {
			t.Fatalf("GetUserByGoogleSub: %v", err)
		}
		if user.HouseholdID == h.user.HouseholdID {
			t.Error("founder should land in a new household, not the harness one")
		}
		hh, err := h.q.GetHouseholdByID(context.Background(), user.HouseholdID)
		if err != nil {
			t.Fatalf("GetHouseholdByID: %v", err)
		}
		if hh.DisplayName != "Erin's Household" {
			t.Errorf("household name: want default, got %q", hh.DisplayName)
		}

		// A real session is issued, and the handshake is deleted + cookie cleared.
		sess := findCookie(rec, sessionCookieName)
		if sess == nil || sess.Value == "" {
			t.Fatal("expected a session cookie")
		}
		if _, err := h.q.GetOnboardingHandshake(context.Background(), token); err == nil {
			t.Error("expected handshake to be deleted after commit")
		}
		if c := findCookie(rec, onboardingCookieName); c == nil || c.MaxAge >= 0 {
			t.Errorf("expected onboarding cookie cleared, got %+v", c)
		}

		// The welcome email fires on the deliberate founder choice (#160/#267).
		if len(h.mailer.sent()) != 1 {
			t.Errorf("welcome email: want 1 send, got %d", len(h.mailer.sent()))
		}
	})

	t.Run("honours an optional household-name override", func(t *testing.T) {
		h := newAuthHarness(t)
		token := mustBeginHandshake(t, h, "sub-named", "named@example.com", "Finn", time.Now().Add(onboardingHandshakeTTL))

		rec := h.onboardingRequest(t, "POST", "/onboarding/choice", token, `{"found":true,"display_name":"  The Vault  "}`)
		requireStatus(t, rec, http.StatusNoContent)

		user, err := h.q.GetUserByGoogleSub(context.Background(), "sub-named")
		if err != nil {
			t.Fatalf("GetUserByGoogleSub: %v", err)
		}
		hh, err := h.q.GetHouseholdByID(context.Background(), user.HouseholdID)
		if err != nil {
			t.Fatalf("GetHouseholdByID: %v", err)
		}
		if hh.DisplayName != "The Vault" {
			t.Errorf("household name: want trimmed override %q, got %q", "The Vault", hh.DisplayName)
		}
	})

	t.Run("rejects an over-long household name", func(t *testing.T) {
		h := newAuthHarness(t)
		token := mustBeginHandshake(t, h, "sub-long", "long@example.com", "Gail", time.Now().Add(onboardingHandshakeTTL))
		long := strings.Repeat("x", maxHouseholdNameLen+1)

		rec := h.onboardingRequest(t, "POST", "/onboarding/choice", token, `{"found":true,"display_name":"`+long+`"}`)
		requireStatus(t, rec, http.StatusBadRequest)

		// Rejected before any write — no account, handshake survives.
		if _, err := h.q.GetUserByGoogleSub(context.Background(), "sub-long"); err == nil {
			t.Error("expected no user row on a rejected choice")
		}
		if _, err := h.q.GetOnboardingHandshake(context.Background(), token); err != nil {
			t.Errorf("handshake should survive a rejected choice: %v", err)
		}
	})

	t.Run("non-founder choice is rejected in this slice", func(t *testing.T) {
		h := newAuthHarness(t)
		token := mustBeginHandshake(t, h, "sub-nofound", "nofound@example.com", "Hugo", time.Now().Add(onboardingHandshakeTTL))
		rec := h.onboardingRequest(t, "POST", "/onboarding/choice", token, `{"found":false}`)
		requireStatus(t, rec, http.StatusBadRequest)
	})

	t.Run("commit without a handshake is 401", func(t *testing.T) {
		h := newAuthHarness(t)
		rec := h.onboardingRequest(t, "POST", "/onboarding/choice", "", `{"found":true}`)
		requireStatus(t, rec, http.StatusUnauthorized)
	})

	t.Run("malformed JSON body is 400 and leaves the handshake intact", func(t *testing.T) {
		h := newAuthHarness(t)
		token := mustBeginHandshake(t, h, "sub-badjson", "badjson@example.com", "Kim", time.Now().Add(onboardingHandshakeTTL))
		rec := h.onboardingRequest(t, "POST", "/onboarding/choice", token, `{not json`)
		requireStatus(t, rec, http.StatusBadRequest)
		if _, err := h.q.GetOnboardingHandshake(context.Background(), token); err != nil {
			t.Errorf("handshake should survive a malformed body: %v", err)
		}
	})
}

// covers: INV-AUTH-12
// An abandoned/expired handshake leaves no partial state, and the sweep removes
// it — re-auth simply shows the gate again (idempotent, ADR-0038).
func TestOnboardingHandshake_ExpirySweep(t *testing.T) {
	h := newAuthHarness(t)
	live := mustBeginHandshake(t, h, "sub-live", "live@example.com", "Iris", time.Now().Add(onboardingHandshakeTTL))
	dead := mustBeginHandshake(t, h, "sub-dead", "dead@example.com", "Jack", time.Now().Add(-time.Minute))

	if err := h.q.DeleteExpiredOnboardingHandshakes(context.Background()); err != nil {
		t.Fatalf("DeleteExpiredOnboardingHandshakes: %v", err)
	}
	if _, err := h.q.GetOnboardingHandshake(context.Background(), dead); err == nil {
		t.Error("expected expired handshake to be swept")
	}
	if _, err := h.q.GetOnboardingHandshake(context.Background(), live); err != nil {
		t.Errorf("live handshake should survive the sweep: %v", err)
	}
}
