package backup

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/httperr"
)

// handleDemoReset wipes and reseeds the shared demo Household (ADR-0041, #217).
// A GitHub Actions cron is the intended caller — there is no user session here,
// so authentication is a dedicated bearer token (constant-time compared),
// deliberately not FLY_API_TOKEN (Fly's own control-plane credential, not an
// app-level secret).
func (h *Handlers) handleDemoReset(w http.ResponseWriter, r *http.Request) {
	if !validDemoResetToken(r, h.demoResetToken) {
		httperr.Write(w, http.StatusUnauthorized, httperr.CodeUnauthorized, nil)
		return
	}

	ctx := r.Context()
	existing, err := h.q.GetUserByEmail(ctx, h.demoEmail)
	if err != nil {
		slog.Error("demo reset: no user for DEMO_EMAIL — has the demo household been founded?", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}

	if err := resetDemoHousehold(ctx, h.pool, h.q, existing.HouseholdID, h.demoEmail, h.demoPassword); err != nil {
		slog.Error("demo reset failed", "err", err)
		httperr.Write(w, http.StatusInternalServerError, httperr.CodeInternal, nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// mountDemoReset wires the reset endpoint directly on r, outside the /backup
// RequireAuth group — this is a CI-triggered admin call, not a member session.
// Called from Mount only when DemoConfig.Enabled: off-demo the route does not
// exist (404), matching the conditional-construction pattern AUTH_GOOGLE_ENABLED
// already uses for the OAuth client (ADR-0039) rather than existing-but-refusing.
func (h *Handlers) mountDemoReset(r chi.Router) {
	r.Post("/admin/demo-reset", h.handleDemoReset)
}

func validDemoResetToken(r *http.Request, want string) bool {
	if want == "" {
		return false
	}
	const prefix = "Bearer "
	got := r.Header.Get("Authorization")
	if !strings.HasPrefix(got, prefix) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(got, prefix)), []byte(want)) == 1
}

// resetDemoHousehold wipes hid (reusing wipeHousehold, the same primitive
// restore/erasure use) and reseeds a fresh demo Household in its place: a
// credentialed local User any visitor signs in as, a credential-less second
// member for SoleOwner/Joint attribution realism, and a couple of toy positions
// so the dashboard isn't empty on first visit (ADR-0041).
//
// The wipe runs in its own transaction (mirrors EraseHousehold); the reseed
// runs after via the same repo constructors production handlers use — those
// manage their own per-call transactions, so this isn't a single atomic
// wipe+reseed. A failure here only ever affects the demo (recoverable by
// re-running the reset), never a real Household.
func resetDemoHousehold(ctx context.Context, pool *pgxpool.Pool, q *db.Queries, hid uuid.UUID, email, password string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("demo reset: begin wipe tx: %w", err)
	}
	if err := wipeHousehold(ctx, tx, hid); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("demo reset: commit wipe: %w", err)
	}

	household, err := q.CreateHousehold(ctx, db.CreateHouseholdParams{
		DisplayName:       "Demo Household",
		ReportingCurrency: "IDR",
	})
	if err != nil {
		return fmt.Errorf("demo reset: create household: %w", err)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("demo reset: hash password: %w", err)
	}
	demoUser, err := q.CreateLocalUser(ctx, db.CreateLocalUserParams{
		HouseholdID: household.ID,
		DisplayName: "Demo",
		Email:       email,
		Locale:      "en-GB",
		TimeZone:    "Asia/Jakarta",
	})
	if err != nil {
		return fmt.Errorf("demo reset: create demo user: %w", err)
	}
	if _, err := q.UpsertLocalCredential(ctx, db.UpsertLocalCredentialParams{
		UserID:       demoUser.ID,
		PasswordHash: hash,
	}); err != nil {
		return fmt.Errorf("demo reset: set demo credential: %w", err)
	}

	// A second, credential-less member — never logged into — purely so
	// SoleOwner/Joint ownership attribution has someone besides "Demo" to point
	// at in the UI. Dormant in the ADR-0039 sense: a real row, no
	// local_credentials, created_by ties it to the demo user as the founder.
	member2, err := q.CreateLocalUser(ctx, db.CreateLocalUserParams{
		HouseholdID: household.ID,
		DisplayName: "Alex",
		Email:       "alex+" + email,
		Locale:      "en-GB",
		TimeZone:    "Asia/Jakarta",
		CreatedBy:   &demoUser.ID,
	})
	if err != nil {
		return fmt.Errorf("demo reset: create second member: %w", err)
	}

	if err := seedDemoData(auth.WithUser(ctx, demoUser), pool, demoUser.ID, member2.ID); err != nil {
		return err
	}
	return nil
}
