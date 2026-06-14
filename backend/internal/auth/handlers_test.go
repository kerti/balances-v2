package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// covers: INV-AUTH-02
func TestHandleStart(t *testing.T) {
	h := newAuthHarness(t)

	t.Run("redirects to Google AuthCodeURL with state cookie", func(t *testing.T) {
		rec := h.doRaw(t, "GET", "/auth/google/start", nil, nil)
		if rec.Code != http.StatusFound {
			t.Fatalf("status: want 302, got %d", rec.Code)
		}
		stateCookie := findCookie(rec, oauthStateCookieName)
		if stateCookie == nil || stateCookie.Value == "" {
			t.Fatal("expected oauth_state cookie to be set with a non-empty value")
		}
		if !stateCookie.HttpOnly {
			t.Error("oauth_state cookie should be HttpOnly")
		}

		loc, err := url.Parse(rec.Header().Get("Location"))
		if err != nil {
			t.Fatalf("parse Location: %v", err)
		}
		if !strings.Contains(loc.Host, "google") {
			t.Errorf("redirect host should be google: got %q", loc.Host)
		}
		if loc.Query().Get("state") != stateCookie.Value {
			t.Errorf("redirect state mismatch: cookie=%q, query=%q",
				stateCookie.Value, loc.Query().Get("state"))
		}
		if loc.Query().Get("client_id") == "" {
			t.Error("redirect missing client_id")
		}
	})

	t.Run("sets invite cookie when ?invite= present", func(t *testing.T) {
		rec := h.doRaw(t, "GET", "/auth/google/start?invite=invite-token-123", nil, nil)
		if rec.Code != http.StatusFound {
			t.Fatalf("status: want 302, got %d", rec.Code)
		}
		c := findCookie(rec, oauthInviteCookieName)
		if c == nil {
			t.Fatal("expected oauth_invite cookie to be set")
		}
		if c.Value != "invite-token-123" {
			t.Errorf("invite cookie value: got %q", c.Value)
		}
	})

	t.Run("no invite cookie when ?invite= absent", func(t *testing.T) {
		rec := h.doRaw(t, "GET", "/auth/google/start", nil, nil)
		if c := findCookie(rec, oauthInviteCookieName); c != nil {
			t.Errorf("unexpected oauth_invite cookie: %+v", c)
		}
	})
}

func TestHandleMe(t *testing.T) {
	h := newAuthHarness(t)

	t.Run("200 returns the authed user", func(t *testing.T) {
		rec := h.do(t, "GET", "/me", nil)
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[meResponse](t, rec)
		if body.ID != h.user.ID {
			t.Errorf("id: want %s, got %s", h.user.ID, body.ID)
		}
		if body.Email != h.user.Email {
			t.Errorf("email: want %q, got %q", h.user.Email, body.Email)
		}
		if body.HouseholdID != h.user.HouseholdID {
			t.Errorf("household_id mismatch")
		}
	})

	t.Run("401 without user in context", func(t *testing.T) {
		rec := h.doRaw(t, "GET", "/me", nil, nil)
		requireStatus(t, rec, http.StatusUnauthorized)
	})
}

func TestHandleListHouseholdMembers(t *testing.T) {
	h := newAuthHarness(t)

	// Add a second member to alice's household so the list has multiple rows
	// and we can verify sort order. CreateUser via the generated query keeps
	// us household-scoped (CreateHouseholdWithUser would build a new one).
	bob, err := h.q.CreateUser(context.Background(), db.CreateUserParams{
		HouseholdID: h.user.HouseholdID,
		DisplayName: "Bob",
		Email:       "bob@example.com",
		GoogleSub:   "test-sub-bob",
		Locale:      "id-ID",
		TimeZone:    "Asia/Jakarta",
		CreatedBy:   &h.user.ID,
	})
	if err != nil {
		t.Fatalf("seed second household member: %v", err)
	}

	t.Run("200 returns both household members sorted by display_name", func(t *testing.T) {
		rec := h.do(t, "GET", "/household/members", nil)
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[[]householdMember](t, rec)
		if len(body) != 2 {
			t.Fatalf("members count: want 2, got %d", len(body))
		}
		// Alice → Bob: alphabetical
		if body[0].ID != h.user.ID {
			t.Errorf("members[0]: want alice, got %s", body[0].DisplayName)
		}
		if body[1].ID != bob.ID {
			t.Errorf("members[1]: want bob, got %s", body[1].DisplayName)
		}
		// Response shape: must not leak internal fields. We decode into
		// householdMember which only has ID/DisplayName/Email — if the
		// handler sent more, the decoder would silently drop it. Spot-check
		// via raw body that google_sub isn't present.
		if strings.Contains(rec.Body.String(), "google_sub") {
			t.Errorf("response leaked google_sub: %s", rec.Body.String())
		}
	})

	t.Run("401 without user in context", func(t *testing.T) {
		rec := h.doRaw(t, "GET", "/household/members", nil, nil)
		requireStatus(t, rec, http.StatusUnauthorized)
	})

	t.Run("tenancy: separate household sees only its own members", func(t *testing.T) {
		// Carol lives in a different household; she should see only herself.
		carolPool := h.pool
		carolQ := db.New(carolPool)
		carolHH, err := carolQ.CreateHousehold(context.Background(), db.CreateHouseholdParams{
			DisplayName:       "Carol's Household",
			ReportingCurrency: "IDR",
		})
		if err != nil {
			t.Fatalf("create carol household: %v", err)
		}
		carol, err := carolQ.CreateUser(context.Background(), db.CreateUserParams{
			HouseholdID: carolHH.ID,
			DisplayName: "Carol",
			Email:       "carol@example.com",
			GoogleSub:   "test-sub-carol",
			Locale:      "id-ID",
			TimeZone:    "Asia/Jakarta",
		})
		if err != nil {
			t.Fatalf("create carol: %v", err)
		}

		rec := h.doRaw(t, "GET", "/household/members", nil, &carol)
		requireStatus(t, rec, http.StatusOK)
		body := decodeBody[[]householdMember](t, rec)
		if len(body) != 1 || body[0].ID != carol.ID {
			t.Errorf("carol should see only herself, got %+v", body)
		}
	})
}

// covers: INV-AUTH-04
func TestHandleLogout(t *testing.T) {
	h := newAuthHarness(t)

	t.Run("204 and deletes the session row when cookie is present", func(t *testing.T) {
		sessionID := mustRandomSessionID(t)
		_, err := h.q.CreateSession(context.Background(), db.CreateSessionParams{
			ID:        sessionID,
			UserID:    h.user.ID,
			ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(30 * time.Minute), Valid: true},
		})
		if err != nil {
			t.Fatalf("seed session: %v", err)
		}

		req := httptest.NewRequest("POST", "/auth/logout", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
		rec := httptest.NewRecorder()
		h.router.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Errorf("status: want 204, got %d (body: %s)", rec.Code, rec.Body.String())
		}

		// Session row should now be gone (GetSessionByID filters out expired
		// AND missing rows the same way).
		_, err = h.q.GetSessionByID(context.Background(), sessionID)
		if err == nil {
			t.Error("expected session to be deleted, GetSessionByID still found it")
		}

		// Cookie cleared.
		c := findCookie(rec, sessionCookieName)
		if c == nil || c.MaxAge >= 0 {
			t.Errorf("expected cleared session cookie, got %+v", c)
		}
	})

	t.Run("204 even without a cookie (idempotent)", func(t *testing.T) {
		rec := h.doRaw(t, "POST", "/auth/logout", nil, nil)
		if rec.Code != http.StatusNoContent {
			t.Errorf("status: want 204, got %d", rec.Code)
		}
		c := findCookie(rec, sessionCookieName)
		if c == nil || c.MaxAge >= 0 {
			t.Errorf("expected cleared session cookie, got %+v", c)
		}
	})
}
