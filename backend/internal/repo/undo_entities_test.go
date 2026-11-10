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

// Undo of a single-item delete for the entities that are not snapshots
// (ADR-0058 §2): investment transaction, income row, FX rate, inflation rate,
// tag. Row state is read from the table directly - deletedAtOf, not the repo's
// Get, which hides a soft-deleted row and so cannot tell "restored" from
// "archived but invisible".

type undoPair struct {
	alice, bob context.Context
	aliceUser  db.User
	pool       *pgxpool.Pool
}

func newUndoPair(t *testing.T) undoPair {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	a := testutil.CreateHouseholdWithUser(t, q, "Alice")
	b := testutil.CreateHouseholdWithUser(t, q, "Bob")
	return undoPair{
		alice:     identity.WithUser(context.Background(), a),
		bob:       identity.WithUser(context.Background(), b),
		aliceUser: a,
		pool:      tdb.Pool,
	}
}

func (p undoPair) archived(t *testing.T, table string, id uuid.UUID) bool {
	t.Helper()
	return deletedAtOf(t, p.pool, table, id) != nil
}

// reportAt reads the month's regenerated report for alice.
func (p undoPair) reportAt(t *testing.T, month time.Time) db.MonthlyReport {
	t.Helper()
	rep, err := repo.NewMonthlyReportRepo(p.pool).GetReport(p.alice, month)
	if err != nil {
		t.Fatalf("GetReport %s: %v", month.Format("2006-01"), err)
	}
	return *rep
}

func decOrZero(d *decimal.Decimal) decimal.Decimal {
	if d == nil {
		return decimal.Zero
	}
	return *d
}

// TestUndoDeleteIncome: restore, report effect, and tenancy for an income row.
// The delete and the restore both move the report watermark, so the month's
// earned income drops and then returns on the next read.
//
// covers: INV-SOFT-DELETE-06, INV-SOFT-DELETE-08, INV-SOFT-DELETE-11
func TestUndoDeleteIncome(t *testing.T) {
	p := newUndoPair(t)
	ar := repo.NewAssetRepo(p.pool)
	ir := repo.NewIncomeRepo(p.pool)

	acct := newBankAccount(p.alice, t, ar, "Checking")
	if _, err := ar.CreateAssetSnapshot(p.alice, repo.CreateAssetSnapshotParams{AssetID: acct.Asset.ID, YearMonth: replSep, Amount: rd("1000"), Currency: "IDR"}); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	mk := func(amount string) *db.Income {
		inc, err := ir.CreateIncome(p.alice, repo.CreateIncomeParams{
			Date: dayOf(2026, time.September, 15).UTC(), Amount: rd(amount), Currency: "IDR", Category: "salary",
			OwnershipType: "sole", SoleOwnerUserID: &p.aliceUser.ID, Regularity: "routine",
		})
		if err != nil {
			t.Fatalf("CreateIncome: %v", err)
		}
		return inc
	}
	kept, gone := mk("30"), mk("70")
	earned := func() decimal.Decimal { return decOrZero(p.reportAt(t, replSep).EarnedIncomeTotal) }
	if got := earned(); !got.Equal(rd("100")) {
		t.Fatalf("earned income before = %s, want 100", got)
	}

	if err := ir.UndoDeleteIncome(p.alice, gone.ID); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("undo of a live row: want ErrNotFound, got %v", err)
	}
	if err := ir.DeleteIncome(p.alice, gone.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := earned(); !got.Equal(rd("30")) {
		t.Fatalf("earned income after delete = %s, want 30", got)
	}

	if err := ir.UndoDeleteIncome(p.bob, gone.ID); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("bob undo: want ErrNotFound, got %v", err)
	}
	if !p.archived(t, "income", gone.ID) {
		t.Fatal("a cross-household undo restored alice's income")
	}

	if err := ir.UndoDeleteIncome(p.alice, gone.ID); err != nil {
		t.Fatalf("undo: %v", err)
	}
	if got := earned(); !got.Equal(rd("100")) {
		t.Errorf("earned income after undo = %s, want 100", got)
	}
	if p.archived(t, "income", gone.ID) || p.archived(t, "income", kept.ID) {
		t.Error("income rows are not both live after the undo")
	}
}

// TestUndoDeleteInvestmentTransaction: a deleted ledger row comes back with its
// effect on the month's investment return; another household cannot restore it;
// and it cannot be restored under a deleted investment.
//
// covers: INV-SOFT-DELETE-06, INV-SOFT-DELETE-08, INV-SOFT-DELETE-11
func TestUndoDeleteInvestmentTransaction(t *testing.T) {
	p := newUndoPair(t)
	ir := repo.NewInvestmentRepo(p.pool)

	stock, err := ir.CreateStock(p.alice, repo.CreateStockParams{
		DisplayName: "BBCA", OwnershipType: "joint", NativeCurrency: "IDR", RiskProfile: "medium", Ticker: "BBCA", Exchange: "IDX",
	})
	if err != nil {
		t.Fatalf("CreateStock: %v", err)
	}
	// August 800000 -> September 900000 with a 100000 buy in September is no
	// return; without the buy the same step reads as 100000 of return.
	aug := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	for _, m := range []struct {
		month      time.Time
		qty, total string
	}{{aug, "80", "800000"}, {replSep, "90", "900000"}} {
		qty, price, total := rd(m.qty), rd("10000"), rd(m.total)
		if _, err := ir.CreateInvestmentSnapshot(p.alice, repo.CreateInvestmentSnapshotParams{
			InvestmentID: stock.Investment.ID, YearMonth: m.month, Amount: total, Currency: "IDR", Quantity: &qty, PricePerUnit: &price,
		}); err != nil {
			t.Fatalf("snapshot: %v", err)
		}
	}
	qty, price, total := rd("10"), rd("10000"), rd("100000")
	buy, err := ir.CreateInvestmentTransaction(p.alice, repo.CreateInvestmentTransactionParams{
		InvestmentID: stock.Investment.ID, TransactionType: repo.TxnTypeBuy,
		TransactionDate: dayOf(2026, time.September, 5).UTC(), Currency: "IDR", Amount: &total, Quantity: &qty, PricePerUnit: &price,
	})
	if err != nil {
		t.Fatalf("buy: %v", err)
	}
	performance := func() decimal.Decimal { return decOrZero(p.reportAt(t, replSep).InvestmentReturnTotal) }
	before := performance()

	if err := ir.DeleteInvestmentTransaction(p.alice, buy.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := performance(); got.Equal(before) {
		t.Fatalf("deleting the buy left investment return at %s: the assertion below could not fail", got)
	}

	if err := ir.UndoDeleteInvestmentTransaction(p.bob, buy.ID); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("bob undo: want ErrNotFound, got %v", err)
	}
	if !p.archived(t, "investment_transactions", buy.ID) {
		t.Fatal("a cross-household undo restored alice's transaction")
	}
	if err := ir.UndoDeleteInvestmentTransaction(p.alice, buy.ID); err != nil {
		t.Fatalf("undo: %v", err)
	}
	if got := performance(); !got.Equal(before) {
		t.Errorf("investment return after undo = %s, want %s", got, before)
	}
	if err := ir.UndoDeleteInvestmentTransaction(p.alice, buy.ID); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("second undo: want ErrNotFound, got %v", err)
	}

	t.Run("under a deleted investment", func(t *testing.T) {
		if err := ir.DeleteInvestmentTransaction(p.alice, buy.ID); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if err := ir.DeleteStock(p.alice, stock.Investment.ID); err != nil {
			t.Fatalf("delete stock: %v", err)
		}
		if err := ir.UndoDeleteInvestmentTransaction(p.alice, buy.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("undo under a deleted parent: want ErrNotFound, got %v", err)
		}
		if !p.archived(t, "investment_transactions", buy.ID) {
			t.Error("a refused undo restored the transaction")
		}
	})
}

// TestUndoDeleteFxRate: the rate returns to the report; the slot is the
// (month, currency) pair, so a different currency in the same month does not
// block the undo but the same currency does; tenancy refuses.
//
// covers: INV-SOFT-DELETE-06, INV-SOFT-DELETE-07, INV-SOFT-DELETE-08, INV-SOFT-DELETE-11
func TestUndoDeleteFxRate(t *testing.T) {
	p := newUndoPair(t)
	fr := repo.NewFxRateRepo(p.pool)
	ar := repo.NewAssetRepo(p.pool)

	if _, err := p.pool.Exec(context.Background(), "UPDATE households SET multi_currency_enabled = true WHERE id = $1", p.aliceUser.HouseholdID); err != nil {
		t.Fatalf("enable multi-currency: %v", err)
	}
	acct, err := ar.CreateBankAccount(p.alice, repo.CreateBankAccountParams{
		DisplayName: "USD acct", OwnershipType: "joint", NativeCurrency: "USD", BankName: "Wise", AccountNumber: "1", AccountType: "savings",
	})
	if err != nil {
		t.Fatalf("CreateBankAccount: %v", err)
	}
	if _, err := ar.CreateAssetSnapshot(p.alice, repo.CreateAssetSnapshotParams{AssetID: acct.Asset.ID, YearMonth: replSep, Amount: rd("10"), Currency: "USD"}); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	usd, err := fr.CreateFxRate(p.alice, repo.CreateFxRateParams{YearMonth: replSep, Currency: "USD", Rate: rd("16000")})
	if err != nil {
		t.Fatalf("CreateFxRate: %v", err)
	}
	nw := func() decimal.Decimal { return p.reportAt(t, replSep).NwTotal }
	if got := nw(); !got.Equal(rd("160000")) {
		t.Fatalf("nw_total with the rate = %s, want 160000", got)
	}

	if err := fr.DeleteFxRate(p.alice, usd.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if got := nw(); got.Equal(rd("160000")) {
		t.Fatalf("deleting the rate left nw_total at %s: the assertion below could not fail", got)
	}

	if err := fr.UndoDeleteFxRate(p.bob, usd.ID); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("bob undo: want ErrNotFound, got %v", err)
	}
	if !p.archived(t, "fx_rates", usd.ID) {
		t.Fatal("a cross-household undo restored alice's rate")
	}

	t.Run("a second currency in the month does not block", func(t *testing.T) {
		if _, err := fr.CreateFxRate(p.alice, repo.CreateFxRateParams{YearMonth: replSep, Currency: "EUR", Rate: rd("17000")}); err != nil {
			t.Fatalf("CreateFxRate EUR: %v", err)
		}
		if err := fr.UndoDeleteFxRate(p.alice, usd.ID); err != nil {
			t.Fatalf("undo: %v", err)
		}
		if got := nw(); !got.Equal(rd("160000")) {
			t.Errorf("nw_total after undo = %s, want 160000", got)
		}
	})

	t.Run("same month and currency re-entered", func(t *testing.T) {
		if err := fr.DeleteFxRate(p.alice, usd.ID); err != nil {
			t.Fatalf("delete: %v", err)
		}
		fresh, err := fr.CreateFxRate(p.alice, repo.CreateFxRateParams{YearMonth: replSep, Currency: "USD", Rate: rd("15000")})
		if err != nil {
			t.Fatalf("CreateFxRate: %v", err)
		}
		if err := fr.UndoDeleteFxRate(p.alice, usd.ID); !errors.Is(err, repo.ErrUndoSlotTaken) {
			t.Fatalf("want ErrUndoSlotTaken, got %v", err)
		}
		if !p.archived(t, "fx_rates", usd.ID) || p.archived(t, "fx_rates", fresh.ID) {
			t.Error("a refused undo changed row state")
		}
	})
}

// covers: INV-SOFT-DELETE-06, INV-SOFT-DELETE-07, INV-SOFT-DELETE-08
func TestUndoDeleteInflationRate(t *testing.T) {
	p := newUndoPair(t)
	r := repo.NewInflationRateRepo(p.pool)

	// A negative rate (deflation) is a legal value and must come back as stored.
	row, err := r.CreateInflationRate(p.alice, repo.CreateInflationRateParams{YearMonth: replSep, Rate: rd("-0.5")})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := r.UndoDeleteInflationRate(p.alice, row.ID); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("undo of a live row: want ErrNotFound, got %v", err)
	}
	if err := r.DeleteInflationRate(p.alice, row.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := r.UndoDeleteInflationRate(p.bob, row.ID); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("bob undo: want ErrNotFound, got %v", err)
	}
	if !p.archived(t, "inflation_rates", row.ID) {
		t.Fatal("a cross-household undo restored alice's rate")
	}

	if err := r.UndoDeleteInflationRate(p.alice, row.ID); err != nil {
		t.Fatalf("undo: %v", err)
	}
	list, err := r.ListInflationRates(p.alice)
	if err != nil || len(list) != 1 || !list[0].Rate.Equal(rd("-0.5")) {
		t.Fatalf("after undo: list=%+v err=%v, want the one rate -0.5", list, err)
	}

	if err := r.DeleteInflationRate(p.alice, row.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	fresh, err := r.CreateInflationRate(p.alice, repo.CreateInflationRateParams{YearMonth: replSep, Rate: rd("3")})
	if err != nil {
		t.Fatalf("recreate: %v", err)
	}
	if err := r.UndoDeleteInflationRate(p.alice, row.ID); !errors.Is(err, repo.ErrUndoSlotTaken) {
		t.Fatalf("want ErrUndoSlotTaken, got %v", err)
	}
	if !p.archived(t, "inflation_rates", row.ID) || p.archived(t, "inflation_rates", fresh.ID) {
		t.Error("a refused undo changed row state")
	}
}
