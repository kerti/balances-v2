package backup

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// covers: INV-FINANCE-28
//
// The demo dataset is seeded as one coherent household cash flow: the Everyday
// Checking account is the reconciling plug (demo_seed.go), so the engine's
// derived Living Expenses must come out positive, plausible, and equal to the
// chosen expense series every month — not the arbitrary residual that a set of
// independent growth curves produced (#497). This runs the real report engine
// over a freshly seeded demo household and holds those properties.
func TestSeedDemoData_Reconciles(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	ctx := context.Background()

	household, err := q.CreateHousehold(ctx, db.CreateHouseholdParams{
		DisplayName:       "Demo Household",
		ReportingCurrency: "IDR",
	})
	if err != nil {
		t.Fatalf("create household: %v", err)
	}
	// Multi-currency on, matching resetDemoHousehold — the USD positions must
	// convert into nw_total for the reconciliation to hold.
	if _, err := q.UpdateHouseholdSettings(ctx, db.UpdateHouseholdSettingsParams{
		ID:                     household.ID,
		DisplayName:            household.DisplayName,
		ReportingCurrency:      household.ReportingCurrency,
		MultiCurrencyEnabled:   true,
		AssumedAnnualInflation: household.AssumedAnnualInflation,
	}); err != nil {
		t.Fatalf("enable multi-currency: %v", err)
	}
	owner, err := q.CreateLocalUser(ctx, db.CreateLocalUserParams{
		HouseholdID: household.ID,
		DisplayName: "Demo",
		Email:       "demo-recon@balances.local",
		Locale:      "en-GB",
		TimeZone:    "Asia/Jakarta",
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	member2, err := q.CreateLocalUser(ctx, db.CreateLocalUserParams{
		HouseholdID: household.ID,
		DisplayName: "Alex",
		Email:       "alex-recon@balances.local",
		Locale:      "en-GB",
		TimeZone:    "Asia/Jakarta",
		CreatedBy:   &owner.ID,
	})
	if err != nil {
		t.Fatalf("create member2: %v", err)
	}

	ownerCtx := identity.WithUser(ctx, owner)
	if err := seedDemoData(ownerCtx, tdb.Pool, owner.ID, member2.ID); err != nil {
		t.Fatalf("seed demo data: %v", err)
	}

	// Materialise the monthly reports through the real engine — the derived
	// Living Expenses and investment returns are what a demo visitor sees.
	if err := repo.NewMonthlyReportRepo(tdb.Pool).RebuildAll(ownerCtx); err != nil {
		t.Fatalf("rebuild reports: %v", err)
	}

	rows, err := tdb.Pool.Query(ctx, `
		SELECT year_month, derived_living_expenses, investment_return_total
		FROM monthly_reports
		WHERE household_id = $1
		ORDER BY year_month`, household.ID)
	if err != nil {
		t.Fatalf("query monthly reports: %v", err)
	}
	defer rows.Close()

	// The chosen expense series, keyed by the month the seeder used, so we can
	// prove the engine's *derived* expenses reproduce it (± a small FX leak from
	// the USD bank/receivable positions the plug deliberately omits).
	expenseByMonth := make(map[time.Time]float64)
	for i, ym := range demoMonths() {
		expenseByMonth[ym] = demoExpenseSeries()[i]
	}

	const (
		lowerBound = 3_000_000.0  // no month should read as near-zero/negative spending
		upperBound = 12_000_000.0 // nor implausibly high for this modest household
		fxLeakTol  = 750_000.0    // USD revaluation + rounding slack around the chosen series
		phantomFlr = -1_000_000.0 // #497: buys inside the window booked multi-million losses
	)

	var nonBaseline int
	for rows.Next() {
		var ym time.Time
		var expenses, investReturn *decimal.Decimal
		if err := rows.Scan(&ym, &expenses, &investReturn); err != nil {
			t.Fatalf("scan report row: %v", err)
		}
		ym = time.Date(ym.Year(), ym.Month(), 1, 0, 0, 0, 0, time.UTC)
		if expenses == nil {
			continue // baseline month books no income statement (ADR-0006)
		}
		nonBaseline++

		exp, _ := expenses.Float64()
		if exp < lowerBound || exp > upperBound {
			t.Errorf("%s: derived living expenses %.0f outside plausible band [%.0f, %.0f]",
				ym.Format("2006-01"), exp, lowerBound, upperBound)
		}
		if want, ok := expenseByMonth[ym]; ok {
			if diff := math.Abs(exp - want); diff > fxLeakTol {
				t.Errorf("%s: derived living expenses %.0f drifted %.0f from the seeded series %.0f (tol %.0f) — the cash ledger no longer mirrors the engine",
					ym.Format("2006-01"), exp, diff, want, fxLeakTol)
			}
		}

		if investReturn != nil {
			ret, _ := investReturn.Float64()
			if ret < phantomFlr {
				t.Errorf("%s: investment_return_total %.0f below %.0f — a Buy is not reflected in the snapshot quantity (#497 phantom loss)",
					ym.Format("2006-01"), ret, phantomFlr)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate report rows: %v", err)
	}
	if nonBaseline < demoMonthCount-1 {
		t.Errorf("materialised %d non-baseline months, want %d", nonBaseline, demoMonthCount-1)
	}

	// The Everyday Checking plug must stay solvent every month — a negative cash
	// balance would betray the reconciliation as fiction.
	var minChecking decimal.Decimal
	if err := tdb.Pool.QueryRow(ctx, `
		SELECT MIN(s.amount)
		FROM asset_snapshots s
		JOIN assets a ON a.id = s.asset_id
		WHERE a.household_id = $1 AND a.display_name = 'Everyday Checking'`,
		household.ID).Scan(&minChecking); err != nil {
		t.Fatalf("query checking balances: %v", err)
	}
	if minChecking.IsNegative() {
		t.Errorf("Everyday Checking dips to %s — the cash plug goes insolvent", minChecking)
	}

	// Every market position's final snapshot quantity must equal the net of its
	// Buy/Sell ledger — the "values tally with the ledger" property at the heart
	// of the #497 fix (and the frontend reconcileQuantity banner). This is what
	// keeps the engine's return as clean price appreciation rather than a phantom
	// loss; without it the seeder could drift snapshot qty from the trades.
	qtyRows, err := tdb.Pool.Query(ctx, `
		SELECT i.display_name,
		       (SELECT s.quantity FROM investment_snapshots s
		          WHERE s.investment_id = i.id ORDER BY s.year_month DESC LIMIT 1) AS final_qty,
		       COALESCE(SUM(CASE t.transaction_type
		                      WHEN 'buy'  THEN t.quantity
		                      WHEN 'sell' THEN -t.quantity
		                      ELSE 0 END), 0) AS net_txn_qty
		FROM investments i
		LEFT JOIN investment_transactions t ON t.investment_id = i.id
		WHERE i.household_id = $1 AND i.subtype IN ('stock', 'mutual_fund', 'gold')
		GROUP BY i.id, i.display_name
		ORDER BY i.display_name`, household.ID)
	if err != nil {
		t.Fatalf("query quantity tally: %v", err)
	}
	defer qtyRows.Close()

	var marketPositions int
	for qtyRows.Next() {
		var name string
		var finalQty, netTxnQty decimal.Decimal
		if err := qtyRows.Scan(&name, &finalQty, &netTxnQty); err != nil {
			t.Fatalf("scan quantity tally: %v", err)
		}
		marketPositions++
		if !finalQty.Equal(netTxnQty) {
			t.Errorf("%s: final snapshot qty %s != net Buy/Sell ledger qty %s — snapshot has drifted from the trades",
				name, finalQty, netTxnQty)
		}
	}
	if err := qtyRows.Err(); err != nil {
		t.Fatalf("iterate quantity tally: %v", err)
	}
	if marketPositions < 6 {
		t.Errorf("checked %d market positions, want the seeded 6 (2 each of stock/mutual_fund/gold)", marketPositions)
	}
}
