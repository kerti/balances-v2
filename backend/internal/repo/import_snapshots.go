package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// ImportSnapshotRow is one snapshot to write during a bulk import. It mirrors
// CreateAssetSnapshotParams minus the asset id, which is supplied once for the
// whole batch.
type ImportSnapshotRow struct {
	YearMonth   time.Time
	Amount      decimal.Decimal
	Currency    string
	AsOfDate    *time.Time
	Description *string
}

// ImportResult summarises an import or its dry-run preview: how many rows
// create a new month, how many replace an existing month's different values
// (archiving the earlier row, ADR-0058 §1), and how many are identical to the
// live snapshot and so write nothing.
type ImportResult struct {
	ToInsert  int
	ToUpdate  int
	Unchanged int
}

// AssetImportMeta returns the display name + native currency of an owned asset,
// used to scope a download template (and as an early ownership check -> 404).
func (r *AssetRepo) AssetImportMeta(ctx context.Context, assetID uuid.UUID) (name, currency string, err error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return "", "", err
	}
	row, err := r.q.GetAssetForImport(ctx, db.GetAssetForImportParams{ID: assetID, HouseholdID: hid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrNotFound
		}
		return "", "", fmt.Errorf("asset import meta: %w", err)
	}
	return row.DisplayName, row.NativeCurrency, nil
}

// ImportAssetSnapshots writes a batch of snapshots for one asset in a single
// transaction (all-or-nothing). A month with no live snapshot is inserted, one
// whose values differ has its live row archived and the new values inserted
// (ADR-0058 §1), and one identical to the live row is skipped. When dryRun is
// true it classifies the rows against the asset's existing months and returns
// without writing. The asset
// must belong to the caller's household, otherwise ErrNotFound.
//
// Classification (insert vs replace vs unchanged) is computed from a snapshot read taken
// before the write; in a single-household app the window between preview and
// commit is not worth locking against.
func (r *AssetRepo) ImportAssetSnapshots(ctx context.Context, assetID uuid.UUID, rows []ImportSnapshotRow, dryRun bool) (*ImportResult, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	// Ownership + existence: ErrNoRows means the asset isn't in this household
	// (or is soft-deleted) -> 404, before we touch anything.
	if _, err := r.q.GetAssetForImport(ctx, db.GetAssetForImportParams{ID: assetID, HouseholdID: hid}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("import: load asset: %w", err)
	}

	existing, err := r.q.ListAssetSnapshotsForAsset(ctx, db.ListAssetSnapshotsForAssetParams{
		AssetID:     assetID,
		HouseholdID: hid,
	})
	if err != nil {
		return nil, fmt.Errorf("import: list existing snapshots: %w", err)
	}
	params := func(row ImportSnapshotRow) db.CreateAssetSnapshotParams {
		return db.CreateAssetSnapshotParams{
			ID:          assetID,
			YearMonth:   row.YearMonth,
			Amount:      row.Amount,
			Currency:    row.Currency,
			AsOfDate:    row.AsOfDate,
			Description: row.Description,
			CreatedBy:   &user,
			HouseholdID: hid,
		}
	}

	live := make(map[string]db.AssetSnapshot, len(existing))
	for _, s := range existing {
		live[s.YearMonth.Format("2006-01")] = s
	}

	var res ImportResult
	for _, row := range rows {
		cur, ok := live[row.YearMonth.Format("2006-01")]
		switch {
		case !ok:
			res.ToInsert++
		case assetSnapshotUnchanged(cur, params(row)):
			res.Unchanged++
		default:
			res.ToUpdate++
		}
	}

	if dryRun || len(rows) == 0 {
		return &res, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("import: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	var written ImportResult
	for _, row := range rows {
		_, _, outcome, err := replaceAssetSnapshot(ctx, qtx, user, params(row))
		if err != nil {
			return nil, err
		}
		switch outcome {
		case replaceInserted:
			written.ToInsert++
		case replaceReplaced:
			written.ToUpdate++
		case replaceUnchanged:
			written.Unchanged++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("import: commit: %w", err)
	}
	return &written, nil
}

// LiabilityImportMeta mirrors AssetImportMeta for the Liability group: display
// name + native currency of an owned liability, doubling as the ownership
// check (-> ErrNotFound on a miss).
func (r *LiabilityRepo) LiabilityImportMeta(ctx context.Context, liabilityID uuid.UUID) (name, currency string, err error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return "", "", err
	}
	row, err := r.q.GetLiabilityForImport(ctx, db.GetLiabilityForImportParams{ID: liabilityID, HouseholdID: hid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrNotFound
		}
		return "", "", fmt.Errorf("liability import meta: %w", err)
	}
	return row.DisplayName, row.NativeCurrency, nil
}

// ImportLiabilitySnapshots is the Liability-group analogue of
// ImportAssetSnapshots: all-or-nothing replace-or-insert in one tx, dry-run classifies
// without writing, ownership enforced via GetLiabilityForImport.
func (r *LiabilityRepo) ImportLiabilitySnapshots(ctx context.Context, liabilityID uuid.UUID, rows []ImportSnapshotRow, dryRun bool) (*ImportResult, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := r.q.GetLiabilityForImport(ctx, db.GetLiabilityForImportParams{ID: liabilityID, HouseholdID: hid}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("import: load liability: %w", err)
	}

	existing, err := r.q.ListLiabilitySnapshotsForLiability(ctx, db.ListLiabilitySnapshotsForLiabilityParams{
		LiabilityID: liabilityID,
		HouseholdID: hid,
	})
	if err != nil {
		return nil, fmt.Errorf("import: list existing snapshots: %w", err)
	}
	params := func(row ImportSnapshotRow) db.CreateLiabilitySnapshotParams {
		return db.CreateLiabilitySnapshotParams{
			ID:          liabilityID,
			YearMonth:   row.YearMonth,
			Amount:      row.Amount,
			Currency:    row.Currency,
			AsOfDate:    row.AsOfDate,
			Description: row.Description,
			CreatedBy:   &user,
			HouseholdID: hid,
		}
	}

	live := make(map[string]db.LiabilitySnapshot, len(existing))
	for _, s := range existing {
		live[s.YearMonth.Format("2006-01")] = s
	}

	var res ImportResult
	for _, row := range rows {
		cur, ok := live[row.YearMonth.Format("2006-01")]
		switch {
		case !ok:
			res.ToInsert++
		case liabilitySnapshotUnchanged(cur, params(row)):
			res.Unchanged++
		default:
			res.ToUpdate++
		}
	}

	if dryRun || len(rows) == 0 {
		return &res, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("import: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	var written ImportResult
	for _, row := range rows {
		_, _, outcome, err := replaceLiabilitySnapshot(ctx, qtx, user, params(row))
		if err != nil {
			return nil, err
		}
		switch outcome {
		case replaceInserted:
			written.ToInsert++
		case replaceReplaced:
			written.ToUpdate++
		case replaceUnchanged:
			written.Unchanged++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("import: commit: %w", err)
	}
	return &written, nil
}

// ReceivableImportMeta mirrors AssetImportMeta for the Receivable group.
func (r *ReceivableRepo) ReceivableImportMeta(ctx context.Context, receivableID uuid.UUID) (name, currency string, err error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return "", "", err
	}
	row, err := r.q.GetReceivableForImport(ctx, db.GetReceivableForImportParams{ID: receivableID, HouseholdID: hid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", ErrNotFound
		}
		return "", "", fmt.Errorf("receivable import meta: %w", err)
	}
	return row.DisplayName, row.NativeCurrency, nil
}

// ImportReceivableSnapshots is the Receivable-group analogue of
// ImportAssetSnapshots.
func (r *ReceivableRepo) ImportReceivableSnapshots(ctx context.Context, receivableID uuid.UUID, rows []ImportSnapshotRow, dryRun bool) (*ImportResult, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	if _, err := r.q.GetReceivableForImport(ctx, db.GetReceivableForImportParams{ID: receivableID, HouseholdID: hid}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("import: load receivable: %w", err)
	}

	existing, err := r.q.ListReceivableSnapshotsForReceivable(ctx, db.ListReceivableSnapshotsForReceivableParams{
		ReceivableID: receivableID,
		HouseholdID:  hid,
	})
	if err != nil {
		return nil, fmt.Errorf("import: list existing snapshots: %w", err)
	}
	params := func(row ImportSnapshotRow) db.CreateReceivableSnapshotParams {
		return db.CreateReceivableSnapshotParams{
			ID:          receivableID,
			YearMonth:   row.YearMonth,
			Amount:      row.Amount,
			Currency:    row.Currency,
			AsOfDate:    row.AsOfDate,
			Description: row.Description,
			CreatedBy:   &user,
			HouseholdID: hid,
		}
	}

	live := make(map[string]db.ReceivableSnapshot, len(existing))
	for _, s := range existing {
		live[s.YearMonth.Format("2006-01")] = s
	}

	var res ImportResult
	for _, row := range rows {
		cur, ok := live[row.YearMonth.Format("2006-01")]
		switch {
		case !ok:
			res.ToInsert++
		case receivableSnapshotUnchanged(cur, params(row)):
			res.Unchanged++
		default:
			res.ToUpdate++
		}
	}

	if dryRun || len(rows) == 0 {
		return &res, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("import: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	var written ImportResult
	for _, row := range rows {
		_, _, outcome, err := replaceReceivableSnapshot(ctx, qtx, user, params(row))
		if err != nil {
			return nil, err
		}
		switch outcome {
		case replaceInserted:
			written.ToInsert++
		case replaceReplaced:
			written.ToUpdate++
		case replaceUnchanged:
			written.Unchanged++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("import: commit: %w", err)
	}
	return &written, nil
}

// ImportInvestmentSnapshotRow is one snapshot to write during an investment
// bulk import. Unlike the flat ImportSnapshotRow it carries the value-shape
// columns: quantity+price_per_unit (stock/mutual_fund/gold) or accrued_interest
// (bond/time_deposit). Amount is always set (derived from quantity×price for
// the quantity-price shape).
type ImportInvestmentSnapshotRow struct {
	YearMonth       time.Time
	Amount          decimal.Decimal
	Currency        string
	Quantity        *decimal.Decimal
	PricePerUnit    *decimal.Decimal
	AccruedInterest *decimal.Decimal
	AsOfDate        *time.Time
	Description     *string
}

// InvestmentImportMeta returns the display name + native currency + subtype of
// an owned investment. The subtype tells the HTTP layer which template shape to
// emit / parse; the lookup doubles as the ownership check (-> ErrNotFound).
func (r *InvestmentRepo) InvestmentImportMeta(ctx context.Context, investmentID uuid.UUID) (name, currency, subtype string, err error) {
	_, hid, err := currentUser(ctx)
	if err != nil {
		return "", "", "", err
	}
	inv, err := r.q.GetInvestmentByID(ctx, db.GetInvestmentByIDParams{ID: investmentID, HouseholdID: hid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", "", ErrNotFound
		}
		return "", "", "", fmt.Errorf("investment import meta: %w", err)
	}
	return inv.DisplayName, inv.NativeCurrency, inv.Subtype, nil
}

// ImportInvestmentSnapshots is the Investment-group analogue of
// ImportAssetSnapshots, with one extra step: every row's value-column shape is
// validated against the parent's subtype up front (so an all-or-nothing import
// never half-commits a bad shape; the DB CHECK is the final backstop).
func (r *InvestmentRepo) ImportInvestmentSnapshots(ctx context.Context, investmentID uuid.UUID, rows []ImportInvestmentSnapshotRow, dryRun bool) (*ImportResult, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}

	inv, err := r.q.GetInvestmentByID(ctx, db.GetInvestmentByIDParams{ID: investmentID, HouseholdID: hid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("import: load investment: %w", err)
	}

	for _, row := range rows {
		if err := validateInvestmentSnapshotShape(inv.Subtype, row.Quantity, row.PricePerUnit, row.AccruedInterest); err != nil {
			return nil, err
		}
	}

	existing, err := r.q.ListInvestmentSnapshotsForInvestment(ctx, db.ListInvestmentSnapshotsForInvestmentParams{
		InvestmentID: investmentID,
		HouseholdID:  hid,
	})
	if err != nil {
		return nil, fmt.Errorf("import: list existing snapshots: %w", err)
	}
	params := func(row ImportInvestmentSnapshotRow) db.CreateInvestmentSnapshotParams {
		return db.CreateInvestmentSnapshotParams{
			ID:              investmentID,
			YearMonth:       row.YearMonth,
			Amount:          row.Amount,
			Currency:        row.Currency,
			Quantity:        row.Quantity,
			PricePerUnit:    row.PricePerUnit,
			AccruedInterest: row.AccruedInterest,
			AsOfDate:        row.AsOfDate,
			Description:     row.Description,
			CreatedBy:       &user,
			HouseholdID:     hid,
		}
	}

	live := make(map[string]db.InvestmentSnapshot, len(existing))
	for _, s := range existing {
		live[s.YearMonth.Format("2006-01")] = s
	}

	var res ImportResult
	for _, row := range rows {
		cur, ok := live[row.YearMonth.Format("2006-01")]
		switch {
		case !ok:
			res.ToInsert++
		case investmentSnapshotUnchanged(cur, params(row)):
			res.Unchanged++
		default:
			res.ToUpdate++
		}
	}

	if dryRun || len(rows) == 0 {
		return &res, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("import: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	var written ImportResult
	for _, row := range rows {
		_, _, outcome, err := replaceInvestmentSnapshot(ctx, qtx, user, params(row))
		if err != nil {
			return nil, err
		}
		switch outcome {
		case replaceInserted:
			written.ToInsert++
		case replaceReplaced:
			written.ToUpdate++
		case replaceUnchanged:
			written.Unchanged++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("import: commit: %w", err)
	}
	return &written, nil
}
