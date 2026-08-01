package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// countLiveChildren counts child rows still carrying `deleted_at IS NULL` for
// one parent, by raw SQL. Deliberately raw: every named read path re-derives
// child visibility by joining the parent and re-checking its tombstone, so
// asserting through the repo would pass even with the cascade removed — the
// children would merely be *hidden*, which is the exact confusion #575 exists
// to end. Only a query with no parent join can tell "tombstoned" from
// "invisible".
func countLiveChildren(t *testing.T, pool *pgxpool.Pool, table, fkCol string, parentID uuid.UUID) int {
	t.Helper()
	var n int
	sql := "SELECT count(*) FROM " + table + " WHERE " + fkCol + " = $1 AND deleted_at IS NULL"
	if err := pool.QueryRow(context.Background(), sql, parentID).Scan(&n); err != nil {
		t.Fatalf("count live %s: %v", table, err)
	}
	return n
}

// childDeletedAt returns one child row's deleted_at by raw SQL, for asserting
// an already-tombstoned child was not re-stamped by the cascade.
func childDeletedAt(t *testing.T, pool *pgxpool.Pool, table string, childID uuid.UUID) *time.Time {
	t.Helper()
	var ts *time.Time
	sql := "SELECT deleted_at FROM " + table + " WHERE id = $1"
	if err := pool.QueryRow(context.Background(), sql, childID).Scan(&ts); err != nil {
		t.Fatalf("read deleted_at from %s: %v", table, err)
	}
	return ts
}

// newCascadeStock creates a stock carrying one snapshot and one buy
// transaction — the Investment group is the only one with two child tables,
// and transactions are where the confirmed #575 orphan lived.
func newCascadeStock(ctx context.Context, t *testing.T, r *repo.InvestmentRepo, name string) *repo.Stock {
	t.Helper()
	stock, err := r.CreateStock(ctx, repo.CreateStockParams{
		DisplayName:    name,
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		RiskProfile:    "medium",
		Ticker:         "BBCA",
		Exchange:       "IDX",
	})
	if err != nil {
		t.Fatalf("CreateStock %q: %v", name, err)
	}

	qty := decimal.NewFromInt(100)
	price := decimal.NewFromInt(9_000)
	total := qty.Mul(price)
	if _, err := r.CreateInvestmentTransaction(ctx, repo.CreateInvestmentTransactionParams{
		InvestmentID:    stock.Investment.ID,
		TransactionType: repo.TxnTypeBuy,
		TransactionDate: time.Date(2026, time.January, 5, 0, 0, 0, 0, time.UTC),
		Currency:        "IDR",
		Amount:          &total,
		Quantity:        &qty,
		PricePerUnit:    &price,
	}); err != nil {
		t.Fatalf("CreateInvestmentTransaction: %v", err)
	}
	if _, err := r.CreateInvestmentSnapshot(ctx, repo.CreateInvestmentSnapshotParams{
		InvestmentID: stock.Investment.ID,
		YearMonth:    ymUTC(2026, time.January),
		Amount:       total,
		Currency:     "IDR",
		Quantity:     &qty,
		PricePerUnit: &price,
	}); err != nil {
		t.Fatalf("CreateInvestmentSnapshot: %v", err)
	}
	return stock
}

// TestSoftDeletePosition_CascadesToChildren is the write-path half of #575:
// deleting a position of any of the four groups must tombstone every child it
// owns, so no live child row outlives its parent. Before the fix each
// SoftDelete<Group> was a bare parent-row UPDATE and every assertion below
// would find its children still live — invisible to today's read paths, but
// waiting for the next consumer that builds an ID list without re-joining the
// parent.
//
// covers: INV-SOFT-DELETE-05
func TestSoftDeletePosition_CascadesToChildren(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)

	jan := ymUTC(2026, time.January)
	feb := ymUTC(2026, time.February)

	t.Run("asset", func(t *testing.T) {
		r := repo.NewAssetRepo(tdb.Pool)
		acct := newBankAccount(ctx, t, r, "Cascade asset")
		for _, ym := range []time.Time{jan, feb} {
			if _, err := r.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{
				AssetID:   acct.Asset.ID,
				YearMonth: ym,
				Amount:    decimal.NewFromInt(100),
				Currency:  "IDR",
			}); err != nil {
				t.Fatalf("CreateAssetSnapshot: %v", err)
			}
		}
		if got := countLiveChildren(t, tdb.Pool, "asset_snapshots", "asset_id", acct.Asset.ID); got != 2 {
			t.Fatalf("live snapshots before delete = %d, want 2", got)
		}

		if err := r.DeleteBankAccount(ctx, acct.Asset.ID); err != nil {
			t.Fatalf("DeleteBankAccount: %v", err)
		}
		if got := countLiveChildren(t, tdb.Pool, "asset_snapshots", "asset_id", acct.Asset.ID); got != 0 {
			t.Errorf("live snapshots after delete = %d, want 0 — cascade did not reach asset_snapshots", got)
		}
	})

	t.Run("liability", func(t *testing.T) {
		r := repo.NewLiabilityRepo(tdb.Pool)
		liab, err := r.CreateLiability(ctx, repo.CreateLiabilityParams{
			DisplayName:      "Cascade liability",
			Subtype:          "personal",
			OwnershipType:    "joint",
			NativeCurrency:   "IDR",
			CounterpartyName: "Lender",
		})
		if err != nil {
			t.Fatalf("CreateLiability: %v", err)
		}
		for _, ym := range []time.Time{jan, feb} {
			if _, err := r.CreateLiabilitySnapshot(ctx, repo.CreateLiabilitySnapshotParams{
				LiabilityID: liab.ID,
				YearMonth:   ym,
				Amount:      decimal.NewFromInt(500),
				Currency:    "IDR",
			}); err != nil {
				t.Fatalf("CreateLiabilitySnapshot: %v", err)
			}
		}
		if got := countLiveChildren(t, tdb.Pool, "liability_snapshots", "liability_id", liab.ID); got != 2 {
			t.Fatalf("live snapshots before delete = %d, want 2", got)
		}

		if err := r.DeleteLiability(ctx, liab.ID); err != nil {
			t.Fatalf("DeleteLiability: %v", err)
		}
		if got := countLiveChildren(t, tdb.Pool, "liability_snapshots", "liability_id", liab.ID); got != 0 {
			t.Errorf("live snapshots after delete = %d, want 0 — cascade did not reach liability_snapshots", got)
		}
	})

	t.Run("receivable", func(t *testing.T) {
		r := repo.NewReceivableRepo(tdb.Pool)
		recv, err := r.CreateReceivable(ctx, repo.CreateReceivableParams{
			DisplayName:      "Cascade receivable",
			OwnershipType:    "joint",
			NativeCurrency:   "IDR",
			CounterpartyName: "Borrower",
		})
		if err != nil {
			t.Fatalf("CreateReceivable: %v", err)
		}
		for _, ym := range []time.Time{jan, feb} {
			if _, err := r.CreateReceivableSnapshot(ctx, repo.CreateReceivableSnapshotParams{
				ReceivableID: recv.ID,
				YearMonth:    ym,
				Amount:       decimal.NewFromInt(750),
				Currency:     "IDR",
			}); err != nil {
				t.Fatalf("CreateReceivableSnapshot: %v", err)
			}
		}
		if got := countLiveChildren(t, tdb.Pool, "receivable_snapshots", "receivable_id", recv.ID); got != 2 {
			t.Fatalf("live snapshots before delete = %d, want 2", got)
		}

		if err := r.DeleteReceivable(ctx, recv.ID); err != nil {
			t.Fatalf("DeleteReceivable: %v", err)
		}
		if got := countLiveChildren(t, tdb.Pool, "receivable_snapshots", "receivable_id", recv.ID); got != 0 {
			t.Errorf("live snapshots after delete = %d, want 0 — cascade did not reach receivable_snapshots", got)
		}
	})

	t.Run("investment cascades to both child tables", func(t *testing.T) {
		r := repo.NewInvestmentRepo(tdb.Pool)
		stock := newCascadeStock(ctx, t, r, "Cascade stock")
		id := stock.Investment.ID

		if got := countLiveChildren(t, tdb.Pool, "investment_snapshots", "investment_id", id); got != 1 {
			t.Fatalf("live snapshots before delete = %d, want 1", got)
		}
		if got := countLiveChildren(t, tdb.Pool, "investment_transactions", "investment_id", id); got != 1 {
			t.Fatalf("live transactions before delete = %d, want 1", got)
		}

		if err := r.DeleteStock(ctx, id); err != nil {
			t.Fatalf("DeleteStock: %v", err)
		}
		if got := countLiveChildren(t, tdb.Pool, "investment_snapshots", "investment_id", id); got != 0 {
			t.Errorf("live snapshots after delete = %d, want 0 — cascade did not reach investment_snapshots", got)
		}
		if got := countLiveChildren(t, tdb.Pool, "investment_transactions", "investment_id", id); got != 0 {
			t.Errorf("live transactions after delete = %d, want 0 — this is the exact #575 orphan shape", got)
		}
	})

	t.Run("position with no children deletes and stays idempotent", func(t *testing.T) {
		r := repo.NewInvestmentRepo(tdb.Pool)
		stock, err := r.CreateStock(ctx, repo.CreateStockParams{
			DisplayName:    "Childless stock",
			OwnershipType:  "joint",
			NativeCurrency: "IDR",
			RiskProfile:    "medium",
			Ticker:         "TLKM",
			Exchange:       "IDX",
		})
		if err != nil {
			t.Fatalf("CreateStock: %v", err)
		}
		if err := r.DeleteStock(ctx, stock.Investment.ID); err != nil {
			t.Fatalf("first DeleteStock: %v", err)
		}
		if err := r.DeleteStock(ctx, stock.Investment.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("second DeleteStock: want ErrNotFound, got %v", err)
		}
	})
}

// TestSoftDeletePosition_CascadeKeepsExistingChildTombstone pins the
// "never double-stamped" half of INV-SOFT-DELETE-01 against the new cascade: a
// snapshot the user deleted last week must keep *its* deleted_at when the
// parent is deleted today, not be re-stamped with the parent's. The cascade
// query's `AND s.deleted_at IS NULL` is what buys this, and it is also what a
// future undelete path would need as its discriminator to avoid resurrecting
// children the user removed by hand.
//
// covers: INV-SOFT-DELETE-05, INV-SOFT-DELETE-01
func TestSoftDeletePosition_CascadeKeepsExistingChildTombstone(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)
	r := repo.NewAssetRepo(tdb.Pool)

	acct := newBankAccount(ctx, t, r, "Pre-deleted child")
	early, err := r.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{
		AssetID:   acct.Asset.ID,
		YearMonth: ymUTC(2026, time.January),
		Amount:    decimal.NewFromInt(100),
		Currency:  "IDR",
	})
	if err != nil {
		t.Fatalf("CreateAssetSnapshot early: %v", err)
	}
	late, err := r.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{
		AssetID:   acct.Asset.ID,
		YearMonth: ymUTC(2026, time.February),
		Amount:    decimal.NewFromInt(200),
		Currency:  "IDR",
	})
	if err != nil {
		t.Fatalf("CreateAssetSnapshot late: %v", err)
	}

	if err := r.DeleteAssetSnapshot(ctx, early.ID); err != nil {
		t.Fatalf("DeleteAssetSnapshot: %v", err)
	}
	stampBefore := childDeletedAt(t, tdb.Pool, "asset_snapshots", early.ID)
	if stampBefore == nil {
		t.Fatalf("early snapshot has no deleted_at after its own delete")
	}

	if err := r.DeleteBankAccount(ctx, acct.Asset.ID); err != nil {
		t.Fatalf("DeleteBankAccount: %v", err)
	}

	stampAfter := childDeletedAt(t, tdb.Pool, "asset_snapshots", early.ID)
	if stampAfter == nil || !stampAfter.Equal(*stampBefore) {
		t.Errorf("already-deleted snapshot re-stamped: %v -> %v", stampBefore, stampAfter)
	}
	if ts := childDeletedAt(t, tdb.Pool, "asset_snapshots", late.ID); ts == nil {
		t.Errorf("live snapshot was not tombstoned by the cascade")
	}
}

// TestSoftDeletePosition_CascadeIsAtomic proves the cascade and the parent
// tombstone share one transaction: with the parent row locked by another
// session, the parent UPDATE blocks and the caller's context deadline aborts
// the delete — and the children, already updated earlier in the same
// transaction, must come back live. A non-transactional cascade would leave
// the position half-deleted: children gone, parent still on the books.
//
// covers: INV-SOFT-DELETE-05
func TestSoftDeletePosition_CascadeIsAtomic(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)
	r := repo.NewAssetRepo(tdb.Pool)

	acct := newBankAccount(ctx, t, r, "Atomic cascade")
	if _, err := r.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{
		AssetID:   acct.Asset.ID,
		YearMonth: ymUTC(2026, time.January),
		Amount:    decimal.NewFromInt(100),
		Currency:  "IDR",
	}); err != nil {
		t.Fatalf("CreateAssetSnapshot: %v", err)
	}

	// Hold a row lock on the parent from a separate session, so the delete's
	// parent UPDATE blocks after its cascade has already run.
	blocker, err := tdb.Pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin blocker tx: %v", err)
	}
	var lockedID uuid.UUID
	if err := blocker.QueryRow(context.Background(),
		"SELECT id FROM assets WHERE id = $1 FOR UPDATE", acct.Asset.ID).Scan(&lockedID); err != nil {
		_ = blocker.Rollback(context.Background())
		t.Fatalf("lock asset row: %v", err)
	}

	deadlineCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	err = r.DeleteBankAccount(deadlineCtx, acct.Asset.ID)
	cancel()

	if rbErr := blocker.Rollback(context.Background()); rbErr != nil {
		t.Fatalf("rollback blocker tx: %v", rbErr)
	}

	if err == nil {
		t.Fatalf("DeleteBankAccount succeeded while the parent row was locked; the lock/timeout setup no longer blocks")
	}

	if got := countLiveChildren(t, tdb.Pool, "asset_snapshots", "asset_id", acct.Asset.ID); got != 1 {
		t.Errorf("live snapshots after aborted delete = %d, want 1 — the cascade was not rolled back with the parent", got)
	}
	if ts := childDeletedAt(t, tdb.Pool, "assets", acct.Asset.ID); ts != nil {
		t.Errorf("parent tombstoned despite the aborted delete: %v", ts)
	}
}

// TestSoftDeletePosition_CascadeSafeForIDArrayQueries is the point of #575
// stated as the consumer sees it. Five child queries take an ID array and skip
// the parent join entirely, trusting the caller to have sourced those IDs from
// a household-scoped, non-deleted list. After a cascade delete they are safe
// even when fed a stale ID: the tombstone is on the child rows themselves, so
// the class of bug — a future import path, backfill or report variant
// resurrecting a deleted position's children — is removed at the write path
// rather than re-litigated per query.
//
// covers: INV-SOFT-DELETE-05
func TestSoftDeletePosition_CascadeSafeForIDArrayQueries(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)
	r := repo.NewInvestmentRepo(tdb.Pool)

	stock := newCascadeStock(ctx, t, r, "Stale ID stock")
	id := stock.Investment.ID

	if err := r.DeleteStock(ctx, id); err != nil {
		t.Fatalf("DeleteStock: %v", err)
	}

	// The stale ID a future caller might still be holding.
	ids := []uuid.UUID{id}

	txns, err := q.ListInvestmentTransactionsByInvestmentIDs(ctx, ids)
	if err != nil {
		t.Fatalf("ListInvestmentTransactionsByInvestmentIDs: %v", err)
	}
	if len(txns) != 0 {
		t.Errorf("ListInvestmentTransactionsByInvestmentIDs returned %d rows for a deleted investment, want 0", len(txns))
	}

	snaps, err := q.ListInvestmentSnapshotsByInvestmentIDs(ctx, ids)
	if err != nil {
		t.Fatalf("ListInvestmentSnapshotsByInvestmentIDs: %v", err)
	}
	if len(snaps) != 0 {
		t.Errorf("ListInvestmentSnapshotsByInvestmentIDs returned %d rows for a deleted investment, want 0", len(snaps))
	}

	latest, err := q.ListLatestInvestmentSnapshotsByInvestmentIDs(ctx, ids)
	if err != nil {
		t.Fatalf("ListLatestInvestmentSnapshotsByInvestmentIDs: %v", err)
	}
	if len(latest) != 0 {
		t.Errorf("ListLatestInvestmentSnapshotsByInvestmentIDs returned %d rows for a deleted investment, want 0", len(latest))
	}
}
