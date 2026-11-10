package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// Undo (ADR-0058 §2). A single-item soft delete is undone by clearing
// `deleted_at` on that one row; a snapshot replace is undone by archiving the
// replacement and restoring the original in one transaction. Every restore
// moves `updated_at`, which is what the report staleness watermark reads, so
// the affected months regenerate on the next read exactly as after a delete.
//
// The live partial unique indexes decide whether the slot is still free: a
// restore into a slot something else filled raises unique_violation, mapped to
// ErrUndoSlotTaken with nothing changed. A restore never reaches across
// households or under a deleted parent — both read as ErrNotFound, like every
// other id that is not the caller's to touch.

// undoRows folds a restore's (rows, err) into the repo's error vocabulary.
func undoRows(op string, rows int64, err error) error {
	if err != nil {
		if isUniqueViolation(err) {
			return ErrUndoSlotTaken
		}
		return fmt.Errorf("%s: %w", op, err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ----- single-row restores ------------------------------------------------

func (r *AssetRepo) UndoDeleteAssetSnapshot(ctx context.Context, snapshotID uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	rows, err := r.q.RestoreAssetSnapshot(ctx, db.RestoreAssetSnapshotParams{ID: snapshotID, HouseholdID: hid, UpdatedBy: &user})
	return undoRows("restore asset snapshot", rows, err)
}

func (r *LiabilityRepo) UndoDeleteLiabilitySnapshot(ctx context.Context, snapshotID uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	rows, err := r.q.RestoreLiabilitySnapshot(ctx, db.RestoreLiabilitySnapshotParams{ID: snapshotID, HouseholdID: hid, UpdatedBy: &user})
	return undoRows("restore liability snapshot", rows, err)
}

func (r *ReceivableRepo) UndoDeleteReceivableSnapshot(ctx context.Context, snapshotID uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	rows, err := r.q.RestoreReceivableSnapshot(ctx, db.RestoreReceivableSnapshotParams{ID: snapshotID, HouseholdID: hid, UpdatedBy: &user})
	return undoRows("restore receivable snapshot", rows, err)
}

func (r *InvestmentRepo) UndoDeleteInvestmentSnapshot(ctx context.Context, snapshotID uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	rows, err := r.q.RestoreInvestmentSnapshot(ctx, db.RestoreInvestmentSnapshotParams{ID: snapshotID, HouseholdID: hid, UpdatedBy: &user})
	return undoRows("restore investment snapshot", rows, err)
}

func (r *InvestmentRepo) UndoDeleteInvestmentTransaction(ctx context.Context, transactionID uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	rows, err := r.q.RestoreInvestmentTransaction(ctx, db.RestoreInvestmentTransactionParams{ID: transactionID, HouseholdID: hid, UpdatedBy: &user})
	return undoRows("restore investment transaction", rows, err)
}

func (r *IncomeRepo) UndoDeleteIncome(ctx context.Context, id uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	rows, err := r.q.RestoreIncome(ctx, db.RestoreIncomeParams{ID: id, HouseholdID: hid, UpdatedBy: &user})
	return undoRows("restore income", rows, err)
}

func (r *FxRateRepo) UndoDeleteFxRate(ctx context.Context, id uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	rows, err := r.q.RestoreFxRate(ctx, db.RestoreFxRateParams{ID: id, HouseholdID: hid, UpdatedBy: &user})
	return undoRows("restore fx rate", rows, err)
}

func (r *InflationRateRepo) UndoDeleteInflationRate(ctx context.Context, id uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	rows, err := r.q.RestoreInflationRate(ctx, db.RestoreInflationRateParams{ID: id, HouseholdID: hid, UpdatedBy: &user})
	return undoRows("restore inflation rate", rows, err)
}

// ----- position restores --------------------------------------------------

// A position delete cascades its tombstone to the children that were live at
// the time, stamping them with the parent's own `deleted_at` (one transaction,
// one now()). The undo restores exactly those children, matched on that shared
// timestamp, so a snapshot or transaction the user had deleted or replaced
// earlier stays archived. Children go first, while the parent is still deleted,
// and a miss on the parent (another household, wrong subtype, not deleted)
// rolls the whole thing back.

func (r *AssetRepo) undoDeleteAsset(ctx context.Context, id uuid.UUID, subtype string) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	if _, err := qtx.CascadeRestoreAssetSnapshots(ctx, db.CascadeRestoreAssetSnapshotsParams{
		AssetID: id, HouseholdID: hid, UpdatedBy: &user,
	}); err != nil {
		return fmt.Errorf("cascade restore asset snapshots: %w", err)
	}
	rows, err := qtx.RestoreAsset(ctx, db.RestoreAssetParams{ID: id, HouseholdID: hid, UpdatedBy: &user, Subtype: subtype})
	if err := undoRows("restore asset", rows, err); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit restore asset: %w", err)
	}
	return nil
}

func (r *AssetRepo) UndoDeleteBankAccount(ctx context.Context, id uuid.UUID) error {
	return r.undoDeleteAsset(ctx, id, "bank_account")
}

func (r *AssetRepo) UndoDeleteProperty(ctx context.Context, id uuid.UUID) error {
	return r.undoDeleteAsset(ctx, id, "property")
}

func (r *AssetRepo) UndoDeleteVehicle(ctx context.Context, id uuid.UUID) error {
	return r.undoDeleteAsset(ctx, id, "vehicle")
}

func (r *LiabilityRepo) UndoDeleteLiability(ctx context.Context, id uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	if _, err := qtx.CascadeRestoreLiabilitySnapshots(ctx, db.CascadeRestoreLiabilitySnapshotsParams{
		LiabilityID: id, HouseholdID: hid, UpdatedBy: &user,
	}); err != nil {
		return fmt.Errorf("cascade restore liability snapshots: %w", err)
	}
	rows, err := qtx.RestoreLiability(ctx, db.RestoreLiabilityParams{ID: id, HouseholdID: hid, UpdatedBy: &user})
	if err := undoRows("restore liability", rows, err); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit restore liability: %w", err)
	}
	return nil
}

func (r *ReceivableRepo) UndoDeleteReceivable(ctx context.Context, id uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	if _, err := qtx.CascadeRestoreReceivableSnapshots(ctx, db.CascadeRestoreReceivableSnapshotsParams{
		ReceivableID: id, HouseholdID: hid, UpdatedBy: &user,
	}); err != nil {
		return fmt.Errorf("cascade restore receivable snapshots: %w", err)
	}
	rows, err := qtx.RestoreReceivable(ctx, db.RestoreReceivableParams{ID: id, HouseholdID: hid, UpdatedBy: &user})
	if err := undoRows("restore receivable", rows, err); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit restore receivable: %w", err)
	}
	return nil
}

func (r *InvestmentRepo) undoDeleteInvestment(ctx context.Context, id uuid.UUID, subtype string) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	if _, err := qtx.CascadeRestoreInvestmentSnapshots(ctx, db.CascadeRestoreInvestmentSnapshotsParams{
		InvestmentID: id, HouseholdID: hid, UpdatedBy: &user,
	}); err != nil {
		return fmt.Errorf("cascade restore investment snapshots: %w", err)
	}
	if _, err := qtx.CascadeRestoreInvestmentTransactions(ctx, db.CascadeRestoreInvestmentTransactionsParams{
		InvestmentID: id, HouseholdID: hid, UpdatedBy: &user,
	}); err != nil {
		return fmt.Errorf("cascade restore investment transactions: %w", err)
	}
	rows, err := qtx.RestoreInvestment(ctx, db.RestoreInvestmentParams{ID: id, HouseholdID: hid, UpdatedBy: &user, Subtype: subtype})
	if err := undoRows("restore investment", rows, err); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit restore investment: %w", err)
	}
	return nil
}

func (r *InvestmentRepo) UndoDeleteStock(ctx context.Context, id uuid.UUID) error {
	return r.undoDeleteInvestment(ctx, id, "stock")
}

func (r *InvestmentRepo) UndoDeleteMutualFund(ctx context.Context, id uuid.UUID) error {
	return r.undoDeleteInvestment(ctx, id, "mutual_fund")
}

func (r *InvestmentRepo) UndoDeleteGold(ctx context.Context, id uuid.UUID) error {
	return r.undoDeleteInvestment(ctx, id, "gold")
}

func (r *InvestmentRepo) UndoDeleteBond(ctx context.Context, id uuid.UUID) error {
	return r.undoDeleteInvestment(ctx, id, "bond")
}

func (r *InvestmentRepo) UndoDeleteTimeDeposit(ctx context.Context, id uuid.UUID) error {
	return r.undoDeleteInvestment(ctx, id, "time_deposit")
}

// ----- undo a replace -----------------------------------------------------

// snapshotSlot is the (position, month) a snapshot occupies.
type snapshotSlot struct {
	position uuid.UUID
	month    time.Time
}

// snapshotUndoOps adapts one snapshot group's sqlc queries to the shared
// undo-replace flow; the four groups are parallel tables, like the rest of the
// package.
type snapshotUndoOps struct {
	slot    func(ctx context.Context, q *db.Queries, id, hid uuid.UUID) (snapshotSlot, error)
	archive func(ctx context.Context, q *db.Queries, id, hid, user uuid.UUID) (int64, error)
	restore func(ctx context.Context, q *db.Queries, id, hid, user uuid.UUID) (int64, error)
}

// undoReplaceSnapshot archives replacementID and restores restoreID in one
// transaction. It refuses (ErrUndoReplacementChanged, nothing changed) unless
// the replacement is the live row for the restored row's own slot: edited again,
// deleted, or from another slot all fail the same way. An id outside the
// caller's household is ErrNotFound.
func undoReplaceSnapshot(ctx context.Context, pool *pgxpool.Pool, q *db.Queries, ops snapshotUndoOps, restoreID, replacementID uuid.UUID) error {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("undo replace: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := q.WithTx(tx)

	orig, err := ops.slot(ctx, qtx, restoreID, hid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("undo replace: load original: %w", err)
	}
	repl, err := ops.slot(ctx, qtx, replacementID, hid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("undo replace: load replacement: %w", err)
	}
	if restoreID == replacementID || orig.position != repl.position || !orig.month.Equal(repl.month) {
		return ErrUndoReplacementChanged
	}

	rows, err := ops.archive(ctx, qtx, replacementID, hid, user)
	if err != nil {
		return fmt.Errorf("undo replace: archive replacement: %w", err)
	}
	if rows == 0 {
		return ErrUndoReplacementChanged
	}
	rows, err = ops.restore(ctx, qtx, restoreID, hid, user)
	if err := undoRows("undo replace: restore original", rows, err); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("undo replace: commit: %w", err)
	}
	return nil
}

func (r *AssetRepo) UndoReplaceAssetSnapshot(ctx context.Context, restoreID, replacementID uuid.UUID) error {
	return undoReplaceSnapshot(ctx, r.pool, r.q, snapshotUndoOps{
		slot: func(ctx context.Context, q *db.Queries, id, hid uuid.UUID) (snapshotSlot, error) {
			s, err := q.GetAssetSnapshotAnyState(ctx, db.GetAssetSnapshotAnyStateParams{ID: id, HouseholdID: hid})
			return snapshotSlot{s.AssetID, s.YearMonth}, err
		},
		archive: func(ctx context.Context, q *db.Queries, id, hid, user uuid.UUID) (int64, error) {
			return q.SoftDeleteAssetSnapshot(ctx, db.SoftDeleteAssetSnapshotParams{ID: id, HouseholdID: hid, UpdatedBy: &user})
		},
		restore: func(ctx context.Context, q *db.Queries, id, hid, user uuid.UUID) (int64, error) {
			return q.RestoreAssetSnapshot(ctx, db.RestoreAssetSnapshotParams{ID: id, HouseholdID: hid, UpdatedBy: &user})
		},
	}, restoreID, replacementID)
}

func (r *LiabilityRepo) UndoReplaceLiabilitySnapshot(ctx context.Context, restoreID, replacementID uuid.UUID) error {
	return undoReplaceSnapshot(ctx, r.pool, r.q, snapshotUndoOps{
		slot: func(ctx context.Context, q *db.Queries, id, hid uuid.UUID) (snapshotSlot, error) {
			s, err := q.GetLiabilitySnapshotAnyState(ctx, db.GetLiabilitySnapshotAnyStateParams{ID: id, HouseholdID: hid})
			return snapshotSlot{s.LiabilityID, s.YearMonth}, err
		},
		archive: func(ctx context.Context, q *db.Queries, id, hid, user uuid.UUID) (int64, error) {
			return q.SoftDeleteLiabilitySnapshot(ctx, db.SoftDeleteLiabilitySnapshotParams{ID: id, HouseholdID: hid, UpdatedBy: &user})
		},
		restore: func(ctx context.Context, q *db.Queries, id, hid, user uuid.UUID) (int64, error) {
			return q.RestoreLiabilitySnapshot(ctx, db.RestoreLiabilitySnapshotParams{ID: id, HouseholdID: hid, UpdatedBy: &user})
		},
	}, restoreID, replacementID)
}

func (r *ReceivableRepo) UndoReplaceReceivableSnapshot(ctx context.Context, restoreID, replacementID uuid.UUID) error {
	return undoReplaceSnapshot(ctx, r.pool, r.q, snapshotUndoOps{
		slot: func(ctx context.Context, q *db.Queries, id, hid uuid.UUID) (snapshotSlot, error) {
			s, err := q.GetReceivableSnapshotAnyState(ctx, db.GetReceivableSnapshotAnyStateParams{ID: id, HouseholdID: hid})
			return snapshotSlot{s.ReceivableID, s.YearMonth}, err
		},
		archive: func(ctx context.Context, q *db.Queries, id, hid, user uuid.UUID) (int64, error) {
			return q.SoftDeleteReceivableSnapshot(ctx, db.SoftDeleteReceivableSnapshotParams{ID: id, HouseholdID: hid, UpdatedBy: &user})
		},
		restore: func(ctx context.Context, q *db.Queries, id, hid, user uuid.UUID) (int64, error) {
			return q.RestoreReceivableSnapshot(ctx, db.RestoreReceivableSnapshotParams{ID: id, HouseholdID: hid, UpdatedBy: &user})
		},
	}, restoreID, replacementID)
}

func (r *InvestmentRepo) UndoReplaceInvestmentSnapshot(ctx context.Context, restoreID, replacementID uuid.UUID) error {
	return undoReplaceSnapshot(ctx, r.pool, r.q, snapshotUndoOps{
		slot: func(ctx context.Context, q *db.Queries, id, hid uuid.UUID) (snapshotSlot, error) {
			s, err := q.GetInvestmentSnapshotAnyState(ctx, db.GetInvestmentSnapshotAnyStateParams{ID: id, HouseholdID: hid})
			return snapshotSlot{s.InvestmentID, s.YearMonth}, err
		},
		archive: func(ctx context.Context, q *db.Queries, id, hid, user uuid.UUID) (int64, error) {
			return q.SoftDeleteInvestmentSnapshot(ctx, db.SoftDeleteInvestmentSnapshotParams{ID: id, HouseholdID: hid, UpdatedBy: &user})
		},
		restore: func(ctx context.Context, q *db.Queries, id, hid, user uuid.UUID) (int64, error) {
			return q.RestoreInvestmentSnapshot(ctx, db.RestoreInvestmentSnapshotParams{ID: id, HouseholdID: hid, UpdatedBy: &user})
		},
	}, restoreID, replacementID)
}
