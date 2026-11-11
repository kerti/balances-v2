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

// TestInvestmentTransaction_CurrencyIsThePositions: a transaction is recorded
// in its position's native currency (#830), on create and on edit, so ledger
// totals and the cost basis never mix currencies.
// covers: INV-INTEGRITY-04
func TestInvestmentTransaction_CurrencyIsThePositions(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)
	r := repo.NewInvestmentRepo(tdb.Pool)

	stock, err := r.CreateStock(ctx, repo.CreateStockParams{
		DisplayName: "BBCA", OwnershipType: "joint", NativeCurrency: "IDR",
		RiskProfile: "medium", Ticker: "BBCA", Exchange: "IDX",
	})
	if err != nil {
		t.Fatalf("CreateStock: %v", err)
	}
	buy := func(currency string) repo.CreateInvestmentTransactionParams {
		return repo.CreateInvestmentTransactionParams{
			InvestmentID:    stock.Investment.ID,
			TransactionType: "buy",
			TransactionDate: time.Date(2026, time.March, 2, 0, 0, 0, 0, time.UTC),
			Currency:        currency,
			Amount:          decPtr(decimal.NewFromInt(850_000)),
			Quantity:        decPtr(decimal.NewFromInt(100)),
			PricePerUnit:    decPtr(decimal.NewFromInt(8_500)),
		}
	}

	if _, err := r.CreateInvestmentTransaction(ctx, buy("USD")); !errors.Is(err, repo.ErrInvalidTransactionShape) {
		t.Fatalf("create in USD on an IDR stock: want ErrInvalidTransactionShape, got %v", err)
	}

	txn, err := r.CreateInvestmentTransaction(ctx, buy("IDR"))
	if err != nil {
		t.Fatalf("create in IDR: %v", err)
	}

	edit := repo.UpdateInvestmentTransactionParams{
		TransactionID:   txn.ID,
		TransactionDate: txn.TransactionDate,
		Currency:        "USD",
		Amount:          txn.Amount,
		Quantity:        txn.Quantity,
		PricePerUnit:    txn.PricePerUnit,
	}
	if _, err := r.UpdateInvestmentTransaction(ctx, edit); !errors.Is(err, repo.ErrInvalidTransactionShape) {
		t.Fatalf("edit to USD: want ErrInvalidTransactionShape, got %v", err)
	}
	edit.Currency = "IDR"
	if _, err := r.UpdateInvestmentTransaction(ctx, edit); err != nil {
		t.Fatalf("edit in IDR: %v", err)
	}
}
