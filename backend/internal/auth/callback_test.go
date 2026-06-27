package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// callbackRequest builds a GET /auth/google/callback request with the given
// query params. Cookies are added via the returned request handle.
func callbackRequest(state, code string) *http.Request {
	req := httptest.NewRequest("GET",
		"/auth/google/callback?state="+state+"&code="+code, nil)
	return req
}

// covers: INV-AUTH-02
func TestHandleCallback_GuardClauses(t *testing.T) {
	h := newAuthHarness(t)
	// Stub the exchanger so accidental fall-through doesn't reach Google.
	h.installStubOAuth(nil, errors.New("should not be called"))

	t.Run("400 missing state cookie", func(t *testing.T) {
		req := callbackRequest("any-state", "any-code")
		rec := httptest.NewRecorder()
		h.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status: want 400, got %d (body: %s)", rec.Code, rec.Body.String())
		}
	})

	t.Run("400 empty state cookie value", func(t *testing.T) {
		req := callbackRequest("any-state", "any-code")
		req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: ""})
		rec := httptest.NewRecorder()
		h.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status: want 400, got %d", rec.Code)
		}
	})

	t.Run("400 state mismatch", func(t *testing.T) {
		req := callbackRequest("query-state", "any-code")
		req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "different-cookie-state"})
		rec := httptest.NewRecorder()
		h.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status: want 400, got %d", rec.Code)
		}
	})

	t.Run("400 missing code", func(t *testing.T) {
		req := callbackRequest("matching", "")
		req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "matching"})
		rec := httptest.NewRecorder()
		h.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status: want 400, got %d", rec.Code)
		}
	})
}

func TestHandleCallback_ExchangeError(t *testing.T) {
	h := newAuthHarness(t)
	stub := h.installStubOAuth(nil, errors.New("token exchange refused"))

	req := callbackRequest("s", "the-code")
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "s"})
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status: want 502, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if stub.lastCode != "the-code" {
		t.Errorf("exchanger received code %q, want %q", stub.lastCode, "the-code")
	}
}

func TestHandleCallback_ExistingUserSignIn(t *testing.T) {
	h := newAuthHarness(t)
	// Harness user already has google_sub "test-sub-Alice" from the fixture.
	const wantPicture = "https://lh3.googleusercontent.com/a/alice.jpg"
	h.installStubOAuth(&googleClaims{
		Sub:           stringOrEmpty(h.user.GoogleSub),
		Email:         h.user.Email,
		EmailVerified: true,
		Name:          h.user.DisplayName,
		Picture:       wantPicture,
	}, nil)

	req := callbackRequest("s", "the-code")
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "s"})
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status: want 302, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != h.h.frontendURL {
		t.Errorf("redirect: want %q, got %q", h.h.frontendURL, loc)
	}

	sessCookie := findCookie(rec, sessionCookieName)
	if sessCookie == nil || sessCookie.Value == "" {
		t.Fatal("expected session cookie to be set with a non-empty value")
	}

	// State + invite cookies should be cleared (MaxAge < 0).
	if c := findCookie(rec, oauthStateCookieName); c == nil || c.MaxAge >= 0 {
		t.Errorf("expected oauth_state cookie cleared, got %+v", c)
	}
	if c := findCookie(rec, oauthInviteCookieName); c == nil || c.MaxAge >= 0 {
		t.Errorf("expected oauth_invite cookie cleared, got %+v", c)
	}

	// Session row should exist for the harness user.
	session, err := h.q.GetSessionByID(context.Background(), sessCookie.Value)
	if err != nil {
		t.Fatalf("GetSessionByID: %v", err)
	}
	if session.UserID != h.user.ID {
		t.Errorf("session.user_id: want %s, got %s", h.user.ID, session.UserID)
	}

	// The login should have backfilled the Google picture onto the existing
	// user row (the fixture user starts with no picture).
	refreshed, err := h.q.GetUserByGoogleSub(context.Background(), stringOrEmpty(h.user.GoogleSub))
	if err != nil {
		t.Fatalf("GetUserByGoogleSub: %v", err)
	}
	if refreshed.PictureUrl == nil || *refreshed.PictureUrl != wantPicture {
		t.Errorf("picture_url: want %q, got %v", wantPicture, refreshed.PictureUrl)
	}
}

// covers: INV-AUTH-14
// An already-onboarded user who opens a fresh invite link is signed in as
// normal but carried a non-blocking notice signal (ADR-0038): the link can't
// move them between Households, so it is explained rather than silently
// dropped. No membership change, and the invitation is left unconsumed.
func TestHandleCallback_ExistingUserInviteIgnored(t *testing.T) {
	h := newAuthHarness(t)
	token := mustSeedInvitation(t, h, "someone-else@example.com", time.Now().Add(24*time.Hour))
	originalHousehold := h.user.HouseholdID

	h.installStubOAuth(&googleClaims{
		Sub:           stringOrEmpty(h.user.GoogleSub),
		Email:         h.user.Email,
		EmailVerified: true,
		Name:          h.user.DisplayName,
	}, nil)

	req := callbackRequest("s", "the-code")
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "s"})
	req.AddCookie(&http.Cookie{Name: oauthInviteCookieName, Value: token})
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status: want 302, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	// The redirect carries the notice signal; the user still reaches the app.
	if loc := rec.Header().Get("Location"); loc != h.h.frontendURL+"/?notice=invite_ignored" {
		t.Errorf("redirect: want notice signal, got %q", loc)
	}
	if c := findCookie(rec, sessionCookieName); c == nil || c.Value == "" {
		t.Error("expected a session cookie (sign-in still succeeds)")
	}

	// No membership change.
	refreshed, err := h.q.GetUserByGoogleSub(context.Background(), stringOrEmpty(h.user.GoogleSub))
	if err != nil {
		t.Fatalf("GetUserByGoogleSub: %v", err)
	}
	if refreshed.HouseholdID != originalHousehold {
		t.Errorf("household must not change; want %s, got %s", originalHousehold, refreshed.HouseholdID)
	}
	// The invitation is not consumed.
	inv, err := h.q.GetInvitationByTokenHash(context.Background(), HashToken(token))
	if err != nil {
		t.Fatalf("GetInvitationByTokenHash: %v", err)
	}
	if inv.UsedAt.Valid {
		t.Error("an ignored invite must stay unconsumed")
	}
}

// covers: INV-AUTH-05, INV-AUTH-12
// Empty-token first sign-in no longer founds a Household directly (ADR-0038):
// it records an onboarding handshake and bounces to the gate, with NO
// users/households row and NO session created yet. The deliberate founding now
// happens at the gate (TestOnboardingChoice_Found).
func TestHandleCallback_NewIdentityBeginsOnboarding(t *testing.T) {
	h := newAuthHarness(t)
	const wantPicture = "https://lh3.googleusercontent.com/a/founder.jpg"
	h.installStubOAuth(&googleClaims{
		Sub:           "new-google-sub-founder",
		Email:         "newfounder@example.com",
		EmailVerified: true,
		Name:          "New Founder",
		Picture:       wantPicture,
	}, nil)

	req := callbackRequest("s", "the-code")
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "s"})
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status: want 302, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != h.h.frontendURL+"/onboarding" {
		t.Errorf("redirect: want %q, got %q", h.h.frontendURL+"/onboarding", loc)
	}

	// No session is issued at the gate's threshold.
	if c := findCookie(rec, sessionCookieName); c != nil && c.Value != "" {
		t.Errorf("expected no session cookie, got %+v", c)
	}
	// No account exists yet — nothing is written until the choice commits.
	if _, err := h.q.GetUserByGoogleSub(context.Background(), "new-google-sub-founder"); err == nil {
		t.Error("expected no user row for an un-committed onboarding handshake")
	}

	// A handshake cookie identifies the verified-but-unaccounted identity, and
	// the row carries the Google claims for the deferred account creation.
	hsCookie := findCookie(rec, onboardingCookieName)
	if hsCookie == nil || hsCookie.Value == "" {
		t.Fatal("expected onboarding handshake cookie to be set")
	}
	hs, err := h.q.GetOnboardingHandshake(context.Background(), hsCookie.Value)
	if err != nil {
		t.Fatalf("GetOnboardingHandshake: %v", err)
	}
	if hs.Email != "newfounder@example.com" {
		t.Errorf("handshake email: got %q", hs.Email)
	}
	if hs.PictureUrl == nil || *hs.PictureUrl != wantPicture {
		t.Errorf("handshake picture_url: want %q, got %v", wantPicture, hs.PictureUrl)
	}
}

// covers: INV-AUTH-12
// A clicked invite link no longer joins silently in the callback (ADR-0038): a
// brand-new invited identity is routed through the handshake + gate like every
// other first sign-in, with the valid invitation recorded as the pre-selection
// hint. No user is created and the invitation stays unused until the gate
// commits a join (TestOnboardingChoice_Join).
func TestHandleCallback_InvitedIdentityBeginsOnboarding(t *testing.T) {
	h := newAuthHarness(t)
	token := mustSeedInvitation(t, h, "invited2@example.com", time.Now().Add(24*time.Hour))
	invite, err := h.q.GetInvitationByTokenHash(context.Background(), HashToken(token))
	if err != nil {
		t.Fatalf("GetInvitationByTokenHash: %v", err)
	}

	h.installStubOAuth(&googleClaims{
		Sub:           "new-google-sub-invited",
		Email:         "invited2@example.com",
		EmailVerified: true,
		Name:          "Invited",
	}, nil)

	req := callbackRequest("s", "the-code")
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "s"})
	req.AddCookie(&http.Cookie{Name: oauthInviteCookieName, Value: token})
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status: want 302, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != h.h.frontendURL+"/onboarding" {
		t.Errorf("redirect: want %q, got %q", h.h.frontendURL+"/onboarding", loc)
	}

	// No account created and the invitation is untouched until the gate commits.
	if _, err := h.q.GetUserByGoogleSub(context.Background(), "new-google-sub-invited"); err == nil {
		t.Error("expected no user row before the gate commits a join")
	}
	inv, err := h.q.GetInvitationByTokenHash(context.Background(), HashToken(token))
	if err != nil {
		t.Fatalf("GetInvitationByTokenHash: %v", err)
	}
	if inv.UsedAt.Valid {
		t.Error("invitation should not be consumed at the callback")
	}

	// The valid clicked link is recorded as the handshake's pre-selection hint.
	hsCookie := findCookie(rec, onboardingCookieName)
	if hsCookie == nil || hsCookie.Value == "" {
		t.Fatal("expected onboarding handshake cookie")
	}
	hs, err := h.q.GetOnboardingHandshake(context.Background(), hsCookie.Value)
	if err != nil {
		t.Fatalf("GetOnboardingHandshake: %v", err)
	}
	if hs.HintInvitationID == nil || *hs.HintInvitationID != invite.ID {
		t.Errorf("hint_invitation_id: want %s, got %v", invite.ID, hs.HintInvitationID)
	}
}

// covers: INV-AUTH-08
// A forwarded/invalid invite token degrades to no hint rather than blocking the
// gate (ADR-0038): a token addressed to a different email leaves the handshake
// hint-less, and the gate's email-scoped lookup never surfaces it. The identity
// still reaches the gate (it can always found its own).
func TestHandleCallback_ForwardedInviteTokenIsNoHint(t *testing.T) {
	h := newAuthHarness(t)
	// Invitation addressed to someone else; this identity clicked a forwarded link.
	token := mustSeedInvitation(t, h, "intended@example.com", time.Now().Add(24*time.Hour))

	h.installStubOAuth(&googleClaims{
		Sub:           "new-google-sub-x",
		Email:         "imposter@example.com",
		EmailVerified: true,
		Name:          "X",
	}, nil)

	req := callbackRequest("s", "the-code")
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "s"})
	req.AddCookie(&http.Cookie{Name: oauthInviteCookieName, Value: token})
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status: want 302, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	hsCookie := findCookie(rec, onboardingCookieName)
	if hsCookie == nil || hsCookie.Value == "" {
		t.Fatal("expected onboarding handshake cookie")
	}
	hs, err := h.q.GetOnboardingHandshake(context.Background(), hsCookie.Value)
	if err != nil {
		t.Fatalf("GetOnboardingHandshake: %v", err)
	}
	if hs.HintInvitationID != nil {
		t.Errorf("a wrong-email token must not become a hint; got %v", hs.HintInvitationID)
	}
}

// TestHandleCallback_ExpiredSessionRowOverwrite verifies that the session
// created by handleCallback is fresh, not a recycled old row. Defensive
// regression test for a hypothetical "session id collision" — unlikely
// given 256 bits of randomness, but cheap to assert.
func TestHandleCallback_FreshSession(t *testing.T) {
	h := newAuthHarness(t)

	// Plant an old expired session for the harness user.
	oldSessionID := mustRandomSessionID(t)
	_, err := h.q.CreateSession(context.Background(), db.CreateSessionParams{
		ID:        oldSessionID,
		UserID:    h.user.ID,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatalf("seed old session: %v", err)
	}

	h.installStubOAuth(&googleClaims{
		Sub:           stringOrEmpty(h.user.GoogleSub),
		Email:         h.user.Email,
		EmailVerified: true,
		Name:          h.user.DisplayName,
	}, nil)

	req := callbackRequest("s", "the-code")
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "s"})
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status: want 302, got %d", rec.Code)
	}
	newCookie := findCookie(rec, sessionCookieName)
	if newCookie == nil {
		t.Fatal("expected new session cookie")
	}
	if newCookie.Value == oldSessionID {
		t.Error("new session id collided with old one — entropy issue?")
	}
}
