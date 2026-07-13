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

// TestMonthlyReportRepo_PDFInputReads covers the three household-scoped read
// methods that feed the PDF export / financial-statistics panel but that the
// dashboard read paths never touch: ReportInflation (inflation series + the
// household's assumed_annual_inflation fallback, ADR-0048), Members (owner-label
// resolution, ADR-0045), and GetPositionDetail (the itemized breakdown behind the
// export, ADR-0045).
func TestMonthlyReportRepo_PDFInputReads(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)

	alice := testutil.CreateHouseholdWithUser(t, q, "AlicePDF")
	aliceCtx := identity.WithUser(context.Background(), alice)

	// One Jan-2026 bank snapshot puts Jan..current in range and gives the itemized
	// breakdown a position to resolve.
	acct := createAsset(t, q, alice.HouseholdID, &alice.ID, nil, "joint")
	_ = createAssetSnapshot(t, q, alice.HouseholdID, acct, ymUTC(2026, time.January), "100")

	r := repo.NewMonthlyReportRepo(tdb.Pool)

	t.Run("ReportInflation returns the series + assumed setting", func(t *testing.T) {
		// Move the household's assumed rate off its 3.5 default so the read is
		// exercised against a stored value, and add one monthly figure.
		if _, err := q.UpdateHouseholdSettings(context.Background(), db.UpdateHouseholdSettingsParams{
			ID: alice.HouseholdID, DisplayName: "AlicePDF's Household", ReportingCurrency: "IDR",
			MultiCurrencyEnabled: false, AssumedAnnualInflation: decimal.RequireFromString("5.25"),
			UpdatedBy: &alice.ID,
		}); err != nil {
			t.Fatalf("UpdateHouseholdSettings: %v", err)
		}
		infl := repo.NewInflationRateRepo(tdb.Pool)
		if _, err := infl.CreateInflationRate(aliceCtx, repo.CreateInflationRateParams{
			YearMonth: ymUTC(2026, time.January), Rate: decimal.RequireFromString("4"),
		}); err != nil {
			t.Fatalf("CreateInflationRate: %v", err)
		}

		rates, assumed, err := r.ReportInflation(aliceCtx)
		if err != nil {
			t.Fatalf("ReportInflation: %v", err)
		}
		if len(rates) != 1 || !rates[0].Rate.Equal(decimal.NewFromInt(4)) {
			t.Errorf("rates: got %+v, want one rate of 4", rates)
		}
		if !assumed.Equal(decimal.RequireFromString("5.25")) {
			t.Errorf("assumed_annual_inflation: got %s, want 5.25", assumed)
		}
	})

	t.Run("Members returns the household users", func(t *testing.T) {
		members, err := r.Members(aliceCtx)
		if err != nil {
			t.Fatalf("Members: %v", err)
		}
		var found bool
		for _, m := range members {
			if m.ID == alice.ID {
				found = true
			}
		}
		if !found {
			t.Errorf("Members did not include alice: %+v", members)
		}
	})

	t.Run("GetPositionDetail resolves the in-range month", func(t *testing.T) {
		details, err := r.GetPositionDetail(aliceCtx, ymUTC(2026, time.January))
		if err != nil {
			t.Fatalf("GetPositionDetail: %v", err)
		}
		var bank *repo.PositionDetail
		for i := range details {
			if details[i].Group == "asset" && details[i].Subtype == "bank_account" {
				bank = &details[i]
			}
		}
		if bank == nil {
			t.Fatalf("no bank_account position in detail: %+v", details)
		}
		if !bank.Amount.Equal(decimal.NewFromInt(100)) {
			t.Errorf("bank amount: got %s, want 100", bank.Amount)
		}
	})

	t.Run("GetPositionDetail out of range is ErrNotFound", func(t *testing.T) {
		if _, err := r.GetPositionDetail(aliceCtx, ymUTC(2020, time.January)); !errors.Is(err, repo.ErrNotFound) {
			t.Errorf("got %v, want ErrNotFound", err)
		}
	})
}
