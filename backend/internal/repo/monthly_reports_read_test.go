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

// TestMonthlyReportRepo_ReadPaths covers the single-month read surface the
// dashboard uses: GetReport (refresh-then-fetch for an in-range month, and
// ErrNotFound for a month outside the snapshot range), plus ReportingCurrency.
// The full generation/staleness wiring is proven in TestMonthlyReportRepo.
func TestMonthlyReportRepo_ReadPaths(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	aliceCtx := auth.WithUser(context.Background(), alice)

	// One Jan-2026 snapshot puts Jan..current in range; 2020 stays out of range.
	acct := createAsset(t, q, alice.HouseholdID, &alice.ID, nil, "joint")
	_ = createAssetSnapshot(t, q, alice.HouseholdID, acct, ymUTC(2026, time.January), "100")

	r := repo.NewMonthlyReportRepo(tdb.Pool)

	t.Run("GetReport returns an in-range month", func(t *testing.T) {
		got, err := r.GetReport(aliceCtx, ymUTC(2026, time.January))
		if err != nil {
			t.Fatalf("GetReport: %v", err)
		}
		if got == nil {
			t.Fatal("GetReport returned nil report")
		}
		if got.YearMonth.Year() != 2026 || got.YearMonth.Month() != time.January {
			t.Errorf("year_month = %s, want 2026-01", got.YearMonth.Format("2006-01"))
		}
		if !got.NwTotal.Equal(decimal.NewFromInt(100)) {
			t.Errorf("nw_total = %s, want 100", got.NwTotal)
		}
	})

	t.Run("GetReport out of range is ErrNotFound", func(t *testing.T) {
		if _, err := r.GetReport(aliceCtx, ymUTC(2020, time.January)); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("got %v, want ErrNotFound", err)
		}
	})

	t.Run("ReportingCurrency returns the household currency", func(t *testing.T) {
		cur, err := r.ReportingCurrency(aliceCtx)
		if err != nil {
			t.Fatalf("ReportingCurrency: %v", err)
		}
		if cur != "IDR" {
			t.Errorf("reporting currency = %q, want IDR", cur)
		}
	})
}

// TestMonthlyReportRepo_MonthRangeCoherence proves the materialized month set
// tracks the engine's [first..last] as the earliest input moves: deleting the
// earliest snapshot shrinks the range and the now-orphaned leading months are
// pruned (DeleteMonthlyReportsOutsideRange) rather than left dangling, and
// re-recording an earlier snapshot extends the range back. A prune that didn't
// fire would leave stale Jan/Feb rows a later GetReport could surface.
//
// covers: INV-STALENESS-02
func TestMonthlyReportRepo_MonthRangeCoherence(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	aliceCtx := auth.WithUser(context.Background(), alice)

	// Jan + Mar snapshots → engine materializes Jan..current (Feb carried).
	acct := createAsset(t, q, alice.HouseholdID, &alice.ID, nil, "joint")
	janSnap := createAssetSnapshot(t, q, alice.HouseholdID, acct, ymUTC(2026, time.January), "100")
	_ = createAssetSnapshot(t, q, alice.HouseholdID, acct, ymUTC(2026, time.March), "300")

	r := repo.NewMonthlyReportRepo(tdb.Pool)

	hasMonth := func(rows []db.MonthlyReport, m time.Time) bool {
		for _, row := range rows {
			if row.YearMonth.Year() == m.Year() && row.YearMonth.Month() == m.Month() {
				return true
			}
		}
		return false
	}

	// Baseline: Jan is in range.
	rows, err := r.ListReports(aliceCtx)
	if err != nil {
		t.Fatalf("ListReports baseline: %v", err)
	}
	if !hasMonth(rows, ymUTC(2026, time.January)) {
		t.Fatalf("baseline missing Jan; range did not start at the earliest snapshot")
	}

	t.Run("deleting the earliest snapshot prunes the leading months", func(t *testing.T) {
		n, err := q.SoftDeleteAssetSnapshot(context.Background(), db.SoftDeleteAssetSnapshotParams{
			ID: janSnap, HouseholdID: alice.HouseholdID, UpdatedBy: &alice.ID,
		})
		if err != nil {
			t.Fatalf("SoftDeleteAssetSnapshot: %v", err)
		}
		if n != 1 {
			t.Fatalf("SoftDeleteAssetSnapshot affected %d rows, want 1", n)
		}

		rows, err := r.ListReports(aliceCtx)
		if err != nil {
			t.Fatalf("ListReports after delete: %v", err)
		}
		if hasMonth(rows, ymUTC(2026, time.January)) || hasMonth(rows, ymUTC(2026, time.February)) {
			t.Errorf("Jan/Feb not pruned after earliest snapshot deleted: %d rows", len(rows))
		}
		if !hasMonth(rows, ymUTC(2026, time.March)) {
			t.Errorf("Mar dropped; range should now start at the new earliest snapshot")
		}
	})

	t.Run("re-recording an earlier snapshot extends the range back", func(t *testing.T) {
		_ = createAssetSnapshot(t, q, alice.HouseholdID, acct, ymUTC(2026, time.January), "100")

		rows, err := r.ListReports(aliceCtx)
		if err != nil {
			t.Fatalf("ListReports after re-record: %v", err)
		}
		if !hasMonth(rows, ymUTC(2026, time.January)) {
			t.Errorf("Jan not restored; range did not extend back to the new earliest snapshot")
		}
	})
}
