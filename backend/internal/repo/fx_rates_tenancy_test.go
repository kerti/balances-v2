package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/db"
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
	aliceCtx := auth.WithUser(context.Background(), alice)
	bobCtx := auth.WithUser(context.Background(), bob)

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
