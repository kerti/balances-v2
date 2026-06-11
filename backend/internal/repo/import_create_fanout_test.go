package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/auth"
	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// Fan-out of CreateBankAccountWithSnapshots (issue #88) to the four remaining
// non-investment groups (#89). Each test covers the commit path: one
// transaction creates the position, assigns the resolved tag, and seeds every
// snapshot. The two flat groups also get their owner/tag lookup methods.

func twoImportRows() []repo.ImportSnapshotRow {
	return []repo.ImportSnapshotRow{
		{YearMonth: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("10000000"), Currency: "IDR"},
		{YearMonth: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("11000000"), Currency: "IDR"},
	}
}

func TestCreatePropertyWithSnapshots(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := auth.WithUser(context.Background(), alice)

	assets := repo.NewAssetRepo(tdb.Pool)
	tags := repo.NewTagRepo(tdb.Pool)
	tag, err := tags.CreateTag(ctx, "Emergency fund", "#22c55e")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}

	property, err := assets.CreatePropertyWithSnapshots(ctx, repo.CreatePropertyParams{
		DisplayName:     "Imported house",
		OwnershipType:   "sole",
		SoleOwnerUserID: &alice.ID,
		NativeCurrency:  "IDR",
		PropertyType:    "house",
	}, &tag.ID, twoImportRows())
	if err != nil {
		t.Fatalf("CreatePropertyWithSnapshots: %v", err)
	}
	if property.Asset.TagID == nil || *property.Asset.TagID != tag.ID {
		t.Errorf("tag_id: want %s, got %v", tag.ID, property.Asset.TagID)
	}
	snaps, err := assets.ListAssetSnapshots(ctx, property.Asset.ID)
	if err != nil {
		t.Fatalf("ListAssetSnapshots: %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("want 2 seeded snapshots, got %d", len(snaps))
	}
}

func TestCreateVehicleWithSnapshots(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := auth.WithUser(context.Background(), alice)

	assets := repo.NewAssetRepo(tdb.Pool)

	vehicle, err := assets.CreateVehicleWithSnapshots(ctx, repo.CreateVehicleParams{
		DisplayName:    "Imported car",
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		VehicleType:    "car",
	}, nil, twoImportRows())
	if err != nil {
		t.Fatalf("CreateVehicleWithSnapshots: %v", err)
	}
	if vehicle.Asset.TagID != nil {
		t.Errorf("untagged vehicle should have nil tag, got %v", vehicle.Asset.TagID)
	}
	snaps, err := assets.ListAssetSnapshots(ctx, vehicle.Asset.ID)
	if err != nil {
		t.Fatalf("ListAssetSnapshots: %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("want 2 seeded snapshots, got %d", len(snaps))
	}
}

func TestCreateLiabilityWithSnapshots(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := auth.WithUser(context.Background(), alice)

	liabilities := repo.NewLiabilityRepo(tdb.Pool)
	tags := repo.NewTagRepo(tdb.Pool)
	tag, err := tags.CreateTag(ctx, "Mortgage", "#ef4444")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}

	row, err := liabilities.CreateLiabilityWithSnapshots(ctx, repo.CreateLiabilityParams{
		DisplayName:      "Imported loan",
		Subtype:          "institutional",
		OwnershipType:    "sole",
		SoleOwnerUserID:  &alice.ID,
		NativeCurrency:   "IDR",
		CounterpartyName: "TestBank",
	}, &tag.ID, twoImportRows())
	if err != nil {
		t.Fatalf("CreateLiabilityWithSnapshots: %v", err)
	}
	if row.TagID == nil || *row.TagID != tag.ID {
		t.Errorf("tag_id: want %s, got %v", tag.ID, row.TagID)
	}
	snaps, err := liabilities.ListLiabilitySnapshots(ctx, row.ID)
	if err != nil {
		t.Fatalf("ListLiabilitySnapshots: %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("want 2 seeded snapshots, got %d", len(snaps))
	}
}

func TestCreateReceivableWithSnapshots(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := auth.WithUser(context.Background(), alice)

	receivables := repo.NewReceivableRepo(tdb.Pool)

	row, err := receivables.CreateReceivableWithSnapshots(ctx, repo.CreateReceivableParams{
		DisplayName:      "Imported receivable",
		OwnershipType:    "joint",
		NativeCurrency:   "IDR",
		CounterpartyName: "A friend",
	}, nil, twoImportRows())
	if err != nil {
		t.Fatalf("CreateReceivableWithSnapshots: %v", err)
	}
	if row.TagID != nil {
		t.Errorf("untagged receivable should have nil tag, got %v", row.TagID)
	}
	snaps, err := receivables.ListReceivableSnapshots(ctx, row.ID)
	if err != nil {
		t.Fatalf("ListReceivableSnapshots: %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("want 2 seeded snapshots, got %d", len(snaps))
	}
}

// TestFlatGroupLookups covers the owner/tag resolution the flat groups added so
// their resolve step can map the Detail-sheet sole_owner email + tag name back
// to ids (the inverse of export). The AssetRepo copies are exercised by
// import_create_test.go; these confirm the LiabilityRepo / ReceivableRepo
// methods are wired to the same household-scoped tables.
func TestFlatGroupLookups(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	ctx := auth.WithUser(context.Background(), alice)

	tags := repo.NewTagRepo(tdb.Pool)
	tag, err := tags.CreateTag(ctx, "Shared", "#22c55e")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}

	liabilities := repo.NewLiabilityRepo(tdb.Pool)
	receivables := repo.NewReceivableRepo(tdb.Pool)

	t.Run("liability lookups", func(t *testing.T) {
		id, found, err := liabilities.LookupUserIDByEmail(ctx, "  ALICE@example.com ")
		if err != nil || !found || id != alice.ID {
			t.Fatalf("owner lookup: found=%v id=%s err=%v", found, id, err)
		}
		if _, found, _ := liabilities.LookupUserIDByEmail(ctx, bob.Email); found {
			t.Error("cross-tenant owner must not resolve")
		}
		tid, err := liabilities.LookupTagIDByName(ctx, "Shared")
		if err != nil || tid == nil || *tid != tag.ID {
			t.Fatalf("tag lookup: got %v err=%v", tid, err)
		}
		if tid, _ := liabilities.LookupTagIDByName(ctx, "Nope"); tid != nil {
			t.Error("unmatched tag must be nil")
		}
	})

	t.Run("receivable lookups", func(t *testing.T) {
		id, found, err := receivables.LookupUserIDByEmail(ctx, "alice@example.com")
		if err != nil || !found || id != alice.ID {
			t.Fatalf("owner lookup: found=%v id=%s err=%v", found, id, err)
		}
		tid, err := receivables.LookupTagIDByName(ctx, "Shared")
		if err != nil || tid == nil || *tid != tag.ID {
			t.Fatalf("tag lookup: got %v err=%v", tid, err)
		}
		if tid, _ := receivables.LookupTagIDByName(ctx, "   "); tid != nil {
			t.Error("blank tag must be nil")
		}
	})
}
