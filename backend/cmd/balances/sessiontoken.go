package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/config"
	"github.com/kerti/balances-v2/backend/internal/db"
)

// sessionTokenCmd is the dev helper `balances session-token <email>`. Sessions
// are hashed at rest (#361), so a raw `SELECT id FROM sessions` — the previous
// shape of `make session-token` — can no longer recover a usable bearer value
// (SHA-256 is one-way). This mints a fresh session directly and prints the
// plaintext once, the same shape `reset-password` already uses for links.
//
// stdout carries the token alone (for copy/paste or piping into curl); the
// human-readable context goes to stderr. Dev/test tooling only — there is no
// password check here, by design: whoever can reach the database can already
// mint themselves a session by hand.
func sessionTokenCmd(args []string) error {
	email, err := parseSessionTokenArgs(args)
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer pool.Close()
	q := db.New(pool)

	user, err := q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("no user with email %q", email)
		}
		return fmt.Errorf("look up user: %w", err)
	}

	plaintext, err := auth.RandomSessionID()
	if err != nil {
		return fmt.Errorf("generate session id: %w", err)
	}
	expiresAt := time.Now().Add(cfg.SessionTTL)
	if _, err := q.CreateSession(ctx, db.CreateSessionParams{
		ID:        auth.HashToken(plaintext),
		UserID:    user.ID,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	fmt.Fprintf(os.Stderr,
		"Session minted for %s (expires %s). For curl smoke tests only:\n",
		email, expiresAt.Format("2006-01-02 15:04 MST"))
	// Sole stdout line: the bearer token, for copy/paste or piping.
	fmt.Println(plaintext)
	return nil
}

// parseSessionTokenArgs extracts the single required <email> positional. Split
// out so the argument contract is unit-testable without a database.
func parseSessionTokenArgs(args []string) (string, error) {
	if len(args) != 1 || args[0] == "" {
		return "", errors.New("usage: balances session-token <email>")
	}
	return args[0], nil
}
