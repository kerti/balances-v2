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

// Replacing a Snapshot archives, never overwrites (ADR-0058 §1). Every write
// that changes the values of a position's snapshot for a month (Edit, bulk
// entry, import) soft-deletes the live row and inserts the new values inside
// the caller's transaction, so the earlier value survives as an ordinary
// soft-deleted row. `supersedes` stays NULL: it keeps its one meaning, "a close
// row displaced this" (ADR-0052 §2), because a compacted backup carries every
// archived row a live row supersedes.
//
// A write whose fields all equal the live row's is unchanged and writes
// nothing, so re-saving or re-importing identical data creates no archived rows.
// The four groups are parallel implementations (the sqlc queries are per-table),
// like the rest of this package.

type replaceOutcome int

const (
	replaceInserted  replaceOutcome = iota // the month had no live row
	replaceReplaced                        // a live row was archived and the new values inserted
	replaceUnchanged                       // identical to the live row, nothing written
)

// BulkResult is the tally of a bulk save: Written is the rows that created or
// replaced a snapshot, Replaced the subset that archived an earlier value,
// Unchanged the rows identical to the live snapshot and so skipped.
type BulkResult struct {
	Written   int
	Replaced  int
	Unchanged int
}

func (b *BulkResult) add(o replaceOutcome) {
	switch o {
	case replaceInserted:
		b.Written++
	case replaceReplaced:
		b.Written++
		b.Replaced++
	case replaceUnchanged:
		b.Unchanged++
	}
}

// AssetSnapshotWrite is the response to a single snapshot write that replaces
// the live row: the new snapshot plus the id of the row it displaced (nil when
// nothing was replaced), which the client needs to offer Undo.
type AssetSnapshotWrite struct {
	db.AssetSnapshot
	ReplacedID *uuid.UUID `json:"replaced_id"`
}

type LiabilitySnapshotWrite struct {
	db.LiabilitySnapshot
	ReplacedID *uuid.UUID `json:"replaced_id"`
}

type ReceivableSnapshotWrite struct {
	db.ReceivableSnapshot
	ReplacedID *uuid.UUID `json:"replaced_id"`
}

type InvestmentSnapshotWrite struct {
	db.InvestmentSnapshot
	ReplacedID *uuid.UUID `json:"replaced_id"`
}

func sameOptDecimal(a, b *decimal.Decimal) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func sameOptString(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// sameOptDate compares calendar dates; as_of_date is a DATE column.
func sameOptDate(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// Decimals compare by value (1.5 == 1.5000), never by string: the column
// returns its stored scale, the request carries whatever the client typed.

func assetSnapshotUnchanged(cur db.AssetSnapshot, p db.CreateAssetSnapshotParams) bool {
	return cur.Amount.Equal(p.Amount) && cur.Currency == p.Currency &&
		sameOptDate(cur.AsOfDate, p.AsOfDate) && sameOptString(cur.Description, p.Description)
}

func liabilitySnapshotUnchanged(cur db.LiabilitySnapshot, p db.CreateLiabilitySnapshotParams) bool {
	return cur.Amount.Equal(p.Amount) && cur.Currency == p.Currency &&
		sameOptDate(cur.AsOfDate, p.AsOfDate) && sameOptString(cur.Description, p.Description)
}

func receivableSnapshotUnchanged(cur db.ReceivableSnapshot, p db.CreateReceivableSnapshotParams) bool {
	return cur.Amount.Equal(p.Amount) && cur.Currency == p.Currency &&
		sameOptDate(cur.AsOfDate, p.AsOfDate) && sameOptString(cur.Description, p.Description)
}

func investmentSnapshotUnchanged(cur db.InvestmentSnapshot, p db.CreateInvestmentSnapshotParams) bool {
	return cur.Amount.Equal(p.Amount) && cur.Currency == p.Currency &&
		sameOptDecimal(cur.Quantity, p.Quantity) &&
		sameOptDecimal(cur.PricePerUnit, p.PricePerUnit) &&
		sameOptDecimal(cur.AccruedInterest, p.AccruedInterest) &&
		sameOptDate(cur.AsOfDate, p.AsOfDate) && sameOptString(cur.Description, p.Description)
}

func mapSnapshotWriteErr(op string, err error) error {
	if asOfMonthViolation(err) {
		return ErrSnapshotDateOutsideMonth
	}
	if snapshotMonthViolation(err) {
		return ErrSnapshotMonthExists
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return fmt.Errorf("%s: %w", op, err)
}

// replaceAssetSnapshot writes p as the live asset snapshot for p.YearMonth
// inside qtx's transaction. It returns the live row after the write and, when
// an earlier live row was archived, that row's id. Create's ownership CTE is
// the tenancy gate for the insert; the read and the archive carry the
// household filter themselves.
func replaceAssetSnapshot(ctx context.Context, qtx *db.Queries, user uuid.UUID, p db.CreateAssetSnapshotParams) (db.AssetSnapshot, *uuid.UUID, replaceOutcome, error) {
	var replacedID *uuid.UUID
	cur, err := qtx.GetAssetSnapshotAtMonth(ctx, db.GetAssetSnapshotAtMonthParams{
		AssetID: p.ID, YearMonth: p.YearMonth, HouseholdID: p.HouseholdID,
	})
	switch {
	case err == nil:
		if assetSnapshotUnchanged(cur, p) {
			return cur, nil, replaceUnchanged, nil
		}
		rows, err := qtx.SoftDeleteAssetSnapshot(ctx, db.SoftDeleteAssetSnapshotParams{
			ID: cur.ID, HouseholdID: p.HouseholdID, UpdatedBy: &user,
		})
		if err != nil {
			return db.AssetSnapshot{}, nil, 0, fmt.Errorf("archive asset snapshot: %w", err)
		}
		if rows == 0 {
			return db.AssetSnapshot{}, nil, 0, ErrNotFound
		}
		id := cur.ID
		replacedID = &id
	case errors.Is(err, pgx.ErrNoRows):
		// No live row: a plain insert.
	default:
		return db.AssetSnapshot{}, nil, 0, fmt.Errorf("get asset snapshot at month: %w", err)
	}
	snap, err := qtx.CreateAssetSnapshot(ctx, p)
	if err != nil {
		return db.AssetSnapshot{}, nil, 0, mapSnapshotWriteErr("create asset snapshot", err)
	}
	if replacedID != nil {
		return snap, replacedID, replaceReplaced, nil
	}
	return snap, nil, replaceInserted, nil
}

func replaceLiabilitySnapshot(ctx context.Context, qtx *db.Queries, user uuid.UUID, p db.CreateLiabilitySnapshotParams) (db.LiabilitySnapshot, *uuid.UUID, replaceOutcome, error) {
	var replacedID *uuid.UUID
	cur, err := qtx.GetLiabilitySnapshotAtMonth(ctx, db.GetLiabilitySnapshotAtMonthParams{
		LiabilityID: p.ID, YearMonth: p.YearMonth, HouseholdID: p.HouseholdID,
	})
	switch {
	case err == nil:
		if liabilitySnapshotUnchanged(cur, p) {
			return cur, nil, replaceUnchanged, nil
		}
		rows, err := qtx.SoftDeleteLiabilitySnapshot(ctx, db.SoftDeleteLiabilitySnapshotParams{
			ID: cur.ID, HouseholdID: p.HouseholdID, UpdatedBy: &user,
		})
		if err != nil {
			return db.LiabilitySnapshot{}, nil, 0, fmt.Errorf("archive liability snapshot: %w", err)
		}
		if rows == 0 {
			return db.LiabilitySnapshot{}, nil, 0, ErrNotFound
		}
		id := cur.ID
		replacedID = &id
	case errors.Is(err, pgx.ErrNoRows):
	default:
		return db.LiabilitySnapshot{}, nil, 0, fmt.Errorf("get liability snapshot at month: %w", err)
	}
	snap, err := qtx.CreateLiabilitySnapshot(ctx, p)
	if err != nil {
		return db.LiabilitySnapshot{}, nil, 0, mapSnapshotWriteErr("create liability snapshot", err)
	}
	if replacedID != nil {
		return snap, replacedID, replaceReplaced, nil
	}
	return snap, nil, replaceInserted, nil
}

func replaceReceivableSnapshot(ctx context.Context, qtx *db.Queries, user uuid.UUID, p db.CreateReceivableSnapshotParams) (db.ReceivableSnapshot, *uuid.UUID, replaceOutcome, error) {
	var replacedID *uuid.UUID
	cur, err := qtx.GetReceivableSnapshotAtMonth(ctx, db.GetReceivableSnapshotAtMonthParams{
		ReceivableID: p.ID, YearMonth: p.YearMonth, HouseholdID: p.HouseholdID,
	})
	switch {
	case err == nil:
		if receivableSnapshotUnchanged(cur, p) {
			return cur, nil, replaceUnchanged, nil
		}
		rows, err := qtx.SoftDeleteReceivableSnapshot(ctx, db.SoftDeleteReceivableSnapshotParams{
			ID: cur.ID, HouseholdID: p.HouseholdID, UpdatedBy: &user,
		})
		if err != nil {
			return db.ReceivableSnapshot{}, nil, 0, fmt.Errorf("archive receivable snapshot: %w", err)
		}
		if rows == 0 {
			return db.ReceivableSnapshot{}, nil, 0, ErrNotFound
		}
		id := cur.ID
		replacedID = &id
	case errors.Is(err, pgx.ErrNoRows):
	default:
		return db.ReceivableSnapshot{}, nil, 0, fmt.Errorf("get receivable snapshot at month: %w", err)
	}
	snap, err := qtx.CreateReceivableSnapshot(ctx, p)
	if err != nil {
		return db.ReceivableSnapshot{}, nil, 0, mapSnapshotWriteErr("create receivable snapshot", err)
	}
	if replacedID != nil {
		return snap, replacedID, replaceReplaced, nil
	}
	return snap, nil, replaceInserted, nil
}

func replaceInvestmentSnapshot(ctx context.Context, qtx *db.Queries, user uuid.UUID, p db.CreateInvestmentSnapshotParams) (db.InvestmentSnapshot, *uuid.UUID, replaceOutcome, error) {
	var replacedID *uuid.UUID
	cur, err := qtx.GetInvestmentSnapshotAtMonth(ctx, db.GetInvestmentSnapshotAtMonthParams{
		InvestmentID: p.ID, YearMonth: p.YearMonth, HouseholdID: p.HouseholdID,
	})
	switch {
	case err == nil:
		if investmentSnapshotUnchanged(cur, p) {
			return cur, nil, replaceUnchanged, nil
		}
		rows, err := qtx.SoftDeleteInvestmentSnapshot(ctx, db.SoftDeleteInvestmentSnapshotParams{
			ID: cur.ID, HouseholdID: p.HouseholdID, UpdatedBy: &user,
		})
		if err != nil {
			return db.InvestmentSnapshot{}, nil, 0, fmt.Errorf("archive investment snapshot: %w", err)
		}
		if rows == 0 {
			return db.InvestmentSnapshot{}, nil, 0, ErrNotFound
		}
		id := cur.ID
		replacedID = &id
	case errors.Is(err, pgx.ErrNoRows):
	default:
		return db.InvestmentSnapshot{}, nil, 0, fmt.Errorf("get investment snapshot at month: %w", err)
	}
	snap, err := qtx.CreateInvestmentSnapshot(ctx, p)
	if err != nil {
		return db.InvestmentSnapshot{}, nil, 0, mapSnapshotWriteErr("create investment snapshot", err)
	}
	if replacedID != nil {
		return snap, replacedID, replaceReplaced, nil
	}
	return snap, nil, replaceInserted, nil
}
