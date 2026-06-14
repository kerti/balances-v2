package repo_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// TestImportMeta covers the *ImportMeta read paths across all four position
// groups. Each returns the position's display name + native currency (plus, for
// investments, its subtype) for an owned position, ErrNotFound for an unknown
// id, and ErrNotFound for a position in another household — the ownership gate
// that scopes the download template and the import ownership check.
// covers: INV-IMPORT-07
func TestImportMeta(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob") // separate household
	aliceCtx := auth.WithUser(context.Background(), alice)
	bobCtx := auth.WithUser(context.Background(), bob)

	t.Run("asset", func(t *testing.T) {
		r := repo.NewAssetRepo(tdb.Pool)
		acct, err := r.CreateBankAccount(aliceCtx, repo.CreateBankAccountParams{
			DisplayName: "Alice BCA", OwnershipType: "joint", NativeCurrency: "USD",
			BankName: "BCA", AccountNumber: "111", AccountType: "savings",
		})
		if err != nil {
			t.Fatalf("CreateBankAccount: %v", err)
		}
		name, currency, err := r.AssetImportMeta(aliceCtx, acct.Asset.ID)
		if err != nil {
			t.Fatalf("AssetImportMeta: %v", err)
		}
		if name != "Alice BCA" || currency != "USD" {
			t.Errorf("meta = (%q, %q), want (Alice BCA, USD)", name, currency)
		}
		if _, _, err := r.AssetImportMeta(aliceCtx, uuid.New()); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("unknown id: got %v, want ErrNotFound", err)
		}
		if _, _, err := r.AssetImportMeta(bobCtx, acct.Asset.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("cross-tenant: got %v, want ErrNotFound", err)
		}
	})

	t.Run("liability", func(t *testing.T) {
		r := repo.NewLiabilityRepo(tdb.Pool)
		l, err := r.CreateLiability(aliceCtx, repo.CreateLiabilityParams{
			DisplayName: "Alice Loan", Subtype: "personal", OwnershipType: "joint",
			NativeCurrency: "EUR", CounterpartyName: "Bank",
		})
		if err != nil {
			t.Fatalf("CreateLiability: %v", err)
		}
		name, currency, err := r.LiabilityImportMeta(aliceCtx, l.ID)
		if err != nil {
			t.Fatalf("LiabilityImportMeta: %v", err)
		}
		if name != "Alice Loan" || currency != "EUR" {
			t.Errorf("meta = (%q, %q), want (Alice Loan, EUR)", name, currency)
		}
		if _, _, err := r.LiabilityImportMeta(aliceCtx, uuid.New()); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("unknown id: got %v, want ErrNotFound", err)
		}
		if _, _, err := r.LiabilityImportMeta(bobCtx, l.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("cross-tenant: got %v, want ErrNotFound", err)
		}
	})

	t.Run("receivable", func(t *testing.T) {
		r := repo.NewReceivableRepo(tdb.Pool)
		rec, err := r.CreateReceivable(aliceCtx, repo.CreateReceivableParams{
			DisplayName: "Alice Loan Out", OwnershipType: "joint",
			NativeCurrency: "SGD", CounterpartyName: "Friend",
		})
		if err != nil {
			t.Fatalf("CreateReceivable: %v", err)
		}
		name, currency, err := r.ReceivableImportMeta(aliceCtx, rec.ID)
		if err != nil {
			t.Fatalf("ReceivableImportMeta: %v", err)
		}
		if name != "Alice Loan Out" || currency != "SGD" {
			t.Errorf("meta = (%q, %q), want (Alice Loan Out, SGD)", name, currency)
		}
		if _, _, err := r.ReceivableImportMeta(aliceCtx, uuid.New()); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("unknown id: got %v, want ErrNotFound", err)
		}
		if _, _, err := r.ReceivableImportMeta(bobCtx, rec.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("cross-tenant: got %v, want ErrNotFound", err)
		}
	})

	t.Run("investment", func(t *testing.T) {
		r := repo.NewInvestmentRepo(tdb.Pool)
		s, err := r.CreateStock(aliceCtx, repo.CreateStockParams{
			DisplayName: "Alice BBCA", OwnershipType: "joint", NativeCurrency: "IDR",
			Ticker: "BBCA", Exchange: "IDX",
			RiskProfile: "medium",
		})
		if err != nil {
			t.Fatalf("CreateStock: %v", err)
		}
		name, currency, subtype, err := r.InvestmentImportMeta(aliceCtx, s.Investment.ID)
		if err != nil {
			t.Fatalf("InvestmentImportMeta: %v", err)
		}
		if name != "Alice BBCA" || currency != "IDR" || subtype != "stock" {
			t.Errorf("meta = (%q, %q, %q), want (Alice BBCA, IDR, stock)", name, currency, subtype)
		}
		if _, _, _, err := r.InvestmentImportMeta(aliceCtx, uuid.New()); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("unknown id: got %v, want ErrNotFound", err)
		}
		if _, _, _, err := r.InvestmentImportMeta(bobCtx, s.Investment.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("cross-tenant: got %v, want ErrNotFound", err)
		}
	})
}
