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

// TestAssetSnapshot_DeleteThenRecreateSameMonth is the regression guard for the
// #57 resolution: a misplaced snapshot is corrected by deleting it and recording
// a new one. That round-trip must be possible for the *same* month, which relies
// on the unique index over (asset_id, year_month) being partial
// (WHERE deleted_at IS NULL) so the soft-deleted row no longer collides. If that
// predicate ever regresses, the second create here fails the duplicate check and
// the "delete and re-record" UX breaks.
func TestAssetSnapshot_DeleteThenRecreateSameMonth(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := auth.WithUser(context.Background(), user)
	r := repo.NewAssetRepo(tdb.Pool)

	account, err := r.CreateBankAccount(ctx, repo.CreateBankAccountParams{
		DisplayName:    "Alice BCA",
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		BankName:       "BCA",
		AccountNumber:  "111",
		AccountType:    "savings",
	})
	if err != nil {
		t.Fatalf("CreateBankAccount: %v", err)
	}

	may := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)
	mk := func(amount int64) (*db.AssetSnapshot, error) {
		return r.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{
			AssetID:   account.Asset.ID,
			YearMonth: may,
			Amount:    decimal.NewFromInt(amount),
			Currency:  "IDR",
		})
	}

	first, err := mk(1_000_000)
	if err != nil {
		t.Fatalf("first create for May: %v", err)
	}
	if err := r.DeleteAssetSnapshot(ctx, first.ID); err != nil {
		t.Fatalf("soft-delete May snapshot: %v", err)
	}

	// Re-recording the same month must succeed now that the prior row is
	// soft-deleted — and produce a genuinely new row, not resurrect the old one.
	second, err := mk(2_000_000)
	if err != nil {
		t.Fatalf("re-create for May after delete: %v", err)
	}
	if second.ID == first.ID {
		t.Fatalf("re-create returned the deleted row's ID %s; want a fresh row", first.ID)
	}

	// And the live list for the asset shows exactly the new reading.
	snaps, err := r.ListAssetSnapshots(ctx, account.Asset.ID)
	if err != nil {
		t.Fatalf("ListAssetSnapshots: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("live snapshots for May: got %d, want 1", len(snaps))
	}
	if !snaps[0].Amount.Equal(decimal.NewFromInt(2_000_000)) {
		t.Fatalf("live snapshot amount = %s; want 2000000", snaps[0].Amount)
	}
}

// TestAssetSnapshot_AsOfDateInMonth verifies the asset_snapshots_as_of_in_month
// CHECK constraint (migration 00003) surfaces as ErrSnapshotDateOutsideMonth on
// both create and update, while in-month and NULL dates pass. The asset path is
// representative — the same suffix-matched constraint exists on all four
// snapshot tables and is mapped by the shared asOfMonthViolation helper.
func TestAssetSnapshot_AsOfDateInMonth(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := auth.WithUser(context.Background(), user)
	r := repo.NewAssetRepo(tdb.Pool)

	account, err := r.CreateBankAccount(ctx, repo.CreateBankAccountParams{
		DisplayName:    "Alice BCA",
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		BankName:       "BCA",
		AccountNumber:  "111",
		AccountType:    "savings",
	})
	if err != nil {
		t.Fatalf("CreateBankAccount: %v", err)
	}
	assetID := account.Asset.ID

	may := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)
	mid := time.Date(2026, time.May, 15, 0, 0, 0, 0, time.UTC)
	out := time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC)

	create := func(asOf *time.Time) (*db.AssetSnapshot, error) {
		return r.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{
			AssetID:   assetID,
			YearMonth: may,
			Amount:    decimal.NewFromInt(1_000_000),
			Currency:  "IDR",
			AsOfDate:  asOf,
		})
	}

	t.Run("create with in-month date succeeds", func(t *testing.T) {
		snap, err := create(&mid)
		if err != nil {
			t.Fatalf("create in-month: %v", err)
		}
		// Clean up so the next subtest's create doesn't hit the unique index.
		if err := r.DeleteAssetSnapshot(ctx, snap.ID); err != nil {
			t.Fatalf("cleanup delete: %v", err)
		}
	})

	t.Run("create with out-of-month date is rejected", func(t *testing.T) {
		_, err := create(&out)
		if !errors.Is(err, repo.ErrSnapshotDateOutsideMonth) {
			t.Fatalf("create out-of-month: want ErrSnapshotDateOutsideMonth, got %v", err)
		}
	})

	t.Run("create with nil date succeeds", func(t *testing.T) {
		snap, err := create(nil)
		if err != nil {
			t.Fatalf("create nil date: %v", err)
		}

		t.Run("update to out-of-month date is rejected", func(t *testing.T) {
			_, err := r.UpdateAssetSnapshot(ctx, repo.UpdateAssetSnapshotParams{
				SnapshotID: snap.ID,
				Amount:     decimal.NewFromInt(2_000_000),
				Currency:   "IDR",
				AsOfDate:   &out,
			})
			if !errors.Is(err, repo.ErrSnapshotDateOutsideMonth) {
				t.Fatalf("update out-of-month: want ErrSnapshotDateOutsideMonth, got %v", err)
			}
		})

		t.Run("update to in-month date succeeds", func(t *testing.T) {
			_, err := r.UpdateAssetSnapshot(ctx, repo.UpdateAssetSnapshotParams{
				SnapshotID: snap.ID,
				Amount:     decimal.NewFromInt(2_000_000),
				Currency:   "IDR",
				AsOfDate:   &mid,
			})
			if err != nil {
				t.Fatalf("update in-month: %v", err)
			}
		})
	})
}

// The CHECK is added to all four snapshot tables by the same migration, but a
// constraint is a per-table object — a renumbered or fat-fingered migration
// could create it on some tables and not others, and the suffix-matching
// helper would still happily map whichever ones fire. These three guard that
// each remaining table actually carries the constraint and that its repo maps
// the violation to ErrSnapshotDateOutsideMonth on create *and* update. Asset is
// covered above in full; here we assert the create-reject, create-accept, and
// update-reject branches that are the unique per-table mapping sites.

func TestLiabilitySnapshot_AsOfDateInMonth(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := auth.WithUser(context.Background(), user)
	r := repo.NewLiabilityRepo(tdb.Pool)

	liability, err := r.CreateLiability(ctx, repo.CreateLiabilityParams{
		DisplayName:      "Alice KPR",
		Subtype:          "institutional",
		OwnershipType:    "joint",
		NativeCurrency:   "IDR",
		CounterpartyName: "Bank BCA",
	})
	if err != nil {
		t.Fatalf("CreateLiability: %v", err)
	}

	may := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)
	mid := time.Date(2026, time.May, 15, 0, 0, 0, 0, time.UTC)
	out := time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC)

	create := func(asOf *time.Time) (*db.LiabilitySnapshot, error) {
		return r.CreateLiabilitySnapshot(ctx, repo.CreateLiabilitySnapshotParams{
			LiabilityID: liability.ID,
			YearMonth:   may,
			Amount:      decimal.NewFromInt(1_000_000),
			Currency:    "IDR",
			AsOfDate:    asOf,
		})
	}

	if _, err := create(&out); !errors.Is(err, repo.ErrSnapshotDateOutsideMonth) {
		t.Fatalf("create out-of-month: want ErrSnapshotDateOutsideMonth, got %v", err)
	}

	snap, err := create(&mid)
	if err != nil {
		t.Fatalf("create in-month: %v", err)
	}

	if _, err := r.UpdateLiabilitySnapshot(ctx, repo.UpdateLiabilitySnapshotParams{
		SnapshotID: snap.ID,
		Amount:     decimal.NewFromInt(2_000_000),
		Currency:   "IDR",
		AsOfDate:   &out,
	}); !errors.Is(err, repo.ErrSnapshotDateOutsideMonth) {
		t.Fatalf("update out-of-month: want ErrSnapshotDateOutsideMonth, got %v", err)
	}
}

func TestReceivableSnapshot_AsOfDateInMonth(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := auth.WithUser(context.Background(), user)
	r := repo.NewReceivableRepo(tdb.Pool)

	receivable, err := r.CreateReceivable(ctx, repo.CreateReceivableParams{
		DisplayName:      "Loan to brother",
		OwnershipType:    "joint",
		NativeCurrency:   "IDR",
		CounterpartyName: "Brother",
	})
	if err != nil {
		t.Fatalf("CreateReceivable: %v", err)
	}

	may := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)
	mid := time.Date(2026, time.May, 15, 0, 0, 0, 0, time.UTC)
	out := time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC)

	create := func(asOf *time.Time) (*db.ReceivableSnapshot, error) {
		return r.CreateReceivableSnapshot(ctx, repo.CreateReceivableSnapshotParams{
			ReceivableID: receivable.ID,
			YearMonth:    may,
			Amount:       decimal.NewFromInt(1_000_000),
			Currency:     "IDR",
			AsOfDate:     asOf,
		})
	}

	if _, err := create(&out); !errors.Is(err, repo.ErrSnapshotDateOutsideMonth) {
		t.Fatalf("create out-of-month: want ErrSnapshotDateOutsideMonth, got %v", err)
	}

	snap, err := create(&mid)
	if err != nil {
		t.Fatalf("create in-month: %v", err)
	}

	if _, err := r.UpdateReceivableSnapshot(ctx, repo.UpdateReceivableSnapshotParams{
		SnapshotID: snap.ID,
		Amount:     decimal.NewFromInt(2_000_000),
		Currency:   "IDR",
		AsOfDate:   &out,
	}); !errors.Is(err, repo.ErrSnapshotDateOutsideMonth) {
		t.Fatalf("update out-of-month: want ErrSnapshotDateOutsideMonth, got %v", err)
	}
}

func TestInvestmentSnapshot_AsOfDateInMonth(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := auth.WithUser(context.Background(), user)
	r := repo.NewInvestmentRepo(tdb.Pool)

	stock, err := r.CreateStock(ctx, repo.CreateStockParams{
		DisplayName:    "Alice BBCA",
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		RiskProfile:    "medium",
		Ticker:         "BBCA",
		Exchange:       "IDX",
	})
	if err != nil {
		t.Fatalf("CreateStock: %v", err)
	}

	may := time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)
	mid := time.Date(2026, time.May, 15, 0, 0, 0, 0, time.UTC)
	out := time.Date(2026, time.April, 30, 0, 0, 0, 0, time.UTC)
	qty := decimal.NewFromInt(100)
	price := decimal.NewFromInt(9_500)

	create := func(asOf *time.Time) (*db.InvestmentSnapshot, error) {
		return r.CreateInvestmentSnapshot(ctx, repo.CreateInvestmentSnapshotParams{
			InvestmentID: stock.Investment.ID,
			YearMonth:    may,
			Amount:       decimal.NewFromInt(950_000),
			Currency:     "IDR",
			Quantity:     &qty,
			PricePerUnit: &price,
			AsOfDate:     asOf,
		})
	}

	if _, err := create(&out); !errors.Is(err, repo.ErrSnapshotDateOutsideMonth) {
		t.Fatalf("create out-of-month: want ErrSnapshotDateOutsideMonth, got %v", err)
	}

	snap, err := create(&mid)
	if err != nil {
		t.Fatalf("create in-month: %v", err)
	}

	if _, err := r.UpdateInvestmentSnapshot(ctx, repo.UpdateInvestmentSnapshotParams{
		SnapshotID:   snap.ID,
		Amount:       decimal.NewFromInt(1_000_000),
		Currency:     "IDR",
		Quantity:     &qty,
		PricePerUnit: &price,
		AsOfDate:     &out,
	}); !errors.Is(err, repo.ErrSnapshotDateOutsideMonth) {
		t.Fatalf("update out-of-month: want ErrSnapshotDateOutsideMonth, got %v", err)
	}
}
