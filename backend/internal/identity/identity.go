// Package identity is the leaf home of the request-scoped authenticated user:
// the context key plus WithUser / UserFromContext. It depends only on the
// generated db types, so any layer that needs the current user (repo, reports,
// backup) can read it without importing internal/auth — which would invert the
// dependency direction and, together with auth -> httperr -> repo, close an
// import cycle. auth owns the session/OAuth machinery and injects the user
// through WithUser; everyone downstream reads it through UserFromContext.
// See ADR-0027 and issue #366.
package identity

import (
	"context"

	"github.com/kerti/balances-v2/backend/internal/db"
)

type contextKey int

const userContextKey contextKey = iota

// UserFromContext returns the authenticated user from the request context, if any.
func UserFromContext(ctx context.Context) (db.User, bool) {
	u, ok := ctx.Value(userContextKey).(db.User)
	return u, ok
}

// WithUser returns a child context with the given User attached. Production
// code goes through auth.SessionMiddleware; tests use this directly to simulate
// an authenticated request without exercising the OAuth and session machinery.
func WithUser(ctx context.Context, u db.User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}
