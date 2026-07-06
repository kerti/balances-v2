package auth

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// countRows is a raw, expiry-agnostic existence check. Every getter sweep()
// touches (GetSessionByID, GetOnboardingHandshake, ConsumePasswordResetToken)
// itself filters `WHERE ... expires_at > now()`, so asserting sweep's effect
// through one of those would pass even if sweep deleted nothing at all — the
// getter's own expiry guard, not the DELETE, would explain the miss. Counting
// the raw row is the only way to prove the row is actually gone.
func countRows(t *testing.T, h *authHarness, table, whereCol string, whereVal any) int {
	t.Helper()
	var count int
	query := "SELECT count(*) FROM " + table + " WHERE " + whereCol + " = $1"
	if err := h.pool.QueryRow(context.Background(), query, whereVal).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

// TestSweep_DeletesExpiredRowsAndEvictsLimiter seeds one expired row of each
// kind sweep() is responsible for (#360) plus an elapsed rate-limiter entry,
// runs a single sweep, and asserts every row is actually gone from the table
// (not merely invisible to an expiry-filtered getter).
func TestSweep_DeletesExpiredRowsAndEvictsLimiter(t *testing.T) {
	h := newAuthHarness(t)
	ctx := context.Background()
	past := time.Now().Add(-time.Hour)

	sessionID, err := randomSessionID()
	if err != nil {
		t.Fatalf("randomSessionID: %v", err)
	}
	if _, err := h.q.CreateSession(ctx, db.CreateSessionParams{
		ID:        sessionID,
		UserID:    h.user.ID,
		ExpiresAt: pgtype.Timestamptz{Time: past, Valid: true},
	}); err != nil {
		t.Fatalf("seed expired session: %v", err)
	}

	handshakeID := mustBeginHandshake(t, h, "expired-sub", "expired-handshake@example.com", "Expired", past)

	tokenHash := "sweep-test-token-hash"
	if _, err := h.q.CreatePasswordResetToken(ctx, db.CreatePasswordResetTokenParams{
		TokenHash: tokenHash,
		UserID:    h.user.ID,
		ExpiresAt: pgtype.Timestamptz{Time: past, Valid: true},
	}); err != nil {
		t.Fatalf("seed expired password reset token: %v", err)
	}

	// Sanity: all three rows exist before sweep runs — otherwise the
	// post-sweep zero counts below would prove nothing.
	if got := countRows(t, h, "sessions", "id", sessionID); got != 1 {
		t.Fatalf("precondition: sessions count = %d, want 1", got)
	}
	if got := countRows(t, h, "onboarding_handshakes", "id", handshakeID); got != 1 {
		t.Fatalf("precondition: onboarding_handshakes count = %d, want 1", got)
	}
	if got := countRows(t, h, "password_reset_tokens", "token_hash", tokenHash); got != 1 {
		t.Fatalf("precondition: password_reset_tokens count = %d, want 1", got)
	}

	h.h.limiter.now = func() time.Time { return time.Now() }
	h.h.limiter.recordFailure("ip:sweep-test")
	h.h.limiter.recordFailure("ip:sweep-test") // second failure imposes a real (elapsed) wait
	h.h.limiter.entries["ip:sweep-test"].blockedUntil = past

	h.h.sweep(ctx)

	if got := countRows(t, h, "sessions", "id", sessionID); got != 0 {
		t.Errorf("expired session should have been deleted by sweep, count = %d", got)
	}
	if got := countRows(t, h, "onboarding_handshakes", "id", handshakeID); got != 0 {
		t.Errorf("expired handshake should have been deleted by sweep, count = %d", got)
	}
	if got := countRows(t, h, "password_reset_tokens", "token_hash", tokenHash); got != 0 {
		t.Errorf("expired password reset token should have been deleted by sweep, count = %d", got)
	}

	h.h.limiter.mu.Lock()
	_, stillThere := h.h.limiter.entries["ip:sweep-test"]
	h.h.limiter.mu.Unlock()
	if stillThere {
		t.Error("elapsed limiter entry should have been evicted by sweep")
	}
}

// TestStartHousekeeping_StopsOnContextCancel asserts the loop returns promptly
// once its context is cancelled, rather than blocking forever on the ticker.
func TestStartHousekeeping_StopsOnContextCancel(t *testing.T) {
	h := newAuthHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		h.h.StartHousekeeping(ctx, time.Hour)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("StartHousekeeping did not return after ctx cancellation")
	}
}

// TestStartHousekeeping_SweepsOnTick exercises the ticker branch itself (the
// other test only ever hits ctx.Done()): a real expired session seeded before
// the loop starts must be physically deleted once the loop has actually
// ticked, proving the wiring from the ticker to sweep(), not just sweep()
// called directly. Uses the same raw-count check as the direct sweep test —
// GetSessionByID's own expiry filter would give a false pass here too.
func TestStartHousekeeping_SweepsOnTick(t *testing.T) {
	h := newAuthHarness(t)
	ctx := context.Background()

	sessionID, err := randomSessionID()
	if err != nil {
		t.Fatalf("randomSessionID: %v", err)
	}
	if _, err := h.q.CreateSession(ctx, db.CreateSessionParams{
		ID:        sessionID,
		UserID:    h.user.ID,
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
	}); err != nil {
		t.Fatalf("seed expired session: %v", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go h.h.StartHousekeeping(runCtx, 10*time.Millisecond)

	deadline := time.Now().Add(2 * time.Second)
	for {
		if countRows(t, h, "sessions", "id", sessionID) == 0 {
			return // swept
		}
		if time.Now().After(deadline) {
			t.Fatal("StartHousekeeping never swept the expired session via its ticker")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
