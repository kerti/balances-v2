package repo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

func createUSDAsset(t *testing.T, q *db.Queries, hid uuid.UUID, subtype string) uuid.UUID {
	t.Helper()
	a, err := q.CreateAsset(context.Background(), db.CreateAssetParams{
		HouseholdID: hid, DisplayName: "USD " + subtype, Subtype: subtype,
		OwnershipType: "joint", NativeCurrency: "USD", EntryType: repo.EntryTypeAcquired,
	})
	if err != nil {
		t.Fatalf("CreateAsset USD: %v", err)
	}
	return a.ID
}

func createSnapshotIn(t *testing.T, q *db.Queries, hid, assetID uuid.UUID, month time.Time, amount, currency string) {
	t.Helper()
	if _, err := q.CreateAssetSnapshot(context.Background(), db.CreateAssetSnapshotParams{
		ID: assetID, HouseholdID: hid, YearMonth: month,
		Amount: decimal.RequireFromString(amount), Currency: currency,
	}); err != nil {
		t.Fatalf("CreateAssetSnapshot: %v", err)
	}
}

// The hub breakdown over a real household: its per-month total is the stored
// dashboard report's nw_assets (same engine pass), a month with no USD rate
// excludes the USD position and names it, a soft-deleted position drops out of
// both, another household's positions never appear, and an unknown group is a
// 404. covers: INV-FINANCE-39, INV-TENANCY-09
func TestMonthlyReportRepo_GroupBreakdown(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	ctx := context.Background()

	alice := testutil.CreateHouseholdWithUser(t, q, "Alice")
	carol := testutil.CreateHouseholdWithUser(t, q, "Carol")
	aliceCtx := identity.WithUser(ctx, alice)
	carolCtx := identity.WithUser(ctx, carol)

	if _, err := q.UpdateHouseholdSettings(ctx, db.UpdateHouseholdSettingsParams{
		ID: alice.HouseholdID, DisplayName: "Alice's Household", ReportingCurrency: "IDR",
		MultiCurrencyEnabled: true, AssumedAnnualInflation: decimal.Zero, UpdatedBy: &alice.ID,
	}); err != nil {
		t.Fatalf("enable multi-currency: %v", err)
	}

	jan, feb := ymUTC(2026, time.January), ymUTC(2026, time.February)

	idrAcct := createAsset(t, q, alice.HouseholdID, &alice.ID, nil, "joint")
	createAssetSnapshot(t, q, alice.HouseholdID, idrAcct, jan, "1000")
	usdAcct := createUSDAsset(t, q, alice.HouseholdID, "bank_account")
	createSnapshotIn(t, q, alice.HouseholdID, usdAcct, jan, "10", "USD")
	deleted := createAsset(t, q, alice.HouseholdID, &alice.ID, nil, "joint")
	createAssetSnapshot(t, q, alice.HouseholdID, deleted, jan, "999999")
	if n, err := q.SoftDeleteAsset(ctx, db.SoftDeleteAssetParams{ID: deleted, HouseholdID: alice.HouseholdID}); err != nil || n != 1 {
		t.Fatalf("soft-delete asset: n=%d err=%v", n, err)
	}

	// Carol's own (IDR) asset must never leak into Alice's breakdown.
	carolAcct := createAsset(t, q, carol.HouseholdID, &carol.ID, nil, "joint")
	createAssetSnapshot(t, q, carol.HouseholdID, carolAcct, jan, "777777")

	// USD's first rate is February; January has none.
	if _, err := q.CreateFxRate(ctx, db.CreateFxRateParams{
		HouseholdID: alice.HouseholdID, YearMonth: feb, Currency: "USD", Rate: decimal.NewFromInt(15000),
	}); err != nil {
		t.Fatalf("CreateFxRate: %v", err)
	}

	r := repo.NewMonthlyReportRepo(tdb.Pool)
	b, err := r.GroupBreakdown(aliceCtx, "asset")
	if err != nil {
		t.Fatalf("GroupBreakdown: %v", err)
	}
	if b.Group != "asset" || b.ReportingCurrency != "IDR" {
		t.Errorf("header: got %s/%s", b.Group, b.ReportingCurrency)
	}
	rows, err := r.ListReports(aliceCtx)
	if err != nil {
		t.Fatalf("ListReports: %v", err)
	}
	if len(b.Months) == 0 {
		t.Fatal("no breakdown months")
	}
	for _, m := range b.Months {
		row := mustMonth(t, rows, m.YearMonth)
		if !m.Total.Equal(row.NwAssets) {
			t.Errorf("%s: hub total %s != stored nw_assets %s", m.YearMonth.Format("2006-01"), m.Total, row.NwAssets)
		}
	}

	janM, febM := b.Months[0], b.Months[1]
	if !janM.YearMonth.Equal(jan) || !febM.YearMonth.Equal(feb) {
		t.Fatalf("months start %s,%s; want Jan,Feb", janM.YearMonth, febM.YearMonth)
	}
	// Jan: only the IDR account counts (the deleted row and Carol's are absent;
	// the USD one has no rate), and the USD one is named.
	if !janM.Total.Equal(decimal.NewFromInt(1000)) {
		t.Errorf("Jan total %s, want 1000", janM.Total)
	}
	if len(janM.MissingFx) != 1 || janM.MissingFx[0].PositionID == nil || *janM.MissingFx[0].PositionID != usdAcct {
		t.Errorf("Jan missing_fx %+v, want the USD account", janM.MissingFx)
	}
	// Feb: 1000 carried + 10 USD carried × 15000.
	if !febM.Total.Equal(decimal.NewFromInt(151000)) {
		t.Errorf("Feb total %s, want 151000", febM.Total)
	}
	if len(febM.MissingFx) != 0 {
		t.Errorf("Feb missing_fx %+v, want none", febM.MissingFx)
	}

	t.Run("tenancy: carol sees only her own", func(t *testing.T) {
		cb, err := r.GroupBreakdown(carolCtx, "asset")
		if err != nil {
			t.Fatalf("GroupBreakdown carol: %v", err)
		}
		if len(cb.Months) == 0 || !cb.Months[0].Total.Equal(decimal.NewFromInt(777777)) {
			t.Errorf("carol breakdown: %+v, want her 777777 only", cb.Months)
		}
	})

	t.Run("empty group", func(t *testing.T) {
		lb, err := r.GroupBreakdown(aliceCtx, "liability")
		if err != nil {
			t.Fatalf("GroupBreakdown liability: %v", err)
		}
		if len(lb.Months) != 0 {
			t.Errorf("liability months: %v, want none", lb.Months)
		}
	})

	t.Run("unknown group is not found", func(t *testing.T) {
		for _, g := range []string{"receivable", "bogus", ""} {
			if _, err := r.GroupBreakdown(aliceCtx, g); !errors.Is(err, repo.ErrNotFound) {
				t.Errorf("GroupBreakdown(%q): want ErrNotFound, got %v", g, err)
			}
		}
	})
}
