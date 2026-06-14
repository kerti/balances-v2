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

// TestPropertyRepo_TenancyIsolation mirrors TestAssetRepo_TenancyIsolation
// for the property subtype. Snapshot-level tenancy is already covered by
// the bank-account leak test (asset snapshots share one table), so this
// test focuses on the property-specific repo surface only.
// covers: INV-TENANCY-02
func TestPropertyRepo_TenancyIsolation(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	aliceUser := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bobUser := testutil.CreateHouseholdWithUser(t, q, "Bob")

	aliceCtx := auth.WithUser(context.Background(), aliceUser)
	bobCtx := auth.WithUser(context.Background(), bobUser)

	r := repo.NewAssetRepo(tdb.Pool)

	aliceProperty, err := r.CreateProperty(aliceCtx, repo.CreatePropertyParams{
		DisplayName:    "Alice House",
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		PropertyType:   "house",
	})
	if err != nil {
		t.Fatalf("alice CreateProperty: %v", err)
	}

	t.Run("bob list excludes alice's property", func(t *testing.T) {
		list, err := r.ListProperties(bobCtx)
		if err != nil {
			t.Fatalf("ListProperties: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("bob saw %d properties; want 0", len(list))
		}
	})

	t.Run("bob get returns ErrNotFound", func(t *testing.T) {
		_, err := r.GetProperty(bobCtx, aliceProperty.Asset.ID)
		if !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("GetProperty: want ErrNotFound, got %v", err)
		}
	})

	t.Run("bob update returns ErrNotFound", func(t *testing.T) {
		_, err := r.UpdateProperty(bobCtx, aliceProperty.Asset.ID, repo.UpdatePropertyParams{
			DisplayName:  "stolen!",
			PropertyType: "house",
		})
		if !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("UpdateProperty: want ErrNotFound, got %v", err)
		}
	})

	t.Run("bob delete returns ErrNotFound", func(t *testing.T) {
		err := r.DeleteProperty(bobCtx, aliceProperty.Asset.ID)
		if !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("DeleteProperty: want ErrNotFound, got %v", err)
		}
	})

	t.Run("alice's property accessed via bank-account or vehicle methods returns ErrNotFound", func(t *testing.T) {
		// Subtype guard: even from alice's own context, fetching a property
		// via GetBankAccount or GetVehicle must return ErrNotFound.
		if _, err := r.GetBankAccount(aliceCtx, aliceProperty.Asset.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("GetBankAccount on property id: want ErrNotFound, got %v", err)
		}
		if _, err := r.GetVehicle(aliceCtx, aliceProperty.Asset.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("GetVehicle on property id: want ErrNotFound, got %v", err)
		}
		if err := r.DeleteBankAccount(aliceCtx, aliceProperty.Asset.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("DeleteBankAccount on property id: want ErrNotFound, got %v", err)
		}
	})

	// ----- Alice happy-path CRUD on her own property -------------------

	t.Run("alice update property persists new display_name", func(t *testing.T) {
		updated, err := r.UpdateProperty(aliceCtx, aliceProperty.Asset.ID, repo.UpdatePropertyParams{
			DisplayName:   "Alice House renamed",
			OwnershipType: "joint",
			PropertyType:  "house",
		})
		if err != nil {
			t.Fatalf("UpdateProperty: %v", err)
		}
		if updated.Asset.DisplayName != "Alice House renamed" {
			t.Errorf("DisplayName: got %q, want %q", updated.Asset.DisplayName, "Alice House renamed")
		}
	})

	t.Run("alice update property flips ownership joint→sole with owner picker", func(t *testing.T) {
		updated, err := r.UpdateProperty(aliceCtx, aliceProperty.Asset.ID, repo.UpdatePropertyParams{
			DisplayName:     "Alice House renamed",
			OwnershipType:   "sole",
			SoleOwnerUserID: &aliceUser.ID,
			PropertyType:    "house",
		})
		if err != nil {
			t.Fatalf("UpdateProperty sole: %v", err)
		}
		if updated.Asset.OwnershipType != "sole" {
			t.Errorf("OwnershipType: got %q, want sole", updated.Asset.OwnershipType)
		}
		if updated.Asset.SoleOwnerUserID == nil || *updated.Asset.SoleOwnerUserID != aliceUser.ID {
			t.Errorf("SoleOwnerUserID: got %v, want %v", updated.Asset.SoleOwnerUserID, aliceUser.ID)
		}
	})

	t.Run("alice list returns property with details and latest snapshot", func(t *testing.T) {
		// Snapshot exercises the latest-snapshot join branch in ListProperties.
		// Without this subtest, only the len==0 early return is covered.
		_, err := r.CreateAssetSnapshot(aliceCtx, repo.CreateAssetSnapshotParams{
			AssetID:   aliceProperty.Asset.ID,
			YearMonth: time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
			Amount:    decimal.NewFromInt(2_500_000_000),
			Currency:  "IDR",
		})
		if err != nil {
			t.Fatalf("alice CreateAssetSnapshot: %v", err)
		}

		list, err := r.ListProperties(aliceCtx)
		if err != nil {
			t.Fatalf("ListProperties: %v", err)
		}
		if len(list) != 1 {
			t.Fatalf("ListProperties: got %d, want 1", len(list))
		}
		item := list[0]
		if item.Asset.ID != aliceProperty.Asset.ID {
			t.Errorf("Asset.ID: got %v, want %v", item.Asset.ID, aliceProperty.Asset.ID)
		}
		if item.Details.PropertyType != "house" {
			t.Errorf("Details.PropertyType: got %q, want %q", item.Details.PropertyType, "house")
		}
		if item.LatestSnapshot == nil {
			t.Fatal("LatestSnapshot: got nil, want populated")
		}
		if !item.LatestSnapshot.Amount.Equal(decimal.NewFromInt(2_500_000_000)) {
			t.Errorf("LatestSnapshot.Amount: got %s, want 2500000000", item.LatestSnapshot.Amount)
		}
	})

	t.Run("alice delete property removes it from get and list", func(t *testing.T) {
		if err := r.DeleteProperty(aliceCtx, aliceProperty.Asset.ID); err != nil {
			t.Fatalf("DeleteProperty: %v", err)
		}
		if _, err := r.GetProperty(aliceCtx, aliceProperty.Asset.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("GetProperty after delete: want ErrNotFound, got %v", err)
		}
		list, err := r.ListProperties(aliceCtx)
		if err != nil {
			t.Fatalf("ListProperties after delete: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("ListProperties after delete: got %d, want 0", len(list))
		}
	})
}
