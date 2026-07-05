package auth

import (
	"context"
	"log/slog"
	"time"
)

// StartHousekeeping runs an hourly sweep for state that is written once and
// never read again after it expires: session rows, onboarding handshakes,
// password-reset tokens (all DB-backed, per-table DeleteExpired* queries), and
// the in-memory login rate-limiter's backoff entries (#360). None of these are
// ever cleaned up by the request path itself — expiry is checked at read time,
// not enforced at write time — so without this loop they accumulate forever.
// Blocks until ctx is cancelled; callers run it in its own goroutine.
func (h *Handlers) StartHousekeeping(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.sweep(ctx)
		}
	}
}

func (h *Handlers) sweep(ctx context.Context) {
	if err := h.q.DeleteExpiredSessions(ctx); err != nil {
		slog.Error("housekeeping: delete expired sessions", "err", err)
	}
	if err := h.q.DeleteExpiredOnboardingHandshakes(ctx); err != nil {
		slog.Error("housekeeping: delete expired onboarding handshakes", "err", err)
	}
	if err := h.q.DeleteExpiredPasswordResetTokens(ctx); err != nil {
		slog.Error("housekeeping: delete expired password reset tokens", "err", err)
	}
	h.limiter.evictExpired()
}
