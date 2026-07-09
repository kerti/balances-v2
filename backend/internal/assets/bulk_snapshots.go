package assets

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
)

// bulkSnapshotRow is one dirty position value in a bulk monthly-entry batch.
type bulkSnapshotRow struct {
	AssetID  string           `json:"asset_id" validate:"required,uuid"`
	Amount   *decimal.Decimal `json:"amount"   validate:"required"`
	Currency string           `json:"currency" validate:"required,iso4217"`
}

// bulkSnapshotReq is a whole bulk monthly-entry save (ADR-0046): one batch-level
// target month + as-of date, and the dirty rows only.
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

// bulkRowError reports one rejected row, keyed by asset so the UI marks exactly
// that row (ADR-0046). Code mirrors the repo's rejection reason.
type bulkRowError struct {
	AssetID string `json:"asset_id"`
	Code    string `json:"code"`
}

type bulkSnapshotErrResp struct {
	Errors []bulkRowError `json:"errors"`
}

// entryRowResp is one asset in the bulk monthly-entry list. PrefillAmount /
// CarriedFrom are null for an asset with no history at or before the month.
type entryRowResp struct {
	AssetID       string  `json:"asset_id"`
	DisplayName   string  `json:"display_name"`
	Currency      string  `json:"currency"`
	PrefillAmount *string `json:"prefill_amount"`
	CarriedFrom   *string `json:"carried_from"`
}

type entryListResp struct {
	YearMonth string         `json:"year_month"`
	Rows      []entryRowResp `json:"rows"`
}

// handleAssetEntryList returns the bulk monthly-entry list for a target month:
// eligible assets with carry-forward prefill (ADR-0046). The when-control
// defaults (target month, as-of date from carryover_date_mode) are composed
// client-side, which already owns that date machinery.
func (h *Handlers) handleAssetEntryList(w http.ResponseWriter, r *http.Request) {
	ym, err := parseYearMonth(r.URL.Query().Get("year_month"))
	if err != nil {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidYearMonth, nil)
		return
	}
	if isFutureYearMonth(ym, h.now()) {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeFutureYearMonth, nil)
		return
	}

	rows, err := h.repo.ListAssetEntryRows(r.Context(), ym)
	if err != nil {
		httperr.WriteRepo(w, "asset entry list", err)
		return
	}

	resp := entryListResp{YearMonth: ym.Format("2006-01"), Rows: make([]entryRowResp, len(rows))}
	for i, row := range rows {
		out := entryRowResp{
			AssetID:     row.AssetID.String(),
			DisplayName: row.DisplayName,
			Currency:    row.Currency,
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

// handleBulkCreateSnapshots writes a bulk monthly-entry batch for the Asset
// group — atomically, dirty-rows-only, upserting on (asset_id, year_month).
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

	rows := make([]repo.BulkAssetSnapshotRow, len(req.Rows))
	for i, row := range req.Rows {
		id, err := uuid.Parse(row.AssetID)
		if err != nil {
			writeInvalidID(w, "asset_id")
			return
		}
		rows[i] = repo.BulkAssetSnapshotRow{
			AssetID:  id,
			Amount:   *row.Amount,
			Currency: row.Currency,
		}
	}

	written, rowErrs, err := h.repo.BulkUpsertAssetSnapshots(r.Context(), repo.BulkUpsertAssetSnapshotsParams{
		YearMonth: ym,
		AsOfDate:  asOf,
		Rows:      rows,
	})
	if err != nil {
		httperr.WriteRepo(w, "bulk asset snapshots", err)
		return
	}
	if len(rowErrs) > 0 {
		resp := bulkSnapshotErrResp{Errors: make([]bulkRowError, len(rowErrs))}
		for i, e := range rowErrs {
			resp.Errors[i] = bulkRowError{AssetID: e.AssetID.String(), Code: e.Reason}
		}
		writeJSON(w, http.StatusUnprocessableEntity, resp)
		return
	}
	writeJSON(w, http.StatusOK, bulkSnapshotResp{Written: written})
}
