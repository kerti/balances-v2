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

// TestVehicleRepo_TenancyIsolation parallels the property leak test for
// the vehicle subtype.
// covers: INV-TENANCY-03
func TestVehicleRepo_TenancyIsolation(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	aliceUser := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bobUser := testutil.CreateHouseholdWithUser(t, q, "Bob")

	aliceCtx := auth.WithUser(context.Background(), aliceUser)
	bobCtx := auth.WithUser(context.Background(), bobUser)

	r := repo.NewAssetRepo(tdb.Pool)

	aliceVehicle, err := r.CreateVehicle(aliceCtx, repo.CreateVehicleParams{
		DisplayName:    "Alice Car",
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		VehicleType:    "car",
	})
	if err != nil {
		t.Fatalf("alice CreateVehicle: %v", err)
	}

	t.Run("bob list excludes alice's vehicle", func(t *testing.T) {
		list, err := r.ListVehicles(bobCtx)
		if err != nil {
			t.Fatalf("ListVehicles: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("bob saw %d vehicles; want 0", len(list))
		}
	})

	t.Run("bob get returns ErrNotFound", func(t *testing.T) {
		_, err := r.GetVehicle(bobCtx, aliceVehicle.Asset.ID)
		if !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("GetVehicle: want ErrNotFound, got %v", err)
		}
	})

	t.Run("bob update returns ErrNotFound", func(t *testing.T) {
		_, err := r.UpdateVehicle(bobCtx, aliceVehicle.Asset.ID, repo.UpdateVehicleParams{
			DisplayName: "stolen!",
			VehicleType: "car",
		})
		if !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("UpdateVehicle: want ErrNotFound, got %v", err)
		}
	})

	t.Run("bob delete returns ErrNotFound", func(t *testing.T) {
		err := r.DeleteVehicle(bobCtx, aliceVehicle.Asset.ID)
		if !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("DeleteVehicle: want ErrNotFound, got %v", err)
		}
	})

	// ----- Alice happy-path CRUD on her own vehicle --------------------

	t.Run("alice update vehicle persists new display_name", func(t *testing.T) {
		updated, err := r.UpdateVehicle(aliceCtx, aliceVehicle.Asset.ID, repo.UpdateVehicleParams{
			DisplayName:   "Alice Car renamed",
			OwnershipType: "joint",
			VehicleType:   "car",
		})
		if err != nil {
			t.Fatalf("UpdateVehicle: %v", err)
		}
		if updated.Asset.DisplayName != "Alice Car renamed" {
			t.Errorf("DisplayName: got %q, want %q", updated.Asset.DisplayName, "Alice Car renamed")
		}
	})

	t.Run("alice update vehicle flips ownership joint→sole with owner picker", func(t *testing.T) {
		updated, err := r.UpdateVehicle(aliceCtx, aliceVehicle.Asset.ID, repo.UpdateVehicleParams{
			DisplayName:     "Alice Car renamed",
			OwnershipType:   "sole",
			SoleOwnerUserID: &aliceUser.ID,
			VehicleType:     "car",
		})
		if err != nil {
			t.Fatalf("UpdateVehicle sole: %v", err)
		}
		if updated.Asset.OwnershipType != "sole" {
			t.Errorf("OwnershipType: got %q, want sole", updated.Asset.OwnershipType)
		}
		if updated.Asset.SoleOwnerUserID == nil || *updated.Asset.SoleOwnerUserID != aliceUser.ID {
			t.Errorf("SoleOwnerUserID: got %v, want %v", updated.Asset.SoleOwnerUserID, aliceUser.ID)
		}
	})

	t.Run("alice list returns vehicle with details and latest snapshot", func(t *testing.T) {
		// Snapshot exercises the latest-snapshot join branch in ListVehicles.
		// Without this subtest, only the len==0 early return is covered.
		_, err := r.CreateAssetSnapshot(aliceCtx, repo.CreateAssetSnapshotParams{
			AssetID:   aliceVehicle.Asset.ID,
			YearMonth: time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
			Amount:    decimal.NewFromInt(180_000_000),
			Currency:  "IDR",
		})
		if err != nil {
			t.Fatalf("alice CreateAssetSnapshot: %v", err)
		}

		list, err := r.ListVehicles(aliceCtx)
		if err != nil {
			t.Fatalf("ListVehicles: %v", err)
		}
		if len(list) != 1 {
			t.Fatalf("ListVehicles: got %d, want 1", len(list))
		}
		item := list[0]
		if item.Asset.ID != aliceVehicle.Asset.ID {
			t.Errorf("Asset.ID: got %v, want %v", item.Asset.ID, aliceVehicle.Asset.ID)
		}
		if item.Details.VehicleType != "car" {
			t.Errorf("Details.VehicleType: got %q, want %q", item.Details.VehicleType, "car")
		}
		if item.LatestSnapshot == nil {
			t.Fatal("LatestSnapshot: got nil, want populated")
		}
		if !item.LatestSnapshot.Amount.Equal(decimal.NewFromInt(180_000_000)) {
			t.Errorf("LatestSnapshot.Amount: got %s, want 180000000", item.LatestSnapshot.Amount)
		}
	})

	t.Run("alice delete vehicle removes it from get and list", func(t *testing.T) {
		if err := r.DeleteVehicle(aliceCtx, aliceVehicle.Asset.ID); err != nil {
			t.Fatalf("DeleteVehicle: %v", err)
		}
		if _, err := r.GetVehicle(aliceCtx, aliceVehicle.Asset.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("GetVehicle after delete: want ErrNotFound, got %v", err)
		}
		list, err := r.ListVehicles(aliceCtx)
		if err != nil {
			t.Fatalf("ListVehicles after delete: %v", err)
		}
		if len(list) != 0 {
			t.Errorf("ListVehicles after delete: got %d, want 0", len(list))
		}
	})
}
