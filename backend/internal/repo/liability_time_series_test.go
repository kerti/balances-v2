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

// TestLiabilityRepo_TimeSeries drives the Liabilities Home value-series
// endpoint (epic #204) against a real DB: the value series sourced from
// snapshots, the empty-household short-circuit, and cross-household isolation
// (the loader fans a batch snapshot query over household-scoped liability ids,
// so Bob must never observe Alice's series).
// covers: INV-TENANCY-04
func TestLiabilityRepo_TimeSeries(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	if alice.HouseholdID == bob.HouseholdID {
		t.Fatalf("fixture: alice and bob ended up in the same household")
	}
	aliceCtx := auth.WithUser(context.Background(), alice)
	bobCtx := auth.WithUser(context.Background(), bob)
	r := repo.NewLiabilityRepo(tdb.Pool)

	// Bob's household is empty → short-circuit returns an empty slice.
	bobEmpty, err := r.LiabilityTimeSeries(bobCtx)
	if err != nil {
		t.Fatalf("bob LiabilityTimeSeries (empty): %v", err)
	}
	if len(bobEmpty) != 0 {
		t.Fatalf("bob saw %d series in an empty household; want 0", len(bobEmpty))
	}

	liab, err := r.CreateLiability(aliceCtx, repo.CreateLiabilityParams{
		DisplayName:      "Alice KPR",
		Subtype:          "institutional",
		OwnershipType:    "joint",
		NativeCurrency:   "IDR",
		CounterpartyName: "Bank BCA",
	})
	if err != nil {
		t.Fatalf("alice CreateLiability: %v", err)
	}
	for _, s := range []struct {
		ym     time.Month
		amount int64
	}{{time.January, 1_400_000_000}, {time.February, 1_390_000_000}} {
		if _, err := r.CreateLiabilitySnapshot(aliceCtx, repo.CreateLiabilitySnapshotParams{
			LiabilityID: liab.ID,
			YearMonth:   time.Date(2026, s.ym, 1, 0, 0, 0, 0, time.UTC),
			Amount:      decimal.NewFromInt(s.amount),
			Currency:    "IDR",
		}); err != nil {
			t.Fatalf("alice CreateLiabilitySnapshot %v: %v", s.ym, err)
		}
	}

	aliceSeries, err := r.LiabilityTimeSeries(aliceCtx)
	if err != nil {
		t.Fatalf("alice LiabilityTimeSeries: %v", err)
	}
	if len(aliceSeries) != 1 {
		t.Fatalf("alice series count: want 1, got %d", len(aliceSeries))
	}
	if got := aliceSeries[0].LiabilityID; got != liab.ID {
		t.Errorf("series liability id: want %s, got %s", liab.ID, got)
	}
	if n := len(aliceSeries[0].ValueSeries); n != 2 {
		t.Fatalf("value series length: want 2, got %d", n)
	}
	if !decimal.NewFromInt(1_400_000_000).Equal(aliceSeries[0].ValueSeries[0].Amount) {
		t.Errorf("value[0]: want 1400000000, got %s", aliceSeries[0].ValueSeries[0].Amount)
	}

	// Bob still observes nothing of Alice's after she has data.
	bobSeries, err := r.LiabilityTimeSeries(bobCtx)
	if err != nil {
		t.Fatalf("bob LiabilityTimeSeries: %v", err)
	}
	if len(bobSeries) != 0 {
		t.Errorf("bob saw %d of alice's series; want 0 (tenancy leak)", len(bobSeries))
	}
}
