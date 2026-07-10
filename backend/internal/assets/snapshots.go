package assets

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/dateguard"
	"github.com/kerti/balances-v2/backend/internal/httperr"
	"github.com/kerti/balances-v2/backend/internal/repo"
)

type createSnapshotReq struct {
	// YearMonth accepts either "YYYY-MM" or "YYYY-MM-DD"; the handler
	// normalises to first-of-month per Q12a.
	YearMonth   string           `json:"year_month"  validate:"required"`
	Amount      *decimal.Decimal `json:"amount"      validate:"required"`
	Currency    string           `json:"currency"    validate:"required,iso4217"`
	AsOfDate    *string          `json:"as_of_date"` // ISO date YYYY-MM-DD
	Description *string          `json:"description"`
}

func (h *Handlers) handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	assetID, err := parseIDParam(r, "id")
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
	var asOf *time.Time
	if req.AsOfDate != nil && *req.AsOfDate != "" {
		t, err := time.Parse("2006-01-02", *req.AsOfDate)
		if err != nil {
			writeInvalidDate(w, "as_of_date")
			return
		}
		if dateguard.IsFuture(t, h.now()) {
			httperr.Write(w, http.StatusBadRequest, httperr.CodeSnapshotFutureDate, nil)
			return
		}
		asOf = &t
	}

	snap, err := h.repo.CreateAssetSnapshot(r.Context(), repo.CreateAssetSnapshotParams{
		AssetID:     assetID,
		YearMonth:   ym,
		Amount:      *req.Amount,
		Currency:    req.Currency,
		AsOfDate:    asOf,
		Description: req.Description,
	})
	if err != nil {
		httperr.WriteRepo(w, "create asset snapshot", err)
		return
	}
	writeJSON(w, http.StatusCreated, snap)
}

func (h *Handlers) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	assetID, err := parseIDParam(r, "id")
	if err != nil {
		writeInvalidID(w, "id")
		return
	}
	snaps, err := h.repo.ListAssetSnapshots(r.Context(), assetID)
	if err != nil {
		httperr.WriteRepo(w, "list asset snapshots", err)
		return
	}
	writeJSON(w, http.StatusOK, snaps)
}

type updateSnapshotReq struct {
	Amount      *decimal.Decimal `json:"amount"      validate:"required"`
	Currency    string           `json:"currency"    validate:"required,iso4217"`
	AsOfDate    *string          `json:"as_of_date"`
	Description *string          `json:"description"`
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

	var asOf *time.Time
	if req.AsOfDate != nil && *req.AsOfDate != "" {
		t, err := time.Parse("2006-01-02", *req.AsOfDate)
		if err != nil {
			writeInvalidDate(w, "as_of_date")
			return
		}
		if dateguard.IsFuture(t, h.now()) {
			httperr.Write(w, http.StatusBadRequest, httperr.CodeSnapshotFutureDate, nil)
			return
		}
		asOf = &t
	}

	snap, err := h.repo.UpdateAssetSnapshot(r.Context(), repo.UpdateAssetSnapshotParams{
		SnapshotID:  snapshotID,
		Amount:      *req.Amount,
		Currency:    req.Currency,
		AsOfDate:    asOf,
		Description: req.Description,
	})
	if err != nil {
		httperr.WriteRepo(w, "update asset snapshot", err)
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
	if err := h.repo.DeleteAssetSnapshot(r.Context(), snapshotID); err != nil {
		httperr.WriteRepo(w, "delete asset snapshot", err)
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
	// Force first-of-month per Q12a even if the caller sent a different day.
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC), nil
}
