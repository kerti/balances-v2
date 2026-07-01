package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/config"
	"github.com/kerti/balances-v2/backend/internal/db"
)

// resetPasswordCmd is the operator escape hatch `balances reset-password <email>`
// (ADR-0039, #284). It mints a one-time set-password link for a local account and
// prints it — the out-of-band recovery path when mail is off or a founder is
// unavailable, and the ONLY way to reset an *active* member (the in-app path
// refuses one). It works regardless of EMAIL_ENABLED because it never sends mail;
// it just prints the link for the operator to relay.
//
// stdout carries the link alone (so it can be copied or piped); everything else —
// the email it is for and the expiry — goes to stderr. The plaintext token is
// never logged.
func resetPasswordCmd(args []string) error {
	email, err := parseResetPasswordArgs(args)
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	// A set-password link is only usable where a member can then sign in with a
	// password. On a Google-only instance (AUTH_LOCAL_ENABLED=false) the minted
	// link would be a dead end, so refuse up front rather than hand one out.
	if !cfg.AuthLocalEnabled {
		return errors.New("local password auth is disabled (AUTH_LOCAL_ENABLED=false); a set-password link would be unusable")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer pool.Close()

	link, expiresAt, err := auth.MintResetPasswordLink(ctx, db.New(pool), cfg.FrontendURL, email)
	if err != nil {
		// The operator-facing sentinels are plain messages, not stack-y wraps.
		switch {
		case errors.Is(err, auth.ErrInvalidEmail),
			errors.Is(err, auth.ErrNoLocalAccount),
			errors.Is(err, auth.ErrGoogleAccount):
			return err
		default:
			return fmt.Errorf("mint reset link: %w", err)
		}
	}

	fmt.Fprintf(os.Stderr,
		"Set-password link for %s (expires %s). Relay it to the member; it is single-use:\n",
		email, expiresAt.Format("2006-01-02 15:04 MST"))
	// Sole stdout line: the link, for copy/paste or piping.
	fmt.Println(link)
	return nil
}

// parseResetPasswordArgs extracts the single required <email> positional. Split
// out so the argument contract is unit-testable without a database or config.
func parseResetPasswordArgs(args []string) (string, error) {
	if len(args) != 1 || args[0] == "" {
		return "", errors.New("usage: balances reset-password <email>")
	}
	return args[0], nil
}
