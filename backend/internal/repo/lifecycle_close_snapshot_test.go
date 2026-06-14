package repo_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/repo"
)

// TestUpdateInvestmentLifecycle_CloseSnapshotRoundTrip covers the manual
// terminate/un-terminate path of UpdateInvestmentLifecycle (ADR-0009) — the
// write-side guarantee the report engine assumes on read (INV-FINANCE-11/-13):
//
//   - a terminal flip (manual Sell, not a Maturity) upserts a truthful 0-value
//     close snapshot at the termination month, so the derived-return formula
//     books gain only and the position leaves no net-worth bubble; and
//   - the inverse correction (back to active) soft-deletes that close snapshot,
//     so the reactivated position carries forward its last real value, not 0.
//
// The Maturity entry point's close snapshot is covered separately by the
// import-create + maturity-edit tests; this pins the UpdateInvestmentLifecycle
// switch the handler suite leaves unasserted.
//
// covers: INV-LIFECYCLE-03, INV-LIFECYCLE-04
func TestUpdateInvestmentLifecycle_CloseSnapshotRoundTrip(t *testing.T) {
	r, ctx := investmentRepoFor(t)

	qty := decimal.RequireFromString("100")
	price := decimal.RequireFromString("9500")
	stock, err := r.CreateStockWithSnapshotsAndLedger(ctx, repo.CreateStockParams{
		DisplayName:    "Round-trip stock",
		OwnershipType:  "joint",
		NativeCurrency: "IDR",
		RiskProfile:    "medium",
		Ticker:         "BBCA",
		Exchange:       "IDX",
	}, nil, []repo.ImportInvestmentSnapshotRow{
		{YearMonth: ym(2026, time.January), Amount: qty.Mul(price), Currency: "IDR", Quantity: &qty, PricePerUnit: &price},
	}, nil)
	if err != nil {
		t.Fatalf("CreateStockWithSnapshotsAndLedger: %v", err)
	}
	id := stock.Investment.ID

	// ----- terminal flip writes the 0-value close at the termination month -----
	termDate := day(2026, time.March, 15)
	note := "sold to broker"
	if _, err := r.UpdateInvestmentLifecycle(ctx, id, repo.LifecycleParams{
		Status:          "sold",
		TerminatedAt:    &termDate,
		TerminationNote: &note,
	}); err != nil {
		t.Fatalf("terminate: %v", err)
	}

	snaps, err := r.ListInvestmentSnapshots(ctx, id)
	if err != nil {
		t.Fatalf("ListInvestmentSnapshots after terminate: %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("after terminate: got %d snapshots, want 2 (Jan value + Mar 0-close)", len(snaps))
	}
	closeYM := ym(2026, time.March)
	var foundClose bool
	for _, s := range snaps {
		if s.YearMonth.Equal(closeYM) {
			foundClose = true
			if !s.Amount.IsZero() {
				t.Errorf("close snapshot amount: got %s, want 0", s.Amount)
			}
		}
	}
	if !foundClose {
		t.Fatalf("no close snapshot written at %s", closeYM.Format("2006-01"))
	}

	// ----- un-terminate drops the close snapshot it wrote -----
	if _, err := r.UpdateInvestmentLifecycle(ctx, id, repo.LifecycleParams{
		Status:       "active",
		TerminatedAt: nil,
	}); err != nil {
		t.Fatalf("un-terminate: %v", err)
	}

	snaps, err = r.ListInvestmentSnapshots(ctx, id)
	if err != nil {
		t.Fatalf("ListInvestmentSnapshots after un-terminate: %v", err)
	}
	if len(snaps) != 1 {
		t.Fatalf("after un-terminate: got %d snapshots, want 1 (Jan value only; close dropped)", len(snaps))
	}
	if snaps[0].Amount.IsZero() || !snaps[0].YearMonth.Equal(ym(2026, time.January)) {
		t.Errorf("surviving snapshot: got %s @ %s, want non-zero @ 2026-01",
			snaps[0].Amount, snaps[0].YearMonth.Format("2006-01"))
	}
}
