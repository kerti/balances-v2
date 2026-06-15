package repo_test

import (
	"context"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// The ownership biconditional CHECK (assets_check and its siblings, migration
// 00001) is the write-side guarantee beneath ATTRIBUTION's routing: a sole row
// MUST name an owner and a joint row MUST NOT. The repo passes ownership
// straight through, so the DB rejects both malformed halves — the failed create
// rolls back and persists nothing. This is the constraint that makes
// INV-ATTRIBUTION-04's "malformed sole" an impossible row in normal operation.
// covers: INV-INTEGRITY-01
func TestAssetRepo_OwnershipBiconditional(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	aliceUser := testutil.CreateHouseholdWithUser(t, q, "Alice")
	aliceCtx := auth.WithUser(context.Background(), aliceUser)
	r := repo.NewAssetRepo(tdb.Pool)

	base := func(name string) repo.CreateBankAccountParams {
		return repo.CreateBankAccountParams{
			DisplayName:    name,
			NativeCurrency: "IDR",
			BankName:       "BCA",
			AccountNumber:  "111",
			AccountType:    "savings",
		}
	}

	t.Run("sole without an owner is rejected", func(t *testing.T) {
		p := base("sole-no-owner")
		p.OwnershipType = "sole"
		p.SoleOwnerUserID = nil
		if _, err := r.CreateBankAccount(aliceCtx, p); err == nil {
			t.Fatal("want a constraint-violation error, got nil")
		}
	})

	t.Run("joint with an owner is rejected", func(t *testing.T) {
		p := base("joint-with-owner")
		p.OwnershipType = "joint"
		p.SoleOwnerUserID = &aliceUser.ID
		if _, err := r.CreateBankAccount(aliceCtx, p); err == nil {
			t.Fatal("want a constraint-violation error, got nil")
		}
	})

	t.Run("an unknown ownership_type is rejected", func(t *testing.T) {
		p := base("bogus-ownership")
		p.OwnershipType = "shared"
		p.SoleOwnerUserID = &aliceUser.ID
		if _, err := r.CreateBankAccount(aliceCtx, p); err == nil {
			t.Fatal("want a constraint-violation error, got nil")
		}
	})

	// Both well-formed halves succeed, and the rejected creates above left
	// nothing behind (the transaction rolled back).
	t.Run("valid sole and joint succeed; rejects persisted nothing", func(t *testing.T) {
		sole := base("valid-sole")
		sole.OwnershipType = "sole"
		sole.SoleOwnerUserID = &aliceUser.ID
		if _, err := r.CreateBankAccount(aliceCtx, sole); err != nil {
			t.Fatalf("valid sole CreateBankAccount: %v", err)
		}

		joint := base("valid-joint")
		joint.OwnershipType = "joint"
		joint.SoleOwnerUserID = nil
		if _, err := r.CreateBankAccount(aliceCtx, joint); err != nil {
			t.Fatalf("valid joint CreateBankAccount: %v", err)
		}

		list, err := r.ListBankAccounts(aliceCtx)
		if err != nil {
			t.Fatalf("ListBankAccounts: %v", err)
		}
		if len(list) != 2 {
			t.Errorf("got %d bank accounts, want 2 (the rejected creates must not persist)", len(list))
		}
	})
}
