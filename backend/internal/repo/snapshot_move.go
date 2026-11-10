package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// Moving a Snapshot to another month (ADR-0046 amendment, ADR-0058 §1-§2). A
// snapshot's year_month is immutable (#76), so a move is the replace mechanism
// pointed at a different slot, in one transaction: the original is archived, the
// target month's live row (if any) is archived the same way, and the same values
// are inserted in the target month. `supersedes` stays null on the new row; it
// keeps its one meaning, "a close row displaced this" (ADR-0052 §2).
//
// The statement date (`as_of_date`) is not carried: the month-integrity CHECK
// pins it to its own month, so on any move to a different month it would fall
// outside the target. The new row's date is NULL, never invented (INV-SNAPSHOTS-13).
//
// Unlike an Edit, a move never takes the "identical values" shortcut: even when
// the target month already holds the very same numbers, the target row is
// archived and re-inserted, because the response (and its Undo) must be able to
// name exactly which row was displaced.
//
// Report months need no extra call: every archive and insert bumps `updated_at`,
// the report staleness watermark, and a stale household regenerates all its
// months on the next read (ADR-0006), so the source and the target month are
// both rebuilt.
//
// The four groups are parallel implementations around one shared flow, like the
// rest of this package.

// moveSource is the group-agnostic slice of the snapshot being moved that the
// shared flow needs to guard the move; the group's own insert closure carries
// the value columns.
type moveSource struct {
	position     uuid.UUID
	month        time.Time
	amount       decimal.Decimal
	supersedes   *uuid.UUID
	terminatedAt *time.Time
}

// isClose reports whether the row is a Position's close snapshot (ADR-0052 §2):
// the 0-value reading its terminal flip wrote at the termination month. A close
// row that displaced something names it in `supersedes`; one that displaced
// nothing is recognised the way un-terminate recognises it
// (revertCloseSnapshot): a zero amount at the termination month of a terminated
// Position.
func (m moveSource) isClose() bool {
	if m.supersedes != nil {
		return true
	}
	return m.terminatedAt != nil && m.amount.IsZero() && monthStart(*m.terminatedAt).Equal(monthStart(m.month))
}

// snapshotMoveOps adapts one group's sqlc queries to the shared move flow. The
// insert closure closes over the loaded source row so it can copy the group's
// value columns.
type snapshotMoveOps[S any] struct {
	load    func(ctx context.Context, q *db.Queries, id, hid uuid.UUID) (moveSource, error)
	checkTo func(ctx context.Context, q *db.Queries, hid uuid.UUID, src moveSource, target time.Time) error
	archive func(ctx context.Context, q *db.Queries, id, hid, user uuid.UUID) (int64, error)
	// liveAt returns the id of the live row at the target month, nil when empty.
	liveAt func(ctx context.Context, q *db.Queries, src moveSource, target time.Time, hid uuid.UUID) (*uuid.UUID, error)
	insert func(ctx context.Context, q *db.Queries, src moveSource, target time.Time, user, hid uuid.UUID) (S, error)
}

func moveSnapshot[S any](ctx context.Context, pool *pgxpool.Pool, q *db.Queries, ops snapshotMoveOps[S], snapshotID uuid.UUID, target time.Time) (S, *uuid.UUID, error) {
	var zero S
	user, hid, err := currentUser(ctx)
	if err != nil {
		return zero, nil, err
	}
	target = monthStart(target)
	tx, err := pool.Begin(ctx)
	if err != nil {
		return zero, nil, fmt.Errorf("move snapshot: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := q.WithTx(tx)

	src, err := ops.load(ctx, qtx, snapshotID, hid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return zero, nil, ErrNotFound
		}
		return zero, nil, fmt.Errorf("move snapshot: load: %w", err)
	}
	if monthStart(src.month).Equal(target) {
		return zero, nil, ErrSnapshotMoveSameMonth
	}
	if src.isClose() {
		return zero, nil, ErrSnapshotCloseNotMovable
	}
	if err := ops.checkTo(ctx, qtx, hid, src, target); err != nil {
		return zero, nil, err
	}

	// Archive the original first, then whatever holds the target month, then
	// insert: the partial unique index only admits one live row per slot.
	rows, err := ops.archive(ctx, qtx, snapshotID, hid, user)
	if err != nil {
		return zero, nil, fmt.Errorf("move snapshot: archive original: %w", err)
	}
	if rows == 0 {
		return zero, nil, ErrNotFound
	}
	displaced, err := ops.liveAt(ctx, qtx, src, target, hid)
	if err != nil {
		return zero, nil, fmt.Errorf("move snapshot: read target month: %w", err)
	}
	if displaced != nil {
		rows, err := ops.archive(ctx, qtx, *displaced, hid, user)
		if err != nil {
			return zero, nil, fmt.Errorf("move snapshot: archive target: %w", err)
		}
		if rows == 0 {
			return zero, nil, ErrNotFound
		}
	}
	moved, err := ops.insert(ctx, qtx, src, target, user, hid)
	if err != nil {
		return zero, nil, mapSnapshotWriteErr("move snapshot: insert", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return zero, nil, fmt.Errorf("move snapshot: commit: %w", err)
	}
	return moved, displaced, nil
}

func (r *AssetRepo) MoveAssetSnapshot(ctx context.Context, snapshotID uuid.UUID, target time.Time) (*AssetSnapshotWrite, error) {
	var row db.AssetSnapshot
	moved, displaced, err := moveSnapshot(ctx, r.pool, r.q, snapshotMoveOps[db.AssetSnapshot]{
		load: func(ctx context.Context, q *db.Queries, id, hid uuid.UUID) (moveSource, error) {
			s, err := q.GetAssetSnapshotByID(ctx, db.GetAssetSnapshotByIDParams{ID: id, HouseholdID: hid})
			if err != nil {
				return moveSource{}, err
			}
			p, err := q.GetAssetByID(ctx, db.GetAssetByIDParams{ID: s.AssetID, HouseholdID: hid})
			if err != nil {
				return moveSource{}, err
			}
			row = s
			return moveSource{s.AssetID, s.YearMonth, s.Amount, s.Supersedes, p.TerminatedAt}, nil
		},
		checkTo: func(context.Context, *db.Queries, uuid.UUID, moveSource, time.Time) error { return nil },
		archive: func(ctx context.Context, q *db.Queries, id, hid, user uuid.UUID) (int64, error) {
			return q.SoftDeleteAssetSnapshot(ctx, db.SoftDeleteAssetSnapshotParams{ID: id, HouseholdID: hid, UpdatedBy: &user})
		},
		liveAt: func(ctx context.Context, q *db.Queries, src moveSource, target time.Time, hid uuid.UUID) (*uuid.UUID, error) {
			cur, err := q.GetAssetSnapshotAtMonth(ctx, db.GetAssetSnapshotAtMonthParams{AssetID: src.position, YearMonth: target, HouseholdID: hid})
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, nil
			}
			return &cur.ID, err
		},
		insert: func(ctx context.Context, q *db.Queries, src moveSource, target time.Time, user, hid uuid.UUID) (db.AssetSnapshot, error) {
			return q.CreateAssetSnapshot(ctx, db.CreateAssetSnapshotParams{
				ID: src.position, YearMonth: target, Amount: row.Amount, Currency: row.Currency,
				Description: row.Description, CreatedBy: &user, HouseholdID: hid,
			})
		},
	}, snapshotID, target)
	if err != nil {
		return nil, err
	}
	return &AssetSnapshotWrite{AssetSnapshot: moved, ReplacedID: displaced}, nil
}

func (r *LiabilityRepo) MoveLiabilitySnapshot(ctx context.Context, snapshotID uuid.UUID, target time.Time) (*LiabilitySnapshotWrite, error) {
	var row db.LiabilitySnapshot
	moved, displaced, err := moveSnapshot(ctx, r.pool, r.q, snapshotMoveOps[db.LiabilitySnapshot]{
		load: func(ctx context.Context, q *db.Queries, id, hid uuid.UUID) (moveSource, error) {
			s, err := q.GetLiabilitySnapshotByID(ctx, db.GetLiabilitySnapshotByIDParams{ID: id, HouseholdID: hid})
			if err != nil {
				return moveSource{}, err
			}
			p, err := q.GetLiabilityByID(ctx, db.GetLiabilityByIDParams{ID: s.LiabilityID, HouseholdID: hid})
			if err != nil {
				return moveSource{}, err
			}
			row = s
			return moveSource{s.LiabilityID, s.YearMonth, s.Amount, s.Supersedes, p.TerminatedAt}, nil
		},
		checkTo: func(context.Context, *db.Queries, uuid.UUID, moveSource, time.Time) error { return nil },
		archive: func(ctx context.Context, q *db.Queries, id, hid, user uuid.UUID) (int64, error) {
			return q.SoftDeleteLiabilitySnapshot(ctx, db.SoftDeleteLiabilitySnapshotParams{ID: id, HouseholdID: hid, UpdatedBy: &user})
		},
		liveAt: func(ctx context.Context, q *db.Queries, src moveSource, target time.Time, hid uuid.UUID) (*uuid.UUID, error) {
			cur, err := q.GetLiabilitySnapshotAtMonth(ctx, db.GetLiabilitySnapshotAtMonthParams{LiabilityID: src.position, YearMonth: target, HouseholdID: hid})
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, nil
			}
			return &cur.ID, err
		},
		insert: func(ctx context.Context, q *db.Queries, src moveSource, target time.Time, user, hid uuid.UUID) (db.LiabilitySnapshot, error) {
			return q.CreateLiabilitySnapshot(ctx, db.CreateLiabilitySnapshotParams{
				ID: src.position, YearMonth: target, Amount: row.Amount, Currency: row.Currency,
				Description: row.Description, CreatedBy: &user, HouseholdID: hid,
			})
		},
	}, snapshotID, target)
	if err != nil {
		return nil, err
	}
	return &LiabilitySnapshotWrite{LiabilitySnapshot: moved, ReplacedID: displaced}, nil
}

func (r *ReceivableRepo) MoveReceivableSnapshot(ctx context.Context, snapshotID uuid.UUID, target time.Time) (*ReceivableSnapshotWrite, error) {
	var row db.ReceivableSnapshot
	moved, displaced, err := moveSnapshot(ctx, r.pool, r.q, snapshotMoveOps[db.ReceivableSnapshot]{
		load: func(ctx context.Context, q *db.Queries, id, hid uuid.UUID) (moveSource, error) {
			s, err := q.GetReceivableSnapshotByID(ctx, db.GetReceivableSnapshotByIDParams{ID: id, HouseholdID: hid})
			if err != nil {
				return moveSource{}, err
			}
			p, err := q.GetReceivableByID(ctx, db.GetReceivableByIDParams{ID: s.ReceivableID, HouseholdID: hid})
			if err != nil {
				return moveSource{}, err
			}
			row = s
			return moveSource{s.ReceivableID, s.YearMonth, s.Amount, s.Supersedes, p.TerminatedAt}, nil
		},
		checkTo: func(context.Context, *db.Queries, uuid.UUID, moveSource, time.Time) error { return nil },
		archive: func(ctx context.Context, q *db.Queries, id, hid, user uuid.UUID) (int64, error) {
			return q.SoftDeleteReceivableSnapshot(ctx, db.SoftDeleteReceivableSnapshotParams{ID: id, HouseholdID: hid, UpdatedBy: &user})
		},
		liveAt: func(ctx context.Context, q *db.Queries, src moveSource, target time.Time, hid uuid.UUID) (*uuid.UUID, error) {
			cur, err := q.GetReceivableSnapshotAtMonth(ctx, db.GetReceivableSnapshotAtMonthParams{ReceivableID: src.position, YearMonth: target, HouseholdID: hid})
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, nil
			}
			return &cur.ID, err
		},
		insert: func(ctx context.Context, q *db.Queries, src moveSource, target time.Time, user, hid uuid.UUID) (db.ReceivableSnapshot, error) {
			return q.CreateReceivableSnapshot(ctx, db.CreateReceivableSnapshotParams{
				ID: src.position, YearMonth: target, Amount: row.Amount, Currency: row.Currency,
				Description: row.Description, CreatedBy: &user, HouseholdID: hid,
			})
		},
	}, snapshotID, target)
	if err != nil {
		return nil, err
	}
	return &ReceivableSnapshotWrite{ReceivableSnapshot: moved, ReplacedID: displaced}, nil
}

func (r *InvestmentRepo) MoveInvestmentSnapshot(ctx context.Context, snapshotID uuid.UUID, target time.Time) (*InvestmentSnapshotWrite, error) {
	var row db.InvestmentSnapshot
	moved, displaced, err := moveSnapshot(ctx, r.pool, r.q, snapshotMoveOps[db.InvestmentSnapshot]{
		load: func(ctx context.Context, q *db.Queries, id, hid uuid.UUID) (moveSource, error) {
			s, err := q.GetInvestmentSnapshotByID(ctx, db.GetInvestmentSnapshotByIDParams{ID: id, HouseholdID: hid})
			if err != nil {
				return moveSource{}, err
			}
			p, err := q.GetInvestmentByID(ctx, db.GetInvestmentByIDParams{ID: s.InvestmentID, HouseholdID: hid})
			if err != nil {
				return moveSource{}, err
			}
			row = s
			return moveSource{s.InvestmentID, s.YearMonth, s.Amount, s.Supersedes, p.TerminatedAt}, nil
		},
		// A time deposit's snapshots are confined to its term (issue #62): the
		// target month must lie inside it, like any other snapshot write.
		checkTo: func(ctx context.Context, q *db.Queries, hid uuid.UUID, src moveSource, target time.Time) error {
			inv, err := q.GetInvestmentByID(ctx, db.GetInvestmentByIDParams{ID: src.position, HouseholdID: hid})
			if err != nil {
				return fmt.Errorf("move snapshot: get parent investment: %w", err)
			}
			bounds, err := timeDepositBounds(ctx, q, inv)
			if err != nil {
				return err
			}
			return bounds.checkSnapshotMonth(target)
		},
		archive: func(ctx context.Context, q *db.Queries, id, hid, user uuid.UUID) (int64, error) {
			return q.SoftDeleteInvestmentSnapshot(ctx, db.SoftDeleteInvestmentSnapshotParams{ID: id, HouseholdID: hid, UpdatedBy: &user})
		},
		liveAt: func(ctx context.Context, q *db.Queries, src moveSource, target time.Time, hid uuid.UUID) (*uuid.UUID, error) {
			cur, err := q.GetInvestmentSnapshotAtMonth(ctx, db.GetInvestmentSnapshotAtMonthParams{InvestmentID: src.position, YearMonth: target, HouseholdID: hid})
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, nil
			}
			return &cur.ID, err
		},
		insert: func(ctx context.Context, q *db.Queries, src moveSource, target time.Time, user, hid uuid.UUID) (db.InvestmentSnapshot, error) {
			return q.CreateInvestmentSnapshot(ctx, db.CreateInvestmentSnapshotParams{
				ID: src.position, YearMonth: target, Amount: row.Amount, Currency: row.Currency,
				Quantity: row.Quantity, PricePerUnit: row.PricePerUnit, AccruedInterest: row.AccruedInterest,
				Description: row.Description, CreatedBy: &user, HouseholdID: hid,
			})
		},
	}, snapshotID, target)
	if err != nil {
		return nil, err
	}
	return &InvestmentSnapshotWrite{InvestmentSnapshot: moved, ReplacedID: displaced}, nil
}
