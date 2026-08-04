// Package income exposes HTTP handlers for the Income flow-event entity.
// Income has no subtype, no extension table, no snapshots — each row is a
// one-shot event. Mounted under /api/income (singular collection endpoint;
// "income" is a mass noun in English — see docs/agents/conventions.md).
package income

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
	repo     *repo.IncomeRepo
	validate *validator.Validate
}

func New(r *repo.IncomeRepo) *Handlers {
	return &Handlers{
		repo:     r,
		validate: httperr.NewValidator(),
	}
}

func (h *Handlers) Mount(r chi.Router) {
	r.Route("/income", func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/", h.handleCreate)
		r.Get("/", h.handleList)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.handleGet)
			r.Patch("/", h.handleUpdate)
			r.Delete("/", h.handleDelete)
		})
	})
}

// ----- requests -----------------------------------------------------------
//
// Category enum (closed set, ADR-0008; DB CHECK in the 00001 baseline, extended
// with `pension` and `interest` in 00012 per ADR-0048) is mirrored in the
// validator tag below.
// Update both the migration CHECK and this tag if it changes.

type createReq struct {
	Date            string           `json:"date"               validate:"required"`
	Amount          *decimal.Decimal `json:"amount"             validate:"required"`
	Currency        string           `json:"currency"           validate:"required,iso4217"`
	Category        string           `json:"category"           validate:"required,oneof=salary business_income rental_income pension interest gift tax_refund insurance_payout other"`
	Description     *string          `json:"description"`
	OwnershipType   string           `json:"ownership_type"     validate:"required,oneof=sole joint"`
	SoleOwnerUserID *uuid.UUID       `json:"sole_owner_user_id" validate:"required_if=OwnershipType sole"`
	Regularity      string           `json:"regularity"         validate:"required,oneof=routine incidental"`
}

type updateReq struct {
	Date            string           `json:"date"               validate:"required"`
	Amount          *decimal.Decimal `json:"amount"             validate:"required"`
	Currency        string           `json:"currency"           validate:"required,iso4217"`
	Category        string           `json:"category"           validate:"required,oneof=salary business_income rental_income pension interest gift tax_refund insurance_payout other"`
	Description     *string          `json:"description"`
	OwnershipType   string           `json:"ownership_type"     validate:"required,oneof=sole joint"`
	SoleOwnerUserID *uuid.UUID       `json:"sole_owner_user_id" validate:"required_if=OwnershipType sole"`
	Regularity      string           `json:"regularity"         validate:"required,oneof=routine incidental"`
}

// ----- handlers -----------------------------------------------------------

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
	date, ok := parseDate(req.Date)
	if !ok {
		writeInvalidDate(w, "date")
		return
	}
	if !req.Amount.IsPositive() {
		writeAmountMustBePositive(w)
		return
	}

	row, err := h.repo.CreateIncome(r.Context(), repo.CreateIncomeParams{
		Date:            date,
		Amount:          *req.Amount,
		Currency:        req.Currency,
		Category:        req.Category,
		Description:     req.Description,
		OwnershipType:   req.OwnershipType,
		SoleOwnerUserID: req.SoleOwnerUserID,
		Regularity:      req.Regularity,
	})
	if err != nil {
		httperr.WriteRepo(w, "create income", err)
		return
	}
	writeJSON(w, http.StatusCreated, row)
}

func (h *Handlers) handleList(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListIncome(r.Context())
	if err != nil {
		httperr.WriteRepo(w, "list income", err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *Handlers) handleGet(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	row, err := h.repo.GetIncome(r.Context(), id)
	if err != nil {
		httperr.WriteRepo(w, "get income", err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *Handlers) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
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
	date, ok := parseDate(req.Date)
	if !ok {
		writeInvalidDate(w, "date")
		return
	}
	if !req.Amount.IsPositive() {
		writeAmountMustBePositive(w)
		return
	}

	row, err := h.repo.UpdateIncome(r.Context(), id, repo.UpdateIncomeParams{
		Date:            date,
		Amount:          *req.Amount,
		Currency:        req.Currency,
		Category:        req.Category,
		Description:     req.Description,
		OwnershipType:   req.OwnershipType,
		SoleOwnerUserID: req.SoleOwnerUserID,
		Regularity:      req.Regularity,
	})
	if err != nil {
		httperr.WriteRepo(w, "update income", err)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (h *Handlers) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	if err := h.repo.DeleteIncome(r.Context(), id); err != nil {
		httperr.WriteRepo(w, "delete income", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ----- helpers ------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body != nil {
		_ = json.NewEncoder(w).Encode(body)
	}
}

// writeInvalidID / writeInvalidDate are small shims around httperr.Write so
// the "invalid path-param" and "unparseable date body field" call sites
// stay at one level of abstraction. writeAmountMustBePositive emits a
// VALIDATION envelope mirroring the validator's `gt` tag — the inline
// `IsPositive()` check is logically the same rule the validator would
// fire if decimal.Decimal supported tag-based comparison.
func writeInvalidID(w http.ResponseWriter, field string) {
	httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidID, map[string]any{"field": field})
}

func writeInvalidDate(w http.ResponseWriter, field string) {
	httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidDate, map[string]any{"field": field})
}

func writeAmountMustBePositive(w http.ResponseWriter) {
	httperr.Write(w, http.StatusBadRequest, httperr.CodeValidation, map[string]any{
		"field": "amount",
		"rule":  "gt",
	})
}

func parseIDParam(r *http.Request, name string) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, name))
}

func parseDate(s string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
