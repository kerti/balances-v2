package repo_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// The Tags report over real households: one tag folds an IDR and a USD account
// into one converted figure, a liability stays separate, a position in a
// currency with no rate is excluded and named (Untagged), a soft-deleted
// position drops out, the net equals the stored dashboard nw_total for the very
// month served, and another household sees none of it.
// covers: INV-FINANCE-40, INV-TENANCY-12
func TestMonthlyReportRepo_TagBreakdown(t *testing.T) {
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

	tr := repo.NewTagRepo(tdb.Pool)
	r := repo.NewMonthlyReportRepo(tdb.Pool)
	aliceTag, err := tr.CreateTag(aliceCtx, "Savings", "#3b82f6")
	if err != nil {
		t.Fatalf("alice CreateTag: %v", err)
	}
	carolTag, err := tr.CreateTag(carolCtx, "Carol's", "#f59e0b")
	if err != nil {
		t.Fatalf("carol CreateTag: %v", err)
	}

	jan := ymUTC(2026, time.January)
	assign := func(c context.Context, g repo.TagGroup, id uuid.UUID, tag uuid.UUID) {
		t.Helper()
		if err := tr.AssignTag(c, g, id, &tag); err != nil {
			t.Fatalf("AssignTag: %v", err)
		}
	}

	idrAcct := createAsset(t, q, alice.HouseholdID, &alice.ID, nil, "joint")
	createAssetSnapshot(t, q, alice.HouseholdID, idrAcct, jan, "1000")
	assign(aliceCtx, repo.TagGroupAsset, idrAcct, aliceTag.ID)
	usdAcct := createUSDAsset(t, q, alice.HouseholdID, "bank_account")
	createSnapshotIn(t, q, alice.HouseholdID, usdAcct, jan, "10", "USD")
	assign(aliceCtx, repo.TagGroupAsset, usdAcct, aliceTag.ID)
	eurAcct := createUSDAsset(t, q, alice.HouseholdID, "bank_account") // untagged, no EUR rate
	createSnapshotIn(t, q, alice.HouseholdID, eurAcct, jan, "5", "EUR")

	debt, err := q.CreateLiability(ctx, db.CreateLiabilityParams{
		HouseholdID: alice.HouseholdID, DisplayName: "Loan", Subtype: "personal", OwnershipType: "joint",
		NativeCurrency: "IDR", CounterpartyName: "Bank", EntryType: repo.EntryTypeAcquired,
	})
	if err != nil {
		t.Fatalf("CreateLiability: %v", err)
	}
	if _, err := q.CreateLiabilitySnapshot(ctx, db.CreateLiabilitySnapshotParams{
		ID: debt.ID, HouseholdID: alice.HouseholdID, YearMonth: jan, Amount: decimal.NewFromInt(300), Currency: "IDR",
	}); err != nil {
		t.Fatalf("CreateLiabilitySnapshot: %v", err)
	}
	assign(aliceCtx, repo.TagGroupLiability, debt.ID, aliceTag.ID)

	deleted := createAsset(t, q, alice.HouseholdID, &alice.ID, nil, "joint")
	createAssetSnapshot(t, q, alice.HouseholdID, deleted, jan, "999999")
	assign(aliceCtx, repo.TagGroupAsset, deleted, aliceTag.ID)
	if n, err := q.SoftDeleteAsset(ctx, db.SoftDeleteAssetParams{ID: deleted, HouseholdID: alice.HouseholdID}); err != nil || n != 1 {
		t.Fatalf("soft-delete asset: n=%d err=%v", n, err)
	}

	carolAcct := createAsset(t, q, carol.HouseholdID, &carol.ID, nil, "joint")
	createAssetSnapshot(t, q, carol.HouseholdID, carolAcct, jan, "777777")
	assign(carolCtx, repo.TagGroupAsset, carolAcct, carolTag.ID)

	if _, err := q.CreateFxRate(ctx, db.CreateFxRateParams{
		HouseholdID: alice.HouseholdID, YearMonth: jan, Currency: "USD", Rate: decimal.NewFromInt(15000),
	}); err != nil {
		t.Fatalf("CreateFxRate: %v", err)
	}

	bd, err := r.TagBreakdown(aliceCtx)
	if err != nil {
		t.Fatalf("TagBreakdown: %v", err)
	}
	if bd.ReportingCurrency != "IDR" || bd.YearMonth == nil {
		t.Fatalf("header: %s / %v", bd.ReportingCurrency, bd.YearMonth)
	}
	if len(bd.Tags) != 1 || bd.Tags[0].TagID == nil || *bd.Tags[0].TagID != aliceTag.ID {
		t.Fatalf("tags %+v, want only alice's tag (Untagged EUR is unconvertible; deleted asset is gone)", bd.Tags)
	}
	// 1000 IDR + 10 USD × 15000 (the Jan rate carried forward to the served month).
	if e := bd.Tags[0]; !e.Holdings.Equal(decimal.NewFromInt(151000)) || !e.Liabilities.Equal(decimal.NewFromInt(300)) {
		t.Errorf("entry %+v, want holdings 151000 / liabilities 300", e)
	}
	if len(bd.MissingFx) != 1 || bd.MissingFx[0].PositionID != eurAcct || bd.MissingFx[0].Currency != "EUR" || bd.MissingFx[0].TagID != nil {
		t.Errorf("missing_fx %+v, want the untagged EUR account", bd.MissingFx)
	}
	if got := bd.FxRatesUsed["USD"]; !got.Equal(decimal.NewFromInt(15000)) {
		t.Errorf("fx_rates_used %v, want USD=15000", bd.FxRatesUsed)
	}

	// The very month served carries the same net worth in the stored report.
	rows, err := r.ListReports(aliceCtx)
	if err != nil {
		t.Fatalf("ListReports: %v", err)
	}
	last := rows[len(rows)-1]
	if !last.YearMonth.Equal(*bd.YearMonth) {
		t.Errorf("served month %s, dashboard default %s", bd.YearMonth.Format("2006-01"), last.YearMonth.Format("2006-01"))
	}
	net := decimal.Zero
	for _, e := range bd.Tags {
		net = net.Add(e.Holdings).Sub(e.Liabilities)
	}
	if !net.Equal(last.NwTotal) || !net.Equal(decimal.NewFromInt(150700)) {
		t.Errorf("tags net %s, stored nw_total %s, want both 150700", net, last.NwTotal)
	}

	t.Run("tenancy: carol sees only her own tag and figures", func(t *testing.T) {
		cb, err := r.TagBreakdown(carolCtx)
		if err != nil {
			t.Fatalf("TagBreakdown carol: %v", err)
		}
		if len(cb.Tags) != 1 || cb.Tags[0].TagID == nil || *cb.Tags[0].TagID != carolTag.ID ||
			!cb.Tags[0].Holdings.Equal(decimal.NewFromInt(777777)) || !cb.Tags[0].Liabilities.IsZero() {
			t.Errorf("carol tags %+v, want only her tag with 777777", cb.Tags)
		}
		if len(cb.MissingFx) != 0 || len(cb.FxRatesUsed) != 0 {
			t.Errorf("carol saw alice's warnings/rates: %+v / %v", cb.MissingFx, cb.FxRatesUsed)
		}
	})

	t.Run("a household with no data gets an empty report", func(t *testing.T) {
		dave := testutil.CreateHouseholdWithUser(t, q, "Dave")
		db0, err := r.TagBreakdown(identity.WithUser(ctx, dave))
		if err != nil {
			t.Fatalf("TagBreakdown dave: %v", err)
		}
		if db0.YearMonth != nil || len(db0.Tags) != 0 {
			t.Errorf("dave: %+v, want empty", db0)
		}
	})

	t.Run("deleting the tag moves its positions to Untagged", func(t *testing.T) {
		if err := tr.DeleteTag(aliceCtx, aliceTag.ID); err != nil {
			t.Fatalf("DeleteTag: %v", err)
		}
		after, err := r.TagBreakdown(aliceCtx)
		if err != nil {
			t.Fatalf("TagBreakdown after delete: %v", err)
		}
		if len(after.Tags) != 1 || after.Tags[0].TagID != nil ||
			!after.Tags[0].Holdings.Equal(decimal.NewFromInt(151000)) || !after.Tags[0].Liabilities.Equal(decimal.NewFromInt(300)) {
			t.Errorf("after delete %+v, want everything under Untagged", after.Tags)
		}
	})
}
