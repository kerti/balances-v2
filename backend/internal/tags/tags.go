// Package tags exposes HTTP handlers for user-defined position Tags
// (ADR-0028). A Tag is a household-scoped grouping label; a Position carries
// at most one. A Position's own Tag is set through its create/update (an
// optional tag_id, written in the same transaction; issue #839). The Tags page
// moves many Positions at once through PUT /api/tags/assignments, and a
// breakdown endpoint (GET /api/tags/breakdown) sums Position value per Tag in
// the household reporting currency, from the dashboard's engine pass (#937).
package tags

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
)

type Handlers struct {
	repo     *repo.TagRepo
	reports  *repo.MonthlyReportRepo
	validate *validator.Validate
}

func New(r *repo.TagRepo, reports *repo.MonthlyReportRepo) *Handlers {
	return &Handlers{repo: r, reports: reports, validate: httperr.NewValidator()}
}

func (h *Handlers) Mount(r chi.Router) {
	r.Route("/tags", func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/", h.handleCreate)
		r.Get("/", h.handleList)
		r.Get("/breakdown", h.handleBreakdown)
		r.Get("/items", h.handleItems)
		r.Put("/assignments", h.handleAssign)
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

// assignReq moves (or, with a nil tag_id, clears) many Positions in one
// write (PUT /api/tags/assignments). The cap is a sanity bound well above a
// household's position count.
type assignReq struct {
	TagID *uuid.UUID      `json:"tag_id"` // nil unassigns
	Items []assignItemReq `json:"items" validate:"required,min=1,max=500,dive"`
}

type assignItemReq struct {
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

// handleBreakdown serves the Tags report in the household reporting currency,
// cut from the same engine pass as the dashboard's reports so its net is the
// dashboard's net worth for the month it names (INV-FINANCE-40).
func (h *Handlers) handleBreakdown(w http.ResponseWriter, r *http.Request) {
	out, err := h.reports.TagBreakdown(r.Context())
	if err != nil {
		httperr.WriteRepo(w, "tag breakdown", err)
		return
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
