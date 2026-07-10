package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/identity"
)

const sessionCookieName = "session"

// RandomSessionID returns a fresh 256-bit random, URL-safe opaque value. Used
// as both the session bearer credential (IssueSession stores HashToken(id) and
// cookies the plaintext) and, unrelatedly, as the onboarding-handshake id
// (local.go, onboarding.go) — those are a different table with no hashing
// requirement. Exported for the `session-token` CLI dev helper, which mints a
// session outside the auth package (no http.ResponseWriter to cookie).
func RandomSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (h *Handlers) setSessionCookie(w http.ResponseWriter, sessionID string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSessionCookie expires the session cookie client-side. Exported for the
// backup package's erasure flow (ADR-0040), which has no household left to
// re-issue a session against once the wipe commits.
func (h *Handlers) ClearSessionCookie(w http.ResponseWriter) {
	h.clearSessionCookie(w)
}

func (h *Handlers) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

// SessionMiddleware reads the session cookie, looks up the session and user,
// touches the session (sliding TTL), and injects the user into the request
// context. Requests without a valid session continue without a user — handlers
// that require authentication wrap themselves with RequireAuth.
func (h *Handlers) SessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			next.ServeHTTP(w, r)
			return
		}

		ctx := r.Context()
		session, err := h.q.GetSessionByID(ctx, HashToken(cookie.Value))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				h.clearSessionCookie(w)
			}
			next.ServeHTTP(w, r)
			return
		}

		user, err := h.q.GetUserByID(ctx, session.UserID)
		if err != nil {
			h.clearSessionCookie(w)
			next.ServeHTTP(w, r)
			return
		}

		newExpiresAt := time.Now().Add(h.sessionTTL)
		_ = h.q.TouchSession(ctx, db.TouchSessionParams{
			ExpiresAt: pgtype.Timestamptz{Time: newExpiresAt, Valid: true},
			ID:        session.ID,
		})
		// session.ID is the hashed at-rest value (looked up above); the cookie
		// must keep the plaintext bearer value the client presented, or the next
		// request's hash-and-lookup would never match this row again.
		h.setSessionCookie(w, cookie.Value, newExpiresAt)

		next.ServeHTTP(w, r.WithContext(identity.WithUser(ctx, user)))
	})
}

// RequireAuth blocks requests that have no authenticated user in context.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := identity.UserFromContext(r.Context()); !ok {
			httperr.Write(w, http.StatusUnauthorized, httperr.CodeUnauthorized, nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}
