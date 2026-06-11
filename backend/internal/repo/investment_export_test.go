package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// TestExportStock_RepoLevel exercises the shared exportCommon resolution that
// the handler tests can't reach: a sole-owned, tagged investment resolves its
// owner email + tag name, and both the snapshot history and transaction ledger
// come back. The ownership gate returns ErrNotFound for unknown / cross-tenant
// ids. (The handler tests cover the joint / blank-owner path and the workbook
// shape per snapshot type.)
func TestExportStock_RepoLevel(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := auth.WithUser(context.Background(), alice)
	bobCtx := auth.WithUser(context.Background(), bob)

	inv := repo.NewInvestmentRepo(tdb.Pool)
	tags := repo.NewTagRepo(tdb.Pool)

	t.Run("sole owner + tag + snapshot + transaction resolve", func(t *testing.T) {
		stock, err := inv.CreateStock(aliceCtx, repo.CreateStockParams{
			DisplayName:     "Sole stock",
			OwnershipType:   "sole",
			SoleOwnerUserID: &alice.ID,
			NativeCurrency:  "IDR",
			RiskProfile:     "medium",
			Ticker:          "BBCA",
			Exchange:        "IDX",
		})
		if err != nil {
			t.Fatalf("CreateStock: %v", err)
		}

		tag, err := tags.CreateTag(aliceCtx, "Core holding", "#22c55e")
		if err != nil {
			t.Fatalf("CreateTag: %v", err)
		}
		if err := tags.AssignTag(aliceCtx, repo.TagGroupInvestment, stock.Investment.ID, &tag.ID); err != nil {
			t.Fatalf("AssignTag: %v", err)
		}

		qty := decimal.RequireFromString("100")
		price := decimal.RequireFromString("10500")
		if _, err := inv.CreateInvestmentSnapshot(aliceCtx, repo.CreateInvestmentSnapshotParams{
			InvestmentID: stock.Investment.ID,
			YearMonth:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Amount:       qty.Mul(price),
			Currency:     "IDR",
			Quantity:     &qty,
			PricePerUnit: &price,
		}); err != nil {
			t.Fatalf("CreateInvestmentSnapshot: %v", err)
		}

		amt := decimal.RequireFromString("1000000")
		if _, err := inv.CreateInvestmentTransaction(aliceCtx, repo.CreateInvestmentTransactionParams{
			InvestmentID:    stock.Investment.ID,
			TransactionType: repo.TxnTypeBuy,
			TransactionDate: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
			Currency:        "IDR",
			Amount:          &amt,
			Quantity:        &qty,
			PricePerUnit:    &price,
		}); err != nil {
			t.Fatalf("CreateInvestmentTransaction: %v", err)
		}

		out, err := inv.ExportStock(aliceCtx, stock.Investment.ID)
		if err != nil {
			t.Fatalf("ExportStock: %v", err)
		}
		if out.OwnerEmail != alice.Email {
			t.Errorf("OwnerEmail = %q, want %q", out.OwnerEmail, alice.Email)
		}
		if out.TagName != "Core holding" {
			t.Errorf("TagName = %q, want Core holding", out.TagName)
		}
		if len(out.Snapshots) != 1 {
			t.Errorf("want 1 snapshot, got %d", len(out.Snapshots))
		}
		if len(out.Transactions) != 1 || out.Transactions[0].TransactionType != repo.TxnTypeBuy {
			t.Errorf("want 1 buy transaction, got %+v", out.Transactions)
		}
		if out.Stock.Details.Ticker != "BBCA" {
			t.Errorf("Details.Ticker = %q, want BBCA", out.Stock.Details.Ticker)
		}
	})

	t.Run("unknown id is ErrNotFound", func(t *testing.T) {
		if _, err := inv.ExportStock(aliceCtx, uuid.New()); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("unknown id: got %v, want ErrNotFound", err)
		}
	})

	t.Run("cross-tenant is ErrNotFound", func(t *testing.T) {
		stock, err := inv.CreateStock(aliceCtx, repo.CreateStockParams{
			DisplayName:    "Alice only",
			OwnershipType:  "joint",
			NativeCurrency: "IDR",
			RiskProfile:    "low",
			Ticker:         "TLKM",
			Exchange:       "IDX",
		})
		if err != nil {
			t.Fatalf("CreateStock: %v", err)
		}
		if _, err := inv.ExportStock(bobCtx, stock.Investment.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("cross-tenant: got %v, want ErrNotFound", err)
		}
	})
}
