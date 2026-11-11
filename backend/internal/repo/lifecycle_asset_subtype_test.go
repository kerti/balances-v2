package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// An Asset's terminal statuses narrow by subtype (#827): a bank account is
// closed, never sold or disposed; a property is sold or disposed, never closed.
// `untracked` stays open to both (ADR-0053 §5). A mismatch is a 400
// (ErrInvalidLifecycle), not a row the UI could never have produced.
//
// covers: INV-LIFECYCLE-10
func TestUpdateAssetLifecycle_RejectsStatusMismatchedWithSubtype(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), alice)
	ar := repo.NewAssetRepo(tdb.Pool)
	termDate := day(2026, time.March, 15)

	acct, err := ar.CreateBankAccount(ctx, repo.CreateBankAccountParams{
		DisplayName: "Everyday account", OwnershipType: "joint", NativeCurrency: "IDR",
		BankName: "Bank", AccountNumber: "111", AccountType: "savings",
	})
	if err != nil {
		t.Fatalf("CreateBankAccount: %v", err)
	}
	prop, err := ar.CreateProperty(ctx, repo.CreatePropertyParams{
		DisplayName: "House", OwnershipType: "joint", NativeCurrency: "IDR", PropertyType: "house",
	})
	if err != nil {
		t.Fatalf("CreateProperty: %v", err)
	}

	for _, status := range []string{"sold", "disposed"} {
		_, err := ar.UpdateAssetLifecycle(ctx, acct.Asset.ID, repo.LifecycleParams{Status: status, TerminatedAt: &termDate})
		if !errors.Is(err, repo.ErrInvalidLifecycle) {
			t.Errorf("bank account %q: got %v, want ErrInvalidLifecycle", status, err)
		}
	}
	if _, err := ar.UpdateAssetLifecycle(ctx, prop.Asset.ID, repo.LifecycleParams{Status: "closed", TerminatedAt: &termDate}); !errors.Is(err, repo.ErrInvalidLifecycle) {
		t.Errorf("property closed: got %v, want ErrInvalidLifecycle", err)
	}

	if _, err := ar.UpdateAssetLifecycle(ctx, acct.Asset.ID, repo.LifecycleParams{Status: "closed", TerminatedAt: &termDate}); err != nil {
		t.Errorf("bank account closed: %v", err)
	}
	if _, err := ar.UpdateAssetLifecycle(ctx, prop.Asset.ID, repo.LifecycleParams{Status: "sold", TerminatedAt: &termDate}); err != nil {
		t.Errorf("property sold: %v", err)
	}

	// A bank account already on "sold" (recorded before the narrowing, or by
	// import/restore) can still be re-saved on that status, e.g. to fix its note.
	legacy, err := ar.CreateBankAccount(ctx, repo.CreateBankAccountParams{
		DisplayName: "Old account", OwnershipType: "joint", NativeCurrency: "IDR",
		BankName: "Bank", AccountNumber: "222", AccountType: "savings",
	})
	if err != nil {
		t.Fatalf("CreateBankAccount: %v", err)
	}
	if _, err := tdb.Pool.Exec(ctx, "UPDATE assets SET status = 'sold', terminated_at = $2 WHERE id = $1", legacy.Asset.ID, termDate); err != nil {
		t.Fatalf("seed legacy sold: %v", err)
	}
	note := "fixed the note"
	if _, err := ar.UpdateAssetLifecycle(ctx, legacy.Asset.ID, repo.LifecycleParams{Status: "sold", TerminatedAt: &termDate, TerminationNote: &note}); err != nil {
		t.Errorf("re-save legacy sold bank account: %v", err)
	}
}
