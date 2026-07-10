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

// TestInvestmentRepo_TimeSeries drives the #22 monthly value+cost endpoint
// end-to-end against a real DB: the ledger-replay cost branch (stock) and the
// flat-principal branch (time_deposit), the value series sourced from snapshots,
// and the empty-household short-circuit. The pure per-month math is covered
// DB-free in cost_basis_test.go; this asserts the loaders, the subtype branch,
// and the snapshot→value plumbing the unit tests can't reach.
func TestInvestmentRepo_TimeSeries(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	aliceCtx := identity.WithUser(context.Background(), alice)
	r := repo.NewInvestmentRepo(tdb.Pool)

	// ---- Stock: one Buy, then two snapshots; cost replays the ledger. ----
	stock, err := r.CreateStock(aliceCtx, repo.CreateStockParams{
		DisplayName:    "Alice BBCA",
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		RiskProfile:    "medium",
		Ticker:         "BBCA",
		Exchange:       "IDX",
	})
	if err != nil {
		t.Fatalf("CreateStock: %v", err)
	}

	buyCash := decimal.NewFromInt(950_000)
	buyQty := decimal.NewFromInt(100)
	buyPrice := decimal.NewFromInt(9_500)
	if _, err := r.CreateInvestmentTransaction(aliceCtx, repo.CreateInvestmentTransactionParams{
		InvestmentID:    stock.Investment.ID,
		TransactionType: repo.TxnTypeBuy,
		TransactionDate: time.Date(2026, time.May, 4, 0, 0, 0, 0, time.UTC),
		Currency:        "IDR",
		Amount:          &buyCash,
		Quantity:        &buyQty,
		PricePerUnit:    &buyPrice,
	}); err != nil {
		t.Fatalf("CreateInvestmentTransaction buy: %v", err)
	}

	mayQty, mayPrice := decimal.NewFromInt(100), decimal.NewFromInt(9_500)
	junQty, junPrice := decimal.NewFromInt(100), decimal.NewFromInt(10_000)
	if _, err := r.CreateInvestmentSnapshot(aliceCtx, repo.CreateInvestmentSnapshotParams{
		InvestmentID: stock.Investment.ID,
		YearMonth:    ymUTC(2026, time.May),
		Amount:       decimal.NewFromInt(950_000),
		Currency:     "IDR",
		Quantity:     &mayQty,
		PricePerUnit: &mayPrice,
	}); err != nil {
		t.Fatalf("CreateInvestmentSnapshot may: %v", err)
	}
	if _, err := r.CreateInvestmentSnapshot(aliceCtx, repo.CreateInvestmentSnapshotParams{
		InvestmentID: stock.Investment.ID,
		YearMonth:    ymUTC(2026, time.June),
		Amount:       decimal.NewFromInt(1_000_000),
		Currency:     "IDR",
		Quantity:     &junQty,
		PricePerUnit: &junPrice,
	}); err != nil {
		t.Fatalf("CreateInvestmentSnapshot jun: %v", err)
	}

	// ---- Time deposit: cost is flat principal, not the ledger. ----
	interestRate := decimal.RequireFromString("5.5")
	td, err := r.CreateTimeDeposit(aliceCtx, repo.CreateTimeDepositParams{
		DisplayName:    "Alice BCA TD",
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		RiskProfile:    "low",
		BankName:       "BCA",
		Principal:      decimal.NewFromInt(50_000_000),
		InterestRate:   interestRate,
		TermMonths:     12,
		PlacementDate:  ymUTC(2026, time.January),
		MaturityDate:   ymUTC(2027, time.January),
		RolloverPolicy: "auto_renew_principal",
	})
	if err != nil {
		t.Fatalf("CreateTimeDeposit: %v", err)
	}
	tdAccrued := decimal.NewFromInt(229_166)
	if _, err := r.CreateInvestmentSnapshot(aliceCtx, repo.CreateInvestmentSnapshotParams{
		InvestmentID:    td.Investment.ID,
		YearMonth:       ymUTC(2026, time.February),
		Amount:          decimal.NewFromInt(50_229_166),
		Currency:        "IDR",
		AccruedInterest: &tdAccrued,
	}); err != nil {
		t.Fatalf("CreateInvestmentSnapshot td: %v", err)
	}

	series, err := r.InvestmentTimeSeries(aliceCtx)
	if err != nil {
		t.Fatalf("InvestmentTimeSeries: %v", err)
	}
	byID := make(map[uuid.UUID]repo.InvestmentTimeSeries, len(series))
	for _, s := range series {
		byID[s.InvestmentID] = s
	}

	t.Run("stock value from snapshots, cost replays the ledger", func(t *testing.T) {
		s, ok := byID[stock.Investment.ID]
		if !ok {
			t.Fatalf("stock missing from time series")
		}
		if len(s.ValueSeries) != 2 || len(s.CostSeries) != 2 {
			t.Fatalf("stock series len: value=%d cost=%d, want 2/2", len(s.ValueSeries), len(s.CostSeries))
		}
		// Value = each month's snapshot amount.
		assertSeriesAmount(t, "stock value[0]", s.ValueSeries[0].Amount, "950000")
		assertSeriesAmount(t, "stock value[1]", s.ValueSeries[1].Amount, "1000000")
		// Cost = the May buy (950000), carried forward through June (no further txn).
		assertSeriesAmount(t, "stock cost[0]", s.CostSeries[0].Cost, "950000")
		assertSeriesAmount(t, "stock cost[1]", s.CostSeries[1].Cost, "950000")
	})

	t.Run("time deposit cost is flat principal", func(t *testing.T) {
		s, ok := byID[td.Investment.ID]
		if !ok {
			t.Fatalf("time deposit missing from time series")
		}
		if len(s.CostSeries) != 1 {
			t.Fatalf("td cost series len = %d, want 1", len(s.CostSeries))
		}
		assertSeriesAmount(t, "td value", s.ValueSeries[0].Amount, "50229166")
		assertSeriesAmount(t, "td cost", s.CostSeries[0].Cost, "50000000")
	})

	t.Run("household with no investments yields an empty slice", func(t *testing.T) {
		bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
		bobCtx := identity.WithUser(context.Background(), bob)
		empty, err := r.InvestmentTimeSeries(bobCtx)
		if err != nil {
			t.Fatalf("InvestmentTimeSeries (empty): %v", err)
		}
		if len(empty) != 0 {
			t.Fatalf("empty household returned %d series, want 0", len(empty))
		}
	})
}

func assertSeriesAmount(t *testing.T, label string, got decimal.Decimal, want string) {
	t.Helper()
	if !got.Equal(decimal.RequireFromString(want)) {
		t.Fatalf("%s = %s, want %s", label, got.String(), want)
	}
}
