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

// TestReceivableRepo_TimeSeries drives the Receivables list total-over-time
// endpoint (epic #204) against a real DB: the value series sourced from
// snapshots, the empty-household short-circuit, and cross-household isolation
// (the loader fans a batch snapshot query over household-scoped receivable
// ids, so Bob must never observe Alice's series).
// covers: INV-TENANCY-05
func TestReceivableRepo_TimeSeries(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob := testutil.CreateHouseholdWithUser(t, q, "Bob")
	if alice.HouseholdID == bob.HouseholdID {
		t.Fatalf("fixture: alice and bob ended up in the same household")
	}
	aliceCtx := auth.WithUser(context.Background(), alice)
	bobCtx := auth.WithUser(context.Background(), bob)
	r := repo.NewReceivableRepo(tdb.Pool)

	// Bob's household is empty → short-circuit returns an empty slice.
	bobEmpty, err := r.ReceivableTimeSeries(bobCtx)
	if err != nil {
		t.Fatalf("bob ReceivableTimeSeries (empty): %v", err)
	}
	if len(bobEmpty) != 0 {
		t.Fatalf("bob saw %d series in an empty household; want 0", len(bobEmpty))
	}

	rec, err := r.CreateReceivable(aliceCtx, repo.CreateReceivableParams{
		DisplayName:      "Loan to brother",
		OwnershipType:    "joint",
		NativeCurrency:   "IDR",
		CounterpartyName: "Brother",
	})
	if err != nil {
		t.Fatalf("alice CreateReceivable: %v", err)
	}
	for _, s := range []struct {
		ym     time.Month
		amount int64
	}{{time.January, 50_000_000}, {time.February, 30_000_000}} {
		if _, err := r.CreateReceivableSnapshot(aliceCtx, repo.CreateReceivableSnapshotParams{
			ReceivableID: rec.ID,
			YearMonth:    time.Date(2026, s.ym, 1, 0, 0, 0, 0, time.UTC),
			Amount:       decimal.NewFromInt(s.amount),
			Currency:     "IDR",
		}); err != nil {
			t.Fatalf("alice CreateReceivableSnapshot %v: %v", s.ym, err)
		}
	}

	aliceSeries, err := r.ReceivableTimeSeries(aliceCtx)
	if err != nil {
		t.Fatalf("alice ReceivableTimeSeries: %v", err)
	}
	if len(aliceSeries) != 1 {
		t.Fatalf("alice series count: want 1, got %d", len(aliceSeries))
	}
	if got := aliceSeries[0].ReceivableID; got != rec.ID {
		t.Errorf("series receivable id: want %s, got %s", rec.ID, got)
	}
	if n := len(aliceSeries[0].ValueSeries); n != 2 {
		t.Fatalf("value series length: want 2, got %d", n)
	}
	if !decimal.NewFromInt(50_000_000).Equal(aliceSeries[0].ValueSeries[0].Amount) {
		t.Errorf("value[0]: want 50000000, got %s", aliceSeries[0].ValueSeries[0].Amount)
	}

	// Bob still observes nothing of Alice's after she has data.
	bobSeries, err := r.ReceivableTimeSeries(bobCtx)
	if err != nil {
		t.Fatalf("bob ReceivableTimeSeries: %v", err)
	}
	if len(bobSeries) != 0 {
		t.Errorf("bob saw %d of alice's series; want 0 (tenancy leak)", len(bobSeries))
	}
}
