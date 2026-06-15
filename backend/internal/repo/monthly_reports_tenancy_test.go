package repo_test

import (
	"context"
	"encoding/json"
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

func ymUTC(y int, m time.Month) time.Time { return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC) }

func decPtr(d decimal.Decimal) *decimal.Decimal { return &d }

// End-to-end plumbing for the materialized report: q -> engine -> upsert ->
// read, plus tenancy isolation and the staleness watermark (no-regen when
// unchanged, regen on input edit). The rule coverage lives in the pure engine
// test; this proves the wiring and JSON round-trip.
// covers: INV-TENANCY-09, INV-STALENESS-01, INV-STALENESS-03
func TestMonthlyReportRepo(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	bob, err := q.CreateUser(context.Background(), db.CreateUserParams{
		HouseholdID: alice.HouseholdID, // Bob shares Alice's household
		DisplayName: "Bob", Email: "bob-hh@example.com", GoogleSub: "test-sub-bob-hh",
		Locale: "id-ID", TimeZone: "Asia/Jakarta", CreatedBy: &alice.ID,
	})
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}
	carol := testutil.CreateHouseholdWithUser(t, q, "Carol") // separate household

	aliceCtx := auth.WithUser(context.Background(), alice)
	carolCtx := auth.WithUser(context.Background(), carol)

	// Joint bank account: snapshots in Jan + Mar (Feb gap -> carry-forward).
	acct := createAsset(t, q, alice.HouseholdID, &alice.ID, nil, "joint")
	janSnap := createAssetSnapshot(t, q, alice.HouseholdID, acct, ymUTC(2026, time.January), "100")
	_ = createAssetSnapshot(t, q, alice.HouseholdID, acct, ymUTC(2026, time.March), "300")

	r := repo.NewMonthlyReportRepo(tdb.Pool)

	t.Run("generates months with carry-forward + breakdown", func(t *testing.T) {
		rows, err := r.ListReports(aliceCtx)
		if err != nil {
			t.Fatalf("ListReports: %v", err)
		}
		if len(rows) < 3 {
			t.Fatalf("got %d months, want >= 3 (Jan..current)", len(rows))
		}

		jan := mustMonth(t, rows, ymUTC(2026, time.January))
		if !jan.NwTotal.Equal(decimal.NewFromInt(100)) {
			t.Errorf("Jan nw_total: got %s, want 100", jan.NwTotal)
		}
		feb := mustMonth(t, rows, ymUTC(2026, time.February))
		if !feb.NwTotal.Equal(decimal.NewFromInt(100)) {
			t.Errorf("Feb nw_total (carry-forward): got %s, want 100", feb.NwTotal)
		}

		var stale []struct {
			PositionID uuid.UUID `json:"position_id"`
			Name       string    `json:"name"`
			Group      string    `json:"group"`
			Subtype    string    `json:"subtype"`
			LastMonth  time.Time `json:"last_month"`
		}
		if err := json.Unmarshal(feb.StalePositions, &stale); err != nil {
			t.Fatalf("unmarshal stale_positions: %v", err)
		}
		if len(stale) != 1 || stale[0].PositionID != acct {
			t.Errorf("Feb stale_positions: got %v, want [%s]", stale, acct)
		}
		// The drill-down payload (#50): label + route metadata + the month the
		// carried-forward snapshot was actually recorded (January).
		if s := stale[0]; s.Name != "Acct" || s.Group != "asset" || s.Subtype != "bank_account" ||
			!s.LastMonth.Equal(ymUTC(2026, time.January)) {
			t.Errorf("Feb stale_positions[0]: got %+v, want name=Acct group=asset subtype=bank_account lastMonth=Jan", s)
		}

		var bd map[string]struct {
			NW decimal.Decimal `json:"nw"`
		}
		if err := json.Unmarshal(jan.UserBreakdowns, &bd); err != nil {
			t.Fatalf("unmarshal user_breakdowns: %v", err)
		}
		if !bd[jointKeyJSON].NW.Equal(decimal.NewFromInt(100)) {
			t.Errorf("Jan joint breakdown: got %s, want 100", bd[jointKeyJSON].NW)
		}
		if _, ok := bd[alice.ID.String()]; !ok {
			t.Errorf("breakdown missing alice key")
		}
		if _, ok := bd[bob.ID.String()]; !ok {
			t.Errorf("breakdown missing bob key")
		}
	})

	t.Run("tenancy: carol sees no reports", func(t *testing.T) {
		rows, err := r.ListReports(carolCtx)
		if err != nil {
			t.Fatalf("ListReports carol: %v", err)
		}
		if len(rows) != 0 {
			t.Errorf("carol saw %d months; want 0", len(rows))
		}
	})

	t.Run("staleness: no regen when inputs unchanged", func(t *testing.T) {
		rows1, err := r.ListReports(aliceCtx)
		if err != nil {
			t.Fatalf("ListReports: %v", err)
		}
		before := mustMonth(t, rows1, ymUTC(2026, time.January)).GeneratedAt
		rows2, err := r.ListReports(aliceCtx)
		if err != nil {
			t.Fatalf("ListReports: %v", err)
		}
		after := mustMonth(t, rows2, ymUTC(2026, time.January)).GeneratedAt
		if !before.Time.Equal(after.Time) {
			t.Errorf("Jan regenerated without input change: %s -> %s", before.Time, after.Time)
		}
	})

	t.Run("staleness: snapshot edit triggers regen", func(t *testing.T) {
		rows1, err := r.ListReports(aliceCtx)
		if err != nil {
			t.Fatalf("ListReports: %v", err)
		}
		before := mustMonth(t, rows1, ymUTC(2026, time.January)).GeneratedAt

		if _, err := q.UpdateAssetSnapshot(context.Background(), db.UpdateAssetSnapshotParams{
			ID: janSnap, HouseholdID: alice.HouseholdID,
			Amount: decimal.NewFromInt(150), Currency: "IDR", UpdatedBy: &alice.ID,
		}); err != nil {
			t.Fatalf("UpdateAssetSnapshot: %v", err)
		}

		rows2, err := r.ListReports(aliceCtx)
		if err != nil {
			t.Fatalf("ListReports: %v", err)
		}
		jan := mustMonth(t, rows2, ymUTC(2026, time.January))
		if !jan.NwTotal.Equal(decimal.NewFromInt(150)) {
			t.Errorf("Jan nw_total after edit: got %s, want 150", jan.NwTotal)
		}
		if !jan.GeneratedAt.Time.After(before.Time) {
			t.Errorf("Jan not regenerated after snapshot edit")
		}
	})

	t.Run("rebuild all: force-regenerates ignoring staleness", func(t *testing.T) {
		rows1, err := r.ListReports(aliceCtx)
		if err != nil {
			t.Fatalf("ListReports: %v", err)
		}
		before := mustMonth(t, rows1, ymUTC(2026, time.January)).GeneratedAt

		if err := r.RebuildAll(aliceCtx); err != nil {
			t.Fatalf("RebuildAll: %v", err)
		}
		rows2, err := r.ListReports(aliceCtx)
		if err != nil {
			t.Fatalf("ListReports: %v", err)
		}
		jan := mustMonth(t, rows2, ymUTC(2026, time.January))
		// generated_at advances even though no input changed — proof the rebuild
		// ignores the staleness watermark.
		if !jan.GeneratedAt.Time.After(before.Time) {
			t.Errorf("RebuildAll did not bump Jan generated_at without input change")
		}
		// The recompute reproduces the same number (150 from the prior edit).
		if !jan.NwTotal.Equal(decimal.NewFromInt(150)) {
			t.Errorf("Jan nw_total after rebuild: got %s, want 150", jan.NwTotal)
		}
	})

	t.Run("rebuild month: rebuilds target only, neighbours untouched", func(t *testing.T) {
		rows1, err := r.ListReports(aliceCtx)
		if err != nil {
			t.Fatalf("ListReports: %v", err)
		}
		janBefore := mustMonth(t, rows1, ymUTC(2026, time.January)).GeneratedAt
		marBefore := mustMonth(t, rows1, ymUTC(2026, time.March)).GeneratedAt

		if err := r.RebuildMonth(aliceCtx, ymUTC(2026, time.January)); err != nil {
			t.Fatalf("RebuildMonth: %v", err)
		}
		rows2, err := r.ListReports(aliceCtx)
		if err != nil {
			t.Fatalf("ListReports: %v", err)
		}
		janAfter := mustMonth(t, rows2, ymUTC(2026, time.January)).GeneratedAt
		marAfter := mustMonth(t, rows2, ymUTC(2026, time.March)).GeneratedAt
		if !janAfter.Time.After(janBefore.Time) {
			t.Errorf("RebuildMonth did not bump Jan generated_at")
		}
		if !marAfter.Time.Equal(marBefore.Time) {
			t.Errorf("RebuildMonth touched March: %s -> %s", marBefore.Time, marAfter.Time)
		}
	})

	t.Run("rebuild month: out of range is not found", func(t *testing.T) {
		err := r.RebuildMonth(aliceCtx, ymUTC(2020, time.January))
		if !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("RebuildMonth out-of-range: got %v, want ErrNotFound", err)
		}
	})
}

// jointKeyJSON mirrors the engine's jointKey constant for the external test
// package (which can't see the unexported original).
const jointKeyJSON = "joint"

func createAsset(t *testing.T, q *db.Queries, hid uuid.UUID, createdBy, soleOwner *uuid.UUID, ownership string) uuid.UUID {
	t.Helper()
	a, err := q.CreateAsset(context.Background(), db.CreateAssetParams{
		HouseholdID: hid, DisplayName: "Acct", Subtype: "bank_account",
		OwnershipType: ownership, SoleOwnerUserID: soleOwner, NativeCurrency: "IDR", CreatedBy: createdBy,
	})
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	return a.ID
}

func createAssetSnapshot(t *testing.T, q *db.Queries, hid, assetID uuid.UUID, month time.Time, amount string) uuid.UUID {
	t.Helper()
	s, err := q.CreateAssetSnapshot(context.Background(), db.CreateAssetSnapshotParams{
		ID: assetID, HouseholdID: hid, YearMonth: month,
		Amount: decimal.RequireFromString(amount), Currency: "IDR", CreatedBy: nil,
	})
	if err != nil {
		t.Fatalf("CreateAssetSnapshot: %v", err)
	}
	return s.ID
}

func mustMonth(t *testing.T, rows []db.MonthlyReport, m time.Time) db.MonthlyReport {
	t.Helper()
	for _, row := range rows {
		if row.YearMonth.Year() == m.Year() && row.YearMonth.Month() == m.Month() {
			return row
		}
	}
	t.Fatalf("no report row for %s", m.Format("2006-01"))
	return db.MonthlyReport{}
}
