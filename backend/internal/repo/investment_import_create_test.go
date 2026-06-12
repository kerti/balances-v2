package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// Repo-level cover for the create-from-list investment seed (issue #90). The
// handler suite drives the happy paths end-to-end; these reach the two branches
// the HTTP layer can't: the snapshot-shape backstop (the importer parses by
// endpoint subtype, so a mismatched shape never arrives over HTTP) and the
// returned subtype aggregate.

func investmentRepoFor(t *testing.T) (*repo.InvestmentRepo, context.Context) {
	t.Helper()
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	return repo.NewInvestmentRepo(tdb.Pool), auth.WithUser(context.Background(), alice)
}

func TestCreateStockWithSnapshotsAndLedger_Aggregate(t *testing.T) {
	r, ctx := investmentRepoFor(t)
	qty := decimal.RequireFromString("100")
	price := decimal.RequireFromString("9500")
	stock, err := r.CreateStockWithSnapshotsAndLedger(ctx, repo.CreateStockParams{
		DisplayName:    "Seeded stock",
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		RiskProfile:    "medium",
		Ticker:         "BBCA",
		Exchange:       "IDX",
	}, nil, []repo.ImportInvestmentSnapshotRow{
		{YearMonth: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Amount: qty.Mul(price), Currency: "IDR", Quantity: &qty, PricePerUnit: &price},
	}, []repo.ImportTransactionRow{
		{TransactionType: "buy", TransactionDate: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), Currency: "IDR", Amount: ptrDec("950000"), Quantity: &qty, PricePerUnit: &price},
	})
	if err != nil {
		t.Fatalf("CreateStockWithSnapshotsAndLedger: %v", err)
	}
	if stock.Details.Ticker != "BBCA" || stock.Investment.Subtype != "stock" {
		t.Errorf("aggregate not populated: %+v", stock)
	}
}

// A snapshot row carrying the wrong value-shape for the subtype (accrued_interest
// on a quantity-price stock) is rejected by the seed's shape backstop, rolling
// the whole create back.
func TestCreateStockWithSnapshotsAndLedger_RejectsMismatchedSnapshotShape(t *testing.T) {
	r, ctx := investmentRepoFor(t)
	accrued := decimal.RequireFromString("1000")
	_, err := r.CreateStockWithSnapshotsAndLedger(ctx, repo.CreateStockParams{
		DisplayName:    "Bad-shape stock",
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		RiskProfile:    "medium",
		Ticker:         "BBCA",
		Exchange:       "IDX",
	}, nil, []repo.ImportInvestmentSnapshotRow{
		// accrued_interest belongs to bond/time_deposit, not stock.
		{YearMonth: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("100"), Currency: "IDR", AccruedInterest: &accrued},
	}, nil)
	if err == nil {
		t.Fatal("want a shape-validation error, got nil")
	}
	list, err := r.ListStocks(ctx)
	if err != nil {
		t.Fatalf("ListStocks: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("rejected create left %d stocks behind (not rolled back)", len(list))
	}
}

func ptrDec(s string) *decimal.Decimal {
	d := decimal.RequireFromString(s)
	return &d
}
