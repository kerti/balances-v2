package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// Operator-CLI set-password mint (ADR-0039, #284). `balances reset-password
// <email>` is the out-of-band escape hatch: it mints a one-time set-password
// link for a local account regardless of mail posture (EMAIL_ENABLED), and it is
// the ONLY path that can reset an *active* member (one who already holds a
// credential) — the in-app founder-assisted reactivation deliberately refuses an
// active member because resetting a live account is impersonation. On the box,
// the operator already has DB access, so there is no member-enumeration concern
// here (unlike the HTTP reset endpoint's generic-204); a precise error is the
// right ergonomics.
//
// This is the DB-facing core, kept out of package main so it runs against the
// shared test container like the rest of auth. The cmd wrapper only adds config
// loading, the AUTH_LOCAL_ENABLED gate, and printing (the plaintext link goes to
// stdout for the operator to relay; it is never logged).

var (
	// ErrInvalidEmail is returned when the argument is not a single valid address.
	ErrInvalidEmail = errors.New("not a valid email address")
	// ErrNoLocalAccount is returned when no live user matches the email. Callers
	// surface this precisely — the operator holds DB access, so there is nothing to
	// hide (contrast the HTTP reset's no-enumeration generic response).
	ErrNoLocalAccount = errors.New("no account found for that email")
	// ErrGoogleAccount is returned when the matched user authenticates via Google:
	// there is no password to reset, and minting a local credential for them would
	// be account-linking, which is out of scope (ADR-0039).
	ErrGoogleAccount = errors.New("account signs in with Google; no password to reset")
)

// MintResetPasswordLink mints a one-time set-password link for the local account
// identified by email and returns the link plus its expiry. It powers the
// operator CLI (#284): it works whether or not the account already holds a
// credential (unlike in-app reactivation, it can reset an active member) and does
// not depend on mail. The token rides the shared primitive (token.go): a
// single-use, short-TTL, ≥256-bit random token stored only as a SHA-256 hash; the
// returned plaintext is the only copy and must never be persisted or logged by
// the caller. Following the link consumes the token via the reset set-password
// path, replacing the credential and signing the member in.
func MintResetPasswordLink(ctx context.Context, q *db.Queries, frontendURL, rawEmail string) (link string, expiresAt time.Time, err error) {
	email, ok := normalizeEmail(rawEmail)
	if !ok {
		return "", time.Time{}, ErrInvalidEmail
	}

	user, err := q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", time.Time{}, ErrNoLocalAccount
		}
		return "", time.Time{}, err
	}
	// A Google member has no local password. Refuse rather than silently minting
	// them a first local credential (account-linking is out of scope, ADR-0039).
	if user.GoogleSub != nil {
		return "", time.Time{}, ErrGoogleAccount
	}

	token, tokenHash, err := GenerateToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt = time.Now().Add(RelayTokenTTL)
	if _, err := q.CreatePasswordResetToken(ctx, db.CreatePasswordResetTokenParams{
		TokenHash: tokenHash,
		UserID:    user.ID,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}); err != nil {
		return "", time.Time{}, err
	}

	return ResetURL(frontendURL, token, user.Locale), expiresAt, nil
}
