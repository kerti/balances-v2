package investments

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/dateguard"
	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
)

// createSnapshotReq accepts all value-shape columns; the repo validates
// which combination is required based on the parent investment's subtype
// (stock/mutual_fund/gold → quantity+price_per_unit, bond/time_deposit →
// accrued_interest). Wrong combos return ErrInvalidSnapshotShape, which the
// handler maps to 400.
type createSnapshotReq struct {
	YearMonth       string           `json:"year_month"        validate:"required"`
	Amount          *decimal.Decimal `json:"amount"            validate:"required"`
	Currency        string           `json:"currency"          validate:"required,iso4217"`
	Quantity        *decimal.Decimal `json:"quantity"`
	PricePerUnit    *decimal.Decimal `json:"price_per_unit"`
	AccruedInterest *decimal.Decimal `json:"accrued_interest"`
	AsOfDate        *string          `json:"as_of_date"`
	Description     *string          `json:"description"`
}

type updateSnapshotReq struct {
	Amount          *decimal.Decimal `json:"amount"            validate:"required"`
	Currency        string           `json:"currency"          validate:"required,iso4217"`
	Quantity        *decimal.Decimal `json:"quantity"`
	PricePerUnit    *decimal.Decimal `json:"price_per_unit"`
	AccruedInterest *decimal.Decimal `json:"accrued_interest"`
	AsOfDate        *string          `json:"as_of_date"`
	Description     *string          `json:"description"`
}

func (h *Handlers) handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	investmentID, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}

	var req createSnapshotReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}

	ym, err := parseYearMonth(req.YearMonth)
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidYearMonth, nil)
		return
	}
	if dateguard.IsFutureMonth(ym, h.now()) {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeFutureYearMonth, nil)
		return
	}
	asOf, ok := parseOptionalDate(req.AsOfDate)
	if !ok {
		writeInvalidDate(w, "as_of_date")
		return
	}
	if asOf != nil && dateguard.IsFuture(*asOf, h.now()) {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeSnapshotFutureDate, nil)
		return
	}

	snap, err := h.repo.CreateInvestmentSnapshot(r.Context(), repo.CreateInvestmentSnapshotParams{
		InvestmentID:    investmentID,
		YearMonth:       ym,
		Amount:          *req.Amount,
		Currency:        req.Currency,
		Quantity:        req.Quantity,
		PricePerUnit:    req.PricePerUnit,
		AccruedInterest: req.AccruedInterest,
		AsOfDate:        asOf,
		Description:     req.Description,
	})
	if err != nil {
		httperr.WriteRepo(w, "create investment snapshot", err)
		return
	}
	writeJSON(w, http.StatusCreated, snap)
}

func (h *Handlers) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	investmentID, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	snaps, err := h.repo.ListInvestmentSnapshots(r.Context(), investmentID)
	if err != nil {
		httperr.WriteRepo(w, "list investment snapshots", err)
		return
	}
	writeJSON(w, http.StatusOK, snaps)
}

func (h *Handlers) handleUpdateSnapshot(w http.ResponseWriter, r *http.Request) {
	snapshotID, err := parseIDParam(r, "snapshotID")
	if err != nil {
		writeInvalidID(w, "snapshot_id")
		return
	}

	var req updateSnapshotReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidJSONBody, nil)
		return
	}
	if err := h.validate.Struct(&req); err != nil {
		httperr.WriteValidation(w, err)
		return
	}

	asOf, ok := parseOptionalDate(req.AsOfDate)
	if !ok {
		writeInvalidDate(w, "as_of_date")
		return
	}
	if asOf != nil && dateguard.IsFuture(*asOf, h.now()) {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeSnapshotFutureDate, nil)
		return
	}

	snap, err := h.repo.UpdateInvestmentSnapshot(r.Context(), repo.UpdateInvestmentSnapshotParams{
		SnapshotID:      snapshotID,
		Amount:          *req.Amount,
		Currency:        req.Currency,
		Quantity:        req.Quantity,
		PricePerUnit:    req.PricePerUnit,
		AccruedInterest: req.AccruedInterest,
		AsOfDate:        asOf,
		Description:     req.Description,
	})
	if err != nil {
		httperr.WriteRepo(w, "update investment snapshot", err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (h *Handlers) handleDeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	snapshotID, err := parseIDParam(r, "snapshotID")
	if err != nil {
		writeInvalidID(w, "snapshot_id")
		return
	}
	if err := h.repo.DeleteInvestmentSnapshot(r.Context(), snapshotID); err != nil {
		httperr.WriteRepo(w, "delete investment snapshot", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

// parseOptionalDate parses an optional ISO date string ("YYYY-MM-DD") into a
// *time.Time. nil-or-empty input yields (nil, true); an unparseable string
// yields (nil, false) so the caller can emit INVALID_DATE with its known
// field name rather than threading the field through here.
func parseOptionalDate(s *string) (*time.Time, bool) {
	if s == nil || *s == "" {
		return nil, true
	}
	t, err := time.Parse("2006-01-02", *s)
	if err != nil {
		return nil, false
	}
	return &t, true
}
