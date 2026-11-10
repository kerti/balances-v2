// Package httpundo builds the Undo handlers (ADR-0058 §2) every deletable entity
// shares, so the route shape, id parsing and error mapping live in one place.
// Each entity's package mounts them beside its DELETE route:
//
//	POST …/{id}/undo-delete              clears deleted_at on that one row
//	POST …/snapshots/{id}/undo-replace   body {"replacement_id"}: archives the
//	                                     replacement, restores the {id} row
//
// Both answer 204 on success; the client refetches, as it does after a delete.
//
// The repo is passed as an accessor plus a method expression, not a bound method
// value, because Mount must stay safe on a nil *Handlers (the httpserver test
// and the route generator mount handlers without a repo): the accessor is only
// called once a request arrives.
package httpundo

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/httperr"
)

// Delete undoes a single-item delete. param is the chi URL parameter holding
// the row id and field the wire name reported when it does not parse.
func Delete[R any](op, param, field string, repo func() R, undo func(R, context.Context, uuid.UUID) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, param))
		if err != nil {
			httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidID, map[string]any{"field": field})
			return
		}
		if err := undo(repo(), r.Context(), id); err != nil {
			httperr.WriteRepo(w, op, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type replaceReq struct {
	ReplacementID uuid.UUID `json:"replacement_id"`
}

// Replace undoes a snapshot replace: the {param} row is the archived one to
// restore, the body names the row that replaced it.
func Replace[R any](op, param, field string, repo func() R, undo func(R, context.Context, uuid.UUID, uuid.UUID) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		restoreID, err := uuid.Parse(chi.URLParam(r, param))
		if err != nil {
			httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidID, map[string]any{"field": field})
			return
		}
		var req replaceReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
			return
		}
		if req.ReplacementID == uuid.Nil {
			httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation,
				map[string]any{"field": "replacement_id", "rule": "required"})
			return
		}
		if err := undo(repo(), r.Context(), restoreID, req.ReplacementID); err != nil {
			httperr.WriteRepo(w, op, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
