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
	"github.com/kerti/balances-v2/backend/internal/identity"
)

func TestWithUserAndUserFromContext(t *testing.T) {
	user := db.User{ID: uuid.New(), DisplayName: "Probe"}

	t.Run("round-trip", func(t *testing.T) {
		ctx := identity.WithUser(context.Background(), user)
		got, ok := identity.UserFromContext(ctx)
		if !ok {
			t.Fatal("UserFromContext: ok=false on WithUser ctx")
		}
		if got.ID != user.ID {
			t.Errorf("user.ID: want %s, got %s", user.ID, got.ID)
		}
	})

	t.Run("empty context yields no user", func(t *testing.T) {
		_, ok := identity.UserFromContext(context.Background())
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
		req = req.WithContext(identity.WithUser(req.Context(), user))
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
		user, ok := identity.UserFromContext(r.Context())
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
			ID:        HashToken(sessionID),
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
			ID:        HashToken(sessionID),
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
		// The refreshed cookie must stay the plaintext bearer value presented,
		// not the hashed at-rest id (#361) — a regression here silently breaks
		// every session after its first sliding-TTL refresh.
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

// TestIssueSession_HashesAtRest verifies sessions get the same at-rest
// treatment as every other credential-shaped secret (HashToken): the stored
// row never equals the plaintext cookie value, and only the hash is what a
// lookup actually matches. Uses raw COUNT(*), not GetSessionByID, to check the
// negative case — an expiry-gated getter would give a false pass on a missing
// row exactly like on a deleted one.
//
// covers: INV-AUTH-28
func TestIssueSession_HashesAtRest(t *testing.T) {
	h := newAuthHarness(t)
	rec := httptest.NewRecorder()
	if err := h.h.IssueSession(context.Background(), rec, h.user.ID, "test-agent"); err != nil {
		t.Fatalf("IssueSession: %v", err)
	}
	cookie := findCookie(rec, sessionCookieName)
	if cookie == nil || cookie.Value == "" {
		t.Fatal("expected a non-empty session cookie")
	}

	if got := countRows(t, h, "sessions", "id", cookie.Value); got != 0 {
		t.Errorf("plaintext cookie value found at rest — sessions must be hashed, count = %d", got)
	}
	if got := countRows(t, h, "sessions", "id", HashToken(cookie.Value)); got != 1 {
		t.Errorf("hashed session id not found at rest, count = %d", got)
	}

	session, err := h.q.GetSessionByID(context.Background(), HashToken(cookie.Value))
	if err != nil {
		t.Fatalf("GetSessionByID(hash): %v", err)
	}
	if session.UserID != h.user.ID {
		t.Errorf("session.user_id: want %s, got %s", h.user.ID, session.UserID)
	}
}

func mustRandomSessionID(t *testing.T) string {
	t.Helper()
	id, err := RandomSessionID()
	if err != nil {
		t.Fatalf("RandomSessionID: %v", err)
	}
	return id
}

// covers: INV-AUTH-03
func TestRandomSessionID_UniqueAndNonEmpty(t *testing.T) {
	seen := make(map[string]bool, 32)
	for range 32 {
		id, err := RandomSessionID()
		if err != nil {
			t.Fatalf("RandomSessionID: %v", err)
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

	// ClearSessionCookie is the exported wrapper the backup package's erasure
	// flow calls (ADR-0040) — it must behave identically to the private helper.
	t.Run("ClearSessionCookie (exported) sets MaxAge<0", func(t *testing.T) {
		rec := httptest.NewRecorder()
		h.h.ClearSessionCookie(rec)
		c := findCookie(rec, sessionCookieName)
		if c == nil {
			t.Fatal("no clear cookie set")
		}
		if c.MaxAge >= 0 {
			t.Errorf("MaxAge: want <0, got %d", c.MaxAge)
		}
	})
}
