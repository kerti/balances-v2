package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
		Sub:           h.user.GoogleSub,
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
	refreshed, err := h.q.GetUserByGoogleSub(context.Background(), h.user.GoogleSub)
	if err != nil {
		t.Fatalf("GetUserByGoogleSub: %v", err)
	}
	if refreshed.PictureUrl == nil || *refreshed.PictureUrl != wantPicture {
		t.Errorf("picture_url: want %q, got %v", wantPicture, refreshed.PictureUrl)
	}
}

// covers: INV-AUTH-05
func TestHandleCallback_NewFounder(t *testing.T) {
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

	// Looking the user up by google_sub should now resolve to a real row in
	// a brand-new household.
	user, err := h.q.GetUserByGoogleSub(context.Background(), "new-google-sub-founder")
	if err != nil {
		t.Fatalf("GetUserByGoogleSub: %v", err)
	}
	if user.HouseholdID == h.user.HouseholdID {
		t.Error("new founder should land in a different household than the harness user")
	}
	if user.Email != "newfounder@example.com" {
		t.Errorf("email: got %q", user.Email)
	}
	if user.PictureUrl == nil || *user.PictureUrl != wantPicture {
		t.Errorf("picture_url: want %q, got %v", wantPicture, user.PictureUrl)
	}
}

// covers: INV-AUTH-07
func TestHandleCallback_InvitedUser(t *testing.T) {
	h := newAuthHarness(t)
	token := mustSeedInvitation(t, h, "invited2@example.com", time.Now().Add(24*time.Hour))

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

	user, err := h.q.GetUserByGoogleSub(context.Background(), "new-google-sub-invited")
	if err != nil {
		t.Fatalf("GetUserByGoogleSub: %v", err)
	}
	if user.HouseholdID != h.user.HouseholdID {
		t.Errorf("invited user should join harness household; want %s, got %s",
			h.user.HouseholdID, user.HouseholdID)
	}

	// Invitation should be marked used.
	inv, err := h.q.GetInvitationByToken(context.Background(), token)
	if err != nil {
		t.Fatalf("GetInvitationByToken: %v", err)
	}
	if !inv.UsedAt.Valid {
		t.Error("expected invitation to be marked used")
	}
}

// covers: INV-AUTH-06
func TestHandleCallback_InvitationError(t *testing.T) {
	h := newAuthHarness(t)
	h.installStubOAuth(&googleClaims{
		Sub:           "new-google-sub-x",
		Email:         "x@example.com",
		EmailVerified: true,
		Name:          "X",
	}, nil)

	req := callbackRequest("s", "the-code")
	req.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: "s"})
	req.AddCookie(&http.Cookie{Name: oauthInviteCookieName, Value: "definitely-not-a-token"})
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: want 400, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invitation") {
		t.Errorf("body should mention 'invitation', got %q", rec.Body.String())
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
		Sub:           h.user.GoogleSub,
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
