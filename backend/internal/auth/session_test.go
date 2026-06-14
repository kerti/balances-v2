package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kerti/balances-v2/backend/internal/db"
)

func TestWithUserAndUserFromContext(t *testing.T) {
	user := db.User{ID: uuid.New(), DisplayName: "Probe"}

	t.Run("round-trip", func(t *testing.T) {
		ctx := WithUser(context.Background(), user)
		got, ok := UserFromContext(ctx)
		if !ok {
			t.Fatal("UserFromContext: ok=false on WithUser ctx")
		}
		if got.ID != user.ID {
			t.Errorf("user.ID: want %s, got %s", user.ID, got.ID)
		}
	})

	t.Run("empty context yields no user", func(t *testing.T) {
		_, ok := UserFromContext(context.Background())
		if ok {
			t.Error("UserFromContext: want ok=false on empty ctx")
		}
	})
}

// covers: INV-AUTH-01
func TestRequireAuth(t *testing.T) {
	called := false
	handler := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("blocks unauthed with 401", func(t *testing.T) {
		called = false
		req := httptest.NewRequest("GET", "/probe", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status: want 401, got %d", rec.Code)
		}
		if called {
			t.Error("next handler should not have been called")
		}
	})

	t.Run("passes authed", func(t *testing.T) {
		called = false
		user := db.User{ID: uuid.New()}
		req := httptest.NewRequest("GET", "/probe", nil)
		req = req.WithContext(WithUser(req.Context(), user))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("status: want 200, got %d", rec.Code)
		}
		if !called {
			t.Error("next handler should have been called")
		}
	})
}

// SessionMiddleware tests use a small probe handler that echoes whether a
// user is in context (via status code) so we can verify ctx propagation
// without rebuilding the full route tree.
// covers: INV-AUTH-03
func TestSessionMiddleware(t *testing.T) {
	h := newAuthHarness(t)

	probe := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := UserFromContext(r.Context())
		if !ok {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id": user.ID.String()})
	})
	handler := h.h.SessionMiddleware(probe)

	t.Run("no cookie passes through without user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/probe", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Errorf("status: want 204, got %d", rec.Code)
		}
		if c := findCookie(rec, sessionCookieName); c != nil {
			t.Errorf("unexpected Set-Cookie for session: %+v", c)
		}
	})

	t.Run("empty cookie value passes through without user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/probe", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: ""})
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Errorf("status: want 204, got %d", rec.Code)
		}
	})

	t.Run("unknown session id clears cookie and passes through without user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/probe", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "nonexistent-session-id"})
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Errorf("status: want 204, got %d", rec.Code)
		}
		c := findCookie(rec, sessionCookieName)
		if c == nil {
			t.Fatal("expected Set-Cookie clearing session")
		}
		if c.MaxAge >= 0 {
			t.Errorf("expected MaxAge<0 (clear), got %d", c.MaxAge)
		}
	})

	t.Run("expired session is treated like unknown", func(t *testing.T) {
		sessionID := mustRandomSessionID(t)
		_, err := h.q.CreateSession(context.Background(), db.CreateSessionParams{
			ID:        sessionID,
			UserID:    h.user.ID,
			ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(-1 * time.Hour), Valid: true},
		})
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}

		req := httptest.NewRequest("GET", "/probe", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Errorf("status: want 204, got %d", rec.Code)
		}
		c := findCookie(rec, sessionCookieName)
		if c == nil || c.MaxAge >= 0 {
			t.Errorf("expected cleared cookie, got %+v", c)
		}
	})

	t.Run("valid session attaches user and refreshes cookie", func(t *testing.T) {
		sessionID := mustRandomSessionID(t)
		originalExpiry := time.Now().Add(10 * time.Minute)
		_, err := h.q.CreateSession(context.Background(), db.CreateSessionParams{
			ID:        sessionID,
			UserID:    h.user.ID,
			ExpiresAt: pgtype.Timestamptz{Time: originalExpiry, Valid: true},
		})
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}

		req := httptest.NewRequest("GET", "/probe", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status: want 200, got %d (body: %s)", rec.Code, rec.Body.String())
		}
		var body map[string]string
		if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body["id"] != h.user.ID.String() {
			t.Errorf("user id: want %s, got %s", h.user.ID, body["id"])
		}
		c := findCookie(rec, sessionCookieName)
		if c == nil {
			t.Fatal("expected refreshed session cookie")
		}
		if c.Value != sessionID {
			t.Errorf("cookie value: want session id preserved, got %q", c.Value)
		}
		// New expiry should be roughly sessionTTL from now (30m) — comfortably
		// past originalExpiry (10m from now).
		if !c.Expires.After(originalExpiry.Add(5 * time.Minute)) {
			t.Errorf("cookie expiry not refreshed: original=%v new=%v", originalExpiry, c.Expires)
		}
	})
}

func mustRandomSessionID(t *testing.T) string {
	t.Helper()
	id, err := randomSessionID()
	if err != nil {
		t.Fatalf("randomSessionID: %v", err)
	}
	return id
}

// covers: INV-AUTH-03
func TestRandomSessionID_UniqueAndNonEmpty(t *testing.T) {
	seen := make(map[string]bool, 32)
	for range 32 {
		id, err := randomSessionID()
		if err != nil {
			t.Fatalf("randomSessionID: %v", err)
		}
		if id == "" {
			t.Fatal("empty session id")
		}
		if seen[id] {
			t.Errorf("duplicate session id %q", id)
		}
		seen[id] = true
	}
}

// covers: INV-AUTH-03
func TestSessionCookieHelpers(t *testing.T) {
	h := newAuthHarness(t)

	t.Run("setSessionCookie writes expected attributes", func(t *testing.T) {
		rec := httptest.NewRecorder()
		expires := time.Now().Add(time.Hour)
		h.h.setSessionCookie(rec, "abc123", expires)
		c := findCookie(rec, sessionCookieName)
		if c == nil {
			t.Fatal("no session cookie set")
		}
		if c.Value != "abc123" {
			t.Errorf("value: got %q", c.Value)
		}
		if !c.HttpOnly {
			t.Error("HttpOnly should be true")
		}
		if c.SameSite != http.SameSiteLaxMode {
			t.Errorf("SameSite: got %v", c.SameSite)
		}
	})

	t.Run("clearSessionCookie sets MaxAge<0", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.h.clearSessionCookie(rec)
		c := findCookie(rec, sessionCookieName)
		if c == nil {
			t.Fatal("no clear cookie set")
		}
		if c.MaxAge >= 0 {
			t.Errorf("MaxAge: want <0, got %d", c.MaxAge)
		}
	})
}
