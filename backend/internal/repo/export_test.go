package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// TestExportProperty / Vehicle / Liability / Receivable mirror
// TestExportBankAccount for the remaining non-investment groups (#86): a sole
// position resolves its owner to an email and its tag to a name, snapshot
// history comes back, and the ownership gate returns ErrNotFound for unknown /
// cross-tenant ids.

// covers: INV-EXPORT-01, INV-EXPORT-02, INV-EXPORT-03, INV-EXPORT-04
func TestExportProperty(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := auth.WithUser(context.Background(), alice)
	bobCtx := auth.WithUser(context.Background(), bob)

	assets := repo.NewAssetRepo(tdb.Pool)
	tags := repo.NewTagRepo(tdb.Pool)

	t.Run("sole owner + tag + snapshots resolve", func(t *testing.T) {
		cost := decimal.RequireFromString("2500000000")
		prop, err := assets.CreateProperty(aliceCtx, repo.CreatePropertyParams{
			DisplayName:     "Family home",
			OwnershipType:   "sole",
			SoleOwnerUserID: &alice.ID,
			NativeCurrency:  "IDR",
			PropertyType:    "house",
			AcquisitionCost: &cost,
		})
		if err != nil {
			t.Fatalf("CreateProperty: %v", err)
		}

		tag, err := tags.CreateTag(aliceCtx, "Primary residence", "#22c55e")
		if err != nil {
			t.Fatalf("CreateTag: %v", err)
		}
		if err := tags.AssignTag(aliceCtx, repo.TagGroupAsset, prop.Asset.ID, &tag.ID); err != nil {
			t.Fatalf("AssignTag: %v", err)
		}

		if _, err := assets.CreateAssetSnapshot(aliceCtx, repo.CreateAssetSnapshotParams{
			AssetID:   prop.Asset.ID,
			YearMonth: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Amount:    decimal.RequireFromString("2600000000"),
			Currency:  "IDR",
		}); err != nil {
			t.Fatalf("CreateAssetSnapshot: %v", err)
		}

		out, err := assets.ExportProperty(aliceCtx, prop.Asset.ID)
		if err != nil {
			t.Fatalf("ExportProperty: %v", err)
		}
		if out.OwnerEmail != alice.Email {
			t.Errorf("OwnerEmail = %q, want %q", out.OwnerEmail, alice.Email)
		}
		if out.TagName != "Primary residence" {
			t.Errorf("TagName = %q, want Primary residence", out.TagName)
		}
		if len(out.Snapshots) != 1 {
			t.Fatalf("want 1 snapshot, got %d", len(out.Snapshots))
		}
		if out.Property.Details.PropertyType != "house" {
			t.Errorf("Details.PropertyType = %q, want house", out.Property.Details.PropertyType)
		}
	})

	t.Run("joint untagged leaves owner + tag blank", func(t *testing.T) {
		prop, err := assets.CreateProperty(aliceCtx, repo.CreatePropertyParams{
			DisplayName:    "Joint land",
			OwnershipType:  "joint",
			NativeCurrency: "IDR",
			PropertyType:   "land",
		})
		if err != nil {
			t.Fatalf("CreateProperty: %v", err)
		}
		out, err := assets.ExportProperty(aliceCtx, prop.Asset.ID)
		if err != nil {
			t.Fatalf("ExportProperty: %v", err)
		}
		if out.OwnerEmail != "" || out.TagName != "" {
			t.Errorf("joint untagged: OwnerEmail=%q TagName=%q, want both empty", out.OwnerEmail, out.TagName)
		}
		if len(out.Snapshots) != 0 {
			t.Errorf("want 0 snapshots, got %d", len(out.Snapshots))
		}
	})

	t.Run("unknown id is ErrNotFound", func(t *testing.T) {
		if _, err := assets.ExportProperty(aliceCtx, uuid.New()); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("unknown id: got %v, want ErrNotFound", err)
		}
	})

	t.Run("cross-tenant is ErrNotFound", func(t *testing.T) {
		prop, err := assets.CreateProperty(aliceCtx, repo.CreatePropertyParams{
			DisplayName:    "Alice only",
			OwnershipType:  "joint",
			NativeCurrency: "IDR",
			PropertyType:   "apartment",
		})
		if err != nil {
			t.Fatalf("CreateProperty: %v", err)
		}
		if _, err := assets.ExportProperty(bobCtx, prop.Asset.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("cross-tenant: got %v, want ErrNotFound", err)
		}
	})
}

// covers: INV-EXPORT-01, INV-EXPORT-02, INV-EXPORT-03, INV-EXPORT-04
func TestExportVehicle(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := auth.WithUser(context.Background(), alice)
	bobCtx := auth.WithUser(context.Background(), bob)

	assets := repo.NewAssetRepo(tdb.Pool)
	tags := repo.NewTagRepo(tdb.Pool)

	t.Run("sole owner + tag + snapshots resolve", func(t *testing.T) {
		carMake := "Toyota"
		year := int32(2020)
		veh, err := assets.CreateVehicle(aliceCtx, repo.CreateVehicleParams{
			DisplayName:     "Daily driver",
			OwnershipType:   "sole",
			SoleOwnerUserID: &alice.ID,
			NativeCurrency:  "IDR",
			VehicleType:     "car",
			Make:            &carMake,
			Year:            &year,
		})
		if err != nil {
			t.Fatalf("CreateVehicle: %v", err)
		}

		tag, err := tags.CreateTag(aliceCtx, "Household car", "#3b82f6")
		if err != nil {
			t.Fatalf("CreateTag: %v", err)
		}
		if err := tags.AssignTag(aliceCtx, repo.TagGroupAsset, veh.Asset.ID, &tag.ID); err != nil {
			t.Fatalf("AssignTag: %v", err)
		}

		if _, err := assets.CreateAssetSnapshot(aliceCtx, repo.CreateAssetSnapshotParams{
			AssetID:   veh.Asset.ID,
			YearMonth: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Amount:    decimal.RequireFromString("180000000"),
			Currency:  "IDR",
		}); err != nil {
			t.Fatalf("CreateAssetSnapshot: %v", err)
		}

		out, err := assets.ExportVehicle(aliceCtx, veh.Asset.ID)
		if err != nil {
			t.Fatalf("ExportVehicle: %v", err)
		}
		if out.OwnerEmail != alice.Email {
			t.Errorf("OwnerEmail = %q, want %q", out.OwnerEmail, alice.Email)
		}
		if out.TagName != "Household car" {
			t.Errorf("TagName = %q, want Household car", out.TagName)
		}
		if len(out.Snapshots) != 1 {
			t.Fatalf("want 1 snapshot, got %d", len(out.Snapshots))
		}
		if out.Vehicle.Details.VehicleType != "car" {
			t.Errorf("Details.VehicleType = %q, want car", out.Vehicle.Details.VehicleType)
		}
	})

	t.Run("joint untagged leaves owner + tag blank", func(t *testing.T) {
		veh, err := assets.CreateVehicle(aliceCtx, repo.CreateVehicleParams{
			DisplayName:    "Shared bike",
			OwnershipType:  "joint",
			NativeCurrency: "IDR",
			VehicleType:    "motorcycle",
		})
		if err != nil {
			t.Fatalf("CreateVehicle: %v", err)
		}
		out, err := assets.ExportVehicle(aliceCtx, veh.Asset.ID)
		if err != nil {
			t.Fatalf("ExportVehicle: %v", err)
		}
		if out.OwnerEmail != "" || out.TagName != "" {
			t.Errorf("joint untagged: OwnerEmail=%q TagName=%q, want both empty", out.OwnerEmail, out.TagName)
		}
		if len(out.Snapshots) != 0 {
			t.Errorf("want 0 snapshots, got %d", len(out.Snapshots))
		}
	})

	t.Run("unknown id is ErrNotFound", func(t *testing.T) {
		if _, err := assets.ExportVehicle(aliceCtx, uuid.New()); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("unknown id: got %v, want ErrNotFound", err)
		}
	})

	t.Run("cross-tenant is ErrNotFound", func(t *testing.T) {
		veh, err := assets.CreateVehicle(aliceCtx, repo.CreateVehicleParams{
			DisplayName:    "Alice only",
			OwnershipType:  "joint",
			NativeCurrency: "IDR",
			VehicleType:    "other",
		})
		if err != nil {
			t.Fatalf("CreateVehicle: %v", err)
		}
		if _, err := assets.ExportVehicle(bobCtx, veh.Asset.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("cross-tenant: got %v, want ErrNotFound", err)
		}
	})
}

// covers: INV-EXPORT-01, INV-EXPORT-02, INV-EXPORT-03, INV-EXPORT-04
func TestExportLiability(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := auth.WithUser(context.Background(), alice)
	bobCtx := auth.WithUser(context.Background(), bob)

	r := repo.NewLiabilityRepo(tdb.Pool)
	tags := repo.NewTagRepo(tdb.Pool)

	t.Run("sole owner + tag + snapshots resolve", func(t *testing.T) {
		principal := decimal.RequireFromString("1400000000")
		l, err := r.CreateLiability(aliceCtx, repo.CreateLiabilityParams{
			DisplayName:      "Home loan",
			Subtype:          "institutional",
			OwnershipType:    "sole",
			SoleOwnerUserID:  &alice.ID,
			NativeCurrency:   "IDR",
			CounterpartyName: "TestBank",
			Principal:        &principal,
		})
		if err != nil {
			t.Fatalf("CreateLiability: %v", err)
		}

		tag, err := tags.CreateTag(aliceCtx, "Mortgage", "#ef4444")
		if err != nil {
			t.Fatalf("CreateTag: %v", err)
		}
		if err := tags.AssignTag(aliceCtx, repo.TagGroupLiability, l.ID, &tag.ID); err != nil {
			t.Fatalf("AssignTag: %v", err)
		}

		if _, err := r.CreateLiabilitySnapshot(aliceCtx, repo.CreateLiabilitySnapshotParams{
			LiabilityID: l.ID,
			YearMonth:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Amount:      decimal.RequireFromString("1350000000"),
			Currency:    "IDR",
		}); err != nil {
			t.Fatalf("CreateLiabilitySnapshot: %v", err)
		}

		out, err := r.ExportLiability(aliceCtx, l.ID)
		if err != nil {
			t.Fatalf("ExportLiability: %v", err)
		}
		if out.OwnerEmail != alice.Email {
			t.Errorf("OwnerEmail = %q, want %q", out.OwnerEmail, alice.Email)
		}
		if out.TagName != "Mortgage" {
			t.Errorf("TagName = %q, want Mortgage", out.TagName)
		}
		if len(out.Snapshots) != 1 {
			t.Fatalf("want 1 snapshot, got %d", len(out.Snapshots))
		}
		if out.Liability.CounterpartyName != "TestBank" {
			t.Errorf("CounterpartyName = %q, want TestBank", out.Liability.CounterpartyName)
		}
	})

	t.Run("joint untagged leaves owner + tag blank", func(t *testing.T) {
		l, err := r.CreateLiability(aliceCtx, repo.CreateLiabilityParams{
			DisplayName:      "Joint debt",
			Subtype:          "personal",
			OwnershipType:    "joint",
			NativeCurrency:   "IDR",
			CounterpartyName: "A friend",
		})
		if err != nil {
			t.Fatalf("CreateLiability: %v", err)
		}
		out, err := r.ExportLiability(aliceCtx, l.ID)
		if err != nil {
			t.Fatalf("ExportLiability: %v", err)
		}
		if out.OwnerEmail != "" || out.TagName != "" {
			t.Errorf("joint untagged: OwnerEmail=%q TagName=%q, want both empty", out.OwnerEmail, out.TagName)
		}
		if len(out.Snapshots) != 0 {
			t.Errorf("want 0 snapshots, got %d", len(out.Snapshots))
		}
	})

	t.Run("unknown id is ErrNotFound", func(t *testing.T) {
		if _, err := r.ExportLiability(aliceCtx, uuid.New()); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("unknown id: got %v, want ErrNotFound", err)
		}
	})

	t.Run("cross-tenant is ErrNotFound", func(t *testing.T) {
		l, err := r.CreateLiability(aliceCtx, repo.CreateLiabilityParams{
			DisplayName:      "Alice only",
			Subtype:          "personal",
			OwnershipType:    "joint",
			NativeCurrency:   "IDR",
			CounterpartyName: "Nobody",
		})
		if err != nil {
			t.Fatalf("CreateLiability: %v", err)
		}
		if _, err := r.ExportLiability(bobCtx, l.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("cross-tenant: got %v, want ErrNotFound", err)
		}
	})
}

// covers: INV-EXPORT-01, INV-EXPORT-02, INV-EXPORT-03, INV-EXPORT-04
func TestExportReceivable(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := auth.WithUser(context.Background(), alice)
	bobCtx := auth.WithUser(context.Background(), bob)

	r := repo.NewReceivableRepo(tdb.Pool)
	tags := repo.NewTagRepo(tdb.Pool)

	t.Run("sole owner + tag + snapshots resolve", func(t *testing.T) {
		due := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
		rv, err := r.CreateReceivable(aliceCtx, repo.CreateReceivableParams{
			DisplayName:      "Loan to friend",
			OwnershipType:    "sole",
			SoleOwnerUserID:  &alice.ID,
			NativeCurrency:   "IDR",
			CounterpartyName: "A friend",
			DueDate:          &due,
		})
		if err != nil {
			t.Fatalf("CreateReceivable: %v", err)
		}

		tag, err := tags.CreateTag(aliceCtx, "IOU", "#eab308")
		if err != nil {
			t.Fatalf("CreateTag: %v", err)
		}
		if err := tags.AssignTag(aliceCtx, repo.TagGroupReceivable, rv.ID, &tag.ID); err != nil {
			t.Fatalf("AssignTag: %v", err)
		}

		if _, err := r.CreateReceivableSnapshot(aliceCtx, repo.CreateReceivableSnapshotParams{
			ReceivableID: rv.ID,
			YearMonth:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Amount:       decimal.RequireFromString("50000000"),
			Currency:     "IDR",
		}); err != nil {
			t.Fatalf("CreateReceivableSnapshot: %v", err)
		}

		out, err := r.ExportReceivable(aliceCtx, rv.ID)
		if err != nil {
			t.Fatalf("ExportReceivable: %v", err)
		}
		if out.OwnerEmail != alice.Email {
			t.Errorf("OwnerEmail = %q, want %q", out.OwnerEmail, alice.Email)
		}
		if out.TagName != "IOU" {
			t.Errorf("TagName = %q, want IOU", out.TagName)
		}
		if len(out.Snapshots) != 1 {
			t.Fatalf("want 1 snapshot, got %d", len(out.Snapshots))
		}
		if out.Receivable.CounterpartyName != "A friend" {
			t.Errorf("CounterpartyName = %q, want A friend", out.Receivable.CounterpartyName)
		}
	})

	t.Run("joint untagged leaves owner + tag blank", func(t *testing.T) {
		rv, err := r.CreateReceivable(aliceCtx, repo.CreateReceivableParams{
			DisplayName:      "Joint receivable",
			OwnershipType:    "joint",
			NativeCurrency:   "IDR",
			CounterpartyName: "Someone",
		})
		if err != nil {
			t.Fatalf("CreateReceivable: %v", err)
		}
		out, err := r.ExportReceivable(aliceCtx, rv.ID)
		if err != nil {
			t.Fatalf("ExportReceivable: %v", err)
		}
		if out.OwnerEmail != "" || out.TagName != "" {
			t.Errorf("joint untagged: OwnerEmail=%q TagName=%q, want both empty", out.OwnerEmail, out.TagName)
		}
		if len(out.Snapshots) != 0 {
			t.Errorf("want 0 snapshots, got %d", len(out.Snapshots))
		}
	})

	t.Run("unknown id is ErrNotFound", func(t *testing.T) {
		if _, err := r.ExportReceivable(aliceCtx, uuid.New()); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("unknown id: got %v, want ErrNotFound", err)
		}
	})

	t.Run("cross-tenant is ErrNotFound", func(t *testing.T) {
		rv, err := r.CreateReceivable(aliceCtx, repo.CreateReceivableParams{
			DisplayName:      "Alice only",
			OwnershipType:    "joint",
			NativeCurrency:   "IDR",
			CounterpartyName: "Nobody",
		})
		if err != nil {
			t.Fatalf("CreateReceivable: %v", err)
		}
		if _, err := r.ExportReceivable(bobCtx, rv.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("cross-tenant: got %v, want ErrNotFound", err)
		}
	})
}
