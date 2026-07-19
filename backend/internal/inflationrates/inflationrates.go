// Package inflationrates exposes HTTP handlers for the manual monthly
// inflation-rate table (ADR-0048). Mounted under /api/inflation-rates. Each rate
// is an annualized (YoY) percentage for its month and feeds the Fund Resilience
// projection (latest <= month, trailing-12 average). Unlike FX rates it is not
// gated by the multi-currency toggle and may be negative (deflation).
package inflationrates

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
	"github.com/kerti/balances-v2/backend/internal/repo"
)

type Handlers struct {
	repo     *repo.InflationRateRepo
	validate *validator.Validate
}

func New(r *repo.InflationRateRepo) *Handlers {
	return &Handlers{repo: r, validate: httperr.NewValidator()}
}

func (h *Handlers) Mount(r chi.Router) {
	r.Route("/inflation-rates", func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/", h.handleCreate)
		r.Get("/", h.handleList)
		r.Route("/{id}", func(r chi.Router) {
			r.Patch("/", h.handleUpdate)
			r.Delete("/", h.handleDelete)
		})
	})
}

type createReq struct {
	YearMonth string           `json:"year_month" validate:"required"`
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
	if !writeRateInBounds(w, *req.Rate) {
		return
	}
	row, err := h.repo.CreateInflationRate(r.Context(), repo.CreateInflationRateParams{
		YearMonth: ym, Rate: *req.Rate,
	})
	if err != nil {
		httperr.WriteRepo(w, "create inflation rate", err)
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

func (h *Handlers) handleList(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListInflationRates(r.Context())
	if err != nil {
		httperr.WriteRepo(w, "list inflation rates", err)
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
	if !writeRateInBounds(w, *req.Rate) {
		return
	}
	row, err := h.repo.UpdateInflationRate(r.Context(), id, *req.Rate)
	if err != nil {
		httperr.WriteRepo(w, "update inflation rate", err)
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
	if err := h.repo.DeleteInflationRate(r.Context(), id); err != nil {
		httperr.WriteRepo(w, "delete inflation rate", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ----- helpers ------------------------------------------------------------

// writeRateInBounds enforces the same (-100, 1000] bound as
// assumed_annual_inflation (auth handler): deflation is valid, but a rate
// <= -100 makes the annual→monthly conversion (1+a/100)^(1/12) take a
// non-positive base — NaN, which the resilience simulation reads as an
// indefinite runway. Writes the validation error and returns false when out of
// bounds.
func writeRateInBounds(w http.ResponseWriter, rate decimal.Decimal) bool {
	if rate.LessThanOrEqual(decimal.NewFromInt(-100)) || rate.GreaterThan(decimal.NewFromInt(1000)) {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation, map[string]any{
			"field": "rate",
			"rule":  "range",
		})
		return false
	}
	return true
}

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
