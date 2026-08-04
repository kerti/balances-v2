package repo_test

import (
	"context"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// A Position's entry type is DECLARED — the report engine cannot infer it, so
// the declaration is the only thing standing between a household and a month
// that reads as huge phantom spending (ADR-0053). That makes two properties of
// the write path load-bearing, and neither is visible from the engine's own
// unit tests.
//
// covers: INV-FINANCE-36
func TestEntryType_DeclarationSurvivesTheWritePath(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), alice)
	r := repo.NewAssetRepo(tdb.Pool)

	// 1. A create path that never learned the field still means what it always
	//    meant. The demo seeder, fixtures and any pre-ADR-0053 API client leave
	//    it empty; normalising to `acquired` is what keeps their months
	//    identical (and is why INV-FINANCE-28 holds unchanged).
	implicit, err := r.CreateBankAccount(ctx, repo.CreateBankAccountParams{
		DisplayName: "Unstated", OwnershipType: "joint", NativeCurrency: "IDR",
		BankName: "Bank", AccountNumber: "111", AccountType: "savings",
	})
	if err != nil {
		t.Fatalf("CreateBankAccount (unstated): %v", err)
	}
	if got := implicit.Asset.EntryType; got != repo.EntryTypeAcquired {
		t.Errorf("unstated entry type persisted as %q, want %q", got, repo.EntryTypeAcquired)
	}

	// 2. A declaration the household actually made is persisted verbatim.
	declared, err := r.CreateBankAccount(ctx, repo.CreateBankAccountParams{
		DisplayName: "Long held", OwnershipType: "joint", NativeCurrency: "IDR",
		BankName: "Bank", AccountNumber: "222", AccountType: "savings",
		EntryType: repo.EntryTypeNewlyTracked,
	})
	if err != nil {
		t.Fatalf("CreateBankAccount (declared): %v", err)
	}
	id := declared.Asset.ID
	if got := declared.Asset.EntryType; got != repo.EntryTypeNewlyTracked {
		t.Fatalf("declared entry type persisted as %q, want %q", got, repo.EntryTypeNewlyTracked)
	}

	// 3. The one that matters: an update that says nothing about the entry type
	//    must LEAVE IT ALONE. A plain assignment would quietly reset it to
	//    `acquired` — the same silent-reset residual ADR-0053 warns restore and
	//    import against, but reachable here from any client that edits a
	//    position without sending the field.
	edited, err := r.UpdateBankAccount(ctx, id, repo.UpdateBankAccountParams{
		DisplayName: "Long held (renamed)", OwnershipType: "joint",
		BankName: "Bank", AccountNumber: "222", AccountType: "savings",
		EntryType: nil,
	})
	if err != nil {
		t.Fatalf("UpdateBankAccount (entry type omitted): %v", err)
	}
	if got := edited.Asset.EntryType; got != repo.EntryTypeNewlyTracked {
		t.Errorf("omitting entry_type on update changed it to %q, want it left at %q",
			got, repo.EntryTypeNewlyTracked)
	}

	// 4. And a household that corrects a wrong declaration is obeyed — the
	//    editable control is the ONLY remedy for a mis-declared entry, since
	//    nothing detects one (ADR-0053 §3).
	corrected := repo.EntryTypeAcquired
	fixed, err := r.UpdateBankAccount(ctx, id, repo.UpdateBankAccountParams{
		DisplayName: "Long held (renamed)", OwnershipType: "joint",
		BankName: "Bank", AccountNumber: "222", AccountType: "savings",
		EntryType: &corrected,
	})
	if err != nil {
		t.Fatalf("UpdateBankAccount (entry type corrected): %v", err)
	}
	if got := fixed.Asset.EntryType; got != repo.EntryTypeAcquired {
		t.Errorf("correcting entry_type left it at %q, want %q", got, repo.EntryTypeAcquired)
	}
}
