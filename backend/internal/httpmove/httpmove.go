// Package httpmove builds the "move a snapshot to another month" handlers
// (ADR-0046 amendment, ADR-0058 §1-§2) every snapshot group shares. Each group's
// package mounts them beside its PATCH/DELETE routes:
//
//	POST …/snapshots/{id}/move        body {"year_month"}: archives the {id} row
//	                                  and records its values in that month,
//	                                  archiving whatever the month held; answers
//	                                  200 with the new snapshot and `replaced_id`
//	                                  (the displaced row, null when the month was
//	                                  empty)
//	POST …/snapshots/{id}/undo-move   body {"moved_id", "displaced_id"?}: archives
//	                                  the moved row, restores the displaced row
//	                                  and the {id} original; answers 204
//
// The repo and the clock are passed as accessors, not bound values, because
// Mount must stay safe on a nil *Handlers (see httpundo).
package httpmove

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/dateguard"
	"github.com/kerti/balances-v2/backend/internal/httperr"
)

type moveReq struct {
	// YearMonth accepts "YYYY-MM" or "YYYY-MM-DD" (normalised to the first of
	// the month), like the create endpoints.
	YearMonth string `json:"year_month"`
}

func parseYearMonth(s string) (time.Time, error) {
	if len(s) == 7 {
		return time.Parse("2006-01", s)
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC), nil
}

// Move moves one snapshot. A target month in the future is refused with the
// same FUTURE_YEAR_MONTH the create endpoints answer.
func Move[R, W any](op, param, field string, repo func() R, now func() time.Time, move func(R, context.Context, uuid.UUID, time.Time) (*W, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, param))
		if err != nil {
			httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidID, map[string]any{"field": field})
			return
		}
		var req moveReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
			return
		}
		if req.YearMonth == "" {
			httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation,
				map[string]any{"field": "year_month", "rule": "required"})
			return
		}
		ym, err := parseYearMonth(req.YearMonth)
		if err != nil {
			httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidYearMonth, nil)
			return
		}
		if dateguard.IsFutureMonth(ym, now()) {
			httperr.Write(w, http.StatusBadRequest, httperr.CodeFutureYearMonth, nil)
			return
		}
		moved, err := move(repo(), r.Context(), id, ym)
		if err != nil {
			httperr.WriteRepo(w, op, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(moved)
	}
}

type undoMoveReq struct {
	MovedID     uuid.UUID  `json:"moved_id"`
	DisplacedID *uuid.UUID `json:"displaced_id"`
}

// Undo reverses a move: the {param} row is the archived original to restore,
// the body names the row the move created and the row it displaced, if any.
func Undo[R any](op, param, field string, repo func() R, undo func(R, context.Context, uuid.UUID, uuid.UUID, *uuid.UUID) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		restoreID, err := uuid.Parse(chi.URLParam(r, param))
		if err != nil {
			httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidID, map[string]any{"field": field})
			return
		}
		var req undoMoveReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
			return
		}
		if req.MovedID == uuid.Nil {
			httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation,
				map[string]any{"field": "moved_id", "rule": "required"})
			return
		}
		if err := undo(repo(), r.Context(), restoreID, req.MovedID, req.DisplacedID); err != nil {
			httperr.WriteRepo(w, op, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
