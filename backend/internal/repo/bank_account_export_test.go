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

// TestExportBankAccount covers the export gather: a sole account resolves its
// owner to an email and its tag to a name, snapshot history comes back, and the
// ownership gate returns ErrNotFound for unknown / cross-tenant ids.
//
// covers: INV-EXPORT-01, INV-EXPORT-02, INV-EXPORT-03, INV-EXPORT-04
func TestExportBankAccount(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	aliceCtx := auth.WithUser(context.Background(), alice)
	bobCtx := auth.WithUser(context.Background(), bob)

	assets := repo.NewAssetRepo(tdb.Pool)
	tags := repo.NewTagRepo(tdb.Pool)

	t.Run("sole owner + tag + snapshots resolve", func(t *testing.T) {
		acct, err := assets.CreateBankAccount(aliceCtx, repo.CreateBankAccountParams{
			DisplayName:     "Main checking",
			OwnershipType:   "sole",
			SoleOwnerUserID: &alice.ID,
			NativeCurrency:  "IDR",
			BankName:        "TestBank",
			AccountNumber:   "1234567890",
			AccountType:     "savings",
		})
		if err != nil {
			t.Fatalf("CreateBankAccount: %v", err)
		}

		tag, err := tags.CreateTag(aliceCtx, "Emergency fund", "#22c55e")
		if err != nil {
			t.Fatalf("CreateTag: %v", err)
		}
		if err := tags.AssignTag(aliceCtx, repo.TagGroupAsset, acct.Asset.ID, &tag.ID); err != nil {
			t.Fatalf("AssignTag: %v", err)
		}

		if _, err := assets.CreateAssetSnapshot(aliceCtx, repo.CreateAssetSnapshotParams{
			AssetID:   acct.Asset.ID,
			YearMonth: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Amount:    decimal.RequireFromString("10000000"),
			Currency:  "IDR",
		}); err != nil {
			t.Fatalf("CreateAssetSnapshot: %v", err)
		}

		out, err := assets.ExportBankAccount(aliceCtx, acct.Asset.ID)
		if err != nil {
			t.Fatalf("ExportBankAccount: %v", err)
		}
		if out.OwnerEmail != alice.Email {
			t.Errorf("OwnerEmail = %q, want %q", out.OwnerEmail, alice.Email)
		}
		if out.TagName != "Emergency fund" {
			t.Errorf("TagName = %q, want Emergency fund", out.TagName)
		}
		if len(out.Snapshots) != 1 {
			t.Fatalf("want 1 snapshot, got %d", len(out.Snapshots))
		}
		if out.Account.Details.BankName != "TestBank" {
			t.Errorf("Details.BankName = %q, want TestBank", out.Account.Details.BankName)
		}
	})

	t.Run("joint account leaves owner + tag blank", func(t *testing.T) {
		acct, err := assets.CreateBankAccount(aliceCtx, repo.CreateBankAccountParams{
			DisplayName:    "Joint savings",
			OwnershipType:  "joint",
			NativeCurrency: "IDR",
			BankName:       "TestBank",
			AccountNumber:  "222",
			AccountType:    "savings",
		})
		if err != nil {
			t.Fatalf("CreateBankAccount: %v", err)
		}
		out, err := assets.ExportBankAccount(aliceCtx, acct.Asset.ID)
		if err != nil {
			t.Fatalf("ExportBankAccount: %v", err)
		}
		if out.OwnerEmail != "" || out.TagName != "" {
			t.Errorf("joint untagged: OwnerEmail=%q TagName=%q, want both empty", out.OwnerEmail, out.TagName)
		}
		if len(out.Snapshots) != 0 {
			t.Errorf("want 0 snapshots, got %d", len(out.Snapshots))
		}
	})

	t.Run("unknown id is ErrNotFound", func(t *testing.T) {
		if _, err := assets.ExportBankAccount(aliceCtx, uuid.New()); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("unknown id: got %v, want ErrNotFound", err)
		}
	})

	t.Run("cross-tenant is ErrNotFound", func(t *testing.T) {
		acct, err := assets.CreateBankAccount(aliceCtx, repo.CreateBankAccountParams{
			DisplayName:    "Alice only",
			OwnershipType:  "joint",
			NativeCurrency: "IDR",
			BankName:       "TestBank",
			AccountNumber:  "333",
			AccountType:    "savings",
		})
		if err != nil {
			t.Fatalf("CreateBankAccount: %v", err)
		}
		if _, err := assets.ExportBankAccount(bobCtx, acct.Asset.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("cross-tenant: got %v, want ErrNotFound", err)
		}
	})
}
