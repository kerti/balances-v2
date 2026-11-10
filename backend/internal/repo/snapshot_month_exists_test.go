package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// TestAssetSnapshot_CreateDuplicateMonthIsInformative is the regression guard
// for #395: creating a second live snapshot for a month that already has one
// used to fall through the repo's generic error path (raw pgconn unique-
// violation -> "create asset snapshot: %w" -> CodeInternal 500, "Server
// error. Please try again."). It must instead map to the dedicated
// ErrSnapshotMonthExists sentinel (409, SNAPSHOT_MONTH_EXISTS), which the FE
// catalog renders as an actionable "edit or delete the existing one" message.
// This is distinct from TestAssetSnapshot_DeleteThenRecreateSameMonth, which
// covers the same unique index but for a *soft-deleted* prior row.
//
// covers: INV-SNAPSHOTS-02
func TestAssetSnapshot_CreateDuplicateMonthIsInformative(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)
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

	if _, err := mk(1_000_000); err != nil {
		t.Fatalf("first create for May: %v", err)
	}
	if _, err := mk(2_000_000); !errors.Is(err, repo.ErrSnapshotMonthExists) {
		t.Fatalf("duplicate create for May: want ErrSnapshotMonthExists, got %v", err)
	}
}

// covers: INV-SNAPSHOTS-02
func TestLiabilitySnapshot_CreateDuplicateMonthIsInformative(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)
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
	mk := func(amount int64) (*db.LiabilitySnapshot, error) {
		return r.CreateLiabilitySnapshot(ctx, repo.CreateLiabilitySnapshotParams{
			LiabilityID: liability.ID,
			YearMonth:   may,
			Amount:      decimal.NewFromInt(amount),
			Currency:    "IDR",
		})
	}

	if _, err := mk(1_000_000); err != nil {
		t.Fatalf("first create for May: %v", err)
	}
	if _, err := mk(2_000_000); !errors.Is(err, repo.ErrSnapshotMonthExists) {
		t.Fatalf("duplicate create for May: want ErrSnapshotMonthExists, got %v", err)
	}
}

// covers: INV-SNAPSHOTS-02
func TestReceivableSnapshot_CreateDuplicateMonthIsInformative(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)
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
	mk := func(amount int64) (*db.ReceivableSnapshot, error) {
		return r.CreateReceivableSnapshot(ctx, repo.CreateReceivableSnapshotParams{
			ReceivableID: receivable.ID,
			YearMonth:    may,
			Amount:       decimal.NewFromInt(amount),
			Currency:     "IDR",
		})
	}

	if _, err := mk(1_000_000); err != nil {
		t.Fatalf("first create for May: %v", err)
	}
	if _, err := mk(2_000_000); !errors.Is(err, repo.ErrSnapshotMonthExists) {
		t.Fatalf("duplicate create for May: want ErrSnapshotMonthExists, got %v", err)
	}
}

// covers: INV-SNAPSHOTS-02
func TestInvestmentSnapshot_CreateDuplicateMonthIsInformative(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)
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
	qty := decimal.NewFromInt(100)
	price := decimal.NewFromInt(9_500)
	mk := func(amount int64) (*db.InvestmentSnapshot, error) {
		return r.CreateInvestmentSnapshot(ctx, repo.CreateInvestmentSnapshotParams{
			InvestmentID: stock.Investment.ID,
			YearMonth:    may,
			Amount:       decimal.NewFromInt(amount),
			Currency:     "IDR",
			Quantity:     &qty,
			PricePerUnit: &price,
		})
	}

	if _, err := mk(950_000); err != nil {
		t.Fatalf("first create for May: %v", err)
	}
	if _, err := mk(1_000_000); !errors.Is(err, repo.ErrSnapshotMonthExists) {
		t.Fatalf("duplicate create for May: want ErrSnapshotMonthExists, got %v", err)
	}
}

// TestAssetSnapshot_ReplaceThenCreateStillRefused pins the Record snapshot
// contract (#788, ADR-0046 amendment, ADR-0058): a month that already has a value
// is replaced through the update path (the earlier row archived, one live row
// left), and a plain create for that month is still refused with
// ErrSnapshotMonthExists. POST is only a guard against a stale tab or a double
// submit; it never produces a second live snapshot, before or after a replace.
//
// covers: INV-SNAPSHOTS-02, INV-SNAPSHOTS-11
func TestAssetSnapshot_ReplaceThenCreateStillRefused(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	user := testutil.CreateHouseholdWithUser(t, q, "Alice")
	ctx := identity.WithUser(context.Background(), user)
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
	mk := func(amount string) (*db.AssetSnapshot, error) {
		return r.CreateAssetSnapshot(ctx, repo.CreateAssetSnapshotParams{
			AssetID:   account.Asset.ID,
			YearMonth: may,
			Amount:    decimal.RequireFromString(amount),
			Currency:  "IDR",
		})
	}

	first, err := mk("1000000.5000")
	if err != nil {
		t.Fatalf("first create for May: %v", err)
	}

	// Replace May through the update path, as the Record snapshot dialog does.
	replaced, err := r.UpdateAssetSnapshot(ctx, repo.UpdateAssetSnapshotParams{
		SnapshotID: first.ID,
		Amount:     decimal.RequireFromString("2000000.2500"),
		Currency:   "IDR",
	})
	if err != nil {
		t.Fatalf("UpdateAssetSnapshot: %v", err)
	}
	if replaced.ReplacedID == nil || *replaced.ReplacedID != first.ID {
		t.Fatalf("replaced_id = %v, want %s", replaced.ReplacedID, first.ID)
	}

	// The guard still answers for a create against the replaced month.
	if _, err := mk("3000000"); !errors.Is(err, repo.ErrSnapshotMonthExists) {
		t.Fatalf("create after replace: want ErrSnapshotMonthExists, got %v", err)
	}

	// Exactly one live row for May, holding the replacing value exactly.
	live, err := r.ListAssetSnapshots(ctx, account.Asset.ID)
	if err != nil {
		t.Fatalf("ListAssetSnapshots: %v", err)
	}
	if len(live) != 1 {
		t.Fatalf("live snapshots = %d, want 1", len(live))
	}
	if live[0].ID != replaced.ID || !live[0].Amount.Equal(decimal.RequireFromString("2000000.25")) {
		t.Fatalf("live = %s %s, want %s 2000000.25", live[0].ID, live[0].Amount, replaced.ID)
	}

	// The displaced value is archived, not gone (raw count, not an expiry-gated getter).
	var archived int
	if err := tdb.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM asset_snapshots WHERE asset_id = $1 AND deleted_at IS NOT NULL`,
		account.Asset.ID).Scan(&archived); err != nil {
		t.Fatalf("count archived: %v", err)
	}
	if archived != 1 {
		t.Fatalf("archived rows = %d, want 1", archived)
	}
}
