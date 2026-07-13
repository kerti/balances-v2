package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// Cross-tenant isolation + alice happy path on the inflation-rate table, plus the
// duplicate-month conflict and the deflation (negative rate) case. The inflation
// repo is the structural twin of the FX-rate repo (TestFxRateRepo_Tenancy /
// INV-TENANCY-11) minus the currency dimension: household-scoped, one rate per
// month, soft-deleted.
// covers: INV-TENANCY-13
func TestInflationRateRepo_Tenancy(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "AliceInfl")
	bob := testutil.CreateHouseholdWithUser(t, q, "BobInfl")
	aliceCtx := identity.WithUser(context.Background(), alice)
	bobCtx := identity.WithUser(context.Background(), bob)

	r := repo.NewInflationRateRepo(tdb.Pool)

	rate, err := r.CreateInflationRate(aliceCtx, repo.CreateInflationRateParams{
		YearMonth: ymUTC(2026, time.January), Rate: decimal.RequireFromString("3.5"),
	})
	if err != nil {
		t.Fatalf("CreateInflationRate: %v", err)
	}

	t.Run("deflation (negative rate) is accepted", func(t *testing.T) {
		neg, err := r.CreateInflationRate(aliceCtx, repo.CreateInflationRateParams{
			YearMonth: ymUTC(2026, time.February), Rate: decimal.RequireFromString("-1.2"),
		})
		if err != nil {
			t.Fatalf("CreateInflationRate(negative): %v", err)
		}
		if !neg.Rate.Equal(decimal.RequireFromString("-1.2")) {
			t.Errorf("rate: got %s, want -1.2", neg.Rate)
		}
	})

	t.Run("duplicate month is a conflict", func(t *testing.T) {
		_, err := r.CreateInflationRate(aliceCtx, repo.CreateInflationRateParams{
			YearMonth: ymUTC(2026, time.January), Rate: decimal.RequireFromString("9"),
		})
		if !errors.Is(err, repo.ErrInflationRateExists) {
			t.Errorf("duplicate create: want ErrInflationRateExists, got %v", err)
		}
	})

	t.Run("bob sees none / cannot mutate alice's rate", func(t *testing.T) {
		list, err := r.ListInflationRates(bobCtx)
		if err != nil {
			t.Fatalf("ListInflationRates(bob): %v", err)
		}
		if len(list) != 0 {
			t.Errorf("bob saw %d rates; want 0", len(list))
		}
		if _, err := r.UpdateInflationRate(bobCtx, rate.ID, decimal.NewFromInt(1)); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("UpdateInflationRate(bob): want ErrNotFound, got %v", err)
		}
		if err := r.DeleteInflationRate(bobCtx, rate.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("DeleteInflationRate(bob): want ErrNotFound, got %v", err)
		}
	})

	t.Run("alice update + delete", func(t *testing.T) {
		updated, err := r.UpdateInflationRate(aliceCtx, rate.ID, decimal.RequireFromString("4.25"))
		if err != nil {
			t.Fatalf("UpdateInflationRate: %v", err)
		}
		if !updated.Rate.Equal(decimal.RequireFromString("4.25")) {
			t.Errorf("rate after update: got %s, want 4.25", updated.Rate)
		}
		if err := r.DeleteInflationRate(aliceCtx, rate.ID); err != nil {
			t.Fatalf("DeleteInflationRate: %v", err)
		}
		// The Feb deflation row remains; only the Jan row is gone.
		list, err := r.ListInflationRates(aliceCtx)
		if err != nil {
			t.Fatalf("ListInflationRates after delete: %v", err)
		}
		for _, ir := range list {
			if ir.ID == rate.ID {
				t.Errorf("deleted Jan rate still listed: %s", ir.ID)
			}
		}
	})

	t.Run("update unknown id is ErrNotFound", func(t *testing.T) {
		if _, err := r.UpdateInflationRate(aliceCtx, uuid.New(), decimal.NewFromInt(1)); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("UpdateInflationRate(unknown): want ErrNotFound, got %v", err)
		}
	})

	t.Run("delete unknown id is ErrNotFound", func(t *testing.T) {
		if err := r.DeleteInflationRate(aliceCtx, uuid.New()); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("DeleteInflationRate(unknown): want ErrNotFound, got %v", err)
		}
	})
}
