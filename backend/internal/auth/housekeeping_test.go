package auth

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// TestSweep_DeletesExpiredRowsAndEvictsLimiter seeds one expired row of each
// kind sweep() is responsible for (#360) plus an elapsed rate-limiter entry,
// runs a single sweep, and asserts everything is gone.
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

	h.h.limiter.now = func() time.Time { return time.Now() }
	h.h.limiter.recordFailure("ip:sweep-test")
	h.h.limiter.recordFailure("ip:sweep-test") // second failure imposes a real (elapsed) wait
	h.h.limiter.entries["ip:sweep-test"].blockedUntil = past

	h.h.sweep(ctx)

	if _, err := h.q.GetSessionByID(ctx, sessionID); err == nil {
		t.Error("expired session should have been deleted by sweep")
	} else if err != pgx.ErrNoRows {
		t.Fatalf("GetSessionByID: unexpected error %v", err)
	}

	if _, err := h.q.GetOnboardingHandshake(ctx, handshakeID); err == nil {
		t.Error("expired handshake should have been deleted by sweep")
	} else if err != pgx.ErrNoRows {
		t.Fatalf("GetOnboardingHandshake: unexpected error %v", err)
	}

	if _, err := h.q.ConsumePasswordResetToken(ctx, tokenHash); err != pgx.ErrNoRows {
		t.Errorf("expired reset token should be gone (ConsumePasswordResetToken should miss), got err=%v", err)
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
