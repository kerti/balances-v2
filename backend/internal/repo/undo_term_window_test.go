package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// Undo and lifecycle terminate bring or write rows without a forward write path
// in front of them, so each re-checks the term window itself (#1002). Every
// refusal below must leave the database exactly as it was: the assertions count
// rows with raw SQL, because a getter would hide an archived row.

type undoTermFx struct {
	tdb  *testutil.TestDB
	r    *repo.InvestmentRepo
	ctx  context.Context
	bctx context.Context // a second household
	bond *repo.Bond      // govt_primary, Buy 2026-01-15, maturity 2027-01-15
	td   *repo.TimeDeposit
	bf   bondWindowFx
}

func newUndoTermFx(t *testing.T) undoTermFx {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	ctx := identity.WithUser(context.Background(), alice)
	r := repo.NewInvestmentRepo(tdb.Pool)

	b, err := r.CreateBond(ctx, bondWindowParams("govt_primary", day(2026, 1, 15), day(2027, 1, 15)))
	if err != nil {
		t.Fatalf("CreateBond: %v", err)
	}
	rate := decimal.RequireFromString("5.5")
	td, err := r.CreateTimeDeposit(ctx, repo.CreateTimeDepositParams{
		DisplayName: "BCA TD", OwnershipType: "joint", NativeCurrency: "IDR", RiskProfile: "medium",
		BankName: "BCA", Principal: decimal.NewFromInt(50_000_000), InterestRate: rate, TermMonths: 12,
		PlacementDate: day(2026, 1, 15), MaturityDate: day(2027, 1, 15), RolloverPolicy: "auto_renew_principal",
	})
	if err != nil {
		t.Fatalf("CreateTimeDeposit: %v", err)
	}
	return undoTermFx{
		tdb: tdb, r: r, ctx: ctx, bctx: identity.WithUser(context.Background(), bob),
		bond: b, td: td, bf: bondWindowFx{r: r, ctx: ctx, bond: b},
	}
}

func (f undoTermFx) tdSnapshot(t *testing.T, month time.Time) *db.InvestmentSnapshot {
	t.Helper()
	accrued := decimal.NewFromInt(100_000)
	s, err := f.r.CreateInvestmentSnapshot(f.ctx, repo.CreateInvestmentSnapshotParams{
		InvestmentID: f.td.Investment.ID, YearMonth: month, Amount: decimal.NewFromInt(50_100_000),
		Currency: "IDR", AccruedInterest: &accrued,
	})
	if err != nil {
		t.Fatalf("TD snapshot %s: %v", month.Format("2006-01"), err)
	}
	return s
}

func (f undoTermFx) bondSnapshot(t *testing.T, month time.Time) *db.InvestmentSnapshot {
	t.Helper()
	s, err := f.bf.snapshot(f.bond.Investment.ID, month)
	if err != nil {
		t.Fatalf("bond snapshot %s: %v", month.Format("2006-01"), err)
	}
	return s
}

func (f undoTermFx) liveSnapshots(t *testing.T, id uuid.UUID) int {
	t.Helper()
	var n int
	if err := f.tdb.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM investment_snapshots WHERE investment_id = $1 AND deleted_at IS NULL`, id).Scan(&n); err != nil {
		t.Fatalf("count snapshots: %v", err)
	}
	return n
}

func (f undoTermFx) liveTxns(t *testing.T, id uuid.UUID) int {
	t.Helper()
	var n int
	if err := f.tdb.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM investment_transactions WHERE investment_id = $1 AND deleted_at IS NULL`, id).Scan(&n); err != nil {
		t.Fatalf("count transactions: %v", err)
	}
	return n
}

func (f undoTermFx) snapshotLive(t *testing.T, id uuid.UUID) bool {
	t.Helper()
	var n int
	if err := f.tdb.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM investment_snapshots WHERE id = $1 AND deleted_at IS NULL`, id).Scan(&n); err != nil {
		t.Fatalf("count snapshot: %v", err)
	}
	return n == 1
}

func (f undoTermFx) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := f.tdb.Pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("exec %q: %v", sql, err)
	}
}

// narrowTD moves the time deposit's placement forward, the way a term edit does
// once the snapshots it would strand have been deleted.
func (f undoTermFx) narrowTD(t *testing.T, placement time.Time) {
	t.Helper()
	rate := decimal.RequireFromString("5.5")
	if _, err := f.r.UpdateTimeDeposit(f.ctx, f.td.Investment.ID, repo.UpdateTimeDepositParams{
		DisplayName: "BCA TD", OwnershipType: "joint", RiskProfile: "medium", BankName: "BCA",
		Principal: decimal.NewFromInt(50_000_000), InterestRate: rate, TermMonths: 9,
		RolloverPolicy: "auto_renew_principal", PlacementDate: placement, MaturityDate: day(2027, 1, 15),
	}); err != nil {
		t.Fatalf("narrow TD term: %v", err)
	}
}

// TestUndoDeleteBondTransaction_RefusedOutsideWindow is the issue's repro: delete
// a Coupon, move the earliest Buy past it, undo. The coupon would come back
// before the term opens, so the undo is refused and the coupon stays archived.
// covers: INV-BONDS-05
func TestUndoDeleteBondTransaction_RefusedOutsideWindow(t *testing.T) {
	f := newUndoTermFx(t)
	id := f.bond.Investment.ID
	coupon, err := f.bf.coupon(id, day(2026, 2, 15))
	if err != nil {
		t.Fatalf("coupon: %v", err)
	}
	if err := f.r.DeleteInvestmentTransaction(f.ctx, coupon.ID); err != nil {
		t.Fatalf("delete coupon: %v", err)
	}
	if err := f.bf.moveBuy(f.bf.earliestBuy(t, id), day(2026, 3, 1)); err != nil {
		t.Fatalf("move the earliest Buy past the coupon: %v", err)
	}

	wantErr(t, "undo the coupon before the window", f.r.UndoDeleteInvestmentTransaction(f.ctx, coupon.ID), repo.ErrOutsideBondTerm)
	if n := f.liveTxns(t, id); n != 1 {
		t.Fatalf("a refused undo must restore nothing: %d live transactions, want 1 (the Buy)", n)
	}

	// Moving the Buy back opens the window again: the same undo now succeeds.
	if err := f.bf.moveBuy(f.bf.earliestBuy(t, id), day(2026, 1, 15)); err != nil {
		t.Fatalf("move the Buy back: %v", err)
	}
	if err := f.r.UndoDeleteInvestmentTransaction(f.ctx, coupon.ID); err != nil {
		t.Fatalf("in-window undo: %v", err)
	}
	if n := f.liveTxns(t, id); n != 2 {
		t.Fatalf("in-window undo must restore the coupon: %d live transactions, want 2", n)
	}
}

// TestUndoDeleteBondTransaction_AfterMaturity: the far end of the window holds
// too. A Buy restored after the narrowed maturity is refused as well, and a
// restored Buy that stays inside the term widens it as before.
// covers: INV-BONDS-05
func TestUndoDeleteBondTransaction_AfterMaturity(t *testing.T) {
	f := newUndoTermFx(t)
	id := f.bond.Investment.ID
	second, err := f.bf.buy(id, day(2026, 6, 1))
	if err != nil {
		t.Fatalf("second Buy: %v", err)
	}
	late, err := f.bf.coupon(id, day(2026, 12, 1))
	if err != nil {
		t.Fatalf("coupon: %v", err)
	}
	for _, txn := range []uuid.UUID{second.ID, late.ID} {
		if err := f.r.DeleteInvestmentTransaction(f.ctx, txn); err != nil {
			t.Fatalf("delete: %v", err)
		}
	}
	if _, err := f.r.UpdateBond(f.ctx, id, f.bf.updateParams(day(2026, 4, 1))); err != nil {
		t.Fatalf("narrow maturity: %v", err)
	}

	wantErr(t, "Buy restored after maturity", f.r.UndoDeleteInvestmentTransaction(f.ctx, second.ID), repo.ErrOutsideBondTerm)
	wantErr(t, "coupon restored after maturity", f.r.UndoDeleteInvestmentTransaction(f.ctx, late.ID), repo.ErrOutsideBondTerm)
	if n := f.liveTxns(t, id); n != 1 {
		t.Fatalf("refused undos must restore nothing: %d live transactions, want 1", n)
	}
}

// TestUndoDeleteBondTransaction_SecondaryMarketHasOnlyMaturityBound: a bond with
// no live Buy has no start, so only the maturity bound can refuse an undo.
// covers: INV-BONDS-05
func TestUndoDeleteBondTransaction_SecondaryMarketHasOnlyMaturityBound(t *testing.T) {
	f := newUndoTermFx(t)
	b, err := f.r.CreateBond(f.ctx, bondWindowParams("secondary_market", time.Time{}, day(2028, 6, 1)))
	if err != nil {
		t.Fatalf("CreateBond: %v", err)
	}
	id := b.Investment.ID
	sf := bondWindowFx{r: f.r, ctx: f.ctx, bond: b}
	old, err := sf.coupon(id, day(2020, 3, 1))
	if err != nil {
		t.Fatalf("coupon: %v", err)
	}
	late, err := sf.coupon(id, day(2028, 5, 1))
	if err != nil {
		t.Fatalf("coupon: %v", err)
	}
	for _, txn := range []uuid.UUID{old.ID, late.ID} {
		if err := f.r.DeleteInvestmentTransaction(f.ctx, txn); err != nil {
			t.Fatalf("delete: %v", err)
		}
	}
	if _, err := f.r.UpdateBond(f.ctx, id, sf.updateParams(day(2024, 1, 1))); err != nil {
		t.Fatalf("narrow maturity: %v", err)
	}

	if err := f.r.UndoDeleteInvestmentTransaction(f.ctx, old.ID); err != nil {
		t.Fatalf("a coupon long before any Buy is inside an open-ended window: %v", err)
	}
	wantErr(t, "coupon after maturity", f.r.UndoDeleteInvestmentTransaction(f.ctx, late.ID), repo.ErrOutsideBondTerm)
	if n := f.liveTxns(t, id); n != 1 {
		t.Fatalf("live transactions = %d, want only the in-window coupon", n)
	}
}

// TestUndoDeleteTimeDepositTransaction_RefusedOutsideWindow: the deposit's
// Maturity is the only transaction it holds, and restoring it after the term has
// been narrowed past it is refused. The term is narrowed in SQL because the
// forward edit would (rightly) refuse while the deposit's close snapshot sits at
// the old maturity month.
// covers: INV-BONDS-03
func TestUndoDeleteTimeDepositTransaction_RefusedOutsideWindow(t *testing.T) {
	f := newUndoTermFx(t)
	id := f.td.Investment.ID
	principal := decimal.NewFromInt(50_000_000)
	interest := decimal.NewFromInt(2_000_000)
	cashOut := "cash_out"
	mat, err := f.r.CreateInvestmentTransaction(f.ctx, repo.CreateInvestmentTransactionParams{
		InvestmentID: id, TransactionType: repo.TxnTypeMaturity, TransactionDate: day(2027, 1, 15),
		Currency: "IDR", PrincipalAmount: &principal, InterestAmount: &interest,
		PrincipalDisposition: &cashOut, InterestDisposition: &cashOut,
	})
	if err != nil {
		t.Fatalf("Maturity: %v", err)
	}
	if err := f.r.DeleteInvestmentTransaction(f.ctx, mat.ID); err != nil {
		t.Fatalf("delete Maturity: %v", err)
	}
	f.exec(t, `UPDATE time_deposit_details SET maturity_date = $2 WHERE investment_id = $1`, id, day(2026, 12, 1))

	wantErr(t, "Maturity restored after the narrowed term", f.r.UndoDeleteInvestmentTransaction(f.ctx, mat.ID), repo.ErrOutsideDepositTerm)
	if n := f.liveTxns(t, id); n != 0 {
		t.Fatalf("a refused undo must restore nothing: %d live transactions, want 0", n)
	}

	f.exec(t, `UPDATE time_deposit_details SET maturity_date = $2 WHERE investment_id = $1`, id, day(2027, 1, 15))
	if err := f.r.UndoDeleteInvestmentTransaction(f.ctx, mat.ID); err != nil {
		t.Fatalf("in-window undo: %v", err)
	}
	if n := f.liveTxns(t, id); n != 1 {
		t.Fatalf("in-window undo must restore the Maturity: %d live transactions, want 1", n)
	}
}

// TestUndoDeleteSnapshot_RefusedOutsideWindow: deleting a snapshot, then narrowing
// the term past its month, then undoing is refused — for a bond (its earliest Buy
// moved later) and a time deposit (its placement moved later).
// covers: INV-BONDS-03, INV-BONDS-05
func TestUndoDeleteSnapshot_RefusedOutsideWindow(t *testing.T) {
	t.Run("bond", func(t *testing.T) {
		f := newUndoTermFx(t)
		id := f.bond.Investment.ID
		snap := f.bondSnapshot(t, ym(2026, time.February))
		if err := f.r.DeleteInvestmentSnapshot(f.ctx, snap.ID); err != nil {
			t.Fatalf("delete snapshot: %v", err)
		}
		if err := f.bf.moveBuy(f.bf.earliestBuy(t, id), day(2026, 4, 1)); err != nil {
			t.Fatalf("move the earliest Buy: %v", err)
		}

		wantErr(t, "undo before the window", f.r.UndoDeleteInvestmentSnapshot(f.ctx, snap.ID), repo.ErrOutsideBondTerm)
		if f.snapshotLive(t, snap.ID) || f.liveSnapshots(t, id) != 0 {
			t.Fatalf("a refused undo must restore nothing")
		}

		if err := f.bf.moveBuy(f.bf.earliestBuy(t, id), day(2026, 1, 15)); err != nil {
			t.Fatalf("move the Buy back: %v", err)
		}
		if err := f.r.UndoDeleteInvestmentSnapshot(f.ctx, snap.ID); err != nil {
			t.Fatalf("in-window undo: %v", err)
		}
		if !f.snapshotLive(t, snap.ID) {
			t.Fatalf("in-window undo must restore the snapshot")
		}
	})

	t.Run("bond after maturity", func(t *testing.T) {
		f := newUndoTermFx(t)
		id := f.bond.Investment.ID
		snap := f.bondSnapshot(t, ym(2026, time.November))
		if err := f.r.DeleteInvestmentSnapshot(f.ctx, snap.ID); err != nil {
			t.Fatalf("delete snapshot: %v", err)
		}
		if _, err := f.r.UpdateBond(f.ctx, id, f.bf.updateParams(day(2026, 6, 1))); err != nil {
			t.Fatalf("narrow maturity: %v", err)
		}
		wantErr(t, "undo after maturity", f.r.UndoDeleteInvestmentSnapshot(f.ctx, snap.ID), repo.ErrOutsideBondTerm)
		if f.liveSnapshots(t, id) != 0 {
			t.Fatalf("a refused undo must restore nothing")
		}
	})

	t.Run("time deposit", func(t *testing.T) {
		f := newUndoTermFx(t)
		id := f.td.Investment.ID
		snap := f.tdSnapshot(t, ym(2026, time.March))
		if err := f.r.DeleteInvestmentSnapshot(f.ctx, snap.ID); err != nil {
			t.Fatalf("delete snapshot: %v", err)
		}
		f.narrowTD(t, day(2026, 4, 1))

		wantErr(t, "undo before the window", f.r.UndoDeleteInvestmentSnapshot(f.ctx, snap.ID), repo.ErrOutsideDepositTerm)
		if f.snapshotLive(t, snap.ID) || f.liveSnapshots(t, id) != 0 {
			t.Fatalf("a refused undo must restore nothing")
		}

		f.narrowTD(t, day(2026, 1, 15))
		if err := f.r.UndoDeleteInvestmentSnapshot(f.ctx, snap.ID); err != nil {
			t.Fatalf("in-window undo: %v", err)
		}
		if !f.snapshotLive(t, snap.ID) {
			t.Fatalf("in-window undo must restore the snapshot")
		}
	})
}

// TestUndoReplaceSnapshot_RefusedOutsideWindow: the replacement sits in the same
// month as the original, and the forward guards stop the term narrowing past a
// live snapshot, so this can only arise from a window changed underneath (a
// pre-#574 backup, a direct edit). The undo still re-checks, so it cannot put an
// out-of-window row back.
// covers: INV-BONDS-03, INV-BONDS-05
func TestUndoReplaceSnapshot_RefusedOutsideWindow(t *testing.T) {
	edit := func(t *testing.T, f undoTermFx, snapID uuid.UUID, amount int64) *repo.InvestmentSnapshotWrite {
		t.Helper()
		accrued := decimal.Zero
		w, err := f.r.UpdateInvestmentSnapshot(f.ctx, repo.UpdateInvestmentSnapshotParams{
			SnapshotID: snapID, Amount: decimal.NewFromInt(amount), Currency: "IDR", AccruedInterest: &accrued,
		})
		if err != nil || w.ReplacedID == nil {
			t.Fatalf("edit snapshot: replaced=%v err=%v", w, err)
		}
		return w
	}

	t.Run("bond", func(t *testing.T) {
		f := newUndoTermFx(t)
		id := f.bond.Investment.ID
		w := edit(t, f, f.bondSnapshot(t, ym(2026, time.June)).ID, 11_000_000)
		f.exec(t, `UPDATE bond_details SET maturity_date = $2 WHERE investment_id = $1`, id, day(2026, 3, 1))

		wantErr(t, "undo replace after the narrowed maturity", f.r.UndoReplaceInvestmentSnapshot(f.ctx, *w.ReplacedID, w.ID), repo.ErrOutsideBondTerm)
		if !f.snapshotLive(t, w.ID) || f.snapshotLive(t, *w.ReplacedID) {
			t.Fatalf("a refused undo-replace must leave the replacement live and the original archived")
		}

		f.exec(t, `UPDATE bond_details SET maturity_date = $2 WHERE investment_id = $1`, id, day(2027, 1, 15))
		if err := f.r.UndoReplaceInvestmentSnapshot(f.ctx, *w.ReplacedID, w.ID); err != nil {
			t.Fatalf("in-window undo replace: %v", err)
		}
		if f.snapshotLive(t, w.ID) || !f.snapshotLive(t, *w.ReplacedID) {
			t.Fatalf("in-window undo-replace must swap the rows back")
		}
	})

	t.Run("time deposit", func(t *testing.T) {
		f := newUndoTermFx(t)
		id := f.td.Investment.ID
		w := edit(t, f, f.tdSnapshot(t, ym(2026, time.March)).ID, 50_200_000)
		f.exec(t, `UPDATE time_deposit_details SET placement_date = $2 WHERE investment_id = $1`, id, day(2026, 5, 1))

		wantErr(t, "undo replace before the narrowed placement", f.r.UndoReplaceInvestmentSnapshot(f.ctx, *w.ReplacedID, w.ID), repo.ErrOutsideDepositTerm)
		if !f.snapshotLive(t, w.ID) || f.snapshotLive(t, *w.ReplacedID) {
			t.Fatalf("a refused undo-replace must leave the replacement live and the original archived")
		}

		f.exec(t, `UPDATE time_deposit_details SET placement_date = $2 WHERE investment_id = $1`, id, day(2026, 1, 15))
		if err := f.r.UndoReplaceInvestmentSnapshot(f.ctx, *w.ReplacedID, w.ID); err != nil {
			t.Fatalf("in-window undo replace: %v", err)
		}
		if f.snapshotLive(t, w.ID) || !f.snapshotLive(t, *w.ReplacedID) {
			t.Fatalf("in-window undo-replace must swap the rows back")
		}
	})
}

// TestUndoMoveSnapshot_RefusedOutsideWindow: a snapshot moved inside the window,
// the term then narrowed past the month it came from, then undone: the original
// would land outside, so the whole undo (the moved row's archive included) rolls
// back. The displaced row variant checks the second restore rolls back too.
// covers: INV-BONDS-03, INV-BONDS-05
func TestUndoMoveSnapshot_RefusedOutsideWindow(t *testing.T) {
	t.Run("bond", func(t *testing.T) {
		f := newUndoTermFx(t)
		id := f.bond.Investment.ID
		orig := f.bondSnapshot(t, ym(2026, time.February))
		displaced := f.bondSnapshot(t, ym(2026, time.June))
		w, err := f.r.MoveInvestmentSnapshot(f.ctx, orig.ID, ym(2026, time.June))
		if err != nil || w.ReplacedID == nil || *w.ReplacedID != displaced.ID {
			t.Fatalf("move: %+v err=%v", w, err)
		}
		if err := f.bf.moveBuy(f.bf.earliestBuy(t, id), day(2026, 4, 1)); err != nil {
			t.Fatalf("move the earliest Buy past February: %v", err)
		}

		wantErr(t, "undo move before the window", f.r.UndoMoveInvestmentSnapshot(f.ctx, orig.ID, w.ID, w.ReplacedID), repo.ErrOutsideBondTerm)
		if !f.snapshotLive(t, w.ID) || f.snapshotLive(t, orig.ID) || f.snapshotLive(t, displaced.ID) || f.liveSnapshots(t, id) != 1 {
			t.Fatalf("a refused undo-move must leave exactly the moved row live")
		}

		if err := f.bf.moveBuy(f.bf.earliestBuy(t, id), day(2026, 1, 15)); err != nil {
			t.Fatalf("move the Buy back: %v", err)
		}
		if err := f.r.UndoMoveInvestmentSnapshot(f.ctx, orig.ID, w.ID, w.ReplacedID); err != nil {
			t.Fatalf("in-window undo move: %v", err)
		}
		if f.snapshotLive(t, w.ID) || !f.snapshotLive(t, orig.ID) || !f.snapshotLive(t, displaced.ID) {
			t.Fatalf("in-window undo-move must restore both rows and archive the moved one")
		}
	})

	t.Run("time deposit", func(t *testing.T) {
		f := newUndoTermFx(t)
		id := f.td.Investment.ID
		orig := f.tdSnapshot(t, ym(2026, time.February))
		w, err := f.r.MoveInvestmentSnapshot(f.ctx, orig.ID, ym(2026, time.June))
		if err != nil || w.ReplacedID != nil {
			t.Fatalf("move: %+v err=%v", w, err)
		}
		f.narrowTD(t, day(2026, 4, 1))

		wantErr(t, "undo move before the window", f.r.UndoMoveInvestmentSnapshot(f.ctx, orig.ID, w.ID, nil), repo.ErrOutsideDepositTerm)
		if !f.snapshotLive(t, w.ID) || f.snapshotLive(t, orig.ID) || f.liveSnapshots(t, id) != 1 {
			t.Fatalf("a refused undo-move must leave exactly the moved row live")
		}

		f.narrowTD(t, day(2026, 1, 15))
		if err := f.r.UndoMoveInvestmentSnapshot(f.ctx, orig.ID, w.ID, nil); err != nil {
			t.Fatalf("in-window undo move: %v", err)
		}
		if f.snapshotLive(t, w.ID) || !f.snapshotLive(t, orig.ID) {
			t.Fatalf("in-window undo-move must restore the original")
		}
	})
}

// TestUndo_UnrelatedLegacyRowsDoNotBlock: the check looks only at the restored
// row. A bond already holding an out-of-window row (pre-#574 data) can still
// undo an in-window delete.
// covers: INV-BONDS-05
func TestUndo_UnrelatedLegacyRowsDoNotBlock(t *testing.T) {
	f := newUndoTermFx(t)
	id := f.bond.Investment.ID
	f.bondSnapshot(t, ym(2026, time.March))
	f.exec(t, `UPDATE bond_details SET maturity_date = $2 WHERE investment_id = $1`, id, day(2026, 2, 20))
	ok := f.bondSnapshot(t, ym(2026, time.February))
	if err := f.r.DeleteInvestmentSnapshot(f.ctx, ok.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := f.r.UndoDeleteInvestmentSnapshot(f.ctx, ok.ID); err != nil {
		t.Fatalf("undo of an in-window row beside a legacy one: %v", err)
	}
}

// TestUndoWindowCheck_Tenancy: bob cannot restore, or probe the window of,
// alice's archived rows; the refusal is not-found and nothing is restored, while
// alice's own undo still works.
// covers: INV-BONDS-05
func TestUndoWindowCheck_Tenancy(t *testing.T) {
	f := newUndoTermFx(t)
	id := f.bond.Investment.ID
	snap := f.bondSnapshot(t, ym(2026, time.February))
	coupon, err := f.bf.coupon(id, day(2026, 2, 15))
	if err != nil {
		t.Fatalf("coupon: %v", err)
	}
	if err := f.r.DeleteInvestmentSnapshot(f.ctx, snap.ID); err != nil {
		t.Fatalf("delete snapshot: %v", err)
	}
	if err := f.r.DeleteInvestmentTransaction(f.ctx, coupon.ID); err != nil {
		t.Fatalf("delete coupon: %v", err)
	}

	wantErr(t, "bob undoes alice's snapshot", f.r.UndoDeleteInvestmentSnapshot(f.bctx, snap.ID), repo.ErrNotFound)
	wantErr(t, "bob undoes alice's coupon", f.r.UndoDeleteInvestmentTransaction(f.bctx, coupon.ID), repo.ErrNotFound)
	if f.liveSnapshots(t, id) != 0 || f.liveTxns(t, id) != 1 {
		t.Fatalf("bob's undo must restore nothing")
	}

	if err := f.r.UndoDeleteInvestmentSnapshot(f.ctx, snap.ID); err != nil {
		t.Fatalf("alice undoes her snapshot: %v", err)
	}
	if err := f.r.UndoDeleteInvestmentTransaction(f.ctx, coupon.ID); err != nil {
		t.Fatalf("alice undoes her coupon: %v", err)
	}
}

// ---- lifecycle terminate ----------------------------------------------------

func (f undoTermFx) terminate(ctx context.Context, id uuid.UUID, status string, at time.Time) error {
	_, err := f.r.UpdateInvestmentLifecycle(ctx, id, repo.LifecycleParams{Status: status, TerminatedAt: &at}, nil)
	return err
}

func (f undoTermFx) untouched(t *testing.T, id uuid.UUID, wantSnapshots int) {
	t.Helper()
	var status string
	var terminated *time.Time
	if err := f.tdb.Pool.QueryRow(context.Background(),
		`SELECT status, terminated_at FROM investments WHERE id = $1`, id).Scan(&status, &terminated); err != nil {
		t.Fatalf("read investment: %v", err)
	}
	if status != repo.StatusActive || terminated != nil {
		t.Fatalf("a refused terminate must leave the position active: status=%s terminated_at=%v", status, terminated)
	}
	if n := f.liveSnapshots(t, id); n != wantSnapshots {
		t.Fatalf("a refused terminate must write no close snapshot: %d live snapshots, want %d", n, wantSnapshots)
	}
}

// TestTerminateInvestment_RefusedOutsideWindow: the close snapshot lands at the
// termination month, so a month outside [earliest Buy, maturity] is refused
// before it is written, on the bond's and the deposit's own bounds.
// covers: INV-BONDS-03, INV-BONDS-05
func TestTerminateInvestment_RefusedOutsideWindow(t *testing.T) {
	t.Run("bond before the earliest Buy", func(t *testing.T) {
		f := newUndoTermFx(t)
		id := f.bond.Investment.ID
		wantErr(t, "terminate before the Buy", f.terminate(f.ctx, id, repo.StatusSold, day(2025, 12, 31)), repo.ErrOutsideBondTerm)
		f.untouched(t, id, 0)
	})

	t.Run("bond after maturity", func(t *testing.T) {
		f := newUndoTermFx(t)
		id := f.bond.Investment.ID
		wantErr(t, "terminate after maturity", f.terminate(f.ctx, id, repo.StatusSold, day(2027, 2, 1)), repo.ErrOutsideBondTerm)
		f.untouched(t, id, 0)
	})

	t.Run("bond with a snapshot in the termination month keeps it", func(t *testing.T) {
		f := newUndoTermFx(t)
		id := f.bond.Investment.ID
		f.bondSnapshot(t, ym(2026, time.February))
		wantErr(t, "terminate before the Buy", f.terminate(f.ctx, id, repo.StatusSold, day(2025, 12, 1)), repo.ErrOutsideBondTerm)
		f.untouched(t, id, 1)
	})

	t.Run("bond in window, both inclusive ends", func(t *testing.T) {
		for _, at := range []time.Time{day(2026, 1, 1), day(2027, 1, 31)} {
			f := newUndoTermFx(t)
			id := f.bond.Investment.ID
			if err := f.terminate(f.ctx, id, repo.StatusSold, at); err != nil {
				t.Fatalf("terminate on %s: %v", at.Format("2006-01-02"), err)
			}
			if n := f.liveSnapshots(t, id); n != 1 {
				t.Fatalf("an in-window terminate writes the close snapshot: %d, want 1", n)
			}
		}
	})

	t.Run("secondary market bond has only the maturity bound", func(t *testing.T) {
		f := newUndoTermFx(t)
		b, err := f.r.CreateBond(f.ctx, bondWindowParams("secondary_market", time.Time{}, day(2028, 6, 1)))
		if err != nil {
			t.Fatalf("CreateBond: %v", err)
		}
		id := b.Investment.ID
		wantErr(t, "terminate after maturity", f.terminate(f.ctx, id, repo.StatusSold, day(2028, 7, 1)), repo.ErrOutsideBondTerm)
		f.untouched(t, id, 0)
		if err := f.terminate(f.ctx, id, repo.StatusSold, day(2019, 1, 1)); err != nil {
			t.Fatalf("a bond with no Buy has no lower bound: %v", err)
		}
	})

	t.Run("time deposit", func(t *testing.T) {
		f := newUndoTermFx(t)
		id := f.td.Investment.ID
		wantErr(t, "terminate before placement", f.terminate(f.ctx, id, repo.StatusMatured, day(2025, 12, 15)), repo.ErrOutsideDepositTerm)
		wantErr(t, "terminate after maturity", f.terminate(f.ctx, id, repo.StatusMatured, day(2027, 2, 15)), repo.ErrOutsideDepositTerm)
		f.untouched(t, id, 0)
		if err := f.terminate(f.ctx, id, repo.StatusMatured, day(2027, 1, 15)); err != nil {
			t.Fatalf("terminate on the maturity day: %v", err)
		}
	})

	t.Run("stock is unbounded", func(t *testing.T) {
		f := newUndoTermFx(t)
		st, err := f.r.CreateStock(f.ctx, repo.CreateStockParams{
			DisplayName: "BBCA", OwnershipType: "joint", NativeCurrency: "IDR", RiskProfile: "high", Ticker: "BBCA",
		})
		if err != nil {
			t.Fatalf("CreateStock: %v", err)
		}
		if err := f.terminate(f.ctx, st.Investment.ID, repo.StatusSold, day(1999, 1, 1)); err != nil {
			t.Fatalf("a position without a term window terminates anywhere: %v", err)
		}
	})

	t.Run("another household cannot terminate it", func(t *testing.T) {
		f := newUndoTermFx(t)
		wantErr(t, "bob terminates alice's bond", f.terminate(f.bctx, f.bond.Investment.ID, repo.StatusSold, day(2026, 6, 1)), repo.ErrNotFound)
		f.untouched(t, f.bond.Investment.ID, 0)
	})
}

// ---- restored Buy that sets the lower bound ---------------------------------

// TestUndoDeleteBuy_CannotStrandHistory: restoring a Buy that becomes the
// bond's earliest moves the term start to it, so the whole live history must lie
// on or after it, as the forward "add a first Buy later than existing entries"
// rule requires. A Buy that leaves the start where it was never looks at other
// rows, so a legacy row elsewhere cannot block it.
// covers: INV-BONDS-05
func TestUndoDeleteBuy_CannotStrandHistory(t *testing.T) {
	newSecondary := func(t *testing.T) (undoTermFx, bondWindowFx, uuid.UUID) {
		f := newUndoTermFx(t)
		b, err := f.r.CreateBond(f.ctx, bondWindowParams("secondary_market", time.Time{}, day(2028, 6, 1)))
		if err != nil {
			t.Fatalf("CreateBond: %v", err)
		}
		return f, bondWindowFx{r: f.r, ctx: f.ctx, bond: b}, b.Investment.ID
	}

	t.Run("a coupon before the restored only Buy is refused", func(t *testing.T) {
		f, sf, id := newSecondary(t)
		buy, err := sf.buy(id, day(2026, 3, 1))
		if err != nil {
			t.Fatalf("Buy: %v", err)
		}
		if err := f.r.DeleteInvestmentTransaction(f.ctx, buy.ID); err != nil {
			t.Fatalf("delete the only Buy: %v", err)
		}
		if _, err := sf.coupon(id, day(2026, 2, 1)); err != nil {
			t.Fatalf("coupon with no Buy to bound it: %v", err)
		}

		wantErr(t, "restore a Buy after the coupon", f.r.UndoDeleteInvestmentTransaction(f.ctx, buy.ID), repo.ErrOutsideBondTerm)
		if n := f.liveTxns(t, id); n != 1 {
			t.Fatalf("a refused undo must restore nothing: %d live transactions, want 1 (the coupon)", n)
		}
	})

	t.Run("a snapshot before the restored only Buy is refused", func(t *testing.T) {
		f, sf, id := newSecondary(t)
		buy, err := sf.buy(id, day(2026, 3, 1))
		if err != nil {
			t.Fatalf("Buy: %v", err)
		}
		if err := f.r.DeleteInvestmentTransaction(f.ctx, buy.ID); err != nil {
			t.Fatalf("delete the only Buy: %v", err)
		}
		if _, err := sf.snapshot(id, ym(2026, time.January)); err != nil {
			t.Fatalf("snapshot with no Buy to bound it: %v", err)
		}
		wantErr(t, "restore a Buy after the snapshot", f.r.UndoDeleteInvestmentTransaction(f.ctx, buy.ID), repo.ErrOutsideBondTerm)
		if n := f.liveTxns(t, id); n != 0 {
			t.Fatalf("a refused undo must restore nothing: %d live transactions, want 0", n)
		}
	})

	t.Run("a restored Buy earlier than everything else is fine", func(t *testing.T) {
		f, sf, id := newSecondary(t)
		buy, err := sf.buy(id, day(2026, 1, 1))
		if err != nil {
			t.Fatalf("Buy: %v", err)
		}
		if err := f.r.DeleteInvestmentTransaction(f.ctx, buy.ID); err != nil {
			t.Fatalf("delete the only Buy: %v", err)
		}
		if _, err := sf.coupon(id, day(2026, 2, 1)); err != nil {
			t.Fatalf("coupon: %v", err)
		}
		if _, err := sf.snapshot(id, ym(2026, time.March)); err != nil {
			t.Fatalf("snapshot: %v", err)
		}
		if err := f.r.UndoDeleteInvestmentTransaction(f.ctx, buy.ID); err != nil {
			t.Fatalf("restored Buy earlier than all history: %v", err)
		}
		if n := f.liveTxns(t, id); n != 2 {
			t.Fatalf("undo must restore the Buy: %d live transactions, want 2", n)
		}
	})

	t.Run("a Buy on the same day as the earliest history is fine", func(t *testing.T) {
		f, sf, id := newSecondary(t)
		buy, err := sf.buy(id, day(2026, 2, 1))
		if err != nil {
			t.Fatalf("Buy: %v", err)
		}
		if err := f.r.DeleteInvestmentTransaction(f.ctx, buy.ID); err != nil {
			t.Fatalf("delete the only Buy: %v", err)
		}
		if _, err := sf.coupon(id, day(2026, 2, 1)); err != nil {
			t.Fatalf("coupon: %v", err)
		}
		if err := f.r.UndoDeleteInvestmentTransaction(f.ctx, buy.ID); err != nil {
			t.Fatalf("Buy on the coupon's day: %v", err)
		}
	})

	t.Run("a legacy row does not block a Buy that does not lower the start", func(t *testing.T) {
		f := newUndoTermFx(t)
		id := f.bond.Investment.ID
		legacy, err := f.bf.coupon(id, day(2026, 2, 1))
		if err != nil {
			t.Fatalf("coupon: %v", err)
		}
		later, err := f.bf.buy(id, day(2026, 6, 1))
		if err != nil {
			t.Fatalf("second Buy: %v", err)
		}
		if err := f.r.DeleteInvestmentTransaction(f.ctx, later.ID); err != nil {
			t.Fatalf("delete the second Buy: %v", err)
		}
		f.exec(t, `UPDATE investment_transactions SET transaction_date = $2 WHERE id = $1`, legacy.ID, day(2025, 6, 1))

		if err := f.r.UndoDeleteInvestmentTransaction(f.ctx, later.ID); err != nil {
			t.Fatalf("a later Buy beside a legacy row: %v", err)
		}
		if n := f.liveTxns(t, id); n != 3 {
			t.Fatalf("undo must restore the Buy: %d live transactions, want 3", n)
		}
	})
}

// ---- terminate settlement day -----------------------------------------------

// TestTerminateInvestment_SettlementHeldToTheDay: the settlement Sell/Maturity is
// written on the termination day without the forward create path, so the day
// must be inside the window itself. The termination month alone admits a day
// before the earliest Buy or after the maturity in the same month. A terminate
// with no settlement writes no transaction and is judged by the month only.
// covers: INV-BONDS-03, INV-BONDS-05
func TestTerminateInvestment_SettlementHeldToTheDay(t *testing.T) {
	sell := func() *repo.InvestmentSettlement {
		q, p := decimal.NewFromInt(10), decimal.NewFromInt(1_000_000)
		return &repo.InvestmentSettlement{Quantity: &q, PricePerUnit: &p}
	}
	maturity := func() *repo.InvestmentSettlement {
		p, i := decimal.NewFromInt(50_000_000), decimal.NewFromInt(2_000_000)
		return &repo.InvestmentSettlement{PrincipalAmount: &p, InterestAmount: &i}
	}
	settle := func(f undoTermFx, id uuid.UUID, status string, at time.Time, s *repo.InvestmentSettlement) error {
		_, err := f.r.UpdateInvestmentLifecycle(f.ctx, id, repo.LifecycleParams{Status: status, TerminatedAt: &at}, s)
		return err
	}

	t.Run("bond sold before its Buy in the same month", func(t *testing.T) {
		f := newUndoTermFx(t)
		id := f.bond.Investment.ID
		wantErr(t, "sell on the 10th, Buy on the 15th", settle(f, id, repo.StatusSold, day(2026, 1, 10), sell()), repo.ErrOutsideBondTerm)
		f.untouched(t, id, 0)
		if n := f.liveTxns(t, id); n != 1 {
			t.Fatalf("a refused terminate must write no settlement: %d live transactions, want 1 (the Buy)", n)
		}
	})

	t.Run("bond sold after maturity in the same month", func(t *testing.T) {
		f := newUndoTermFx(t)
		id := f.bond.Investment.ID
		wantErr(t, "sell the day after maturity", settle(f, id, repo.StatusSold, day(2027, 1, 16), sell()), repo.ErrOutsideBondTerm)
		f.untouched(t, id, 0)
	})

	t.Run("bond inclusive edges settle", func(t *testing.T) {
		for _, at := range []time.Time{day(2026, 1, 15), day(2027, 1, 15)} {
			f := newUndoTermFx(t)
			id := f.bond.Investment.ID
			if err := settle(f, id, repo.StatusSold, at, sell()); err != nil {
				t.Fatalf("sell on %s: %v", at.Format("2006-01-02"), err)
			}
			if n := f.liveTxns(t, id); n != 2 {
				t.Fatalf("an in-window sale writes its settlement: %d live transactions, want 2", n)
			}
		}
	})

	t.Run("bond with no settlement is judged by the month only", func(t *testing.T) {
		f := newUndoTermFx(t)
		if err := f.terminate(f.ctx, f.bond.Investment.ID, repo.StatusSold, day(2026, 1, 10)); err != nil {
			t.Fatalf("no settlement, no transaction to hold to the day: %v", err)
		}
	})

	t.Run("time deposit maturity outside placement..maturity to the day", func(t *testing.T) {
		f := newUndoTermFx(t)
		id := f.td.Investment.ID
		wantErr(t, "maturity the day before placement", settle(f, id, repo.StatusMatured, day(2026, 1, 14), maturity()), repo.ErrOutsideDepositTerm)
		wantErr(t, "maturity the day after maturity", settle(f, id, repo.StatusMatured, day(2027, 1, 16), maturity()), repo.ErrOutsideDepositTerm)
		f.untouched(t, id, 0)
		if n := f.liveTxns(t, id); n != 0 {
			t.Fatalf("a refused terminate must write no settlement: %d live transactions, want 0", n)
		}
	})

	t.Run("time deposit inclusive edges settle", func(t *testing.T) {
		for _, at := range []time.Time{day(2026, 1, 15), day(2027, 1, 15)} {
			f := newUndoTermFx(t)
			id := f.td.Investment.ID
			if err := settle(f, id, repo.StatusMatured, at, maturity()); err != nil {
				t.Fatalf("maturity on %s: %v", at.Format("2006-01-02"), err)
			}
			if n := f.liveTxns(t, id); n != 1 {
				t.Fatalf("an in-window maturity writes its settlement: %d live transactions, want 1", n)
			}
		}
	})
}
