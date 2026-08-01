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

// deletedAtOf returns one row's deleted_at by raw SQL — used both to assert an
// already-tombstoned child was not re-stamped, and to assert a parent was not
// tombstoned by a delete that failed partway.
func deletedAtOf(t *testing.T, pool *pgxpool.Pool, table string, id uuid.UUID) *time.Time {
	t.Helper()
	var ts *time.Time
	sql := "SELECT deleted_at FROM " + table + " WHERE id = $1"
	if err := pool.QueryRow(context.Background(), sql, id).Scan(&ts); err != nil {
		t.Fatalf("read deleted_at from %s: %v", table, err)
	}
	return ts
}

// anyRowID returns some row id from a child table for one parent, so a test can
// lock it and make the cascade that touches it block.
func anyRowID(t *testing.T, pool *pgxpool.Pool, table, fkCol string, parentID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	sql := "SELECT id FROM " + table + " WHERE " + fkCol + " = $1 LIMIT 1"
	if err := pool.QueryRow(context.Background(), sql, parentID).Scan(&id); err != nil {
		t.Fatalf("pick a row from %s: %v", table, err)
	}
	return id
}

// lockRow holds a row lock on one row from a separate session until the
// returned release is called, so the statement in the delete's transaction that
// touches that row blocks until the caller's context deadline aborts it. This
// is how both failure-injection tests below reach an error branch that is
// otherwise unreachable from outside the package: nothing about a well-formed
// delete can be made to fail on demand, but anything can be made to *wait*.
func lockRow(t *testing.T, pool *pgxpool.Pool, table string, id uuid.UUID) (release func()) {
	t.Helper()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin blocker tx: %v", err)
	}
	var locked uuid.UUID
	sql := "SELECT id FROM " + table + " WHERE id = $1 FOR UPDATE"
	if err := tx.QueryRow(context.Background(), sql, id).Scan(&locked); err != nil {
		_ = tx.Rollback(context.Background())
		t.Fatalf("lock %s row: %v", table, err)
	}
	return func() {
		if err := tx.Rollback(context.Background()); err != nil {
			t.Errorf("rollback blocker tx: %v", err)
		}
	}
}

// childRef names a child table and the column holding its parent's id.
type childRef struct{ table, fkCol string }

// cascadeGroup is one position group's fixture + the repo delete under test, so
// the failure-injection tests below can run identically across all four rather
// than pinning the transaction only where it happened to be written first.
type cascadeGroup struct {
	name     string
	parent   string
	children []childRef
	// build creates a position carrying one row in each of its child tables and
	// returns its id plus the delete call under test.
	build func(ctx context.Context, t *testing.T, pool *pgxpool.Pool, name string) (uuid.UUID, func(context.Context) error)
}

func cascadeGroups() []cascadeGroup {
	return []cascadeGroup{
		{
			name:     "asset",
			parent:   "assets",
			children: []childRef{{"asset_snapshots", "asset_id"}},
			build: func(ctx context.Context, t *testing.T, pool *pgxpool.Pool, name string) (uuid.UUID, func(context.Context) error) {
				r := repo.NewAssetRepo(pool)
				acct := newBankAccount(ctx, t, r, name)
				if _, err := r.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{
					AssetID:   acct.Asset.ID,
					YearMonth: ymUTC(2026, time.January),
					Amount:    decimal.NewFromInt(100),
					Currency:  "IDR",
				}); err != nil {
					t.Fatalf("CreateAssetSnapshot: %v", err)
				}
				return acct.Asset.ID, func(c context.Context) error { return r.DeleteBankAccount(c, acct.Asset.ID) }
			},
		},
		{
			name:     "liability",
			parent:   "liabilities",
			children: []childRef{{"liability_snapshots", "liability_id"}},
			build: func(ctx context.Context, t *testing.T, pool *pgxpool.Pool, name string) (uuid.UUID, func(context.Context) error) {
				r := repo.NewLiabilityRepo(pool)
				liab, err := r.CreateLiability(ctx, repo.CreateLiabilityParams{
					DisplayName:      name,
					Subtype:          "personal",
					OwnershipType:    "joint",
					NativeCurrency:   "IDR",
					CounterpartyName: "Lender",
				})
				if err != nil {
					t.Fatalf("CreateLiability: %v", err)
				}
				if _, err := r.CreateLiabilitySnapshot(ctx, repo.CreateLiabilitySnapshotParams{
					LiabilityID: liab.ID,
					YearMonth:   ymUTC(2026, time.January),
					Amount:      decimal.NewFromInt(500),
					Currency:    "IDR",
				}); err != nil {
					t.Fatalf("CreateLiabilitySnapshot: %v", err)
				}
				return liab.ID, func(c context.Context) error { return r.DeleteLiability(c, liab.ID) }
			},
		},
		{
			name:     "receivable",
			parent:   "receivables",
			children: []childRef{{"receivable_snapshots", "receivable_id"}},
			build: func(ctx context.Context, t *testing.T, pool *pgxpool.Pool, name string) (uuid.UUID, func(context.Context) error) {
				r := repo.NewReceivableRepo(pool)
				recv, err := r.CreateReceivable(ctx, repo.CreateReceivableParams{
					DisplayName:      name,
					OwnershipType:    "joint",
					NativeCurrency:   "IDR",
					CounterpartyName: "Borrower",
				})
				if err != nil {
					t.Fatalf("CreateReceivable: %v", err)
				}
				if _, err := r.CreateReceivableSnapshot(ctx, repo.CreateReceivableSnapshotParams{
					ReceivableID: recv.ID,
					YearMonth:    ymUTC(2026, time.January),
					Amount:       decimal.NewFromInt(750),
					Currency:     "IDR",
				}); err != nil {
					t.Fatalf("CreateReceivableSnapshot: %v", err)
				}
				return recv.ID, func(c context.Context) error { return r.DeleteReceivable(c, recv.ID) }
			},
		},
		{
			name:   "investment",
			parent: "investments",
			// Ordered as the cascade runs them, so locking the second child
			// exercises a failure *after* the first cascade already succeeded
			// inside the transaction.
			children: []childRef{
				{"investment_snapshots", "investment_id"},
				{"investment_transactions", "investment_id"},
			},
			build: func(ctx context.Context, t *testing.T, pool *pgxpool.Pool, name string) (uuid.UUID, func(context.Context) error) {
				r := repo.NewInvestmentRepo(pool)
				stock := newCascadeStock(ctx, t, r, name)
				return stock.Investment.ID, func(c context.Context) error { return r.DeleteStock(c, stock.Investment.ID) }
			},
		},
	}
}

// assertNothingDeleted is the shared postcondition of both failure-injection
// tests: an aborted delete leaves the position exactly as it was — parent live,
// every child live. A half-applied delete is the failure mode, in either
// direction (children tombstoned under a live parent, or the reverse).
func assertNothingDeleted(t *testing.T, pool *pgxpool.Pool, g cascadeGroup, parentID uuid.UUID) {
	t.Helper()
	if ts := deletedAtOf(t, pool, g.parent, parentID); ts != nil {
		t.Errorf("%s: parent tombstoned despite the aborted delete: %v", g.name, ts)
	}
	for _, c := range g.children {
		if got := countLiveChildren(t, pool, c.table, c.fkCol, parentID); got != 1 {
			t.Errorf("%s: live rows in %s after aborted delete = %d, want 1 — the cascade was not rolled back", g.name, c.table, got)
		}
	}
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
	stampBefore := deletedAtOf(t, tdb.Pool, "asset_snapshots", early.ID)
	if stampBefore == nil {
		t.Fatalf("early snapshot has no deleted_at after its own delete")
	}

	if err := r.DeleteBankAccount(ctx, acct.Asset.ID); err != nil {
		t.Fatalf("DeleteBankAccount: %v", err)
	}

	stampAfter := deletedAtOf(t, tdb.Pool, "asset_snapshots", early.ID)
	if stampAfter == nil || !stampAfter.Equal(*stampBefore) {
		t.Errorf("already-deleted snapshot re-stamped: %v -> %v", stampBefore, stampAfter)
	}
	if ts := deletedAtOf(t, tdb.Pool, "asset_snapshots", late.ID); ts == nil {
		t.Errorf("live snapshot was not tombstoned by the cascade")
	}
}

// blockedDeleteTimeout bounds how long a delete is allowed to sit on a lock
// before the test gives up on it. The lock is held for the whole call, so the
// blocked statement can never succeed no matter how slow the runner is — this
// only decides how long the test waits to find that out.
const blockedDeleteTimeout = time.Second

// TestSoftDeletePosition_CascadeIsAtomic proves the cascade and the parent
// tombstone share one transaction, for every group. With the parent row locked
// by another session, the parent UPDATE blocks and the caller's context
// deadline aborts the delete — and the children, already updated earlier in the
// same transaction, must come back live. A non-transactional cascade would
// leave the position half-deleted: children gone, parent still on the books.
//
// covers: INV-SOFT-DELETE-05
func TestSoftDeletePosition_CascadeIsAtomic(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)

	for _, g := range cascadeGroups() {
		t.Run(g.name, func(t *testing.T) {
			parentID, del := g.build(ctx, t, tdb.Pool, "Atomic "+g.name)

			release := lockRow(t, tdb.Pool, g.parent, parentID)
			deadlineCtx, cancel := context.WithTimeout(ctx, blockedDeleteTimeout)
			err := del(deadlineCtx)
			cancel()
			release()

			if err == nil {
				t.Fatalf("%s: delete succeeded while the parent row was locked; the lock/timeout setup no longer blocks", g.name)
			}
			assertNothingDeleted(t, tdb.Pool, g, parentID)
		})
	}
}

// TestSoftDeletePosition_CascadeFailureLeavesPositionIntact is the acceptance
// criterion stated the way it actually reads — "failure mid-cascade rolls back
// the parent tombstone too". CascadeIsAtomic above injects the failure at the
// parent, which is the mirror image; here the failure lands inside the cascade
// itself, one subtest per child table.
//
// For Investment that matters twice over: locking a transaction row makes the
// *second* cascade block after the snapshot cascade has already succeeded
// inside the transaction, so this is the only test that proves an in-flight
// cascade is undone rather than merely never started.
//
// covers: INV-SOFT-DELETE-05
func TestSoftDeletePosition_CascadeFailureLeavesPositionIntact(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)

	for _, g := range cascadeGroups() {
		for _, c := range g.children {
			t.Run(g.name+"/"+c.table, func(t *testing.T) {
				parentID, del := g.build(ctx, t, tdb.Pool, "Blocked "+c.table)

				release := lockRow(t, tdb.Pool, c.table, anyRowID(t, tdb.Pool, c.table, c.fkCol, parentID))
				deadlineCtx, cancel := context.WithTimeout(ctx, blockedDeleteTimeout)
				err := del(deadlineCtx)
				cancel()
				release()

				if err == nil {
					t.Fatalf("%s: delete succeeded while a %s row was locked; the cascade is not touching that table", g.name, c.table)
				}
				assertNothingDeleted(t, tdb.Pool, g, parentID)
			})
		}
	}
}

// TestSoftDeletePosition_DeleteRejectsUnusableCaller pins the two guards a
// delete hits before it can open its transaction: no identity on the context,
// and a context already cancelled by the time the request reaches the repo
// (a client that hung up mid-request). Neither may report success, and neither
// may leave a tombstone behind.
//
// Scoped to Liability and Receivable on purpose: their Delete is the entry
// point, whereas DeleteBankAccount/DeleteStock run a Get guard first that
// returns on the same two conditions, so the identical branches inside
// softDeleteAsset/softDeleteInvestment are unreachable from outside the
// package rather than untested.
//
// covers: INV-SOFT-DELETE-05
func TestSoftDeletePosition_DeleteRejectsUnusableCaller(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)

	for _, g := range cascadeGroups() {
		if g.name != "liability" && g.name != "receivable" {
			continue
		}
		t.Run(g.name, func(t *testing.T) {
			t.Run("no identity on the context", func(t *testing.T) {
				parentID, del := g.build(ctx, t, tdb.Pool, "Anon "+g.name)
				if err := del(context.Background()); !errors.Is(err, repo.ErrUnauthenticated) {
					t.Errorf("want ErrUnauthenticated, got %v", err)
				}
				assertNothingDeleted(t, tdb.Pool, g, parentID)
			})

			t.Run("context already cancelled", func(t *testing.T) {
				parentID, del := g.build(ctx, t, tdb.Pool, "Cancelled "+g.name)
				deadCtx, cancel := context.WithCancel(ctx)
				cancel()
				if err := del(deadCtx); !errors.Is(err, context.Canceled) {
					t.Errorf("want context.Canceled, got %v", err)
				}
				assertNothingDeleted(t, tdb.Pool, g, parentID)
			})
		})
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
