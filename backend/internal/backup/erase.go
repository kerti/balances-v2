package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/httperr"
)

// eraseHouseholdReq carries the confirm-by-name gate: the caller must type the
// Household's exact display_name back, checked server-side before any wipe
// (ADR-0040).
type eraseHouseholdReq struct {
	HouseholdName string `json:"household_name"`
}

// eraseHouseholdResp is the erase-commit response. Erased is always true on a
// 2xx (a failure takes the error path).
type eraseHouseholdResp struct {
	Erased bool `json:"erased"`
}

// handleEraseHousehold permanently deletes the caller's Household — every
// Position, Snapshot, Transaction, Income event, Tag, User, session, and
// credential (ADR-0040, #300). This is "restore with no load": it reuses
// wipeHousehold, the same primitive ADR-0036's restore uses to clear a
// Household before loading a backup into it.
//
// Founder-only (the lineage root, created_by IS NULL) — a peer member is
// refused (403). The confirm-by-name check is server-enforced: the request
// must echo the Household's exact display_name or it is refused (400) before
// any wipe. There is nothing to preview server-side (unlike restore, which
// validates an uploaded file) — the frontend already renders the Household's
// real name before the user types it, so this is a single call, not a
// preview/commit pair.
//
// The wipe deletes every session in the Household, including the caller's
// own — there is no household left to re-issue one against, so the cookie is
// cleared instead (the frontend routes to a dedicated post-erasure screen).
// A best-effort notification follows: a confirmation to the founder and a
// deletion notice to every other member, captured BEFORE the wipe runs since
// there is nothing left to query afterwards.
func (h *Handlers) handleEraseHousehold(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		httperr.Write(w, http.StatusUnauthorized, httperr.CodeUnauthorized, nil)
		return
	}
	if user.CreatedBy != nil {
		httperr.Write(w, http.StatusForbidden, httperr.CodeForbidden, nil)
		return
	}
	if h.demoMode {
		// Every demo visitor shares one identity (ADR-0041, #217) — a single click
		// here would lock out every subsequent visitor until the next nightly
		// reset, so Erasure is blocked outright while every other mutation stays
		// live.
		httperr.Write(w, http.StatusForbidden, httperr.CodeErasureDisabledDemo, nil)
		return
	}

	var req eraseHouseholdReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}

	ctx := r.Context()
	household, err := h.q.GetHouseholdForExport(ctx, user.HouseholdID)
	if err != nil {
		httperr.WriteRepo(w, "backup erase", err)
		return
	}
	if strings.TrimSpace(req.HouseholdName) != household.DisplayName {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeHouseholdNameMismatch, nil)
		return
	}

	// Captured before the wipe — there is nothing left to query afterwards
	// (unlike restore, which reloads the household).
	members, err := h.q.ListUsersForExport(ctx, db.ListUsersForExportParams{
		HouseholdID:    user.HouseholdID,
		IncludeDeleted: false,
	})
	if err != nil {
		httperr.WriteRepo(w, "backup erase", err)
		return
	}

	if err := EraseHousehold(ctx, h.pool, user.HouseholdID); err != nil {
		httperr.WriteRepo(w, "backup erase", err)
		return
	}

	// Must precede writeJSON (it sets a cookie header).
	h.sessions.ClearSessionCookie(w)

	// Best-effort (#300): the destructive work already committed, so a mail
	// outage must never reflect on it.
	h.notifier.NotifyErasure(ctx, members, user.ID, household.DisplayName)

	writeJSON(w, http.StatusOK, eraseHouseholdResp{Erased: true})
}

// EraseHousehold permanently deletes hid and everything under it in one
// transaction, reusing wipeHousehold — the same primitive Restore uses to
// clear a Household before loading — with nothing loaded after it (ADR-0040).
func EraseHousehold(ctx context.Context, pool *pgxpool.Pool, hid uuid.UUID) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("erase: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := wipeHousehold(ctx, tx, hid); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
