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

// newBankAccount is a small fixture that creates a household bank account
// through the repo (so it carries a real bank_accounts row and is gathered by
// the report engine), used by the soft-delete tests below.
func newBankAccount(ctx context.Context, t *testing.T, r *repo.AssetRepo, name string) *repo.BankAccount {
	t.Helper()
	acct, err := r.CreateBankAccount(ctx, repo.CreateBankAccountParams{
		DisplayName:    name,
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		BankName:       "BCA",
		AccountNumber:  "111",
		AccountType:    "savings",
	})
	if err != nil {
		t.Fatalf("CreateBankAccount %q: %v", name, err)
	}
	return acct
}

// TestSoftDelete_Idempotent pins the write-path half of the zone: a delete
// stamps deleted_at and leaves the row in place, and because the SoftDelete*
// query itself carries `AND deleted_at IS NULL`, a second delete against the
// same id matches zero rows and returns ErrNotFound rather than hard-removing,
// double-stamping, or silently succeeding. Asserted at both the snapshot and
// the position level (the two SoftDelete shapes share this contract).
//
// covers: INV-SOFT-DELETE-01
func TestSoftDelete_Idempotent(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := auth.WithUser(context.Background(), user)
	r := repo.NewAssetRepo(tdb.Pool)

	t.Run("snapshot delete is idempotent", func(t *testing.T) {
		acct := newBankAccount(ctx, t, r, "Idempotent snap")
		snap, err := r.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{
			AssetID:   acct.Asset.ID,
			YearMonth: ymUTC(2026, time.January),
			Amount:    decimal.NewFromInt(100),
			Currency:  "IDR",
		})
		if err != nil {
			t.Fatalf("CreateAssetSnapshot: %v", err)
		}

		if err := r.DeleteAssetSnapshot(ctx, snap.ID); err != nil {
			t.Fatalf("first DeleteAssetSnapshot: %v", err)
		}
		if err := r.DeleteAssetSnapshot(ctx, snap.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("second DeleteAssetSnapshot: want ErrNotFound, got %v", err)
		}
	})

	t.Run("position delete is idempotent", func(t *testing.T) {
		acct := newBankAccount(ctx, t, r, "Idempotent acct")

		if err := r.DeleteBankAccount(ctx, acct.Asset.ID); err != nil {
			t.Fatalf("first DeleteBankAccount: %v", err)
		}
		if err := r.DeleteBankAccount(ctx, acct.Asset.ID); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("second DeleteBankAccount: want ErrNotFound, got %v", err)
		}
	})
}

// TestMonthlyReport_GatherExcludesDeletedSnapshot is the highest-stakes row in
// the zone: a soft-deleted snapshot must contribute zero to the next net-worth
// computation for its month. Two joint accounts each hold 100 in Jan-2026, so
// the report reads 200; deleting one snapshot and re-fetching (GetReport
// refreshes on the bumped staleness watermark) must drop the report to 100. A
// gather query that leaked the tombstone would keep the report at 200 — silently
// overstating net worth after the user removed the holding.
//
// covers: INV-SOFT-DELETE-03, INV-STALENESS-01
func TestMonthlyReport_GatherExcludesDeletedSnapshot(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := auth.WithUser(context.Background(), user)
	r := repo.NewAssetRepo(tdb.Pool)

	jan := ymUTC(2026, time.January)
	mkSnap := func(assetID uuid.UUID, amount int64) *db.AssetSnapshot {
		t.Helper()
		s, err := r.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{
			AssetID:   assetID,
			YearMonth: jan,
			Amount:    decimal.NewFromInt(amount),
			Currency:  "IDR",
		})
		if err != nil {
			t.Fatalf("CreateAssetSnapshot: %v", err)
		}
		return s
	}

	acctA := newBankAccount(ctx, t, r, "Keep")
	acctB := newBankAccount(ctx, t, r, "Delete")
	_ = mkSnap(acctA.Asset.ID, 100)
	snapB := mkSnap(acctB.Asset.ID, 100)

	mr := repo.NewMonthlyReportRepo(tdb.Pool)

	rep, err := mr.GetReport(ctx, jan)
	if err != nil {
		t.Fatalf("GetReport before delete: %v", err)
	}
	if !rep.NwTotal.Equal(decimal.NewFromInt(200)) {
		t.Fatalf("nw_total before delete = %s, want 200", rep.NwTotal)
	}

	if err := r.DeleteAssetSnapshot(ctx, snapB.ID); err != nil {
		t.Fatalf("DeleteAssetSnapshot: %v", err)
	}

	rep, err = mr.GetReport(ctx, jan)
	if err != nil {
		t.Fatalf("GetReport after delete: %v", err)
	}
	if !rep.NwTotal.Equal(decimal.NewFromInt(100)) {
		t.Errorf("nw_total after delete = %s, want 100 — deleted snapshot leaked into the gather", rep.NwTotal)
	}
}
