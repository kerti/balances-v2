package receivables

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
)

// Bulk monthly-entry for the Receivable group (ADR-0046). Structural twin of the
// Asset handler — same amount-only shape, dirty-only atomic save, per-row 422.
// Receivables are a flat group with no subtype, so the entry rows carry a
// constant empty `subtype`; the FE renders one ungrouped list.

type bulkSnapshotRow struct {
	ReceivableID string           `json:"receivable_id" validate:"required,uuid"`
	Amount       *decimal.Decimal `json:"amount"        validate:"required"`
	Currency     string           `json:"currency"      validate:"required,iso4217"`
}

type bulkSnapshotReq struct {
	YearMonth string  `json:"year_month" validate:"required"`
	AsOfDate  *string `json:"as_of_date"`
	// No `required` — an empty batch (nothing dirty) is a clean no-op, not an
	// error; the FE may fire Save with no changes. Each present row is still
	// validated via dive.
	Rows []bulkSnapshotRow `json:"rows" validate:"dive"`
}

type bulkSnapshotResp struct {
	Written int `json:"written"`
}

// bulkRowError reports one rejected row, keyed by receivable so the UI marks
// exactly that row (ADR-0046).
type bulkRowError struct {
	ReceivableID string `json:"receivable_id"`
	Code         string `json:"code"`
}

type bulkSnapshotErrResp struct {
	Errors []bulkRowError `json:"errors"`
}

// entryRowResp is one receivable in the bulk monthly-entry list. PrefillAmount /
// CarriedFrom are null for a receivable with no history at or before the month.
// Subtype is always empty (receivables are flat) but kept in the shape so the
// entry-list DTO is uniform across the amount-only groups.
type entryRowResp struct {
	ReceivableID    string  `json:"receivable_id"`
	DisplayName     string  `json:"display_name"`
	Currency        string  `json:"currency"`
	Subtype         string  `json:"subtype"`
	OwnershipType   string  `json:"ownership_type"`
	SoleOwnerUserID *string `json:"sole_owner_user_id"`
	PrefillAmount   *string `json:"prefill_amount"`
	CarriedFrom     *string `json:"carried_from"`
}

type entryListResp struct {
	YearMonth string         `json:"year_month"`
	Rows      []entryRowResp `json:"rows"`
}

// handleEntryList returns the bulk monthly-entry list for a target month:
// eligible receivables with carry-forward prefill (ADR-0046).
func (h *Handlers) handleEntryList(w http.ResponseWriter, r *http.Request) {
	ym, err := parseYearMonth(r.URL.Query().Get("year_month"))
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidYearMonth, nil)
		return
	}
	if isFutureYearMonth(ym, h.now()) {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeFutureYearMonth, nil)
		return
	}

	rows, err := h.repo.ListReceivableEntryRows(r.Context(), ym)
	if err != nil {
		httperr.WriteRepo(w, "receivable entry list", err)
		return
	}

	resp := entryListResp{YearMonth: ym.Format("2006-01"), Rows: make([]entryRowResp, len(rows))}
	for i, row := range rows {
		out := entryRowResp{
			ReceivableID:  row.ReceivableID.String(),
			DisplayName:   row.DisplayName,
			Currency:      row.Currency,
			OwnershipType: row.OwnershipType,
		}
		if row.SoleOwnerUserID != nil {
			s := row.SoleOwnerUserID.String()
			out.SoleOwnerUserID = &s
		}
		if row.PrefillAmount != nil {
			s := row.PrefillAmount.String()
			out.PrefillAmount = &s
		}
		if row.CarriedFrom != nil {
			s := row.CarriedFrom.Format("2006-01")
			out.CarriedFrom = &s
		}
		resp.Rows[i] = out
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleBulkCreateSnapshots writes a bulk monthly-entry batch for the
// Receivable group — atomically, dirty-rows-only, upserting on
// (receivable_id, year_month).
func (h *Handlers) handleBulkCreateSnapshots(w http.ResponseWriter, r *http.Request) {
	var req bulkSnapshotReq
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

	rows := make([]repo.BulkReceivableSnapshotRow, len(req.Rows))
	for i, row := range req.Rows {
		id, err := uuid.Parse(row.ReceivableID)
		if err != nil {
			writeInvalidID(w, "receivable_id")
			return
		}
		rows[i] = repo.BulkReceivableSnapshotRow{
			ReceivableID: id,
			Amount:       *row.Amount,
			Currency:     row.Currency,
		}
	}

	written, rowErrs, err := h.repo.BulkUpsertReceivableSnapshots(r.Context(), repo.BulkUpsertReceivableSnapshotsParams{
		YearMonth: ym,
		AsOfDate:  asOf,
		Rows:      rows,
	})
	if err != nil {
		httperr.WriteRepo(w, "bulk receivable snapshots", err)
		return
	}
	if len(rowErrs) > 0 {
		resp := bulkSnapshotErrResp{Errors: make([]bulkRowError, len(rowErrs))}
		for i, e := range rowErrs {
			resp.Errors[i] = bulkRowError{ReceivableID: e.PositionID.String(), Code: e.Reason}
		}
		writeJSON(w, http.StatusUnprocessableEntity, resp)
		return
	}
	writeJSON(w, http.StatusOK, bulkSnapshotResp{Written: written})
}
