// Package fxrates exposes HTTP handlers for the manual monthly FX-rate table
// (ADR-0002). Mounted under /api/fx-rates. Rates are entered when a household
// turns multi-currency on; the report engine applies them (latest <= month).
package fxrates

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/httpundo"
	"github.com/kerti/balances-v2/backend/internal/repo"
)

type Handlers struct {
	repo     *repo.FxRateRepo
	validate *validator.Validate
}

func New(r *repo.FxRateRepo) *Handlers {
	return &Handlers{repo: r, validate: httperr.NewValidator()}
}

func (h *Handlers) Mount(r chi.Router) {
	r.Route("/fx-rates", func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/", h.handleCreate)
		r.Get("/", h.handleList)
		r.Get("/currencies", h.handleCurrencies)
		r.Route("/{id}", func(r chi.Router) {
			r.Patch("/", h.handleUpdate)
			r.Delete("/", h.handleDelete)
			r.Post("/undo-delete", httpundo.Delete("undo delete fx rate", "id", "id", h.repository, (*repo.FxRateRepo).UndoDeleteFxRate))
		})
	})
}

type createReq struct {
	YearMonth string           `json:"year_month" validate:"required"`
	Currency  string           `json:"currency"   validate:"required,iso4217"`
	Rate      *decimal.Decimal `json:"rate"       validate:"required"`
}

type updateReq struct {
	Rate *decimal.Decimal `json:"rate" validate:"required"`
}

func (h *Handlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}
	ym, ok := parseYearMonth(req.YearMonth)
	if !ok {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidYearMonth, nil)
		return
	}
	if !req.Rate.IsPositive() {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidRate, nil)
		return
	}
	row, err := h.repo.CreateFxRate(r.Context(), repo.CreateFxRateParams{
		YearMonth: ym, Currency: req.Currency, Rate: *req.Rate,
	})
	if err != nil {
		httperr.WriteRepo(w, "create fx rate", err)
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

func (h *Handlers) handleList(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListFxRates(r.Context())
	if err != nil {
		httperr.WriteRepo(w, "list fx rates", err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// handleCurrencies lists the foreign currencies the household's items use, for
// the New-rate currency select.
func (h *Handlers) handleCurrencies(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListUsedForeignCurrencies(r.Context())
	if err != nil {
		httperr.WriteRepo(w, "list used foreign currencies", err)
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
	var req updateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}
	if !req.Rate.IsPositive() {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidRate, nil)
		return
	}
	row, err := h.repo.UpdateFxRate(r.Context(), id, *req.Rate)
	if err != nil {
		httperr.WriteRepo(w, "update fx rate", err)
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
	if err := h.repo.DeleteFxRate(r.Context(), id); err != nil {
		httperr.WriteRepo(w, "delete fx rate", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ----- helpers ------------------------------------------------------------

// parseYearMonth accepts "YYYY-MM" or "YYYY-MM-DD" and returns the
// first-of-month UTC. An unparseable string yields (zero, false).
func parseYearMonth(s string) (time.Time, bool) {
	if t, err := time.Parse("2006-01", s); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC), true
	}
	return time.Time{}, false
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

// repository hands httpundo the repo lazily: Mount must stay safe on a nil
// *Handlers, so no field is read until a request arrives.
func (h *Handlers) repository() *repo.FxRateRepo { return h.repo }
