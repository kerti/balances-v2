package repo_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// Cross-tenant isolation + alice happy path on the FX-rate table, plus the
// duplicate-month/currency conflict.
// covers: INV-TENANCY-11
func TestFxRateRepo_Tenancy(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "AliceFx")
	bob := testutil.CreateHouseholdWithUser(t, q, "BobFx")
	aliceCtx := identity.WithUser(context.Background(), alice)
	bobCtx := identity.WithUser(context.Background(), bob)

	r := repo.NewFxRateRepo(tdb.Pool)

	rate, err := r.CreateFxRate(aliceCtx, repo.CreateFxRateParams{
		YearMonth: ymUTC(2026, time.January), Currency: "USD", Rate: decimal.NewFromInt(16000),
	})
	if err != nil {
		t.Fatalf("CreateFxRate: %v", err)
	}

	t.Run("duplicate month+currency is a conflict", func(t *testing.T) {
		_, err := r.CreateFxRate(aliceCtx, repo.CreateFxRateParams{
			YearMonth: ymUTC(2026, time.January), Currency: "USD", Rate: decimal.NewFromInt(17000),
		})
		if !errors.Is(err, repo.ErrFxRateExists) {
			t.Errorf("duplicate create: want ErrFxRateExists, got %v", err)
		}
	})

	t.Run("bob sees none / cannot mutate", func(t *testing.T) {
		list, err := r.ListFxRates(bobCtx)
		if err != nil {
			t.Fatalf("ListFxRates(bob): %v", err)
		}
		if len(list) != 0 {
			t.Errorf("bob saw %d rates; want 0", len(list))
		}
		if _, err := r.UpdateFxRate(bobCtx, rate.ID, decimal.NewFromInt(1)); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("UpdateFxRate(bob): want ErrNotFound, got %v", err)
		}
		if err := r.DeleteFxRate(bobCtx, rate.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("DeleteFxRate(bob): want ErrNotFound, got %v", err)
		}
	})

	t.Run("alice update + delete", func(t *testing.T) {
		updated, err := r.UpdateFxRate(aliceCtx, rate.ID, decimal.NewFromInt(16500))
		if err != nil {
			t.Fatalf("UpdateFxRate: %v", err)
		}
		if !updated.Rate.Equal(decimal.NewFromInt(16500)) {
			t.Errorf("rate after update: got %s, want 16500", updated.Rate)
		}
		if err := r.DeleteFxRate(aliceCtx, rate.ID); err != nil {
			t.Fatalf("DeleteFxRate: %v", err)
		}
		list, err := r.ListFxRates(aliceCtx)
		if err != nil {
			t.Fatalf("ListFxRates after delete: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("alice rates after delete: got %d, want 0", len(list))
		}
	})
}

// The New-rate currency select offers every foreign currency a live position
// is recorded in (not only those missing a rate), never the reporting
// currency, never a deleted position's, and never another household's.
// covers: INV-FX-05, INV-TENANCY-11
func TestFxRateRepo_ListUsedForeignCurrencies(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "AliceCur")
	bob := testutil.CreateHouseholdWithUser(t, q, "BobCur")
	aliceCtx := identity.WithUser(context.Background(), alice)
	bobCtx := identity.WithUser(context.Background(), bob)

	assets := repo.NewAssetRepo(tdb.Pool)
	fx := repo.NewFxRateRepo(tdb.Pool)
	month := ymUTC(2026, time.May)

	add := func(ctx context.Context, name, cur string) uuid.UUID {
		t.Helper()
		acct, err := assets.CreateBankAccount(ctx, repo.CreateBankAccountParams{
			DisplayName: name, OwnershipType: "joint", NativeCurrency: cur,
			BankName: "Bank", AccountNumber: "1", AccountType: "savings",
		})
		if err != nil {
			t.Fatalf("CreateBankAccount %s: %v", name, err)
		}
		if _, err := assets.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{
			AssetID: acct.Asset.ID, YearMonth: month, Amount: decimal.NewFromInt(100), Currency: cur,
		}); err != nil {
			t.Fatalf("CreateAssetSnapshot %s: %v", name, err)
		}
		return acct.Asset.ID
	}

	add(aliceCtx, "A idr", "IDR")
	add(aliceCtx, "A usd", "USD")
	add(aliceCtx, "A usd 2", "USD")
	add(aliceCtx, "A sgd", "SGD")
	gone := add(aliceCtx, "A jpy", "JPY")
	if err := assets.DeleteBankAccount(aliceCtx, gone); err != nil {
		t.Fatalf("DeleteBankAccount: %v", err)
	}
	add(bobCtx, "B eur", "EUR")

	// A rate already existing for USD must not hide it: other months still need one.
	if _, err := fx.CreateFxRate(aliceCtx, repo.CreateFxRateParams{
		YearMonth: month, Currency: "USD", Rate: decimal.NewFromInt(16000),
	}); err != nil {
		t.Fatalf("CreateFxRate: %v", err)
	}

	got, err := fx.ListUsedForeignCurrencies(aliceCtx)
	if err != nil {
		t.Fatalf("alice: %v", err)
	}
	if want := []string{"SGD", "USD"}; !slices.Equal(got, want) {
		t.Errorf("alice currencies: got %v, want %v", got, want)
	}

	gotBob, err := fx.ListUsedForeignCurrencies(bobCtx)
	if err != nil {
		t.Fatalf("bob: %v", err)
	}
	if want := []string{"EUR"}; !slices.Equal(gotBob, want) {
		t.Errorf("bob currencies: got %v, want %v", gotBob, want)
	}
}
