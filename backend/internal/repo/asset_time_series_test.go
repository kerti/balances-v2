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

// TestAssetRepo_TimeSeries drives the Assets Home value-series endpoint (epic
// #204) against a real DB: the value series sourced from snapshots, the
// empty-household short-circuit, and — critically — cross-household isolation,
// since the loader fans a batch snapshot query over household-scoped asset ids.
// Bob must never observe Alice's asset series.
// covers: INV-TENANCY-01
func TestAssetRepo_TimeSeries(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	if alice.HouseholdID == bob.HouseholdID {
		t.Fatalf("fixture: alice and bob ended up in the same household")
	}
	aliceCtx := auth.WithUser(context.Background(), alice)
	bobCtx := auth.WithUser(context.Background(), bob)
	r := repo.NewAssetRepo(tdb.Pool)

	// Bob's household is empty → short-circuit returns an empty slice.
	bobEmpty, err := r.AssetTimeSeries(bobCtx)
	if err != nil {
		t.Fatalf("bob AssetTimeSeries (empty): %v", err)
	}
	if len(bobEmpty) != 0 {
		t.Fatalf("bob saw %d series in an empty household; want 0", len(bobEmpty))
	}

	// Alice creates a bank account and two snapshots.
	account, err := r.CreateBankAccount(aliceCtx, repo.CreateBankAccountParams{
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
	for _, s := range []struct {
		ym     time.Month
		amount int64
	}{{time.January, 1_000_000}, {time.February, 1_500_000}} {
		if _, err := r.CreateAssetSnapshot(aliceCtx, repo.CreateAssetSnapshotParams{
			AssetID:   account.Asset.ID,
			YearMonth: time.Date(2026, s.ym, 1, 0, 0, 0, 0, time.UTC),
			Amount:    decimal.NewFromInt(s.amount),
			Currency:  "IDR",
		}); err != nil {
			t.Fatalf("alice CreateAssetSnapshot %v: %v", s.ym, err)
		}
	}

	// Alice sees her own two-point value series.
	aliceSeries, err := r.AssetTimeSeries(aliceCtx)
	if err != nil {
		t.Fatalf("alice AssetTimeSeries: %v", err)
	}
	if len(aliceSeries) != 1 {
		t.Fatalf("alice series count: want 1, got %d", len(aliceSeries))
	}
	if got := aliceSeries[0].AssetID; got != account.Asset.ID {
		t.Errorf("series asset id: want %s, got %s", account.Asset.ID, got)
	}
	if n := len(aliceSeries[0].ValueSeries); n != 2 {
		t.Fatalf("value series length: want 2, got %d", n)
	}
	if !decimal.NewFromInt(1_000_000).Equal(aliceSeries[0].ValueSeries[0].Amount) {
		t.Errorf("value[0]: want 1000000, got %s", aliceSeries[0].ValueSeries[0].Amount)
	}
	if !decimal.NewFromInt(1_500_000).Equal(aliceSeries[0].ValueSeries[1].Amount) {
		t.Errorf("value[1]: want 1500000, got %s", aliceSeries[0].ValueSeries[1].Amount)
	}

	// Bob still observes nothing of Alice's after she has data.
	bobSeries, err := r.AssetTimeSeries(bobCtx)
	if err != nil {
		t.Fatalf("bob AssetTimeSeries: %v", err)
	}
	if len(bobSeries) != 0 {
		t.Errorf("bob saw %d of alice's series; want 0 (tenancy leak)", len(bobSeries))
	}
}
