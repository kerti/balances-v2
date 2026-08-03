package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// Position lifecycle (ADR-0009). Each group defines its own status enum; the
// DB CHECK on each core table enforces the same value set, and migration 00012
// enforces the status/terminated_at biconditional. The sets below let the repo
// reject an unknown status with a clean 400 before it reaches the DB (where a
// constraint violation would otherwise surface as a 500).
const (
	StatusActive = "active"
	// StatusMatured is the terminal status a Maturity transaction flips a Bond
	// or TimeDeposit to (ADR-0009). Named because the flip is automatic in
	// CreateInvestmentTransaction, not user-supplied.
	StatusMatured = "matured"
)

var (
	assetStatuses      = []string{StatusActive, "closed", "sold", "disposed"}
	liabilityStatuses  = []string{StatusActive, "paid_off", "forgiven", "written_off"}
	receivableStatuses = []string{StatusActive, "collected", "written_off"}
	investmentStatuses = []string{StatusActive, "sold", "matured"}
)

// LifecycleParams is the group-agnostic input for a lifecycle mutation. The
// terminate action is the same shape across all four groups; only the set of
// valid status values differs.
type LifecycleParams struct {
	Status          string
	TerminatedAt    *time.Time
	TerminationNote *string
}

// validatePositionLifecycle enforces, for any group: (a) status is one the
// group defines, and (b) the status/terminated_at biconditional — active means
// no termination date, any terminal status means a date is present. Returns
// ErrInvalidLifecycle (→ 400) on any violation.
func validatePositionLifecycle(allowed []string, p LifecycleParams) error {
	known := false
	for _, s := range allowed {
		if s == p.Status {
			known = true
			break
		}
	}
	if !known {
		return fmt.Errorf("%w: unknown status %q", ErrInvalidLifecycle, p.Status)
	}
	if p.Status == StatusActive && p.TerminatedAt != nil {
		return fmt.Errorf("%w: active position must not carry a termination date", ErrInvalidLifecycle)
	}
	if p.Status != StatusActive && p.TerminatedAt == nil {
		return fmt.Errorf("%w: %s position requires a termination date", ErrInvalidLifecycle, p.Status)
	}
	return nil
}

func (r *AssetRepo) UpdateAssetLifecycle(ctx context.Context, id uuid.UUID, p LifecycleParams) (*db.Asset, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := validatePositionLifecycle(assetStatuses, p); err != nil {
		return nil, err
	}

	// Pre-read for the native currency the close snapshot is denominated in and
	// the prior terminated_at (the month whose close snapshot an un-terminate
	// reverses).
	asset, err := r.q.GetAssetByID(ctx, db.GetAssetByIDParams{ID: id, HouseholdID: hid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get asset for lifecycle: %w", err)
	}
	priorTerminatedAt := asset.TerminatedAt

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	row, err := qtx.UpdateAssetLifecycle(ctx, db.UpdateAssetLifecycleParams{
		ID:              id,
		HouseholdID:     hid,
		Status:          p.Status,
		TerminatedAt:    p.TerminatedAt,
		TerminationNote: p.TerminationNote,
		UpdatedBy:       &user,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update asset lifecycle: %w", err)
	}

	ops := assetCloseSnapshotOps(qtx, id, asset.NativeCurrency, user, hid)
	if err := applyCloseSnapshot(ctx, ops, p, priorTerminatedAt); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return &row, nil
}

func (r *LiabilityRepo) UpdateLiabilityLifecycle(ctx context.Context, id uuid.UUID, p LifecycleParams) (*db.Liability, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := validatePositionLifecycle(liabilityStatuses, p); err != nil {
		return nil, err
	}

	liability, err := r.q.GetLiabilityByID(ctx, db.GetLiabilityByIDParams{ID: id, HouseholdID: hid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get liability for lifecycle: %w", err)
	}
	priorTerminatedAt := liability.TerminatedAt

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	row, err := qtx.UpdateLiabilityLifecycle(ctx, db.UpdateLiabilityLifecycleParams{
		ID:              id,
		HouseholdID:     hid,
		Status:          p.Status,
		TerminatedAt:    p.TerminatedAt,
		TerminationNote: p.TerminationNote,
		UpdatedBy:       &user,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update liability lifecycle: %w", err)
	}

	ops := liabilityCloseSnapshotOps(qtx, id, liability.NativeCurrency, user, hid)
	if err := applyCloseSnapshot(ctx, ops, p, priorTerminatedAt); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return &row, nil
}

func (r *ReceivableRepo) UpdateReceivableLifecycle(ctx context.Context, id uuid.UUID, p LifecycleParams) (*db.Receivable, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := validatePositionLifecycle(receivableStatuses, p); err != nil {
		return nil, err
	}

	receivable, err := r.q.GetReceivableByID(ctx, db.GetReceivableByIDParams{ID: id, HouseholdID: hid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get receivable for lifecycle: %w", err)
	}
	priorTerminatedAt := receivable.TerminatedAt

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	row, err := qtx.UpdateReceivableLifecycle(ctx, db.UpdateReceivableLifecycleParams{
		ID:              id,
		HouseholdID:     hid,
		Status:          p.Status,
		TerminatedAt:    p.TerminatedAt,
		TerminationNote: p.TerminationNote,
		UpdatedBy:       &user,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update receivable lifecycle: %w", err)
	}

	ops := receivableCloseSnapshotOps(qtx, id, receivable.NativeCurrency, user, hid)
	if err := applyCloseSnapshot(ctx, ops, p, priorTerminatedAt); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return &row, nil
}

// UpdateInvestmentLifecycle terminates or reactivates an investment. Investment
// was the first group to write close-snapshot data on a lifecycle flip, because
// it is the only group with cash-flow transactions feeding the derived return
// (ADR-0008): without a truthful 0 close snapshot the return formula (Δvalue +
// cash_out − cash_in) double-counts the payout, the bug #17 introduced for
// Maturity and #25 removed at its source. ADR-0052 generalised the rule to all
// four groups — a position that left the portfolio during month M holds nothing
// at month-end whatever group it belongs to — so the branching below now lives
// in applyCloseSnapshot and the only investment-specific part left is the
// subtype-shaped 0 row.
func (r *InvestmentRepo) UpdateInvestmentLifecycle(ctx context.Context, id uuid.UUID, p LifecycleParams) (*db.Investment, error) {
	user, hid, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := validatePositionLifecycle(investmentStatuses, p); err != nil {
		return nil, err
	}

	// Pre-read for the subtype (close-snapshot shape) and the prior
	// terminated_at (the month whose close snapshot an un-terminate clears).
	inv, err := r.q.GetInvestmentByID(ctx, db.GetInvestmentByIDParams{ID: id, HouseholdID: hid})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get investment for lifecycle: %w", err)
	}
	priorTerminatedAt := inv.TerminatedAt

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := r.q.WithTx(tx)

	row, err := qtx.UpdateInvestmentLifecycle(ctx, db.UpdateInvestmentLifecycleParams{
		ID:              id,
		HouseholdID:     hid,
		Status:          p.Status,
		TerminatedAt:    p.TerminatedAt,
		TerminationNote: p.TerminationNote,
		UpdatedBy:       &user,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update investment lifecycle: %w", err)
	}

	ops := investmentCloseSnapshotOps(qtx, id, inv.Subtype, inv.NativeCurrency, user, hid)
	if err := applyCloseSnapshot(ctx, ops, p, priorTerminatedAt); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return &row, nil
}

// closeSnapshotRow is the group-agnostic slice of a snapshot row the
// close-snapshot machinery reads. createdAt doubles as the discriminator that
// pairs a close row with the row it displaced — see closeSnapshotOps.archivedAt.
type closeSnapshotRow struct {
	id        uuid.UUID
	amount    decimal.Decimal
	createdAt pgtype.Timestamptz
}

// closeSnapshotOps binds one group's six snapshot statements so the
// terminate/un-terminate logic below can be written once for all four
// (ADR-0052 §1). liveAt and archivedAt return pgx.ErrNoRows when nothing
// matches; every other method treats "no row touched" as an error, because each
// is only ever called on a row just read inside the same transaction.
type closeSnapshotOps struct {
	liveAt      func(ctx context.Context, month time.Time) (closeSnapshotRow, error)
	archivedAt  func(ctx context.Context, month time.Time, displacedAt pgtype.Timestamptz) (uuid.UUID, error)
	archive     func(ctx context.Context, id uuid.UUID) error
	restore     func(ctx context.Context, id uuid.UUID) error
	insertZero  func(ctx context.Context, month time.Time, asOf time.Time) error
	refreshZero func(ctx context.Context, id uuid.UUID, asOf time.Time) error
}

// applyCloseSnapshot is the whole of ADR-0052 §1–2: a terminal flip writes a
// truthful 0-value close snapshot at the termination month, and the
// un-terminate correction (ADR-0009's correction affordance) puts back what was
// there. Neither branch fires when the flip changes nothing about termination —
// an active→active edit, or a status change between two terminal values that
// keeps the same month, still re-asserts the close row, which is idempotent in
// effect if not in rows.
func applyCloseSnapshot(ctx context.Context, ops closeSnapshotOps, p LifecycleParams, priorTerminatedAt *time.Time) error {
	switch {
	case p.Status != StatusActive && p.TerminatedAt != nil:
		return writeCloseSnapshot(ctx, ops, *p.TerminatedAt)
	case p.Status == StatusActive && priorTerminatedAt != nil:
		return revertCloseSnapshot(ctx, ops, *priorTerminatedAt)
	}
	return nil
}

// writeCloseSnapshot displaces whatever the user recorded at the termination
// month and inserts the 0-value close row in its place. Displacement is
// soft-delete + insert rather than #25's in-place overwrite (ADR-0052 §2): the
// partial unique index is `(position_id, year_month) WHERE deleted_at IS NULL`,
// so the archived row survives alongside the close row and un-terminate can
// hand it back. The archive UPDATE and the INSERT share one transaction, so the
// archived row's deleted_at and the close row's created_at are the same
// transaction timestamp — that pairing is what revertCloseSnapshot matches on.
func writeCloseSnapshot(ctx context.Context, ops closeSnapshotOps, terminatedAt time.Time) error {
	month := monthStart(terminatedAt)
	live, err := ops.liveAt(ctx, month)
	switch {
	case err == nil:
		if live.amount.IsZero() {
			// The month already holds a close row — a re-asserted terminal flip,
			// or a maturity-date edit that stayed inside the month. Refresh it in
			// place rather than displacing it: that keeps its created_at, which
			// is the only handle on the row it originally displaced, and stops
			// repeated flips piling up tombstones.
			return ops.refreshZero(ctx, live.id, terminatedAt)
		}
		if err := ops.archive(ctx, live.id); err != nil {
			return err
		}
	case errors.Is(err, pgx.ErrNoRows):
		// No snapshot recorded that month — nothing to displace.
	default:
		return err
	}
	return ops.insertZero(ctx, month, terminatedAt)
}

// revertCloseSnapshot is writeCloseSnapshot's inverse. It only ever removes a
// zero-amount row: a non-zero live snapshot at the termination month is one the
// user recorded while the position was terminated (snapshots, unlike
// transactions, are not blocked on a terminated position), and theirs wins over
// anything we archived. With the close row gone the month is free, so the row it
// displaced is restored — leaving the reactivated position carrying its own
// recorded value, not 0 and not a hole (INV-LIFECYCLE-04).
func revertCloseSnapshot(ctx context.Context, ops closeSnapshotOps, priorTerminatedAt time.Time) error {
	month := monthStart(priorTerminatedAt)
	live, err := ops.liveAt(ctx, month)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	if !live.amount.IsZero() {
		return nil
	}
	if err := ops.archive(ctx, live.id); err != nil {
		return err
	}
	displaced, err := ops.archivedAt(ctx, month, live.createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// The close row displaced nothing, or what it displaced was itself
			// a close row from an earlier terminate cycle (the archivedAt query
			// filters those out). Either way the month is correctly left empty
			// and the carry-forward rule takes over.
			return nil
		}
		return err
	}
	return ops.restore(ctx, displaced)
}

func assetCloseSnapshotOps(qtx *db.Queries, id uuid.UUID, currency string, user, hid uuid.UUID) closeSnapshotOps {
	return closeSnapshotOps{
		liveAt: func(ctx context.Context, month time.Time) (closeSnapshotRow, error) {
			s, err := qtx.GetAssetSnapshotAtMonth(ctx, db.GetAssetSnapshotAtMonthParams{
				AssetID: id, YearMonth: month, HouseholdID: hid,
			})
			if err != nil {
				return closeSnapshotRow{}, err
			}
			return closeSnapshotRow{id: s.ID, amount: s.Amount, createdAt: s.CreatedAt}, nil
		},
		archivedAt: func(ctx context.Context, month time.Time, displacedAt pgtype.Timestamptz) (uuid.UUID, error) {
			s, err := qtx.GetArchivedAssetSnapshotAtMonth(ctx, db.GetArchivedAssetSnapshotAtMonthParams{
				AssetID: id, YearMonth: month, HouseholdID: hid, ArchivedAt: displacedAt,
			})
			if err != nil {
				return uuid.Nil, err
			}
			return s.ID, nil
		},
		archive: func(ctx context.Context, snapID uuid.UUID) error {
			n, err := qtx.SoftDeleteAssetSnapshot(ctx, db.SoftDeleteAssetSnapshotParams{
				ID: snapID, HouseholdID: hid, UpdatedBy: &user,
			})
			return closeSnapshotWriteErr("archive asset snapshot", n, err)
		},
		restore: func(ctx context.Context, snapID uuid.UUID) error {
			n, err := qtx.RestoreAssetSnapshot(ctx, db.RestoreAssetSnapshotParams{
				ID: snapID, HouseholdID: hid, UpdatedBy: &user,
			})
			return closeSnapshotWriteErr("restore asset snapshot", n, err)
		},
		insertZero: func(ctx context.Context, month time.Time, asOf time.Time) error {
			if _, err := qtx.CreateAssetSnapshot(ctx, db.CreateAssetSnapshotParams{
				ID:          id,
				YearMonth:   month,
				Amount:      decimal.Zero,
				Currency:    currency,
				AsOfDate:    &asOf,
				CreatedBy:   &user,
				HouseholdID: hid,
			}); err != nil {
				return fmt.Errorf("close snapshot on termination: %w", err)
			}
			return nil
		},
		refreshZero: func(ctx context.Context, snapID uuid.UUID, asOf time.Time) error {
			if _, err := qtx.UpdateAssetSnapshot(ctx, db.UpdateAssetSnapshotParams{
				ID:          snapID,
				HouseholdID: hid,
				Amount:      decimal.Zero,
				Currency:    currency,
				AsOfDate:    &asOf,
				UpdatedBy:   &user,
			}); err != nil {
				return fmt.Errorf("refresh close snapshot: %w", err)
			}
			return nil
		},
	}
}

func liabilityCloseSnapshotOps(qtx *db.Queries, id uuid.UUID, currency string, user, hid uuid.UUID) closeSnapshotOps {
	return closeSnapshotOps{
		liveAt: func(ctx context.Context, month time.Time) (closeSnapshotRow, error) {
			s, err := qtx.GetLiabilitySnapshotAtMonth(ctx, db.GetLiabilitySnapshotAtMonthParams{
				LiabilityID: id, YearMonth: month, HouseholdID: hid,
			})
			if err != nil {
				return closeSnapshotRow{}, err
			}
			return closeSnapshotRow{id: s.ID, amount: s.Amount, createdAt: s.CreatedAt}, nil
		},
		archivedAt: func(ctx context.Context, month time.Time, displacedAt pgtype.Timestamptz) (uuid.UUID, error) {
			s, err := qtx.GetArchivedLiabilitySnapshotAtMonth(ctx, db.GetArchivedLiabilitySnapshotAtMonthParams{
				LiabilityID: id, YearMonth: month, HouseholdID: hid, ArchivedAt: displacedAt,
			})
			if err != nil {
				return uuid.Nil, err
			}
			return s.ID, nil
		},
		archive: func(ctx context.Context, snapID uuid.UUID) error {
			n, err := qtx.SoftDeleteLiabilitySnapshot(ctx, db.SoftDeleteLiabilitySnapshotParams{
				ID: snapID, HouseholdID: hid, UpdatedBy: &user,
			})
			return closeSnapshotWriteErr("archive liability snapshot", n, err)
		},
		restore: func(ctx context.Context, snapID uuid.UUID) error {
			n, err := qtx.RestoreLiabilitySnapshot(ctx, db.RestoreLiabilitySnapshotParams{
				ID: snapID, HouseholdID: hid, UpdatedBy: &user,
			})
			return closeSnapshotWriteErr("restore liability snapshot", n, err)
		},
		insertZero: func(ctx context.Context, month time.Time, asOf time.Time) error {
			if _, err := qtx.CreateLiabilitySnapshot(ctx, db.CreateLiabilitySnapshotParams{
				ID:          id,
				YearMonth:   month,
				Amount:      decimal.Zero,
				Currency:    currency,
				AsOfDate:    &asOf,
				CreatedBy:   &user,
				HouseholdID: hid,
			}); err != nil {
				return fmt.Errorf("close snapshot on termination: %w", err)
			}
			return nil
		},
		refreshZero: func(ctx context.Context, snapID uuid.UUID, asOf time.Time) error {
			if _, err := qtx.UpdateLiabilitySnapshot(ctx, db.UpdateLiabilitySnapshotParams{
				ID:          snapID,
				HouseholdID: hid,
				Amount:      decimal.Zero,
				Currency:    currency,
				AsOfDate:    &asOf,
				UpdatedBy:   &user,
			}); err != nil {
				return fmt.Errorf("refresh close snapshot: %w", err)
			}
			return nil
		},
	}
}

func receivableCloseSnapshotOps(qtx *db.Queries, id uuid.UUID, currency string, user, hid uuid.UUID) closeSnapshotOps {
	return closeSnapshotOps{
		liveAt: func(ctx context.Context, month time.Time) (closeSnapshotRow, error) {
			s, err := qtx.GetReceivableSnapshotAtMonth(ctx, db.GetReceivableSnapshotAtMonthParams{
				ReceivableID: id, YearMonth: month, HouseholdID: hid,
			})
			if err != nil {
				return closeSnapshotRow{}, err
			}
			return closeSnapshotRow{id: s.ID, amount: s.Amount, createdAt: s.CreatedAt}, nil
		},
		archivedAt: func(ctx context.Context, month time.Time, displacedAt pgtype.Timestamptz) (uuid.UUID, error) {
			s, err := qtx.GetArchivedReceivableSnapshotAtMonth(ctx, db.GetArchivedReceivableSnapshotAtMonthParams{
				ReceivableID: id, YearMonth: month, HouseholdID: hid, ArchivedAt: displacedAt,
			})
			if err != nil {
				return uuid.Nil, err
			}
			return s.ID, nil
		},
		archive: func(ctx context.Context, snapID uuid.UUID) error {
			n, err := qtx.SoftDeleteReceivableSnapshot(ctx, db.SoftDeleteReceivableSnapshotParams{
				ID: snapID, HouseholdID: hid, UpdatedBy: &user,
			})
			return closeSnapshotWriteErr("archive receivable snapshot", n, err)
		},
		restore: func(ctx context.Context, snapID uuid.UUID) error {
			n, err := qtx.RestoreReceivableSnapshot(ctx, db.RestoreReceivableSnapshotParams{
				ID: snapID, HouseholdID: hid, UpdatedBy: &user,
			})
			return closeSnapshotWriteErr("restore receivable snapshot", n, err)
		},
		insertZero: func(ctx context.Context, month time.Time, asOf time.Time) error {
			if _, err := qtx.CreateReceivableSnapshot(ctx, db.CreateReceivableSnapshotParams{
				ID:          id,
				YearMonth:   month,
				Amount:      decimal.Zero,
				Currency:    currency,
				AsOfDate:    &asOf,
				CreatedBy:   &user,
				HouseholdID: hid,
			}); err != nil {
				return fmt.Errorf("close snapshot on termination: %w", err)
			}
			return nil
		},
		refreshZero: func(ctx context.Context, snapID uuid.UUID, asOf time.Time) error {
			if _, err := qtx.UpdateReceivableSnapshot(ctx, db.UpdateReceivableSnapshotParams{
				ID:          snapID,
				HouseholdID: hid,
				Amount:      decimal.Zero,
				Currency:    currency,
				AsOfDate:    &asOf,
				UpdatedBy:   &user,
			}); err != nil {
				return fmt.Errorf("refresh close snapshot: %w", err)
			}
			return nil
		},
	}
}

// investmentCloseSnapshotOps differs from the other three only in insertZero:
// the investment_snapshot_shape CHECK demands quantity/price_per_unit for
// stock/mutual_fund/gold and accrued_interest for bond/time_deposit, all 0 here.
func investmentCloseSnapshotOps(qtx *db.Queries, id uuid.UUID, subtype, currency string, user, hid uuid.UUID) closeSnapshotOps {
	return closeSnapshotOps{
		liveAt: func(ctx context.Context, month time.Time) (closeSnapshotRow, error) {
			s, err := qtx.GetInvestmentSnapshotAtMonth(ctx, db.GetInvestmentSnapshotAtMonthParams{
				InvestmentID: id, YearMonth: month, HouseholdID: hid,
			})
			if err != nil {
				return closeSnapshotRow{}, err
			}
			return closeSnapshotRow{id: s.ID, amount: s.Amount, createdAt: s.CreatedAt}, nil
		},
		archivedAt: func(ctx context.Context, month time.Time, displacedAt pgtype.Timestamptz) (uuid.UUID, error) {
			s, err := qtx.GetArchivedInvestmentSnapshotAtMonth(ctx, db.GetArchivedInvestmentSnapshotAtMonthParams{
				InvestmentID: id, YearMonth: month, HouseholdID: hid, ArchivedAt: displacedAt,
			})
			if err != nil {
				return uuid.Nil, err
			}
			return s.ID, nil
		},
		archive: func(ctx context.Context, snapID uuid.UUID) error {
			n, err := qtx.SoftDeleteInvestmentSnapshot(ctx, db.SoftDeleteInvestmentSnapshotParams{
				ID: snapID, HouseholdID: hid, UpdatedBy: &user,
			})
			return closeSnapshotWriteErr("archive investment snapshot", n, err)
		},
		restore: func(ctx context.Context, snapID uuid.UUID) error {
			n, err := qtx.RestoreInvestmentSnapshot(ctx, db.RestoreInvestmentSnapshotParams{
				ID: snapID, HouseholdID: hid, UpdatedBy: &user,
			})
			return closeSnapshotWriteErr("restore investment snapshot", n, err)
		},
		insertZero: func(ctx context.Context, month time.Time, asOf time.Time) error {
			zero := decimal.Zero
			params := db.CreateInvestmentSnapshotParams{
				ID:          id,
				YearMonth:   month,
				Amount:      zero,
				Currency:    currency,
				AsOfDate:    &asOf,
				CreatedBy:   &user,
				HouseholdID: hid,
			}
			switch subtype {
			case "bond", "time_deposit":
				params.AccruedInterest = &zero
			default: // stock, mutual_fund, gold
				params.Quantity = &zero
				params.PricePerUnit = &zero
			}
			if _, err := qtx.CreateInvestmentSnapshot(ctx, params); err != nil {
				return fmt.Errorf("close snapshot on termination: %w", err)
			}
			return nil
		},
		refreshZero: func(ctx context.Context, snapID uuid.UUID, asOf time.Time) error {
			zero := decimal.Zero
			params := db.UpdateInvestmentSnapshotParams{
				ID:          snapID,
				HouseholdID: hid,
				Amount:      zero,
				Currency:    currency,
				AsOfDate:    &asOf,
				UpdatedBy:   &user,
			}
			switch subtype {
			case "bond", "time_deposit":
				params.AccruedInterest = &zero
			default: // stock, mutual_fund, gold
				params.Quantity = &zero
				params.PricePerUnit = &zero
			}
			if _, err := qtx.UpdateInvestmentSnapshot(ctx, params); err != nil {
				return fmt.Errorf("refresh close snapshot: %w", err)
			}
			return nil
		},
	}
}

// closeSnapshotWriteErr folds the :execrows contract of the archive/restore
// statements into one check. Both are called only on a row read in the same
// transaction, so RowsAffected == 0 means the row moved underneath us — an
// anomaly worth rolling the whole flip back for, not something to swallow.
func closeSnapshotWriteErr(what string, rows int64, err error) error {
	if err != nil {
		return fmt.Errorf("%s: %w", what, err)
	}
	if rows == 0 {
		return fmt.Errorf("%s: no row affected", what)
	}
	return nil
}
