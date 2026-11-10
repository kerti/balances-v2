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

// TestIncomeRepo_BulkCreate_TenancyAndAtomicity: the copy-forward save (#846)
// stamps every row with the caller's household, and a failing row rolls the
// whole batch back.
// covers: INV-TENANCY-10
func TestIncomeRepo_BulkCreate_TenancyAndAtomicity(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	aliceUser := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bobUser := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := identity.WithUser(context.Background(), aliceUser)
	bobCtx := identity.WithUser(context.Background(), bobUser)
	r := repo.NewIncomeRepo(tdb.Pool)

	row := func(cat string) repo.CreateIncomeParams {
		return repo.CreateIncomeParams{
			Date:          time.Date(2026, time.June, 30, 0, 0, 0, 0, time.UTC),
			Amount:        decimal.NewFromInt(1_000),
			Currency:      "IDR",
			Category:      cat,
			OwnershipType: "joint",
			Regularity:    "routine",
		}
	}

	t.Run("rows land in the caller's household only", func(t *testing.T) {
		got, err := r.CreateIncomeBulk(aliceCtx, []repo.CreateIncomeParams{row("salary"), row("pension")})
		if err != nil {
			t.Fatalf("CreateIncomeBulk: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("got %d rows, want 2", len(got))
		}
		for _, g := range got {
			if g.HouseholdID != aliceUser.HouseholdID {
				t.Errorf("row household %s, want alice's %s", g.HouseholdID, aliceUser.HouseholdID)
			}
			if _, err := r.GetIncome(bobCtx, g.ID); !errors.Is(err, repo.ErrNotFound) {
				t.Errorf("bob GetIncome: want ErrNotFound, got %v", err)
			}
		}
		bobList, err := r.ListIncome(bobCtx)
		if err != nil {
			t.Fatalf("bob ListIncome: %v", err)
		}
		if len(bobList) != 0 {
			t.Errorf("bob saw %d rows; want 0", len(bobList))
		}
	})

	t.Run("a failing row rolls the whole batch back", func(t *testing.T) {
		bad := row("gift")
		bad.OwnershipType = "sole" // sole without an owner violates the ownership CHECK
		before, _ := r.ListIncome(aliceCtx)
		if _, err := r.CreateIncomeBulk(aliceCtx, []repo.CreateIncomeParams{row("rental_income"), bad}); err == nil {
			t.Fatal("want error from the malformed row")
		}
		after, _ := r.ListIncome(aliceCtx)
		if len(after) != len(before) {
			t.Errorf("alice has %d rows after a failed batch, want %d", len(after), len(before))
		}
	})
}
