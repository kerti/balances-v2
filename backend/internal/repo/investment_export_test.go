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
//
// covers: INV-EXPORT-01, INV-EXPORT-02, INV-EXPORT-03, INV-EXPORT-04
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

// TestExportInvestmentSubtypes_RepoLevel covers the four remaining export
// wrappers (mutual fund, bond, gold, time deposit). Each is a thin Get +
// assemble over the shared exportCommon (proven by the Stock test above), so
// these assert the subtype Detail is carried through on the happy path and that
// an unknown id propagates ErrNotFound from the wrapper's Get.
//
// covers: INV-EXPORT-01, INV-EXPORT-02
func TestExportInvestmentSubtypes_RepoLevel(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	aliceCtx := auth.WithUser(context.Background(), alice)
	inv := repo.NewInvestmentRepo(tdb.Pool)

	t.Run("mutual fund", func(t *testing.T) {
		mf, err := inv.CreateMutualFund(aliceCtx, repo.CreateMutualFundParams{
			DisplayName:    "Index fund",
			OwnershipType:  "joint",
			NativeCurrency: "IDR",
			RiskProfile:    "medium",
			FundCode:       "IDX-IDXR",
			FundType:       "equity",
		})
		if err != nil {
			t.Fatalf("CreateMutualFund: %v", err)
		}
		out, err := inv.ExportMutualFund(aliceCtx, mf.Investment.ID)
		if err != nil {
			t.Fatalf("ExportMutualFund: %v", err)
		}
		if out.MutualFund.Details.FundCode != "IDX-IDXR" {
			t.Errorf("FundCode = %q, want IDX-IDXR", out.MutualFund.Details.FundCode)
		}
		if out.OwnerEmail != "" {
			t.Errorf("joint OwnerEmail = %q, want empty", out.OwnerEmail)
		}
		if _, err := inv.ExportMutualFund(aliceCtx, uuid.New()); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("unknown id: got %v, want ErrNotFound", err)
		}
	})

	t.Run("bond", func(t *testing.T) {
		bond, err := inv.CreateBond(aliceCtx, repo.CreateBondParams{
			DisplayName:     "Secondary bond",
			OwnershipType:   "joint",
			NativeCurrency:  "IDR",
			RiskProfile:     "low",
			BondType:        "secondary_market",
			Issuer:          "Govt",
			CouponRate:      decimal.RequireFromString("6.5"),
			CouponFrequency: "semi_annual",
			MaturityDate:    time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
		})
		if err != nil {
			t.Fatalf("CreateBond: %v", err)
		}
		out, err := inv.ExportBond(aliceCtx, bond.Investment.ID)
		if err != nil {
			t.Fatalf("ExportBond: %v", err)
		}
		if out.Bond.Details.Issuer != "Govt" {
			t.Errorf("Issuer = %q, want Govt", out.Bond.Details.Issuer)
		}
		if _, err := inv.ExportBond(aliceCtx, uuid.New()); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("unknown id: got %v, want ErrNotFound", err)
		}
	})

	t.Run("gold", func(t *testing.T) {
		gold, err := inv.CreateGold(aliceCtx, repo.CreateGoldParams{
			DisplayName:    "Gold bars",
			OwnershipType:  "joint",
			NativeCurrency: "IDR",
			RiskProfile:    "low",
			Form:           "bar",
			Purity:         decimal.RequireFromString("0.9999"),
		})
		if err != nil {
			t.Fatalf("CreateGold: %v", err)
		}
		out, err := inv.ExportGold(aliceCtx, gold.Investment.ID)
		if err != nil {
			t.Fatalf("ExportGold: %v", err)
		}
		if out.Gold.Details.Form != "bar" {
			t.Errorf("Form = %q, want bar", out.Gold.Details.Form)
		}
		if _, err := inv.ExportGold(aliceCtx, uuid.New()); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("unknown id: got %v, want ErrNotFound", err)
		}
	})

	t.Run("time deposit", func(t *testing.T) {
		td, err := inv.CreateTimeDeposit(aliceCtx, repo.CreateTimeDepositParams{
			DisplayName:    "TD 12mo",
			OwnershipType:  "joint",
			NativeCurrency: "IDR",
			RiskProfile:    "low",
			BankName:       "TestBank",
			Principal:      decimal.RequireFromString("100000000"),
			InterestRate:   decimal.RequireFromString("5.5"),
			TermMonths:     12,
			PlacementDate:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			MaturityDate:   time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			RolloverPolicy: "no_rollover",
		})
		if err != nil {
			t.Fatalf("CreateTimeDeposit: %v", err)
		}
		out, err := inv.ExportTimeDeposit(aliceCtx, td.Investment.ID)
		if err != nil {
			t.Fatalf("ExportTimeDeposit: %v", err)
		}
		if out.TimeDeposit.Details.BankName != "TestBank" {
			t.Errorf("BankName = %q, want TestBank", out.TimeDeposit.Details.BankName)
		}
		if _, err := inv.ExportTimeDeposit(aliceCtx, uuid.New()); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("unknown id: got %v, want ErrNotFound", err)
		}
	})
}
