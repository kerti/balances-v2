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

// TestLookupUserIDByEmail covers the inverse of the Detail-sheet sole_owner
// convention: a household member's email resolves to their id (case-
// insensitively), an unknown / cross-tenant email is "not found" rather than an
// error.
// covers: INV-IMPORT-04
func TestLookupUserIDByEmail(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob") // different household
	aliceCtx := auth.WithUser(context.Background(), alice)

	assets := repo.NewAssetRepo(tdb.Pool)

	t.Run("resolves a member, case-insensitive", func(t *testing.T) {
		id, found, err := assets.LookupUserIDByEmail(aliceCtx, "  ALICE@example.com ")
		if err != nil {
			t.Fatalf("lookup: %v", err)
		}
		if !found || id != alice.ID {
			t.Fatalf("want found alice id %s, got found=%v id=%s", alice.ID, found, id)
		}
	})

	t.Run("unknown email not found", func(t *testing.T) {
		_, found, err := assets.LookupUserIDByEmail(aliceCtx, "nobody@example.com")
		if err != nil {
			t.Fatalf("lookup: %v", err)
		}
		if found {
			t.Fatal("unknown email should not resolve")
		}
	})

	t.Run("cross-tenant email not found", func(t *testing.T) {
		_, found, err := assets.LookupUserIDByEmail(aliceCtx, bob.Email)
		if err != nil {
			t.Fatalf("lookup: %v", err)
		}
		if found {
			t.Fatal("a member of another household must not resolve")
		}
	})
}

// TestLookupTagIDByName covers the inverse of the Detail-sheet tag convention:
// a household Tag name resolves to its id; an unmatched / cross-tenant / blank
// name returns (nil, nil) so the create-import leaves the position untagged.
// covers: INV-IMPORT-04
func TestLookupTagIDByName(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := auth.WithUser(context.Background(), alice)
	bobCtx := auth.WithUser(context.Background(), bob)

	assets := repo.NewAssetRepo(tdb.Pool)
	tags := repo.NewTagRepo(tdb.Pool)

	tag, err := tags.CreateTag(aliceCtx, "Emergency fund", "#22c55e")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}
	if _, err := tags.CreateTag(bobCtx, "Bob's tag", "#ef4444"); err != nil {
		t.Fatalf("CreateTag bob: %v", err)
	}

	t.Run("resolves an existing tag", func(t *testing.T) {
		id, err := assets.LookupTagIDByName(aliceCtx, "Emergency fund")
		if err != nil {
			t.Fatalf("lookup: %v", err)
		}
		if id == nil || *id != tag.ID {
			t.Fatalf("want tag id %s, got %v", tag.ID, id)
		}
	})

	t.Run("unmatched name leaves it unassigned", func(t *testing.T) {
		id, err := assets.LookupTagIDByName(aliceCtx, "Does not exist")
		if err != nil {
			t.Fatalf("lookup: %v", err)
		}
		if id != nil {
			t.Fatalf("unmatched name must be nil, got %v", id)
		}
	})

	t.Run("cross-tenant name leaves it unassigned", func(t *testing.T) {
		id, err := assets.LookupTagIDByName(aliceCtx, "Bob's tag")
		if err != nil {
			t.Fatalf("lookup: %v", err)
		}
		if id != nil {
			t.Fatalf("another household's tag must not resolve, got %v", id)
		}
	})

	t.Run("blank name returns nil without a query", func(t *testing.T) {
		id, err := assets.LookupTagIDByName(aliceCtx, "   ")
		if err != nil {
			t.Fatalf("lookup: %v", err)
		}
		if id != nil {
			t.Fatalf("blank name must be nil, got %v", id)
		}
	})
}

// TestCreateBankAccountWithSnapshots covers the commit path of create-import:
// one transaction creates the position, assigns the resolved tag, and seeds
// every snapshot.
// covers: INV-IMPORT-03
func TestCreateBankAccountWithSnapshots(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	aliceCtx := auth.WithUser(context.Background(), alice)

	assets := repo.NewAssetRepo(tdb.Pool)
	tags := repo.NewTagRepo(tdb.Pool)

	tag, err := tags.CreateTag(aliceCtx, "Emergency fund", "#22c55e")
	if err != nil {
		t.Fatalf("CreateTag: %v", err)
	}

	params := repo.CreateBankAccountParams{
		DisplayName:     "Imported checking",
		OwnershipType:   "sole",
		SoleOwnerUserID: &alice.ID,
		NativeCurrency:  "IDR",
		BankName:        "TestBank",
		AccountNumber:   "1234567890",
		AccountType:     "savings",
	}
	rows := []repo.ImportSnapshotRow{
		{YearMonth: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("10000000"), Currency: "IDR"},
		{YearMonth: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), Amount: decimal.RequireFromString("11000000"), Currency: "IDR"},
	}

	acct, err := assets.CreateBankAccountWithSnapshots(aliceCtx, params, &tag.ID, rows)
	if err != nil {
		t.Fatalf("CreateBankAccountWithSnapshots: %v", err)
	}
	if acct.Asset.DisplayName != "Imported checking" {
		t.Errorf("display_name: got %q", acct.Asset.DisplayName)
	}
	if acct.Asset.TagID == nil || *acct.Asset.TagID != tag.ID {
		t.Errorf("tag_id: want %s, got %v", tag.ID, acct.Asset.TagID)
	}
	if acct.Asset.SoleOwnerUserID == nil || *acct.Asset.SoleOwnerUserID != alice.ID {
		t.Errorf("sole_owner_user_id: want %s, got %v", alice.ID, acct.Asset.SoleOwnerUserID)
	}

	snaps, err := assets.ListAssetSnapshots(aliceCtx, acct.Asset.ID)
	if err != nil {
		t.Fatalf("ListAssetSnapshots: %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("want 2 seeded snapshots, got %d", len(snaps))
	}

	t.Run("untagged when tagID nil", func(t *testing.T) {
		untagged, err := assets.CreateBankAccountWithSnapshots(aliceCtx, repo.CreateBankAccountParams{
			DisplayName:    "No tag",
			OwnershipType:  "joint",
			NativeCurrency: "IDR",
			BankName:       "TestBank",
			AccountNumber:  "999",
			AccountType:    "current",
		}, nil, nil)
		if err != nil {
			t.Fatalf("create untagged: %v", err)
		}
		if untagged.Asset.TagID != nil {
			t.Errorf("tag_id should be nil, got %v", untagged.Asset.TagID)
		}
	})
}
