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

// ----- bulk monthly-entry, accrued shape (ADR-0046, #424) --------------------
//
// The accrued twin of the qty×price handlers above, for the Bond/TimeDeposit
// branch of the Investment group. A row carries the total value (amount) and the
// accrued-interest component — both stored as given (quantity/price null), the
// accrued branch of investment_snapshot_shape. Unlike qty×price there is no
// server-side derivation: a bond's total value already *is* its snapshot amount.

// bulkInvestmentAccruedSnapshotRow is one dirty accrued position in a bulk
// monthly-entry batch (ADR-0046, #424). The row carries the two tab-stops the
// per-position accrued dialog takes: the total value and the accrued component.
type bulkInvestmentAccruedSnapshotRow struct {
	InvestmentID    string           `json:"investment_id"    validate:"required,uuid"`
	Amount          *decimal.Decimal `json:"amount"           validate:"required"`
	AccruedInterest *decimal.Decimal `json:"accrued_interest" validate:"required"`
	Currency        string           `json:"currency"         validate:"required,iso4217"`
}

// bulkInvestmentAccruedSnapshotReq is a whole accrued bulk monthly-entry save:
// one batch-level target month + as-of date, and the dirty rows only.
type bulkInvestmentAccruedSnapshotReq struct {
	YearMonth string  `json:"year_month" validate:"required"`
	AsOfDate  *string `json:"as_of_date"`
	// No `required` — an empty batch (nothing dirty) is a clean no-op, not an
	// error. Each present row is still validated via dive.
	Rows []bulkInvestmentAccruedSnapshotRow `json:"rows" validate:"dive"`
}

// investmentAccruedEntryRowResp is one investment in the accrued bulk
// monthly-entry list. PrefillAmount / PrefillAccruedInterest / CarriedFrom are
// null for an investment with no history at or before the month.
// CouponDisposition is null for a time deposit (no bond_details row); the client
// treats null as pays_out (accrued default 0).
type investmentAccruedEntryRowResp struct {
	InvestmentID           string  `json:"investment_id"`
	DisplayName            string  `json:"display_name"`
	Currency               string  `json:"currency"`
	Subtype                string  `json:"subtype"`
	OwnershipType          string  `json:"ownership_type"`
	SoleOwnerUserID        *string `json:"sole_owner_user_id"`
	CouponDisposition      *string `json:"coupon_disposition"`
	PrefillAmount          *string `json:"prefill_amount"`
	PrefillAccruedInterest *string `json:"prefill_accrued_interest"`
	CarriedFrom            *string `json:"carried_from"`
}

type investmentAccruedEntryListResp struct {
	YearMonth string                          `json:"year_month"`
	Rows      []investmentAccruedEntryRowResp `json:"rows"`
}

// handleInvestmentAccruedEntryList returns the accrued bulk monthly-entry list
// for a target month: eligible Bond/TimeDeposit with carry-forward prefill and
// coupon disposition (ADR-0046). The when-control defaults are composed
// client-side.
func (h *Handlers) handleInvestmentAccruedEntryList(w http.ResponseWriter, r *http.Request) {
	ym, err := parseYearMonth(r.URL.Query().Get("year_month"))
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidYearMonth, nil)
		return
	}
	if isFutureYearMonth(ym, h.now()) {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeFutureYearMonth, nil)
		return
	}

	rows, err := h.repo.ListInvestmentAccruedEntryRows(r.Context(), ym)
	if err != nil {
		httperr.WriteRepo(w, "accrued entry list", err)
		return
	}

	resp := investmentAccruedEntryListResp{YearMonth: ym.Format("2006-01"), Rows: make([]investmentAccruedEntryRowResp, len(rows))}
	for i, row := range rows {
		out := investmentAccruedEntryRowResp{
			InvestmentID:      row.InvestmentID.String(),
			DisplayName:       row.DisplayName,
			Currency:          row.Currency,
			Subtype:           row.Subtype,
			OwnershipType:     row.OwnershipType,
			CouponDisposition: row.CouponDisposition,
		}
		if row.SoleOwnerUserID != nil {
			s := row.SoleOwnerUserID.String()
			out.SoleOwnerUserID = &s
		}
		if row.PrefillAmount != nil {
			s := row.PrefillAmount.String()
			out.PrefillAmount = &s
		}
		if row.PrefillAccruedInterest != nil {
			s := row.PrefillAccruedInterest.String()
			out.PrefillAccruedInterest = &s
		}
		if row.CarriedFrom != nil {
			s := row.CarriedFrom.Format("2006-01")
			out.CarriedFrom = &s
		}
		resp.Rows[i] = out
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleBulkCreateInvestmentAccruedSnapshots writes an accrued bulk
// monthly-entry batch for the Investment group — atomically, dirty-rows-only,
// upserting on (investment_id, year_month) with the total value stored as
// `amount` and accrued_interest set (quantity/price null).
func (h *Handlers) handleBulkCreateInvestmentAccruedSnapshots(w http.ResponseWriter, r *http.Request) {
	var req bulkInvestmentAccruedSnapshotReq
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

	rows := make([]repo.BulkInvestmentAccruedSnapshotRow, len(req.Rows))
	for i, row := range req.Rows {
		id, err := uuid.Parse(row.InvestmentID)
		if err != nil {
			writeInvalidID(w, "investment_id")
			return
		}
		rows[i] = repo.BulkInvestmentAccruedSnapshotRow{
			InvestmentID:    id,
			Amount:          *row.Amount,
			AccruedInterest: *row.AccruedInterest,
			Currency:        row.Currency,
		}
	}

	written, rowErrs, err := h.repo.BulkUpsertInvestmentAccruedSnapshots(r.Context(), repo.BulkUpsertInvestmentAccruedSnapshotsParams{
		YearMonth: ym,
		AsOfDate:  asOf,
		Rows:      rows,
	})
	if err != nil {
		httperr.WriteRepo(w, "bulk accrued snapshots", err)
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
