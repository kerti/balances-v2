package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// BulkAssetSnapshotRow is one position's value in a bulk monthly-entry batch
// (ADR-0046). The caller sends only the rows the user changed (dirty-only); the
// target month and as-of date are batch-level, not per-row.
type BulkAssetSnapshotRow struct {
	AssetID  uuid.UUID
	Amount   decimal.Decimal
	Currency string
}

// BulkUpsertAssetSnapshotsParams carries a whole batch: one target month, one
// as-of date, N dirty rows.
type BulkUpsertAssetSnapshotsParams struct {
	YearMonth time.Time
	AsOfDate  *time.Time
	Rows      []BulkAssetSnapshotRow
}

// Reasons a bulk-entry row is rejected before any write.
const (
	BulkRowIneligible = "ineligible" // asset not owned, deleted, or not eligible for the target month
)

// BulkSnapshotRowError identifies one row that failed pre-write validation,
// keyed by asset so the UI can mark exactly that row (ADR-0046).
type BulkSnapshotRowError struct {
	AssetID uuid.UUID
	Reason  string
}

// AssetEntryRow is one row of the bulk monthly-entry list (ADR-0046): an
// eligible asset with its carry-forward prefill. PrefillAmount/CarriedFrom are
// nil for an asset with no snapshot at or before the target month.
type AssetEntryRow struct {
	AssetID       uuid.UUID
	DisplayName   string
	Currency      string
	PrefillAmount *decimal.Decimal
	CarriedFrom   *time.Time
}

// ListAssetEntryRows returns the bulk monthly-entry list for a target month:
// every eligible asset with its most-recent snapshot at or before that month as
// the carry-forward prefill.
func (r *AssetRepo) ListAssetEntryRows(ctx context.Context, yearMonth time.Time) ([]AssetEntryRow, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	assets, err := r.q.ListEligibleAssetsForMonth(ctx, db.ListEligibleAssetsForMonthParams{
		HouseholdID: hid,
		YearMonth:   yearMonth,
	})
	if err != nil {
		return nil, fmt.Errorf("asset entry rows: list eligible: %w", err)
	}
	if len(assets) == 0 {
		return nil, nil
	}

	ids := make([]uuid.UUID, len(assets))
	for i, a := range assets {
		ids[i] = a.ID
	}
	latest, err := r.q.ListLatestSnapshotsByAssetIDsAsOfMonth(ctx, db.ListLatestSnapshotsByAssetIDsAsOfMonthParams{
		AssetIds:  ids,
		YearMonth: yearMonth,
	})
	if err != nil {
		return nil, fmt.Errorf("asset entry rows: list prefill: %w", err)
	}
	prefill := make(map[uuid.UUID]db.ListLatestSnapshotsByAssetIDsAsOfMonthRow, len(latest))
	for _, s := range latest {
		prefill[s.AssetID] = s
	}

	rows := make([]AssetEntryRow, len(assets))
	for i, a := range assets {
		row := AssetEntryRow{
			AssetID:     a.ID,
			DisplayName: a.DisplayName,
			Currency:    a.NativeCurrency,
		}
		if s, ok := prefill[a.ID]; ok {
			amt := s.Amount
			ym := s.YearMonth
			row.PrefillAmount = &amt
			row.CarriedFrom = &ym
		}
		rows[i] = row
	}
	return rows, nil
}

// BulkUpsertAssetSnapshots writes a bulk monthly-entry batch in a single
// transaction — all-or-nothing (ADR-0046). Every row is validated for
// month-aware eligibility first; if any row is ineligible the batch writes
// nothing and the offending rows are returned so the caller can surface a
// per-row error. Otherwise each row upserts on (asset_id, year_month), so
// re-entering a month edits its snapshot rather than inserting a duplicate.
// Returns the number of rows written and any per-row rejections.
func (r *AssetRepo) BulkUpsertAssetSnapshots(ctx context.Context, p BulkUpsertAssetSnapshotsParams) (int, []BulkSnapshotRowError, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return 0, nil, err
	}
	if len(p.Rows) == 0 {
		return 0, nil, nil
	}

	eligible, err := r.q.ListEligibleAssetIDsForMonth(ctx, db.ListEligibleAssetIDsForMonthParams{
		HouseholdID: hid,
		YearMonth:   p.YearMonth,
	})
	if err != nil {
		return 0, nil, fmt.Errorf("bulk asset snapshots: list eligible: %w", err)
	}
	eligibleSet := make(map[uuid.UUID]struct{}, len(eligible))
	for _, id := range eligible {
		eligibleSet[id] = struct{}{}
	}

	var rowErrs []BulkSnapshotRowError
	for _, row := range p.Rows {
		if _, ok := eligibleSet[row.AssetID]; !ok {
			rowErrs = append(rowErrs, BulkSnapshotRowError{AssetID: row.AssetID, Reason: BulkRowIneligible})
		}
	}
	if len(rowErrs) > 0 {
		return 0, rowErrs, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, nil, fmt.Errorf("bulk asset snapshots: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	for _, row := range p.Rows {
		if _, err := qtx.UpsertAssetSnapshot(ctx, db.UpsertAssetSnapshotParams{
			ID:          row.AssetID,
			YearMonth:   p.YearMonth,
			Amount:      row.Amount,
			Currency:    row.Currency,
			AsOfDate:    p.AsOfDate,
			Description: nil,
			CreatedBy:   &user,
			HouseholdID: hid,
		}); err != nil {
			return 0, nil, fmt.Errorf("bulk asset snapshots: upsert %s: %w", row.AssetID, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, nil, fmt.Errorf("bulk asset snapshots: commit: %w", err)
	}
	return len(p.Rows), nil, nil
}
