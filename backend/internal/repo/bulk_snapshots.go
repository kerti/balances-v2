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
// keyed by position so the UI can mark exactly that row (ADR-0046). Shared
// across the amount-only groups (Asset/Liability/Receivable) — PositionID is
// the asset/liability/receivable id depending on which bulk save produced it.
type BulkSnapshotRowError struct {
	PositionID uuid.UUID
	Reason     string
}

// AssetEntryRow is one row of the bulk monthly-entry list (ADR-0046): an
// eligible asset with its carry-forward prefill. PrefillAmount/CarriedFrom are
// nil for an asset with no snapshot at or before the target month.
type AssetEntryRow struct {
	AssetID         uuid.UUID
	DisplayName     string
	Currency        string
	Subtype         string
	OwnershipType   string
	SoleOwnerUserID *uuid.UUID
	PrefillAmount   *decimal.Decimal
	CarriedFrom     *time.Time
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
			AssetID:         a.ID,
			DisplayName:     a.DisplayName,
			Currency:        a.NativeCurrency,
			Subtype:         a.Subtype,
			OwnershipType:   a.OwnershipType,
			SoleOwnerUserID: a.SoleOwnerUserID,
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
// per-row error. Otherwise each row is written through the replace path (ADR-0058 §1): a
// month with no live snapshot is inserted, one with different values has its
// live row archived and the new values inserted, and one with identical values
// is skipped. Returns the written/replaced/unchanged tally and any per-row
// rejections.
func (r *AssetRepo) BulkUpsertAssetSnapshots(ctx context.Context, p BulkUpsertAssetSnapshotsParams) (BulkResult, []BulkSnapshotRowError, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return BulkResult{}, nil, err
	}
	if len(p.Rows) == 0 {
		return BulkResult{}, nil, nil
	}

	eligible, err := r.q.ListEligibleAssetIDsForMonth(ctx, db.ListEligibleAssetIDsForMonthParams{
		HouseholdID: hid,
		YearMonth:   p.YearMonth,
	})
	if err != nil {
		return BulkResult{}, nil, fmt.Errorf("bulk asset snapshots: list eligible: %w", err)
	}
	eligibleSet := make(map[uuid.UUID]struct{}, len(eligible))
	for _, id := range eligible {
		eligibleSet[id] = struct{}{}
	}

	var rowErrs []BulkSnapshotRowError
	for _, row := range p.Rows {
		if _, ok := eligibleSet[row.AssetID]; !ok {
			rowErrs = append(rowErrs, BulkSnapshotRowError{PositionID: row.AssetID, Reason: BulkRowIneligible})
		}
	}
	if len(rowErrs) > 0 {
		return BulkResult{}, rowErrs, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return BulkResult{}, nil, fmt.Errorf("bulk asset snapshots: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	var res BulkResult
	for _, row := range p.Rows {
		_, _, outcome, err := replaceAssetSnapshot(ctx, qtx, user, db.CreateAssetSnapshotParams{
			ID:          row.AssetID,
			YearMonth:   p.YearMonth,
			Amount:      row.Amount,
			Currency:    row.Currency,
			AsOfDate:    p.AsOfDate,
			Description: nil,
			CreatedBy:   &user,
			HouseholdID: hid,
		})
		if err != nil {
			return BulkResult{}, nil, err
		}
		res.add(outcome)
	}

	if err := tx.Commit(ctx); err != nil {
		return BulkResult{}, nil, fmt.Errorf("bulk asset snapshots: commit: %w", err)
	}
	return res, nil, nil
}

// ----- Liability bulk monthly-entry (ADR-0046) -----------------------------
//
// Structural twin of the Asset methods above, against the liabilities /
// liability_snapshots tables. Kept as a parallel implementation rather than a
// generic one because the sqlc-generated queries are per-table (distinct
// param/row types) and the repo package already duplicates asset/liability/
// receivable this way.

// BulkLiabilitySnapshotRow is one liability's value in a bulk monthly-entry
// batch — dirty-only; the target month and as-of date are batch-level.
type BulkLiabilitySnapshotRow struct {
	LiabilityID uuid.UUID
	Amount      decimal.Decimal
	Currency    string
}

// BulkUpsertLiabilitySnapshotsParams carries a whole batch: one target month,
// one as-of date, N dirty rows.
type BulkUpsertLiabilitySnapshotsParams struct {
	YearMonth time.Time
	AsOfDate  *time.Time
	Rows      []BulkLiabilitySnapshotRow
}

// LiabilityEntryRow is one row of the bulk monthly-entry list: an eligible
// liability with its carry-forward prefill. PrefillAmount/CarriedFrom are nil
// for a liability with no snapshot at or before the target month.
type LiabilityEntryRow struct {
	LiabilityID     uuid.UUID
	DisplayName     string
	Currency        string
	Subtype         string
	OwnershipType   string
	SoleOwnerUserID *uuid.UUID
	PrefillAmount   *decimal.Decimal
	CarriedFrom     *time.Time
}

// ListLiabilityEntryRows returns the bulk monthly-entry list for a target
// month: every eligible liability with its most-recent snapshot at or before
// that month as the carry-forward prefill.
func (r *LiabilityRepo) ListLiabilityEntryRows(ctx context.Context, yearMonth time.Time) ([]LiabilityEntryRow, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	liabilities, err := r.q.ListEligibleLiabilitiesForMonth(ctx, db.ListEligibleLiabilitiesForMonthParams{
		HouseholdID: hid,
		YearMonth:   yearMonth,
	})
	if err != nil {
		return nil, fmt.Errorf("liability entry rows: list eligible: %w", err)
	}
	if len(liabilities) == 0 {
		return nil, nil
	}

	ids := make([]uuid.UUID, len(liabilities))
	for i, l := range liabilities {
		ids[i] = l.ID
	}
	latest, err := r.q.ListLatestSnapshotsByLiabilityIDsAsOfMonth(ctx, db.ListLatestSnapshotsByLiabilityIDsAsOfMonthParams{
		LiabilityIds: ids,
		YearMonth:    yearMonth,
	})
	if err != nil {
		return nil, fmt.Errorf("liability entry rows: list prefill: %w", err)
	}
	prefill := make(map[uuid.UUID]db.ListLatestSnapshotsByLiabilityIDsAsOfMonthRow, len(latest))
	for _, s := range latest {
		prefill[s.LiabilityID] = s
	}

	rows := make([]LiabilityEntryRow, len(liabilities))
	for i, l := range liabilities {
		row := LiabilityEntryRow{
			LiabilityID:     l.ID,
			DisplayName:     l.DisplayName,
			Currency:        l.NativeCurrency,
			Subtype:         l.Subtype,
			OwnershipType:   l.OwnershipType,
			SoleOwnerUserID: l.SoleOwnerUserID,
		}
		if s, ok := prefill[l.ID]; ok {
			amt := s.Amount
			ym := s.YearMonth
			row.PrefillAmount = &amt
			row.CarriedFrom = &ym
		}
		rows[i] = row
	}
	return rows, nil
}

// BulkUpsertLiabilitySnapshots writes a bulk monthly-entry batch in a single
// transaction — all-or-nothing (ADR-0046). Every row is validated for
// month-aware eligibility first; if any row is ineligible the batch writes
// nothing and the offending rows are returned. Otherwise each row is
// written through the replace path (ADR-0058 §1), as for assets. Returns the
// written/replaced/unchanged tally and any per-row rejections.
func (r *LiabilityRepo) BulkUpsertLiabilitySnapshots(ctx context.Context, p BulkUpsertLiabilitySnapshotsParams) (BulkResult, []BulkSnapshotRowError, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return BulkResult{}, nil, err
	}
	if len(p.Rows) == 0 {
		return BulkResult{}, nil, nil
	}

	eligible, err := r.q.ListEligibleLiabilityIDsForMonth(ctx, db.ListEligibleLiabilityIDsForMonthParams{
		HouseholdID: hid,
		YearMonth:   p.YearMonth,
	})
	if err != nil {
		return BulkResult{}, nil, fmt.Errorf("bulk liability snapshots: list eligible: %w", err)
	}
	eligibleSet := make(map[uuid.UUID]struct{}, len(eligible))
	for _, id := range eligible {
		eligibleSet[id] = struct{}{}
	}

	var rowErrs []BulkSnapshotRowError
	for _, row := range p.Rows {
		if _, ok := eligibleSet[row.LiabilityID]; !ok {
			rowErrs = append(rowErrs, BulkSnapshotRowError{PositionID: row.LiabilityID, Reason: BulkRowIneligible})
		}
	}
	if len(rowErrs) > 0 {
		return BulkResult{}, rowErrs, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return BulkResult{}, nil, fmt.Errorf("bulk liability snapshots: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	var res BulkResult
	for _, row := range p.Rows {
		_, _, outcome, err := replaceLiabilitySnapshot(ctx, qtx, user, db.CreateLiabilitySnapshotParams{
			ID:          row.LiabilityID,
			YearMonth:   p.YearMonth,
			Amount:      row.Amount,
			Currency:    row.Currency,
			AsOfDate:    p.AsOfDate,
			Description: nil,
			CreatedBy:   &user,
			HouseholdID: hid,
		})
		if err != nil {
			return BulkResult{}, nil, err
		}
		res.add(outcome)
	}

	if err := tx.Commit(ctx); err != nil {
		return BulkResult{}, nil, fmt.Errorf("bulk liability snapshots: commit: %w", err)
	}
	return res, nil, nil
}

// ----- Receivable bulk monthly-entry (ADR-0046) ----------------------------
//
// Structural twin of the Asset/Liability methods, against receivables /
// receivable_snapshots. Receivables are a flat group with no subtype, so
// ReceivableEntryRow carries none — the entry view renders one ungrouped list.

// BulkReceivableSnapshotRow is one receivable's value in a bulk monthly-entry
// batch — dirty-only; the target month and as-of date are batch-level.
type BulkReceivableSnapshotRow struct {
	ReceivableID uuid.UUID
	Amount       decimal.Decimal
	Currency     string
}

// BulkUpsertReceivableSnapshotsParams carries a whole batch: one target month,
// one as-of date, N dirty rows.
type BulkUpsertReceivableSnapshotsParams struct {
	YearMonth time.Time
	AsOfDate  *time.Time
	Rows      []BulkReceivableSnapshotRow
}

// ReceivableEntryRow is one row of the bulk monthly-entry list: an eligible
// receivable with its carry-forward prefill. PrefillAmount/CarriedFrom are nil
// for a receivable with no snapshot at or before the target month.
type ReceivableEntryRow struct {
	ReceivableID    uuid.UUID
	DisplayName     string
	Currency        string
	OwnershipType   string
	SoleOwnerUserID *uuid.UUID
	PrefillAmount   *decimal.Decimal
	CarriedFrom     *time.Time
}

// ListReceivableEntryRows returns the bulk monthly-entry list for a target
// month: every eligible receivable with its most-recent snapshot at or before
// that month as the carry-forward prefill.
func (r *ReceivableRepo) ListReceivableEntryRows(ctx context.Context, yearMonth time.Time) ([]ReceivableEntryRow, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	receivables, err := r.q.ListEligibleReceivablesForMonth(ctx, db.ListEligibleReceivablesForMonthParams{
		HouseholdID: hid,
		YearMonth:   yearMonth,
	})
	if err != nil {
		return nil, fmt.Errorf("receivable entry rows: list eligible: %w", err)
	}
	if len(receivables) == 0 {
		return nil, nil
	}

	ids := make([]uuid.UUID, len(receivables))
	for i, rv := range receivables {
		ids[i] = rv.ID
	}
	latest, err := r.q.ListLatestSnapshotsByReceivableIDsAsOfMonth(ctx, db.ListLatestSnapshotsByReceivableIDsAsOfMonthParams{
		ReceivableIds: ids,
		YearMonth:     yearMonth,
	})
	if err != nil {
		return nil, fmt.Errorf("receivable entry rows: list prefill: %w", err)
	}
	prefill := make(map[uuid.UUID]db.ListLatestSnapshotsByReceivableIDsAsOfMonthRow, len(latest))
	for _, s := range latest {
		prefill[s.ReceivableID] = s
	}

	rows := make([]ReceivableEntryRow, len(receivables))
	for i, rv := range receivables {
		row := ReceivableEntryRow{
			ReceivableID:    rv.ID,
			DisplayName:     rv.DisplayName,
			Currency:        rv.NativeCurrency,
			OwnershipType:   rv.OwnershipType,
			SoleOwnerUserID: rv.SoleOwnerUserID,
		}
		if s, ok := prefill[rv.ID]; ok {
			amt := s.Amount
			ym := s.YearMonth
			row.PrefillAmount = &amt
			row.CarriedFrom = &ym
		}
		rows[i] = row
	}
	return rows, nil
}

// BulkUpsertReceivableSnapshots writes a bulk monthly-entry batch in a single
// transaction — all-or-nothing (ADR-0046). Every row is validated for
// month-aware eligibility first; if any row is ineligible the batch writes
// nothing and the offending rows are returned. Otherwise each row is
// written through the replace path (ADR-0058 §1), as for assets. Returns the
// written/replaced/unchanged tally and any per-row rejections.
func (r *ReceivableRepo) BulkUpsertReceivableSnapshots(ctx context.Context, p BulkUpsertReceivableSnapshotsParams) (BulkResult, []BulkSnapshotRowError, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return BulkResult{}, nil, err
	}
	if len(p.Rows) == 0 {
		return BulkResult{}, nil, nil
	}

	eligible, err := r.q.ListEligibleReceivableIDsForMonth(ctx, db.ListEligibleReceivableIDsForMonthParams{
		HouseholdID: hid,
		YearMonth:   p.YearMonth,
	})
	if err != nil {
		return BulkResult{}, nil, fmt.Errorf("bulk receivable snapshots: list eligible: %w", err)
	}
	eligibleSet := make(map[uuid.UUID]struct{}, len(eligible))
	for _, id := range eligible {
		eligibleSet[id] = struct{}{}
	}

	var rowErrs []BulkSnapshotRowError
	for _, row := range p.Rows {
		if _, ok := eligibleSet[row.ReceivableID]; !ok {
			rowErrs = append(rowErrs, BulkSnapshotRowError{PositionID: row.ReceivableID, Reason: BulkRowIneligible})
		}
	}
	if len(rowErrs) > 0 {
		return BulkResult{}, rowErrs, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return BulkResult{}, nil, fmt.Errorf("bulk receivable snapshots: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	var res BulkResult
	for _, row := range p.Rows {
		_, _, outcome, err := replaceReceivableSnapshot(ctx, qtx, user, db.CreateReceivableSnapshotParams{
			ID:          row.ReceivableID,
			YearMonth:   p.YearMonth,
			Amount:      row.Amount,
			Currency:    row.Currency,
			AsOfDate:    p.AsOfDate,
			Description: nil,
			CreatedBy:   &user,
			HouseholdID: hid,
		})
		if err != nil {
			return BulkResult{}, nil, err
		}
		res.add(outcome)
	}

	if err := tx.Commit(ctx); err != nil {
		return BulkResult{}, nil, fmt.Errorf("bulk receivable snapshots: commit: %w", err)
	}
	return res, nil, nil
}

// ----- Investment bulk monthly-entry, qty×price shape (ADR-0046, #423) -------
//
// Structural twin of the amount-only methods above, but against the qty×price
// branch of the Investment group — Stock/MutualFund/Gold, whose snapshots take
// quantity + price_per_unit (accrued_interest null) per ADR-0022's shape XOR.
// Bond/TimeDeposit (the accrued branch) are a separate slice (#424). The row
// carries the two tab-stops; the stored `amount` is derived server-side as
// quantity × price_per_unit so the persisted total never trusts client
// arithmetic. Eligibility is filtered to the three qty×price subtypes, so an
// accrued-shape investment can never be written through this path.

// BulkInvestmentSnapshotRow is one investment's qty×price value in a bulk
// monthly-entry batch — dirty-only; the target month and as-of date are
// batch-level. Amount is not carried: it is derived as Quantity × PricePerUnit.
type BulkInvestmentSnapshotRow struct {
	InvestmentID uuid.UUID
	Quantity     decimal.Decimal
	PricePerUnit decimal.Decimal
	Currency     string
}

// BulkUpsertInvestmentSnapshotsParams carries a whole batch: one target month,
// one as-of date, N dirty rows.
type BulkUpsertInvestmentSnapshotsParams struct {
	YearMonth time.Time
	AsOfDate  *time.Time
	Rows      []BulkInvestmentSnapshotRow
}

// InvestmentEntryRow is one row of the qty×price bulk monthly-entry list: an
// eligible Stock/MutualFund/Gold with its carry-forward prefill. PrefillQuantity
// / PrefillPrice / CarriedFrom are nil for an investment with no snapshot at or
// before the target month.
type InvestmentEntryRow struct {
	InvestmentID    uuid.UUID
	DisplayName     string
	Currency        string
	Subtype         string
	OwnershipType   string
	SoleOwnerUserID *uuid.UUID
	PrefillQuantity *decimal.Decimal
	PrefillPrice    *decimal.Decimal
	CarriedFrom     *time.Time
}

// ListInvestmentEntryRows returns the qty×price bulk monthly-entry list for a
// target month: every eligible Stock/MutualFund/Gold with the quantity + price
// of its most-recent snapshot at or before that month as the carry-forward
// prefill.
func (r *InvestmentRepo) ListInvestmentEntryRows(ctx context.Context, yearMonth time.Time) ([]InvestmentEntryRow, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	investments, err := r.q.ListEligibleQtyPriceInvestmentsForMonth(ctx, db.ListEligibleQtyPriceInvestmentsForMonthParams{
		HouseholdID: hid,
		YearMonth:   yearMonth,
	})
	if err != nil {
		return nil, fmt.Errorf("investment entry rows: list eligible: %w", err)
	}
	if len(investments) == 0 {
		return nil, nil
	}

	ids := make([]uuid.UUID, len(investments))
	for i, iv := range investments {
		ids[i] = iv.ID
	}
	latest, err := r.q.ListLatestQtyPriceSnapshotsByInvestmentIDsAsOfMonth(ctx, db.ListLatestQtyPriceSnapshotsByInvestmentIDsAsOfMonthParams{
		InvestmentIds: ids,
		YearMonth:     yearMonth,
	})
	if err != nil {
		return nil, fmt.Errorf("investment entry rows: list prefill: %w", err)
	}
	prefill := make(map[uuid.UUID]db.ListLatestQtyPriceSnapshotsByInvestmentIDsAsOfMonthRow, len(latest))
	for _, s := range latest {
		prefill[s.InvestmentID] = s
	}

	rows := make([]InvestmentEntryRow, len(investments))
	for i, iv := range investments {
		row := InvestmentEntryRow{
			InvestmentID:    iv.ID,
			DisplayName:     iv.DisplayName,
			Currency:        iv.NativeCurrency,
			Subtype:         iv.Subtype,
			OwnershipType:   iv.OwnershipType,
			SoleOwnerUserID: iv.SoleOwnerUserID,
		}
		if s, ok := prefill[iv.ID]; ok {
			ym := s.YearMonth
			row.PrefillQuantity = s.Quantity
			row.PrefillPrice = s.PricePerUnit
			row.CarriedFrom = &ym
		}
		rows[i] = row
	}
	return rows, nil
}

// BulkUpsertInvestmentSnapshots writes a qty×price bulk monthly-entry batch in a
// single transaction — all-or-nothing (ADR-0046). Every row is validated for
// month-aware eligibility (owned, not deleted, still within its termination
// bound, and one of the three qty×price subtypes) before any write; if any row
// is ineligible the batch writes nothing and the offending rows are returned.
// Otherwise each row goes through the replace path (ADR-0058 §1) with amount
// derived as quantity × price_per_unit and accrued_interest null (the shape
// CHECK's qty×price branch). Returns the written/replaced/unchanged tally and
// any per-row rejections.
func (r *InvestmentRepo) BulkUpsertInvestmentSnapshots(ctx context.Context, p BulkUpsertInvestmentSnapshotsParams) (BulkResult, []BulkSnapshotRowError, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return BulkResult{}, nil, err
	}
	if len(p.Rows) == 0 {
		return BulkResult{}, nil, nil
	}

	eligible, err := r.q.ListEligibleQtyPriceInvestmentIDsForMonth(ctx, db.ListEligibleQtyPriceInvestmentIDsForMonthParams{
		HouseholdID: hid,
		YearMonth:   p.YearMonth,
	})
	if err != nil {
		return BulkResult{}, nil, fmt.Errorf("bulk investment snapshots: list eligible: %w", err)
	}
	eligibleSet := make(map[uuid.UUID]struct{}, len(eligible))
	for _, id := range eligible {
		eligibleSet[id] = struct{}{}
	}

	var rowErrs []BulkSnapshotRowError
	for _, row := range p.Rows {
		if _, ok := eligibleSet[row.InvestmentID]; !ok {
			rowErrs = append(rowErrs, BulkSnapshotRowError{PositionID: row.InvestmentID, Reason: BulkRowIneligible})
		}
	}
	if len(rowErrs) > 0 {
		return BulkResult{}, rowErrs, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return BulkResult{}, nil, fmt.Errorf("bulk investment snapshots: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	var res BulkResult
	for _, row := range p.Rows {
		qty := row.Quantity
		price := row.PricePerUnit
		amount := qty.Mul(price)
		_, _, outcome, err := replaceInvestmentSnapshot(ctx, qtx, user, db.CreateInvestmentSnapshotParams{
			ID:              row.InvestmentID,
			YearMonth:       p.YearMonth,
			Amount:          amount,
			Currency:        row.Currency,
			Quantity:        &qty,
			PricePerUnit:    &price,
			AccruedInterest: nil,
			AsOfDate:        p.AsOfDate,
			Description:     nil,
			CreatedBy:       &user,
			HouseholdID:     hid,
		})
		if err != nil {
			return BulkResult{}, nil, err
		}
		res.add(outcome)
	}

	if err := tx.Commit(ctx); err != nil {
		return BulkResult{}, nil, fmt.Errorf("bulk investment snapshots: commit: %w", err)
	}
	return res, nil, nil
}

// ----- Investment bulk monthly-entry, accrued shape (ADR-0046, #424) ----------
//
// Structural twin of the qty×price methods above, but against the accrued
// branch of the Investment group — Bond/TimeDeposit, whose snapshots take
// accrued_interest (quantity/price null) per ADR-0022's shape XOR. Unlike
// qty×price, `amount` is entered directly (a bond's total value already *is*
// its snapshot amount, not a derived product), so a row carries the two figures
// the per-position accrued dialog takes: the total value and the accrued
// component. Eligibility is filtered to the two accrued subtypes, so a
// qty×price investment can never be written through this path.

// BulkInvestmentAccruedSnapshotRow is one investment's accrued value in a bulk
// monthly-entry batch — dirty-only; the target month and as-of date are
// batch-level. Amount is the total value; AccruedInterest the accrued component
// (quantity/price stay null). A bond's AccruedInterest is stored as given and is
// required; a time deposit's is derived by the server as Amount − principal in
// force (#844), so for one it is ignored and may be nil.
type BulkInvestmentAccruedSnapshotRow struct {
	InvestmentID    uuid.UUID
	Amount          decimal.Decimal
	AccruedInterest *decimal.Decimal
	Currency        string
}

// BulkUpsertInvestmentAccruedSnapshotsParams carries a whole batch: one target
// month, one as-of date, N dirty rows.
type BulkUpsertInvestmentAccruedSnapshotsParams struct {
	YearMonth time.Time
	AsOfDate  *time.Time
	Rows      []BulkInvestmentAccruedSnapshotRow
}

// InvestmentAccruedEntryRow is one row of the accrued bulk monthly-entry list:
// an eligible Bond/TimeDeposit with its carry-forward prefill and (for bonds)
// coupon disposition. PrefillAmount / PrefillAccruedInterest / CarriedFrom are
// nil for an investment with no snapshot at or before the target month.
// CouponDisposition is nil for a time deposit (no bond_details row); the entry
// list treats nil as pays_out (accrued default 0).
type InvestmentAccruedEntryRow struct {
	InvestmentID           uuid.UUID
	DisplayName            string
	Currency               string
	Subtype                string
	OwnershipType          string
	SoleOwnerUserID        *uuid.UUID
	CouponDisposition      *string
	Principal              *decimal.Decimal
	PrefillAmount          *decimal.Decimal
	PrefillAccruedInterest *decimal.Decimal
	CarriedFrom            *time.Time
}

// ListInvestmentAccruedEntryRows returns the accrued bulk monthly-entry list for
// a target month: every eligible Bond/TimeDeposit with the total value + accrued
// interest of its most-recent snapshot at or before that month as the
// carry-forward prefill, plus each bond's coupon disposition so the entry view
// can seed the accrued default.
func (r *InvestmentRepo) ListInvestmentAccruedEntryRows(ctx context.Context, yearMonth time.Time) ([]InvestmentAccruedEntryRow, error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	investments, err := r.q.ListEligibleAccruedInvestmentsForMonth(ctx, db.ListEligibleAccruedInvestmentsForMonthParams{
		HouseholdID: hid,
		YearMonth:   yearMonth,
	})
	if err != nil {
		return nil, fmt.Errorf("accrued entry rows: list eligible: %w", err)
	}
	if len(investments) == 0 {
		return nil, nil
	}

	ids := make([]uuid.UUID, len(investments))
	for i, iv := range investments {
		ids[i] = iv.ID
	}
	latest, err := r.q.ListLatestAccruedSnapshotsByInvestmentIDsAsOfMonth(ctx, db.ListLatestAccruedSnapshotsByInvestmentIDsAsOfMonthParams{
		InvestmentIds: ids,
		YearMonth:     yearMonth,
	})
	if err != nil {
		return nil, fmt.Errorf("accrued entry rows: list prefill: %w", err)
	}
	prefill := make(map[uuid.UUID]db.ListLatestAccruedSnapshotsByInvestmentIDsAsOfMonthRow, len(latest))
	for _, s := range latest {
		prefill[s.InvestmentID] = s
	}

	rows := make([]InvestmentAccruedEntryRow, len(investments))
	for i, iv := range investments {
		row := InvestmentAccruedEntryRow{
			InvestmentID:      iv.ID,
			DisplayName:       iv.DisplayName,
			Currency:          iv.NativeCurrency,
			Subtype:           iv.Subtype,
			OwnershipType:     iv.OwnershipType,
			SoleOwnerUserID:   iv.SoleOwnerUserID,
			CouponDisposition: iv.CouponDisposition,
			Principal:         iv.Principal,
		}
		if s, ok := prefill[iv.ID]; ok {
			amt := s.Amount
			ym := s.YearMonth
			row.PrefillAmount = &amt
			row.PrefillAccruedInterest = s.AccruedInterest
			row.CarriedFrom = &ym
		}
		rows[i] = row
	}
	return rows, nil
}

// BulkUpsertInvestmentAccruedSnapshots writes an accrued bulk monthly-entry
// batch in a single transaction — all-or-nothing (ADR-0046). Every row is
// validated for month-aware eligibility (owned, not deleted, still within its
// termination bound, and one of the two accrued subtypes) before any write; if
// any row is ineligible the batch writes nothing and the offending rows are
// returned. Otherwise each row goes through the replace path (ADR-0058 §1) with the
// total value stored as `amount` and accrued_interest set (quantity/price null —
// the shape CHECK's accrued branch). Returns the written/replaced/unchanged
// tally and any per-row rejections.
func (r *InvestmentRepo) BulkUpsertInvestmentAccruedSnapshots(ctx context.Context, p BulkUpsertInvestmentAccruedSnapshotsParams) (BulkResult, []BulkSnapshotRowError, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return BulkResult{}, nil, err
	}
	if len(p.Rows) == 0 {
		return BulkResult{}, nil, nil
	}

	eligible, err := r.q.ListEligibleAccruedInvestmentIDsForMonth(ctx, db.ListEligibleAccruedInvestmentIDsForMonthParams{
		HouseholdID: hid,
		YearMonth:   p.YearMonth,
	})
	if err != nil {
		return BulkResult{}, nil, fmt.Errorf("bulk accrued snapshots: list eligible: %w", err)
	}
	eligibleSet := make(map[uuid.UUID]struct{}, len(eligible))
	for _, id := range eligible {
		eligibleSet[id] = struct{}{}
	}

	var rowErrs []BulkSnapshotRowError
	for _, row := range p.Rows {
		if _, ok := eligibleSet[row.InvestmentID]; !ok {
			rowErrs = append(rowErrs, BulkSnapshotRowError{PositionID: row.InvestmentID, Reason: BulkRowIneligible})
		}
	}
	if len(rowErrs) > 0 {
		return BulkResult{}, rowErrs, nil
	}

	// A time deposit's accrued_interest is derived from its Value, not taken from
	// the client (#844); a bond's is required and stored as given. A row with a
	// principal here is a time deposit — time_deposit_details exists only for one.
	ids := make([]uuid.UUID, len(p.Rows))
	for i, row := range p.Rows {
		ids[i] = row.InvestmentID
	}
	principalRows, err := r.q.ListTimeDepositPrincipalsByInvestmentIDs(ctx, db.ListTimeDepositPrincipalsByInvestmentIDsParams{
		InvestmentIds: ids,
		HouseholdID:   hid,
	})
	if err != nil {
		return BulkResult{}, nil, fmt.Errorf("bulk accrued snapshots: list principals: %w", err)
	}
	principals := make(map[uuid.UUID]decimal.Decimal, len(principalRows))
	for _, pr := range principalRows {
		principals[pr.InvestmentID] = pr.Principal
	}
	accruedFor := make([]decimal.Decimal, len(p.Rows))
	for i, row := range p.Rows {
		if principal, isTD := principals[row.InvestmentID]; isTD {
			accruedFor[i] = row.Amount.Sub(principal)
			continue
		}
		if err := validateInvestmentSnapshotShape("bond", nil, nil, row.AccruedInterest); err != nil {
			return BulkResult{}, nil, err
		}
		accruedFor[i] = *row.AccruedInterest
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return BulkResult{}, nil, fmt.Errorf("bulk accrued snapshots: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	var res BulkResult
	for i, row := range p.Rows {
		amount := row.Amount
		accrued := accruedFor[i]
		_, _, outcome, err := replaceInvestmentSnapshot(ctx, qtx, user, db.CreateInvestmentSnapshotParams{
			ID:              row.InvestmentID,
			YearMonth:       p.YearMonth,
			Amount:          amount,
			Currency:        row.Currency,
			Quantity:        nil,
			PricePerUnit:    nil,
			AccruedInterest: &accrued,
			AsOfDate:        p.AsOfDate,
			Description:     nil,
			CreatedBy:       &user,
			HouseholdID:     hid,
		})
		if err != nil {
			return BulkResult{}, nil, err
		}
		res.add(outcome)
	}

	if err := tx.Commit(ctx); err != nil {
		return BulkResult{}, nil, fmt.Errorf("bulk accrued snapshots: commit: %w", err)
	}
	return res, nil, nil
}
