package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// A bond has no placement column: its term opens at the date of its earliest
// live Buy and closes at maturity_date, both ends inclusive (#574). These tests
// pin that window on every write path the repo owns.

type bondWindowFx struct {
	r    *repo.InvestmentRepo
	ctx  context.Context
	bond *repo.Bond
}

// newBondWindowFx creates a govt_primary bond placed 2026-01-15 (seeding the
// placement Buy) maturing 2027-01-15.
func newBondWindowFx(t *testing.T) bondWindowFx {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)
	r := repo.NewInvestmentRepo(tdb.Pool)
	b, err := r.CreateBond(ctx, bondWindowParams("govt_primary", day(2026, 1, 15), day(2027, 1, 15)))
	if err != nil {
		t.Fatalf("CreateBond: %v", err)
	}
	return bondWindowFx{r: r, ctx: ctx, bond: b}
}

func bondWindowParams(bondType string, placement, maturity time.Time) repo.CreateBondParams {
	p := repo.CreateBondParams{
		DisplayName: "ORI Window", OwnershipType: "joint", NativeCurrency: "IDR", RiskProfile: "low",
		BondType: bondType, Issuer: "Republik Indonesia", CouponRate: decimal.RequireFromString("6.25"),
		CouponFrequency: "monthly", MaturityDate: maturity,
	}
	if bondType == "govt_primary" {
		face := decimal.NewFromInt(10_000_000)
		p.FaceValue = &face
		p.PlacementDate = &placement
	}
	return p
}

func (f bondWindowFx) updateParams(maturity time.Time) repo.UpdateBondParams {
	return repo.UpdateBondParams{
		DisplayName: "ORI Window", OwnershipType: "joint", RiskProfile: "low",
		BondType: f.bond.Details.BondType, Issuer: "Republik Indonesia",
		CouponRate: decimal.RequireFromString("6.25"), CouponFrequency: "monthly",
		CouponDisposition: repo.CouponDispositionPaysOut, MaturityDate: maturity,
	}
}

func (f bondWindowFx) snapshot(id uuid.UUID, month time.Time) (*db.InvestmentSnapshot, error) {
	accrued := decimal.Zero
	return f.r.CreateInvestmentSnapshot(f.ctx, repo.CreateInvestmentSnapshotParams{
		InvestmentID: id, YearMonth: month, Amount: decimal.NewFromInt(10_000_000),
		Currency: "IDR", AccruedInterest: &accrued,
	})
}

func (f bondWindowFx) coupon(id uuid.UUID, date time.Time) (*db.InvestmentTransaction, error) {
	amt := decimal.NewFromInt(46_875)
	return f.r.CreateInvestmentTransaction(f.ctx, repo.CreateInvestmentTransactionParams{
		InvestmentID: id, TransactionType: repo.TxnTypeCoupon, TransactionDate: date,
		Currency: "IDR", Amount: &amt,
	})
}

func (f bondWindowFx) buy(id uuid.UUID, date time.Time) (*db.InvestmentTransaction, error) {
	qty := decimal.RequireFromString("1")
	price := decimal.NewFromInt(1_000_000)
	amt := qty.Mul(price)
	return f.r.CreateInvestmentTransaction(f.ctx, repo.CreateInvestmentTransactionParams{
		InvestmentID: id, TransactionType: repo.TxnTypeBuy, TransactionDate: date,
		Currency: "IDR", Amount: &amt, Quantity: &qty, PricePerUnit: &price,
	})
}

func (f bondWindowFx) moveBuy(txn db.InvestmentTransaction, date time.Time) error {
	_, err := f.r.UpdateInvestmentTransaction(f.ctx, repo.UpdateInvestmentTransactionParams{
		TransactionID: txn.ID, TransactionDate: date, Currency: txn.Currency,
		Amount: txn.Amount, Quantity: txn.Quantity, PricePerUnit: txn.PricePerUnit,
	})
	return err
}

// earliestBuy returns the bond's earliest live Buy as the repo lists it.
func (f bondWindowFx) earliestBuy(t *testing.T, id uuid.UUID) db.InvestmentTransaction {
	t.Helper()
	txns, err := f.r.ListInvestmentTransactions(f.ctx, id)
	if err != nil {
		t.Fatalf("ListInvestmentTransactions: %v", err)
	}
	var out *db.InvestmentTransaction
	for i := range txns {
		if txns[i].TransactionType != repo.TxnTypeBuy {
			continue
		}
		if out == nil || txns[i].TransactionDate.Before(out.TransactionDate) {
			out = &txns[i]
		}
	}
	if out == nil {
		t.Fatalf("bond has no live Buy")
	}
	return *out
}

func wantErr(t *testing.T, label string, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("%s: want %v, got %v", label, want, got)
	}
}

// TestBond_RejectsInvertedTerm: maturity must be strictly after the placement
// Buy the create seeds.
// covers: INV-BONDS-05
func TestBond_RejectsInvertedTerm(t *testing.T) {
	f := newBondWindowFx(t)

	_, err := f.r.CreateBond(f.ctx, bondWindowParams("govt_primary", day(2026, 6, 1), day(2026, 6, 1)))
	wantErr(t, "maturity == placement", err, repo.ErrInvalidBondTerm)

	_, err = f.r.CreateBond(f.ctx, bondWindowParams("govt_primary", day(2026, 1, 1), day(2025, 1, 1)))
	wantErr(t, "maturity before placement", err, repo.ErrInvalidBondTerm)

	// Nothing was written for the rejected creates (the first bond is the only one).
	list, err := f.r.ListBonds(f.ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("rejected creates must leave no row: len=%d err=%v", len(list), err)
	}

	_, err = f.r.CreateBond(f.ctx, bondWindowParams("govt_primary", day(2026, 1, 1), day(2026, 1, 2)))
	if err != nil {
		t.Fatalf("one-day term is a valid window: %v", err)
	}
}

// TestBondSnapshot_ConfinedToWindow: snapshot months sit in [earliest Buy month,
// maturity month], inclusive, on create and on move.
// covers: INV-BONDS-05
func TestBondSnapshot_ConfinedToWindow(t *testing.T) {
	f := newBondWindowFx(t)
	id := f.bond.Investment.ID

	_, err := f.snapshot(id, ym(2025, time.December))
	wantErr(t, "month before the earliest Buy", err, repo.ErrOutsideBondTerm)
	_, err = f.snapshot(id, ym(2027, time.February))
	wantErr(t, "month after maturity", err, repo.ErrOutsideBondTerm)

	// A Buy on the 15th still admits its whole month, and the maturity month.
	first, err := f.snapshot(id, ym(2026, time.January))
	if err != nil {
		t.Fatalf("Buy-month snapshot: %v", err)
	}
	if _, err := f.snapshot(id, ym(2027, time.January)); err != nil {
		t.Fatalf("maturity-month snapshot: %v", err)
	}

	// Moving into the forbidden months is refused too.
	_, err = f.r.MoveInvestmentSnapshot(f.ctx, first.ID, ym(2025, time.December))
	wantErr(t, "move before the window", err, repo.ErrOutsideBondTerm)
	_, err = f.r.MoveInvestmentSnapshot(f.ctx, first.ID, ym(2027, time.February))
	wantErr(t, "move after the window", err, repo.ErrOutsideBondTerm)
	if _, err := f.r.MoveInvestmentSnapshot(f.ctx, first.ID, ym(2026, time.March)); err != nil {
		t.Fatalf("move inside the window: %v", err)
	}
}

// TestBondTransaction_ConfinedToWindow: transactions fall in [earliest Buy,
// maturity] to the day, inclusive, on create and on edit.
// covers: INV-BONDS-05
func TestBondTransaction_ConfinedToWindow(t *testing.T) {
	f := newBondWindowFx(t)
	id := f.bond.Investment.ID

	_, err := f.coupon(id, day(2026, 1, 14))
	wantErr(t, "coupon the day before the earliest Buy", err, repo.ErrOutsideBondTerm)
	_, err = f.coupon(id, day(2027, 1, 16))
	wantErr(t, "coupon the day after maturity", err, repo.ErrOutsideBondTerm)

	onBuy, err := f.coupon(id, day(2026, 1, 15))
	if err != nil {
		t.Fatalf("coupon on the Buy day: %v", err)
	}
	if _, err := f.coupon(id, day(2027, 1, 15)); err != nil {
		t.Fatalf("coupon on the maturity day: %v", err)
	}

	// Editing a coupon out of the window is refused as well.
	_, err = f.r.UpdateInvestmentTransaction(f.ctx, repo.UpdateInvestmentTransactionParams{
		TransactionID: onBuy.ID, TransactionDate: day(2025, 12, 1), Currency: "IDR", Amount: onBuy.Amount,
	})
	wantErr(t, "edit coupon before the window", err, repo.ErrOutsideBondTerm)
}

// TestBond_MaturityEditCannotStrandHistory: editing maturity is refused when it
// is not after the earliest Buy, or leaves a snapshot/transaction past it.
// covers: INV-BONDS-05
func TestBond_MaturityEditCannotStrandHistory(t *testing.T) {
	f := newBondWindowFx(t)
	id := f.bond.Investment.ID

	// The issue's third repro: maturity at or before the only Buy.
	_, err := f.r.UpdateBond(f.ctx, id, f.updateParams(day(2000, 1, 1)))
	wantErr(t, "maturity before the Buy", err, repo.ErrInvalidBondTerm)
	_, err = f.r.UpdateBond(f.ctx, id, f.updateParams(day(2026, 1, 15)))
	wantErr(t, "maturity on the Buy day", err, repo.ErrInvalidBondTerm)

	if _, err := f.snapshot(id, ym(2026, time.June)); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	coupon, err := f.coupon(id, day(2026, 9, 15))
	if err != nil {
		t.Fatalf("coupon: %v", err)
	}

	// Maturity in May strands the June snapshot; in July the September coupon is
	// caught instead once the snapshot is out of the way.
	_, err = f.r.UpdateBond(f.ctx, id, f.updateParams(day(2026, 5, 20)))
	wantErr(t, "maturity would strand the snapshot", err, repo.ErrOutsideBondTerm)
	_, err = f.r.UpdateBond(f.ctx, id, f.updateParams(day(2026, 9, 14)))
	wantErr(t, "maturity would strand the coupon", err, repo.ErrOutsideBondTerm)

	// A rejected edit changed nothing.
	got, err := f.r.GetBond(f.ctx, id)
	if err != nil || !got.Details.MaturityDate.Equal(day(2027, 1, 15)) {
		t.Fatalf("rejected edit must not persist: maturity=%v err=%v", got.Details.MaturityDate, err)
	}

	// Widening, and narrowing exactly onto the last entry, are fine.
	if _, err := f.r.UpdateBond(f.ctx, id, f.updateParams(day(2028, 1, 15))); err != nil {
		t.Fatalf("widen maturity: %v", err)
	}
	if _, err := f.r.UpdateBond(f.ctx, id, f.updateParams(coupon.TransactionDate)); err != nil {
		t.Fatalf("narrow onto the last transaction day: %v", err)
	}
}

// TestBond_EarliestBuyEditAndDeleteCannotStrandHistory: moving the earliest Buy
// later, or deleting it, is refused when history would fall before the new
// earliest Buy; widening is fine.
// covers: INV-BONDS-05
func TestBond_EarliestBuyEditAndDeleteCannotStrandHistory(t *testing.T) {
	f := newBondWindowFx(t)
	id := f.bond.Investment.ID
	buy := f.earliestBuy(t, id)

	if _, err := f.snapshot(id, ym(2026, time.February)); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if _, err := f.coupon(id, day(2026, 2, 15)); err != nil {
		t.Fatalf("coupon: %v", err)
	}

	// Later than the February snapshot: stranded.
	wantErr(t, "move Buy past the snapshot", f.moveBuy(buy, day(2026, 3, 1)), repo.ErrOutsideBondTerm)
	// Later than the coupon but still in February's month: the coupon is stranded.
	wantErr(t, "move Buy past the coupon", f.moveBuy(buy, day(2026, 2, 20)), repo.ErrOutsideBondTerm)
	// At or after maturity: the term would be empty.
	wantErr(t, "move Buy to maturity", f.moveBuy(buy, day(2027, 1, 15)), repo.ErrInvalidBondTerm)
	wantErr(t, "move Buy past maturity", f.moveBuy(buy, day(2027, 6, 1)), repo.ErrInvalidBondTerm)
	if got := f.earliestBuy(t, id); !got.TransactionDate.Equal(day(2026, 1, 15)) {
		t.Fatalf("rejected edits must not persist: Buy is now %v", got.TransactionDate)
	}

	// A second, later Buy: deleting the earliest one would leave February's
	// snapshot before the new earliest Buy (March).
	march, err := f.buy(id, day(2026, 3, 10))
	if err != nil {
		t.Fatalf("second Buy: %v", err)
	}
	wantErr(t, "delete earliest Buy", f.r.DeleteInvestmentTransaction(f.ctx, buy.ID), repo.ErrOutsideBondTerm)

	// Within the tolerance the move/delete are fine: Buy onto the snapshot month,
	// earlier, and finally the second Buy goes (it is not the earliest).
	if err := f.moveBuy(buy, day(2026, 2, 1)); err != nil {
		t.Fatalf("move earliest Buy inside the history: %v", err)
	}
	if err := f.moveBuy(buy, day(2025, 11, 3)); err != nil {
		t.Fatalf("move earliest Buy earlier: %v", err)
	}
	if err := f.r.DeleteInvestmentTransaction(f.ctx, march.ID); err != nil {
		t.Fatalf("delete a non-earliest Buy: %v", err)
	}

	// A non-earliest Buy past maturity falls outside the window.
	_, err = f.buy(id, day(2027, 1, 16))
	wantErr(t, "Buy after maturity", err, repo.ErrOutsideBondTerm)
}

// TestBond_SecondBuyAndSoleBuyBehaviour: a Buy earlier than the current earliest
// Buy widens the window; deleting the only Buy leaves no lower bound at all.
// covers: INV-BONDS-05
func TestBond_SecondBuyAndSoleBuyBehaviour(t *testing.T) {
	f := newBondWindowFx(t)
	id := f.bond.Investment.ID
	buy := f.earliestBuy(t, id)

	if _, err := f.snapshot(id, ym(2026, time.January)); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	early, err := f.buy(id, day(2025, 10, 1))
	if err != nil {
		t.Fatalf("an earlier Buy widens the window: %v", err)
	}
	if _, err := f.snapshot(id, ym(2025, time.October)); err != nil {
		t.Fatalf("snapshot in the widened window: %v", err)
	}
	// Deleting the original (no longer earliest) Buy is fine, and the earlier one
	// still bounds the window.
	if err := f.r.DeleteInvestmentTransaction(f.ctx, buy.ID); err != nil {
		t.Fatalf("delete the later Buy: %v", err)
	}
	_, err = f.snapshot(id, ym(2025, time.September))
	wantErr(t, "month before the (new) earliest Buy", err, repo.ErrOutsideBondTerm)

	// Deleting the only Buy removes the lower bound: nothing is stranded because
	// there is no window start (the secondary_market no-Buy rule applies).
	if err := f.r.DeleteInvestmentTransaction(f.ctx, early.ID); err != nil {
		t.Fatalf("delete the only Buy: %v", err)
	}
	if _, err := f.snapshot(id, ym(2020, time.January)); err != nil {
		t.Fatalf("no Buy, no lower bound: %v", err)
	}
}

// TestBond_SoftDeletedRowsDoNotCount: an archived Buy does not open the window,
// and an archived snapshot/transaction cannot block a window edit.
// covers: INV-BONDS-05
func TestBond_SoftDeletedRowsDoNotCount(t *testing.T) {
	f := newBondWindowFx(t)
	id := f.bond.Investment.ID
	buy := f.earliestBuy(t, id)

	early, err := f.buy(id, day(2025, 6, 1))
	if err != nil {
		t.Fatalf("early Buy: %v", err)
	}
	if err := f.r.DeleteInvestmentTransaction(f.ctx, early.ID); err != nil {
		t.Fatalf("archive the early Buy: %v", err)
	}
	// The window starts at the live earliest Buy (Jan 2026), not the archived one.
	_, err = f.snapshot(id, ym(2025, time.June))
	wantErr(t, "month the archived Buy would have admitted", err, repo.ErrOutsideBondTerm)
	if err := f.moveBuy(buy, day(2026, 2, 1)); err != nil {
		t.Fatalf("move the live Buy later, the archived one must not block it: %v", err)
	}

	// An archived snapshot past the new maturity does not strand anything.
	snap, err := f.snapshot(id, ym(2026, time.December))
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if err := f.r.DeleteInvestmentSnapshot(f.ctx, snap.ID); err != nil {
		t.Fatalf("archive snapshot: %v", err)
	}
	if _, err := f.r.UpdateBond(f.ctx, id, f.updateParams(day(2026, 6, 1))); err != nil {
		t.Fatalf("an archived snapshot must not block narrowing the maturity: %v", err)
	}
}

// TestBond_SecondaryMarketWithoutBuy: with no Buy there is no window start, so
// only the maturity upper bound applies; the first Buy may not strand history.
// covers: INV-BONDS-05
func TestBond_SecondaryMarketWithoutBuy(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)
	r := repo.NewInvestmentRepo(tdb.Pool)
	b, err := r.CreateBond(ctx, bondWindowParams("secondary_market", time.Time{}, day(2028, 6, 1)))
	if err != nil {
		t.Fatalf("CreateBond: %v", err)
	}
	f := bondWindowFx{r: r, ctx: ctx, bond: b}
	id := b.Investment.ID

	// No Buy: any month up to maturity, nothing after it.
	if _, err := f.snapshot(id, ym(2020, time.January)); err != nil {
		t.Fatalf("no lower bound without a Buy: %v", err)
	}
	if _, err := f.coupon(id, day(2021, 3, 1)); err != nil {
		t.Fatalf("coupon without a Buy: %v", err)
	}
	_, err = f.snapshot(id, ym(2028, time.July))
	wantErr(t, "snapshot after maturity", err, repo.ErrOutsideBondTerm)
	_, err = f.coupon(id, day(2028, 6, 2))
	wantErr(t, "coupon after maturity", err, repo.ErrOutsideBondTerm)

	// Maturity edits still guard history, and need no Buy to compare against.
	_, err = r.UpdateBond(ctx, id, f.updateParams(day(2020, 6, 1)))
	wantErr(t, "maturity would strand the 2021 coupon", err, repo.ErrOutsideBondTerm)

	// The first Buy opens the window: dated after the oldest entry it would strand
	// the 2020 snapshot, so it is refused; dated before it, it is accepted.
	_, err = f.buy(id, day(2022, 1, 1))
	wantErr(t, "first Buy after existing history", err, repo.ErrOutsideBondTerm)
	_, err = f.buy(id, day(2028, 6, 2))
	wantErr(t, "first Buy after maturity", err, repo.ErrInvalidBondTerm)
	if _, err := f.buy(id, day(2019, 12, 31)); err != nil {
		t.Fatalf("first Buy before the history: %v", err)
	}
}

// TestBond_WindowWritePathsTenancy: bob cannot probe or mutate alice's bond
// window through the Buy delete/edit paths (404), and alice's still works.
// covers: INV-BONDS-05
func TestBond_WindowWritePathsTenancy(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := identity.WithUser(context.Background(), alice)
	bobCtx := identity.WithUser(context.Background(), bob)
	r := repo.NewInvestmentRepo(tdb.Pool)

	b, err := r.CreateBond(aliceCtx, bondWindowParams("govt_primary", day(2026, 1, 15), day(2027, 1, 15)))
	if err != nil {
		t.Fatalf("CreateBond: %v", err)
	}
	f := bondWindowFx{r: r, ctx: aliceCtx, bond: b}
	buy := f.earliestBuy(t, b.Investment.ID)

	wantErr(t, "bob deletes alice's Buy", r.DeleteInvestmentTransaction(bobCtx, buy.ID), repo.ErrNotFound)
	_, err = r.UpdateBond(bobCtx, b.Investment.ID, f.updateParams(day(2000, 1, 1)))
	wantErr(t, "bob edits alice's maturity", err, repo.ErrNotFound)
	_, err = r.CreateInvestmentTransaction(bobCtx, repo.CreateInvestmentTransactionParams{
		InvestmentID: b.Investment.ID, TransactionType: repo.TxnTypeBuy, TransactionDate: day(2026, 2, 1),
		Currency: "IDR", Amount: buy.Amount, Quantity: buy.Quantity, PricePerUnit: buy.PricePerUnit,
	})
	wantErr(t, "bob adds a Buy", err, repo.ErrNotFound)

	var live int
	if err := tdb.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM investment_transactions WHERE investment_id = $1 AND deleted_at IS NULL`,
		b.Investment.ID).Scan(&live); err != nil || live != 1 {
		t.Fatalf("alice's ledger must be untouched by bob: live=%d err=%v", live, err)
	}

	// Alice's own happy path through the same code: edit then delete the sole Buy.
	if err := f.moveBuy(buy, day(2026, 2, 1)); err != nil {
		t.Fatalf("alice moves her Buy: %v", err)
	}
	if err := r.DeleteInvestmentTransaction(aliceCtx, buy.ID); err != nil {
		t.Fatalf("alice deletes her Buy: %v", err)
	}
	if err := tdb.Pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM investment_transactions WHERE investment_id = $1 AND deleted_at IS NULL`,
		b.Investment.ID).Scan(&live); err != nil || live != 0 {
		t.Fatalf("alice's delete must archive the row: live=%d err=%v", live, err)
	}
}

// TestBond_MonthlyEntryListsOnlyInWindowMonths: the bulk accrued entry list, and
// so its all-or-nothing write, only offers a bond for months in its window.
// covers: INV-BONDS-05
func TestBond_MonthlyEntryListsOnlyInWindowMonths(t *testing.T) {
	f := newBondWindowFx(t) // window 2026-01 .. 2027-01
	has := func(month time.Time) bool {
		rows, err := f.r.ListInvestmentAccruedEntryRows(f.ctx, month)
		if err != nil {
			t.Fatalf("ListInvestmentAccruedEntryRows: %v", err)
		}
		for _, row := range rows {
			if row.InvestmentID == f.bond.Investment.ID {
				return true
			}
		}
		return false
	}
	for _, c := range []struct {
		month time.Time
		want  bool
	}{
		{ym(2025, time.December), false}, {ym(2026, time.January), true},
		{ym(2027, time.January), true}, {ym(2027, time.February), false},
	} {
		if got := has(c.month); got != c.want {
			t.Errorf("entry list for %s: listed=%v, want %v", c.month.Format("2006-01"), got, c.want)
		}
	}

	zeroAccrued := decimal.Zero
	_, rowErrs, err := f.r.BulkUpsertInvestmentAccruedSnapshots(f.ctx, repo.BulkUpsertInvestmentAccruedSnapshotsParams{
		YearMonth: ym(2025, time.December),
		Rows: []repo.BulkInvestmentAccruedSnapshotRow{{
			InvestmentID: f.bond.Investment.ID, Currency: "IDR",
			Amount: decimal.NewFromInt(10_000_000), AccruedInterest: &zeroAccrued,
		}},
	})
	if err != nil || len(rowErrs) != 1 {
		t.Fatalf("a month before the window must be rejected as ineligible: rowErrs=%v err=%v", rowErrs, err)
	}

	// A bond with no Buy has no lower bound but still stops at maturity.
	sec, err := f.r.CreateBond(f.ctx, bondWindowParams("secondary_market", time.Time{}, day(2028, 6, 1)))
	if err != nil {
		t.Fatalf("secondary bond: %v", err)
	}
	rows, err := f.r.ListInvestmentAccruedEntryRows(f.ctx, ym(2015, time.January))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, row := range rows {
		found = found || row.InvestmentID == sec.Investment.ID
	}
	if !found {
		t.Errorf("a Buy-less bond must be offered for any month up to maturity")
	}
}

// TestBondSnapshotImport_ConfinedToWindow: the workbook snapshot import holds a
// bond's months inside its window the way the manual paths do — nothing written,
// in the preview and the commit alike.
// covers: INV-BONDS-05
func TestBondSnapshotImport_ConfinedToWindow(t *testing.T) {
	f := newBondWindowFx(t)
	id := f.bond.Investment.ID
	accrued := decimal.Zero
	row := func(month time.Time) repo.ImportInvestmentSnapshotRow {
		return repo.ImportInvestmentSnapshotRow{YearMonth: month, Amount: decimal.NewFromInt(10_000_000), Currency: "IDR", AccruedInterest: &accrued}
	}
	for _, dry := range []bool{true, false} {
		_, err := f.r.ImportInvestmentSnapshots(f.ctx, id, []repo.ImportInvestmentSnapshotRow{row(ym(2026, time.March)), row(ym(2025, time.December))}, dry)
		wantErr(t, "import row before the window", err, repo.ErrOutsideBondTerm)
		_, err = f.r.ImportInvestmentSnapshots(f.ctx, id, []repo.ImportInvestmentSnapshotRow{row(ym(2027, time.February))}, dry)
		wantErr(t, "import row after the window", err, repo.ErrOutsideBondTerm)
	}
	snaps, err := f.r.ListInvestmentSnapshots(f.ctx, id)
	if err != nil || len(snaps) != 0 {
		t.Fatalf("a refused import writes nothing: snaps=%d err=%v", len(snaps), err)
	}
	if _, err := f.r.ImportInvestmentSnapshots(f.ctx, id, []repo.ImportInvestmentSnapshotRow{row(ym(2026, time.March))}, false); err != nil {
		t.Fatalf("in-window import: %v", err)
	}
}

// TestCreateBondWithSnapshotsAndLedger_TermWindow: the create-from-list seed
// path applies the same window and rolls the whole seed back on a violation.
// covers: INV-BONDS-05
func TestCreateBondWithSnapshotsAndLedger_TermWindow(t *testing.T) {
	r, ctx := investmentRepoFor(t)
	zero := decimal.Zero
	snap := func(month time.Time) repo.ImportInvestmentSnapshotRow {
		return repo.ImportInvestmentSnapshotRow{YearMonth: month, Amount: decimal.NewFromInt(10_000_000), Currency: "IDR", AccruedInterest: &zero}
	}
	buy := func(date time.Time) repo.ImportTransactionRow {
		return repo.ImportTransactionRow{
			TransactionType: "buy", TransactionDate: date, Currency: "IDR",
			Amount: ptrDec("10000000"), Quantity: ptrDec("10"), PricePerUnit: ptrDec("1000000"),
		}
	}
	coupon := func(date time.Time) repo.ImportTransactionRow {
		return repo.ImportTransactionRow{TransactionType: "coupon", TransactionDate: date, Currency: "IDR", Amount: ptrDec("46875")}
	}
	params := bondWindowParams("secondary_market", time.Time{}, day(2027, 1, 15))

	_, err := r.CreateBondWithSnapshotsAndLedger(ctx, params, nil, nil, []repo.ImportTransactionRow{buy(day(2027, 1, 15))})
	wantErr(t, "earliest Buy on maturity", err, repo.ErrInvalidBondTerm)
	_, err = r.CreateBondWithSnapshotsAndLedger(ctx, params, nil, []repo.ImportInvestmentSnapshotRow{snap(ym(2025, time.December))}, []repo.ImportTransactionRow{buy(day(2026, 1, 15))})
	wantErr(t, "snapshot before the earliest Buy", err, repo.ErrOutsideBondTerm)
	_, err = r.CreateBondWithSnapshotsAndLedger(ctx, params, nil, nil, []repo.ImportTransactionRow{buy(day(2026, 1, 15)), coupon(day(2025, 12, 1))})
	wantErr(t, "coupon before the earliest Buy", err, repo.ErrOutsideBondTerm)
	_, err = r.CreateBondWithSnapshotsAndLedger(ctx, params, nil, []repo.ImportInvestmentSnapshotRow{snap(ym(2027, time.February))}, nil)
	wantErr(t, "snapshot after maturity, no Buy", err, repo.ErrOutsideBondTerm)

	list, err := r.ListBonds(ctx)
	if err != nil || len(list) != 0 {
		t.Fatalf("every refused seed must roll back: bonds=%d err=%v", len(list), err)
	}

	if _, err := r.CreateBondWithSnapshotsAndLedger(ctx, params, nil,
		[]repo.ImportInvestmentSnapshotRow{snap(ym(2026, time.January))},
		[]repo.ImportTransactionRow{coupon(day(2026, 2, 1)), buy(day(2026, 1, 15))}); err != nil {
		t.Fatalf("a window-respecting seed must succeed regardless of row order: %v", err)
	}
}
