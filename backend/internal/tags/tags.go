// Package tags exposes HTTP handlers for user-defined position Tags
// (ADR-0028). A Tag is a household-scoped grouping label; a Position carries
// at most one. Assignment is a dedicated endpoint (PUT /api/tags/assignments)
// because a Tag is orthogonal to a Position's identity, and a breakdown
// endpoint (GET /api/tags/breakdown) sums Position value per (tag, currency).
package tags

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
)

type Handlers struct {
	repo     *repo.TagRepo
	validate *validator.Validate
}

func New(r *repo.TagRepo) *Handlers {
	return &Handlers{repo: r, validate: httperr.NewValidator()}
}

func (h *Handlers) Mount(r chi.Router) {
	r.Route("/tags", func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/", h.handleCreate)
		r.Get("/", h.handleList)
		r.Get("/breakdown", h.handleBreakdown)
		r.Get("/items", h.handleItems)
		r.Put("/assignments", h.handleAssign)
		r.Put("/assignments/bulk", h.handleAssignBulk)
		r.Route("/{id}", func(r chi.Router) {
			r.Patch("/", h.handleUpdate)
			r.Delete("/", h.handleDelete)
		})
	})
}

// ----- requests -----------------------------------------------------------

type writeReq struct {
	Name  string `json:"name"  validate:"required"`
	Color string `json:"color" validate:"required"`
}

type assignReq struct {
	Group      string     `json:"group"       validate:"required,oneof=asset liability receivable investment"`
	PositionID uuid.UUID  `json:"position_id" validate:"required"`
	TagID      *uuid.UUID `json:"tag_id"` // nil unassigns
}

// assignBulkReq moves (or, with a nil tag_id, clears) many Positions in one
// write. The cap is a sanity bound well above a household's position count.
type assignBulkReq struct {
	TagID *uuid.UUID         `json:"tag_id"` // nil unassigns
	Items []assignBulkItemRq `json:"items" validate:"required,min=1,max=500,dive"`
}

type assignBulkItemRq struct {
	Group      string    `json:"group"       validate:"required,oneof=asset liability receivable investment"`
	PositionID uuid.UUID `json:"position_id" validate:"required"`
}

// ----- handlers -----------------------------------------------------------

func (h *Handlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req writeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}
	row, err := h.repo.CreateTag(r.Context(), req.Name, req.Color)
	if err != nil {
		httperr.WriteRepo(w, "create tag", err)
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

func (h *Handlers) handleList(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListTags(r.Context())
	if err != nil {
		httperr.WriteRepo(w, "list tags", err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidID, map[string]any{"field": "id"})
		return
	}
	var req writeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}
	row, err := h.repo.UpdateTag(r.Context(), id, req.Name, req.Color)
	if err != nil {
		httperr.WriteRepo(w, "update tag", err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *Handlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidID, map[string]any{"field": "id"})
		return
	}
	if err := h.repo.DeleteTag(r.Context(), id); err != nil {
		httperr.WriteRepo(w, "delete tag", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) handleAssign(w http.ResponseWriter, r *http.Request) {
	var req assignReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}
	if err := h.repo.AssignTag(r.Context(), repo.TagGroup(req.Group), req.PositionID, req.TagID); err != nil {
		httperr.WriteRepo(w, "assign tag", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) handleAssignBulk(w http.ResponseWriter, r *http.Request) {
	var req assignBulkReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}
	items := make([]repo.TagItemRef, len(req.Items))
	for i, it := range req.Items {
		items[i] = repo.TagItemRef{Group: repo.TagGroup(it.Group), PositionID: it.PositionID}
	}
	if err := h.repo.AssignTagBatch(r.Context(), req.TagID, items); err != nil {
		httperr.WriteRepo(w, "assign tags", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handlers) handleItems(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.ListTaggableItems(r.Context())
	if err != nil {
		httperr.WriteRepo(w, "list taggable items", err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// breakdownRow renames the generated row's `grp` to the wire-friendly `group`.
type breakdownRow struct {
	TagID    *uuid.UUID      `json:"tag_id"`
	Group    string          `json:"group"`
	Currency string          `json:"currency"`
	Total    decimal.Decimal `json:"total"`
}

func (h *Handlers) handleBreakdown(w http.ResponseWriter, r *http.Request) {
	rows, err := h.repo.TagBreakdown(r.Context())
	if err != nil {
		httperr.WriteRepo(w, "tag breakdown", err)
		return
	}
	out := make([]breakdownRow, len(rows))
	for i, row := range rows {
		out[i] = breakdownRow{TagID: row.TagID, Group: row.Grp, Currency: row.Currency, Total: row.Total}
	}
	writeJSON(w, http.StatusOK, out)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}
