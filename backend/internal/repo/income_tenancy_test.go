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

// TestIncomeRepo_TenancyIsolation exercises cross-tenant rejection and the
// alice-side happy path on the Income flow-event entity. Income has no
// snapshots/transactions (flat per ADR-0008), so the surface is just CRUD.
//
// Populated-list assertion is included so the List query's read path is
// exercised — bare cross-tenant tests would only hit the empty short-circuit
// (M4.4 lesson, see HANDOFF "Tenancy test pattern").
// covers: INV-TENANCY-10
func TestIncomeRepo_TenancyIsolation(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	aliceUser := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bobUser := testutil.CreateHouseholdWithUser(t, q, "Bob")
	if aliceUser.HouseholdID == bobUser.HouseholdID {
		t.Fatalf("fixture: alice and bob ended up in the same household")
	}

	aliceCtx := auth.WithUser(context.Background(), aliceUser)
	bobCtx := auth.WithUser(context.Background(), bobUser)

	r := repo.NewIncomeRepo(tdb.Pool)

	aliceIncome, err := r.CreateIncome(aliceCtx, repo.CreateIncomeParams{
		Date:            time.Date(2026, time.May, 15, 0, 0, 0, 0, time.UTC),
		Amount:          decimal.NewFromInt(15_000_000),
		Currency:        "IDR",
		Category:        "salary",
		OwnershipType:   "sole",
		SoleOwnerUserID: &aliceUser.ID,
		Regularity:      "routine",
	})
	if err != nil {
		t.Fatalf("alice CreateIncome: %v", err)
	}

	// ----- Bob can't observe Alice's income ----------------------------

	t.Run("bob list excludes alice's income", func(t *testing.T) {
		list, err := r.ListIncome(bobCtx)
		if err != nil {
			t.Fatalf("ListIncome: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("bob saw %d income rows; want 0", len(list))
		}
	})

	t.Run("bob get returns ErrNotFound", func(t *testing.T) {
		_, err := r.GetIncome(bobCtx, aliceIncome.ID)
		if !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("GetIncome: want ErrNotFound, got %v", err)
		}
	})

	// ----- Bob can't mutate Alice's income -----------------------------

	t.Run("bob update returns ErrNotFound", func(t *testing.T) {
		_, err := r.UpdateIncome(bobCtx, aliceIncome.ID, repo.UpdateIncomeParams{
			Date:          aliceIncome.Date,
			Amount:        decimal.NewFromInt(1),
			Currency:      "IDR",
			Category:      "gift",
			OwnershipType: "joint",
			Regularity:    "routine",
		})
		if !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("UpdateIncome: want ErrNotFound, got %v", err)
		}
	})

	t.Run("bob delete returns ErrNotFound", func(t *testing.T) {
		err := r.DeleteIncome(bobCtx, aliceIncome.ID)
		if !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("DeleteIncome: want ErrNotFound, got %v", err)
		}
	})

	// ----- Sanity: Alice still sees her income (populated list) --------

	t.Run("alice still sees her income", func(t *testing.T) {
		list, err := r.ListIncome(aliceCtx)
		if err != nil {
			t.Fatalf("ListIncome: %v", err)
		}
		if len(list) != 1 {
			t.Fatalf("alice saw %d income rows; want 1", len(list))
		}
		if list[0].ID != aliceIncome.ID {
			t.Errorf("alice's income id mismatch: got %s, want %s", list[0].ID, aliceIncome.ID)
		}
		if list[0].Category != "salary" {
			t.Errorf("alice's category: got %q, want salary", list[0].Category)
		}
	})

	// ----- Alice happy-path CRUD --------------------------------------

	t.Run("alice update persists new amount, category, and regularity", func(t *testing.T) {
		updated, err := r.UpdateIncome(aliceCtx, aliceIncome.ID, repo.UpdateIncomeParams{
			Date:            aliceIncome.Date,
			Amount:          decimal.NewFromInt(16_000_000),
			Currency:        "IDR",
			Category:        "business_income",
			OwnershipType:   "sole",
			SoleOwnerUserID: &aliceUser.ID,
			Regularity:      "incidental",
		})
		if err != nil {
			t.Fatalf("UpdateIncome: %v", err)
		}
		if !updated.Amount.Equal(decimal.NewFromInt(16_000_000)) {
			t.Errorf("amount: got %s, want 16000000", updated.Amount)
		}
		if updated.Category != "business_income" {
			t.Errorf("category: got %q, want business_income", updated.Category)
		}
		if updated.Regularity != "incidental" {
			t.Errorf("regularity: got %q, want incidental", updated.Regularity)
		}
	})

	t.Run("alice delete removes from get and list", func(t *testing.T) {
		if err := r.DeleteIncome(aliceCtx, aliceIncome.ID); err != nil {
			t.Fatalf("DeleteIncome: %v", err)
		}
		if _, err := r.GetIncome(aliceCtx, aliceIncome.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("GetIncome after delete: want ErrNotFound, got %v", err)
		}
		list, err := r.ListIncome(aliceCtx)
		if err != nil {
			t.Fatalf("ListIncome after delete: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("ListIncome after delete: got %d, want 0", len(list))
		}
	})
}
