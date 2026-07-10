package investments

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
)

// bulkInvestmentSnapshotRow is one dirty qty×price position in a bulk
// monthly-entry batch (ADR-0046, #423). The row carries the two tab-stops
// (quantity, price per unit); the stored total is derived server-side as
// quantity × price_per_unit, so no client-computed amount is trusted.
type bulkInvestmentSnapshotRow struct {
	InvestmentID string           `json:"investment_id"  validate:"required,uuid"`
	Quantity     *decimal.Decimal `json:"quantity"       validate:"required"`
	PricePerUnit *decimal.Decimal `json:"price_per_unit" validate:"required"`
	Currency     string           `json:"currency"       validate:"required,iso4217"`
}

// bulkInvestmentSnapshotReq is a whole qty×price bulk monthly-entry save: one
// batch-level target month + as-of date, and the dirty rows only.
type bulkInvestmentSnapshotReq struct {
	YearMonth string  `json:"year_month" validate:"required"`
	AsOfDate  *string `json:"as_of_date"`
	// No `required` — an empty batch (nothing dirty) is a clean no-op, not an
	// error. Each present row is still validated via dive.
	Rows []bulkInvestmentSnapshotRow `json:"rows" validate:"dive"`
}

type bulkInvestmentSnapshotResp struct {
	Written int `json:"written"`
}

// bulkInvestmentRowError reports one rejected row, keyed by investment so the UI
// marks exactly that row (ADR-0046).
type bulkInvestmentRowError struct {
	InvestmentID string `json:"investment_id"`
	Code         string `json:"code"`
}

type bulkInvestmentSnapshotErrResp struct {
	Errors []bulkInvestmentRowError `json:"errors"`
}

// investmentEntryRowResp is one investment in the qty×price bulk monthly-entry
// list. PrefillQuantity / PrefillPrice / CarriedFrom are null for an investment
// with no history at or before the month.
type investmentEntryRowResp struct {
	InvestmentID    string  `json:"investment_id"`
	DisplayName     string  `json:"display_name"`
	Currency        string  `json:"currency"`
	Subtype         string  `json:"subtype"`
	OwnershipType   string  `json:"ownership_type"`
	SoleOwnerUserID *string `json:"sole_owner_user_id"`
	PrefillQuantity *string `json:"prefill_quantity"`
	PrefillPrice    *string `json:"prefill_price"`
	CarriedFrom     *string `json:"carried_from"`
}

type investmentEntryListResp struct {
	YearMonth string                   `json:"year_month"`
	Rows      []investmentEntryRowResp `json:"rows"`
}

// handleInvestmentEntryList returns the qty×price bulk monthly-entry list for a
// target month: eligible Stock/MutualFund/Gold with carry-forward prefill
// (ADR-0046). The when-control defaults are composed client-side.
func (h *Handlers) handleInvestmentEntryList(w http.ResponseWriter, r *http.Request) {
	ym, err := parseYearMonth(r.URL.Query().Get("year_month"))
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidYearMonth, nil)
		return
	}
	if isFutureYearMonth(ym, h.now()) {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeFutureYearMonth, nil)
		return
	}

	rows, err := h.repo.ListInvestmentEntryRows(r.Context(), ym)
	if err != nil {
		httperr.WriteRepo(w, "investment entry list", err)
		return
	}

	resp := investmentEntryListResp{YearMonth: ym.Format("2006-01"), Rows: make([]investmentEntryRowResp, len(rows))}
	for i, row := range rows {
		out := investmentEntryRowResp{
			InvestmentID:  row.InvestmentID.String(),
			DisplayName:   row.DisplayName,
			Currency:      row.Currency,
			Subtype:       row.Subtype,
			OwnershipType: row.OwnershipType,
		}
		if row.SoleOwnerUserID != nil {
			s := row.SoleOwnerUserID.String()
			out.SoleOwnerUserID = &s
		}
		if row.PrefillQuantity != nil {
			s := row.PrefillQuantity.String()
			out.PrefillQuantity = &s
		}
		if row.PrefillPrice != nil {
			s := row.PrefillPrice.String()
			out.PrefillPrice = &s
		}
		if row.CarriedFrom != nil {
			s := row.CarriedFrom.Format("2006-01")
			out.CarriedFrom = &s
		}
		resp.Rows[i] = out
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleBulkCreateInvestmentSnapshots writes a qty×price bulk monthly-entry
// batch for the Investment group — atomically, dirty-rows-only, upserting on
// (investment_id, year_month) with amount derived as quantity × price_per_unit.
func (h *Handlers) handleBulkCreateInvestmentSnapshots(w http.ResponseWriter, r *http.Request) {
	var req bulkInvestmentSnapshotReq
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
	if isFutureYearMonth(ym, h.now()) {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeFutureYearMonth, nil)
		return
	}

	var asOf *time.Time
	if req.AsOfDate != nil && *req.AsOfDate != "" {
		t, err := time.Parse("2006-01-02", *req.AsOfDate)
		if err != nil {
			writeInvalidDate(w, "as_of_date")
			return
		}
		if isFutureDate(t, h.now()) {
			httperr.Write(w, http.StatusBadRequest, httperr.CodeSnapshotFutureDate, nil)
			return
		}
		asOf = &t
	}

	rows := make([]repo.BulkInvestmentSnapshotRow, len(req.Rows))
	for i, row := range req.Rows {
		id, err := uuid.Parse(row.InvestmentID)
		if err != nil {
			writeInvalidID(w, "investment_id")
			return
		}
		rows[i] = repo.BulkInvestmentSnapshotRow{
			InvestmentID: id,
			Quantity:     *row.Quantity,
			PricePerUnit: *row.PricePerUnit,
			Currency:     row.Currency,
		}
	}

	written, rowErrs, err := h.repo.BulkUpsertInvestmentSnapshots(r.Context(), repo.BulkUpsertInvestmentSnapshotsParams{
		YearMonth: ym,
		AsOfDate:  asOf,
		Rows:      rows,
	})
	if err != nil {
		httperr.WriteRepo(w, "bulk investment snapshots", err)
		return
	}
	if len(rowErrs) > 0 {
		resp := bulkInvestmentSnapshotErrResp{Errors: make([]bulkInvestmentRowError, len(rowErrs))}
		for i, e := range rowErrs {
			resp.Errors[i] = bulkInvestmentRowError{InvestmentID: e.PositionID.String(), Code: e.Reason}
		}
		writeJSON(w, http.StatusUnprocessableEntity, resp)
		return
	}
	writeJSON(w, http.StatusOK, bulkInvestmentSnapshotResp{Written: written})
}
