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

// TestAssetRepo_TenancyIsolation verifies that a user in Household B cannot
// observe or mutate assets and snapshots belonging to Household A through
// any repository method. This is the day-one leak test required by ADR-0005
// and ADR-0021 — every position-shaped endpoint needs equivalent coverage as
// they ship.
// covers: INV-TENANCY-01
func TestAssetRepo_TenancyIsolation(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	aliceUser := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bobUser := testutil.CreateHouseholdWithUser(t, q, "Bob")

	if aliceUser.HouseholdID == bobUser.HouseholdID {
		t.Fatalf("fixture: alice and bob ended up in the same household")
	}

	aliceCtx := auth.WithUser(context.Background(), aliceUser)
	bobCtx := auth.WithUser(context.Background(), bobUser)

	r := repo.NewAssetRepo(tdb.Pool)

	// Alice creates a bank account and a snapshot.
	aliceAccount, err := r.CreateBankAccount(aliceCtx, repo.CreateBankAccountParams{
		DisplayName:    "Alice BCA",
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		BankName:       "BCA",
		AccountNumber:  "111",
		AccountType:    "savings",
	})
	if err != nil {
		t.Fatalf("alice CreateBankAccount: %v", err)
	}

	aliceSnap, err := r.CreateAssetSnapshot(aliceCtx, repo.CreateAssetSnapshotParams{
		AssetID:   aliceAccount.Asset.ID,
		YearMonth: time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
		Amount:    decimal.NewFromInt(1_000_000),
		Currency:  "IDR",
	})
	if err != nil {
		t.Fatalf("alice CreateAssetSnapshot: %v", err)
	}

	// ----- Bob can't observe Alice's bank account ----------------------

	t.Run("bob list excludes alice's account", func(t *testing.T) {
		list, err := r.ListBankAccounts(bobCtx)
		if err != nil {
			t.Fatalf("ListBankAccounts: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("bob saw %d bank accounts; want 0", len(list))
		}
	})

	t.Run("bob get returns ErrNotFound", func(t *testing.T) {
		_, err := r.GetBankAccount(bobCtx, aliceAccount.Asset.ID)
		if !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("GetBankAccount: want ErrNotFound, got %v", err)
		}
	})

	// ----- Bob can't mutate Alice's bank account ------------------------

	t.Run("bob update returns ErrNotFound", func(t *testing.T) {
		_, err := r.UpdateBankAccount(bobCtx, aliceAccount.Asset.ID, repo.UpdateBankAccountParams{
			DisplayName:   "stolen!",
			OwnershipType: "joint",
			BankName:      "BCA",
			AccountNumber: "111",
			AccountType:   "savings",
		})
		if !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("UpdateBankAccount: want ErrNotFound, got %v", err)
		}
	})

	t.Run("bob delete returns ErrNotFound", func(t *testing.T) {
		err := r.DeleteBankAccount(bobCtx, aliceAccount.Asset.ID)
		if !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("DeleteBankAccount: want ErrNotFound, got %v", err)
		}
	})

	// ----- Bob can't observe or mutate Alice's snapshots ----------------

	t.Run("bob list snapshots is empty", func(t *testing.T) {
		snaps, err := r.ListAssetSnapshots(bobCtx, aliceAccount.Asset.ID)
		if err != nil {
			t.Fatalf("ListAssetSnapshots: %v", err)
		}
		if len(snaps) != 0 {
			t.Errorf("bob saw %d snapshots; want 0", len(snaps))
		}
	})

	t.Run("bob create snapshot under alice's asset is not allowed", func(t *testing.T) {
		_, err := r.CreateAssetSnapshot(bobCtx, repo.CreateAssetSnapshotParams{
			AssetID:   aliceAccount.Asset.ID,
			YearMonth: time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
			Amount:    decimal.NewFromInt(999),
			Currency:  "IDR",
		})
		if !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("CreateAssetSnapshot: want ErrNotFound, got %v", err)
		}
	})

	t.Run("bob update alice's snapshot is not allowed", func(t *testing.T) {
		_, err := r.UpdateAssetSnapshot(bobCtx, repo.UpdateAssetSnapshotParams{
			SnapshotID: aliceSnap.ID,
			Amount:     decimal.NewFromInt(7),
			Currency:   "IDR",
		})
		if !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("UpdateAssetSnapshot: want ErrNotFound, got %v", err)
		}
	})

	t.Run("bob delete alice's snapshot is not allowed", func(t *testing.T) {
		err := r.DeleteAssetSnapshot(bobCtx, aliceSnap.ID)
		if !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("DeleteAssetSnapshot: want ErrNotFound, got %v", err)
		}
	})

	// ----- Sanity: Alice can still see her stuff after Bob's prodding --

	t.Run("alice still sees her account and snapshot", func(t *testing.T) {
		list, err := r.ListBankAccounts(aliceCtx)
		if err != nil {
			t.Fatalf("ListBankAccounts: %v", err)
		}
		if len(list) != 1 {
			t.Fatalf("alice saw %d bank accounts; want 1", len(list))
		}
		if list[0].LatestSnapshot == nil || list[0].LatestSnapshot.ID != aliceSnap.ID {
			t.Errorf("alice's latest_snapshot mismatch: %+v", list[0].LatestSnapshot)
		}
	})

	// ----- Alice happy-path CRUD on her own account and snapshot -------
	// These exercise the success branches of Update*, Delete*, and the
	// shared softDeleteAsset helper — paths the cross-tenant tests above
	// never reach because they bail out at the GetX subtype/tenancy guard.

	t.Run("alice update account persists new display_name", func(t *testing.T) {
		updated, err := r.UpdateBankAccount(aliceCtx, aliceAccount.Asset.ID, repo.UpdateBankAccountParams{
			DisplayName:   "Alice BCA renamed",
			OwnershipType: "joint",
			BankName:      "BCA",
			AccountNumber: "111",
			AccountType:   "savings",
		})
		if err != nil {
			t.Fatalf("UpdateBankAccount: %v", err)
		}
		if updated.Asset.DisplayName != "Alice BCA renamed" {
			t.Errorf("DisplayName: got %q, want %q", updated.Asset.DisplayName, "Alice BCA renamed")
		}
	})

	t.Run("alice update account flips ownership joint→sole with owner picker", func(t *testing.T) {
		updated, err := r.UpdateBankAccount(aliceCtx, aliceAccount.Asset.ID, repo.UpdateBankAccountParams{
			DisplayName:     "Alice BCA renamed",
			OwnershipType:   "sole",
			SoleOwnerUserID: &aliceUser.ID,
			BankName:        "BCA",
			AccountNumber:   "111",
			AccountType:     "savings",
		})
		if err != nil {
			t.Fatalf("UpdateBankAccount sole: %v", err)
		}
		if updated.Asset.OwnershipType != "sole" {
			t.Errorf("OwnershipType: got %q, want sole", updated.Asset.OwnershipType)
		}
		if updated.Asset.SoleOwnerUserID == nil || *updated.Asset.SoleOwnerUserID != aliceUser.ID {
			t.Errorf("SoleOwnerUserID: got %v, want %v", updated.Asset.SoleOwnerUserID, aliceUser.ID)
		}
	})

	t.Run("alice update snapshot persists new amount", func(t *testing.T) {
		updated, err := r.UpdateAssetSnapshot(aliceCtx, repo.UpdateAssetSnapshotParams{
			SnapshotID: aliceSnap.ID,
			Amount:     decimal.NewFromInt(42),
			Currency:   "IDR",
		})
		if err != nil {
			t.Fatalf("UpdateAssetSnapshot: %v", err)
		}
		if !updated.Amount.Equal(decimal.NewFromInt(42)) {
			t.Errorf("Amount: got %s, want 42", updated.Amount)
		}
	})

	t.Run("alice delete snapshot removes it from list", func(t *testing.T) {
		if err := r.DeleteAssetSnapshot(aliceCtx, aliceSnap.ID); err != nil {
			t.Fatalf("DeleteAssetSnapshot: %v", err)
		}
		snaps, err := r.ListAssetSnapshots(aliceCtx, aliceAccount.Asset.ID)
		if err != nil {
			t.Fatalf("ListAssetSnapshots: %v", err)
		}
		for _, s := range snaps {
			if s.ID == aliceSnap.ID {
				t.Errorf("deleted snapshot still in list")
			}
		}
	})

	t.Run("alice delete account removes it from get and list", func(t *testing.T) {
		if err := r.DeleteBankAccount(aliceCtx, aliceAccount.Asset.ID); err != nil {
			t.Fatalf("DeleteBankAccount: %v", err)
		}
		if _, err := r.GetBankAccount(aliceCtx, aliceAccount.Asset.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("GetBankAccount after delete: want ErrNotFound, got %v", err)
		}
		list, err := r.ListBankAccounts(aliceCtx)
		if err != nil {
			t.Fatalf("ListBankAccounts after delete: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("ListBankAccounts after delete: got %d, want 0", len(list))
		}
	})
}
