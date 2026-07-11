package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// TestMonthlyReportRepo_MixedPortfolio exercises loadEngineInput with one
// position of every group plus income and an investment transaction — the other
// report tests seed assets only, so the liability/receivable/investment loaders,
// every snapshot loader, and the income + transaction loaders went unrun. Beyond
// coverage it asserts each group lands in its own net-worth bucket with the
// right sign (nwTotal = assets + receivables + investments − liabilities), which
// a mis-wired loader would break.
func TestMonthlyReportRepo_MixedPortfolio(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	aliceCtx := identity.WithUser(context.Background(), alice)

	jan := ymUTC(2026, time.January)
	decPtr := func(s string) *decimal.Decimal { d := decimal.RequireFromString(s); return &d }

	// Asset: bank account, 100 in Jan.
	acct := createAsset(t, q, alice.HouseholdID, &alice.ID, nil, "joint")
	_ = createAssetSnapshot(t, q, alice.HouseholdID, acct, jan, "100")

	// Liability: 40 in Jan (stored positive, subtracted from nwTotal).
	lr := repo.NewLiabilityRepo(tdb.Pool)
	liab, err := lr.CreateLiability(aliceCtx, repo.CreateLiabilityParams{
		DisplayName: "Car Loan", Subtype: "personal", OwnershipType: "joint",
		NativeCurrency: "IDR", CounterpartyName: "Bank",
	})
	if err != nil {
		t.Fatalf("CreateLiability: %v", err)
	}
	if _, err := lr.ImportLiabilitySnapshots(aliceCtx, liab.ID,
		[]repo.ImportSnapshotRow{{YearMonth: jan, Amount: decimal.NewFromInt(40), Currency: "IDR"}}, false); err != nil {
		t.Fatalf("ImportLiabilitySnapshots: %v", err)
	}

	// Receivable: 20 in Jan.
	rr := repo.NewReceivableRepo(tdb.Pool)
	rec, err := rr.CreateReceivable(aliceCtx, repo.CreateReceivableParams{
		DisplayName: "Loan to Carol", OwnershipType: "joint", NativeCurrency: "IDR", CounterpartyName: "Carol",
	})
	if err != nil {
		t.Fatalf("CreateReceivable: %v", err)
	}
	if _, err := rr.ImportReceivableSnapshots(aliceCtx, rec.ID,
		[]repo.ImportSnapshotRow{{YearMonth: jan, Amount: decimal.NewFromInt(20), Currency: "IDR"}}, false); err != nil {
		t.Fatalf("ImportReceivableSnapshots: %v", err)
	}

	// Investment: a stock valued 50 in Jan, plus a Buy transaction in Jan.
	ir := repo.NewInvestmentRepo(tdb.Pool)
	stock, err := ir.CreateStock(aliceCtx, repo.CreateStockParams{
		DisplayName: "BBCA", OwnershipType: "joint", NativeCurrency: "IDR", Ticker: "BBCA", Exchange: "IDX",
		RiskProfile: "medium",
	})
	if err != nil {
		t.Fatalf("CreateStock: %v", err)
	}
	if _, err := ir.ImportInvestmentSnapshots(aliceCtx, stock.Investment.ID,
		[]repo.ImportInvestmentSnapshotRow{{
			YearMonth: jan, Amount: decimal.NewFromInt(50), Currency: "IDR",
			Quantity: decPtr("5"), PricePerUnit: decPtr("10"),
		}}, false); err != nil {
		t.Fatalf("ImportInvestmentSnapshots: %v", err)
	}
	buyAmt, buyQty, buyPrice := decimal.NewFromInt(50), decimal.NewFromInt(5), decimal.NewFromInt(10)
	if _, err := ir.CreateInvestmentTransaction(aliceCtx, repo.CreateInvestmentTransactionParams{
		InvestmentID: stock.Investment.ID, TransactionType: repo.TxnTypeBuy,
		TransactionDate: time.Date(2026, time.January, 4, 0, 0, 0, 0, time.UTC), Currency: "IDR",
		Amount: &buyAmt, Quantity: &buyQty, PricePerUnit: &buyPrice,
	}); err != nil {
		t.Fatalf("CreateInvestmentTransaction: %v", err)
	}

	// Income: 30 salary in Jan.
	incr := repo.NewIncomeRepo(tdb.Pool)
	if _, err := incr.CreateIncome(aliceCtx, repo.CreateIncomeParams{
		Date: time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC), Amount: decimal.NewFromInt(30),
		Currency: "IDR", Category: "salary", OwnershipType: "sole", SoleOwnerUserID: &alice.ID,
		Regularity: "routine",
	}); err != nil {
		t.Fatalf("CreateIncome: %v", err)
	}

	// FX rate: a USD rate for Jan. Loaded into the engine input regardless of the
	// IDR reporting currency, so it exercises the fx-rate loader without changing
	// the all-IDR net worth.
	if _, err := repo.NewFxRateRepo(tdb.Pool).CreateFxRate(aliceCtx, repo.CreateFxRateParams{
		YearMonth: jan, Currency: "USD", Rate: decimal.NewFromInt(16000),
	}); err != nil {
		t.Fatalf("CreateFxRate: %v", err)
	}

	rows, err := repo.NewMonthlyReportRepo(tdb.Pool).ListReports(aliceCtx)
	if err != nil {
		t.Fatalf("ListReports: %v", err)
	}
	j := mustMonth(t, rows, jan)

	for _, c := range []struct {
		name string
		got  decimal.Decimal
		want int64
	}{
		{"nwAssets", j.NwAssets, 100},
		{"nwLiabilities", j.NwLiabilities, 40},
		{"nwReceivables", j.NwReceivables, 20},
		{"nwInvestments", j.NwInvestments, 50},
		{"nwTotal", j.NwTotal, 130}, // 100 + 20 + 50 − 40
	} {
		if !c.got.Equal(decimal.NewFromInt(c.want)) {
			t.Errorf("%s = %s, want %d", c.name, c.got, c.want)
		}
	}

	if j.EarnedIncomeTotal == nil {
		t.Fatal("EarnedIncomeTotal is nil; want 30")
	}
	if !j.EarnedIncomeTotal.Equal(decimal.NewFromInt(30)) {
		t.Errorf("EarnedIncomeTotal = %s, want 30", j.EarnedIncomeTotal)
	}
}

// TestMonthlyReportRepo_RebuildAllNoData covers the no-input early return:
// RebuildAll on a household with no snapshots generates nothing and is a no-op,
// leaving the report list empty rather than erroring.
func TestMonthlyReportRepo_RebuildAllNoData(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	aliceCtx := identity.WithUser(context.Background(), alice)

	r := repo.NewMonthlyReportRepo(tdb.Pool)
	if err := r.RebuildAll(aliceCtx); err != nil {
		t.Fatalf("RebuildAll with no data: %v", err)
	}
	rows, err := r.ListReports(aliceCtx)
	if err != nil {
		t.Fatalf("ListReports: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("got %d reports for an empty household; want 0", len(rows))
	}
}

// TestMonthlyReportRepo_Unauthenticated covers the currentUser guard on every
// public method: with no user in the context each returns ErrUnauthenticated
// before touching the database.
func TestMonthlyReportRepo_Unauthenticated(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	r := repo.NewMonthlyReportRepo(tdb.Pool)
	ctx := context.Background() // no identity.WithUser

	month := ymUTC(2026, time.January)
	cases := []struct {
		name string
		call func() error
	}{
		{"ListReports", func() error { _, err := r.ListReports(ctx); return err }},
		{"ReportingCurrency", func() error { _, err := r.ReportingCurrency(ctx); return err }},
		{"GetReport", func() error { _, err := r.GetReport(ctx, month); return err }},
		{"RebuildAll", func() error { return r.RebuildAll(ctx) }},
		{"RebuildMonth", func() error { return r.RebuildMonth(ctx, month) }},
	}
	for _, c := range cases {
		if err := c.call(); !errors.Is(err, repo.ErrUnauthenticated) {
			t.Errorf("%s: got %v, want ErrUnauthenticated", c.name, err)
		}
	}
}
