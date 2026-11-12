package backup

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/identity"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/testutil"
)

// covers: INV-FINANCE-28, INV-FINANCE-03, INV-FINANCE-05, INV-FINANCE-35
//
// The demo dataset is hand-written to reconcile as one household cash flow: the
// Everyday Checking balance is written against every other table, so the Living
// Expenses the report engine derives come out plausible and seasonal in every
// month — not the arbitrary residual that independent growth curves produced
// (#497). This seeds a fresh demo household at the two extremes of the table
// calendar — a window ending in January (the oldest row used February..December,
// the newest only January) and one ending in December (the oldest row unused) —
// which together reach every cell the window can ever show, runs the real report
// engine over it and holds those properties. It also holds the lifecycle the
// seeder drives through the app's real paths: positions not updated this month
// are flagged stale (and only those), closed positions carry their terminal
// state and leave the active totals, and a matured time deposit's rollover chain
// links and reconciles.
func TestSeedDemoData_Reconciles(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
	}{
		{"window ending in January", nowIn(2027, time.January)},
		{"window ending in December", nowIn(2027, time.December)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checkSeededDemoReconciles(t, tc.now)
		})
	}
}

func checkSeededDemoReconciles(t *testing.T, now time.Time) {
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
		TimeZone:    demoTimeZone,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	member2, err := q.CreateLocalUser(ctx, db.CreateLocalUserParams{
		HouseholdID: household.ID,
		DisplayName: "Alex",
		Email:       "alex-recon@balances.local",
		Locale:      "en-GB",
		TimeZone:    demoTimeZone,
		CreatedBy:   &owner.ID,
	})
	if err != nil {
		t.Fatalf("create member2: %v", err)
	}

	ownerCtx := identity.WithUser(ctx, owner)
	if err := seedDemoDataAt(ownerCtx, tdb.Pool, owner.ID, member2.ID, now); err != nil {
		t.Fatalf("seed demo data: %v", err)
	}
	months := demoMonthsAt(now)

	// Materialise the monthly reports through the real engine — the derived
	// Living Expenses and investment returns are what a demo visitor sees.
	if err := repo.NewMonthlyReportRepo(tdb.Pool).RebuildAll(ownerCtx); err != nil {
		t.Fatalf("rebuild reports: %v", err)
	}

	rows, err := tdb.Pool.Query(ctx, `
		SELECT year_month, derived_living_expenses, investment_return_total,
		       nw_total, nw_assets, nw_liabilities, nw_receivables, nw_investments,
		       COALESCE((SELECT array_agg(e->>'name' ORDER BY e->>'name') FROM jsonb_array_elements(stale_positions) e), '{}'),
		       jsonb_array_length(missing_fx), jsonb_array_length(unsettled_terminations)
		FROM monthly_reports
		WHERE household_id = $1 AND year_month BETWEEN $2 AND $3
		ORDER BY year_month`, household.ID, months[0], months[len(months)-1])
	if err != nil {
		t.Fatalf("query monthly reports: %v", err)
	}
	defer rows.Close()

	const (
		lowerBound = 3_000_000.0  // no month should read as near-zero/negative spending
		upperBound = 12_000_000.0 // nor implausibly high for this modest household
	)

	type monthRow struct {
		ym       time.Time
		expenses float64
	}
	var (
		reported    int
		nonBaseline []monthRow
		prevInvest  decimal.Decimal
	)
	for rows.Next() {
		var ym time.Time
		var expenses, investReturn *decimal.Decimal
		var nwTotal, nwAssets, nwLiabs, nwRecv, nwInvest decimal.Decimal
		var stale []string
		var missingFx, unsettled int
		if err := rows.Scan(&ym, &expenses, &investReturn, &nwTotal, &nwAssets, &nwLiabs, &nwRecv, &nwInvest, &stale, &missingFx, &unsettled); err != nil {
			t.Fatalf("scan report row: %v", err)
		}
		reported++
		label := ym.Format("2006-01")

		if sum := nwAssets.Add(nwInvest).Add(nwRecv).Sub(nwLiabs); !sum.Equal(nwTotal) {
			t.Errorf("%s: nw_total %s != assets + investments + receivables - liabilities (%s)", label, nwTotal, sum)
		}
		if !nwTotal.IsPositive() || nwAssets.IsNegative() || nwLiabs.IsNegative() || nwRecv.IsNegative() || nwInvest.IsNegative() {
			t.Errorf("%s: a net-worth component is negative or net worth is not positive: total=%s assets=%s liabilities=%s receivables=%s investments=%s",
				label, nwTotal, nwAssets, nwLiabs, nwRecv, nwInvest)
		}
		// Only the positions demoNotUpdated names are behind, and only in the
		// trailing months it says (distance 0 is the current month); every other
		// position carries a snapshot every month. Closed positions are never stale,
		// and every termination is settled by a Sell or Maturity (ADR-0052 §7).
		distance := len(months) - reported
		if want := demoStaleNames(distance); fmt.Sprint(stale) != fmt.Sprint(want) {
			t.Errorf("%s (%d months before the current one): stale positions %v, want exactly %v", label, distance, stale, want)
		}
		if missingFx != 0 || unsettled != 0 {
			t.Errorf("%s: %d missing FX, %d unsettled terminations — every USD month has a rate and every termination a settlement", label, missingFx, unsettled)
		}

		if expenses != nil {
			exp, _ := expenses.Float64()
			if exp < lowerBound || exp > upperBound {
				t.Errorf("%s: derived living expenses %.0f outside plausible band [%.0f, %.0f]", label, exp, lowerBound, upperBound)
			}
			nonBaseline = append(nonBaseline, monthRow{ym: ym, expenses: exp})
		}
		// A genuine market slide is a real, negative return, so the floor is a
		// share of the pool rather than a flat amount; a Buy the snapshot quantity
		// does not reflect (#497) is caught exactly by the quantity tally below.
		if investReturn != nil && prevInvest.IsPositive() {
			floor := prevInvest.Mul(decimal.RequireFromString("-0.10"))
			if investReturn.LessThan(floor) {
				t.Errorf("%s: investment_return_total %s is worse than -10%% of the pool (%s)", label, investReturn, prevInvest)
			}
		}
		prevInvest = nwInvest
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate report rows: %v", err)
	}
	if reported != demoMonthCount {
		t.Errorf("materialised %d months in the seeded window, want %d", reported, demoMonthCount)
	}
	if len(nonBaseline) != demoMonthCount-1 {
		t.Errorf("materialised %d non-baseline months, want %d", len(nonBaseline), demoMonthCount-1)
	}

	// School fees land every July: in every calendar year the window holds in
	// full, July's derived expenses sit well above the rest of that year's.
	byYear := map[int][]monthRow{}
	for _, m := range nonBaseline {
		byYear[m.ym.Year()] = append(byYear[m.ym.Year()], m)
	}
	var fullYears int
	for year, ms := range byYear {
		if len(ms) != 12 {
			continue
		}
		fullYears++
		var july, others float64
		for _, m := range ms {
			if m.ym.Month() == time.July {
				july = m.expenses
			} else {
				others += m.expenses
			}
		}
		if mean := others / 11; july < mean+1_000_000 {
			t.Errorf("%d: July living expenses %.0f are not at least 1M above that year's other months (mean %.0f) — school fees go missing", year, july, mean)
		}
	}
	if fullYears < 4 {
		t.Errorf("only %d calendar years held in full, want at least 4", fullYears)
	}

	// The Everyday Checking balance must stay solvent every month — a negative
	// cash balance would betray the reconciliation as fiction — and no seeded
	// snapshot, of any group, is negative.
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
	var negatives int
	if err := tdb.Pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM asset_snapshots s JOIN assets p ON p.id = s.asset_id WHERE p.household_id = $1 AND s.amount < 0)
		     + (SELECT count(*) FROM liability_snapshots s JOIN liabilities p ON p.id = s.liability_id WHERE p.household_id = $1 AND s.amount < 0)
		     + (SELECT count(*) FROM receivable_snapshots s JOIN receivables p ON p.id = s.receivable_id WHERE p.household_id = $1 AND s.amount < 0)
		     + (SELECT count(*) FROM investment_snapshots s JOIN investments p ON p.id = s.investment_id WHERE p.household_id = $1 AND s.amount < 0)`,
		household.ID).Scan(&negatives); err != nil {
		t.Fatalf("count negative snapshots: %v", err)
	}
	if negatives != 0 {
		t.Errorf("%d seeded snapshots are negative", negatives)
	}

	// Every market position's snapshot quantity, in every month, must equal the
	// net of its Buy/Sell ledger up to that month — the "values tally with the
	// ledger" property at the heart of the #497 fix (and the frontend
	// reconcileQuantity banner). This is what keeps the engine's return a clean
	// price movement rather than a phantom loss.
	qtyRows, err := tdb.Pool.Query(ctx, `
		SELECT i.display_name, s.year_month, s.quantity,
		       COALESCE((SELECT SUM(CASE t.transaction_type
		                              WHEN 'buy'  THEN t.quantity
		                              WHEN 'sell' THEN -t.quantity
		                              ELSE 0 END)
		                   FROM investment_transactions t
		                  WHERE t.investment_id = i.id AND t.deleted_at IS NULL
		                    AND t.transaction_date < (s.year_month + INTERVAL '1 month')), 0) AS net_txn_qty
		FROM investment_snapshots s
		JOIN investments i ON i.id = s.investment_id
		WHERE i.household_id = $1 AND i.subtype IN ('stock', 'mutual_fund', 'gold') AND s.deleted_at IS NULL
		ORDER BY i.display_name, s.year_month`, household.ID)
	if err != nil {
		t.Fatalf("query quantity tally: %v", err)
	}
	defer qtyRows.Close()

	held := map[string]int{}
	for qtyRows.Next() {
		var name string
		var ym time.Time
		var qty, netTxnQty decimal.Decimal
		if err := qtyRows.Scan(&name, &ym, &qty, &netTxnQty); err != nil {
			t.Fatalf("scan quantity tally: %v", err)
		}
		held[name]++
		if !qty.Equal(netTxnQty) {
			t.Errorf("%s %s: snapshot qty %s != net Buy/Sell ledger qty %s — the snapshot has drifted from the trades",
				name, ym.Format("2006-01"), qty, netTxnQty)
		}
	}
	if err := qtyRows.Err(); err != nil {
		t.Fatalf("iterate quantity tally: %v", err)
	}
	if len(held) != 7 {
		t.Errorf("checked %d market positions, want the seeded 7 (2 each of stock/mutual_fund/gold, plus the closed TLKM)", len(held))
	}

	checkSeededLifecycle(t, tdb.Pool, household.ID, months, months[len(months)-1])
}

// demoStaleNames is who the report should flag stale in the month `distance`
// months before the current one: the positions behind by more than that.
func demoStaleNames(distance int) []string {
	names := []string{}
	for name, behind := range demoNotUpdated {
		if behind > distance {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// covers: INV-BACKUP-15
//
// Seeding the real clock's window is what the nightly reset does; this pins that
// it succeeds at whatever month it runs and leaves the 72-month trail behind.
func TestSeedDemoData_SeedsTheFullWindow(t *testing.T) {
	tdb := testutil.NewTestDB(t)
	q := db.New(tdb.Pool)
	ctx := context.Background()
	household, err := q.CreateHousehold(ctx, db.CreateHouseholdParams{DisplayName: "Demo Household", ReportingCurrency: "IDR"})
	if err != nil {
		t.Fatalf("create household: %v", err)
	}
	owner, err := q.CreateLocalUser(ctx, db.CreateLocalUserParams{
		HouseholdID: household.ID, DisplayName: "Demo", Email: "demo-window@balances.local", Locale: "en-GB", TimeZone: demoTimeZone,
	})
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	member2, err := q.CreateLocalUser(ctx, db.CreateLocalUserParams{
		HouseholdID: household.ID, DisplayName: "Alex", Email: "alex-window@balances.local", Locale: "en-GB", TimeZone: demoTimeZone, CreatedBy: &owner.ID,
	})
	if err != nil {
		t.Fatalf("create member2: %v", err)
	}
	if err := seedDemoData(identity.WithUser(ctx, owner), tdb.Pool, owner.ID, member2.ID); err != nil {
		t.Fatalf("seed demo data: %v", err)
	}

	months := demoMonthsAt(time.Now())
	var first, last time.Time
	var distinct int
	if err := tdb.Pool.QueryRow(ctx, `
		SELECT MIN(s.year_month), MAX(s.year_month), COUNT(DISTINCT s.year_month)
		FROM asset_snapshots s JOIN assets a ON a.id = s.asset_id
		WHERE a.household_id = $1 AND a.display_name = 'Everyday Checking'`, household.ID).Scan(&first, &last, &distinct); err != nil {
		t.Fatalf("query checking window: %v", err)
	}
	if distinct != demoMonthCount || !first.Equal(months[0]) || !last.Equal(months[len(months)-1]) {
		t.Errorf("checking trail = %d months %s..%s, want %d months %s..%s", distinct,
			first.Format("2006-01"), last.Format("2006-01"), demoMonthCount,
			months[0].Format("2006-01"), months[len(months)-1].Format("2006-01"))
	}
	var positions int
	if err := tdb.Pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM assets WHERE household_id = $1)
		     + (SELECT count(*) FROM investments WHERE household_id = $1)
		     + (SELECT count(*) FROM liabilities WHERE household_id = $1)
		     + (SELECT count(*) FROM receivables WHERE household_id = $1)`, household.ID).Scan(&positions); err != nil {
		t.Fatalf("count positions: %v", err)
	}
	if positions != 26 {
		t.Errorf("%d positions seeded, want 26 (8 assets, 14 investments, 2 liabilities, 2 receivables)", positions)
	}
}

// positionRow is a position's lifecycle columns as stored.
type positionRow struct {
	id         uuid.UUID
	status     string
	terminated *time.Time
	note       *string
	rolledFrom *uuid.UUID
}

func positionByName(t *testing.T, pool *pgxpool.Pool, table string, hid uuid.UUID, name string) positionRow {
	t.Helper()
	var r positionRow
	rolled := "NULL::uuid"
	if table == "investments" {
		rolled = "rolled_from_investment_id"
	}
	if err := pool.QueryRow(context.Background(),
		"SELECT id, status, terminated_at, termination_note, "+rolled+" FROM "+table+
			" WHERE household_id = $1 AND display_name = $2 AND deleted_at IS NULL", hid, name,
	).Scan(&r.id, &r.status, &r.terminated, &r.note, &r.rolledFrom); err != nil {
		t.Fatalf("%s %q: %v", table, name, err)
	}
	return r
}

// snapshotMonths lists a position's live snapshot months with their amounts.
func snapshotMonths(t *testing.T, pool *pgxpool.Pool, table, fk string, id uuid.UUID) (months []time.Time, amounts []decimal.Decimal) {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		"SELECT year_month, amount FROM "+table+" WHERE "+fk+" = $1 AND deleted_at IS NULL ORDER BY year_month", id)
	if err != nil {
		t.Fatalf("%s snapshots: %v", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var ym time.Time
		var amt decimal.Decimal
		if err := rows.Scan(&ym, &amt); err != nil {
			t.Fatalf("scan %s snapshot: %v", table, err)
		}
		months = append(months, ym)
		amounts = append(amounts, amt)
	}
	return months, amounts
}

// assertClosedAt holds what a closed position must look like through the app's
// lifecycle path (ADR-0052 §1): the terminal status and date, a zero close
// snapshot in the termination month, and nothing after it. before is the number
// of months recorded ahead of the close.
func assertClosedAt(t *testing.T, pool *pgxpool.Pool, table, snapTable, fk string, p positionRow, wantStatus string, closedAt time.Time, before int) {
	t.Helper()
	label := fmt.Sprintf("%s %s", table, p.id)
	if p.status != wantStatus || p.terminated == nil || !p.terminated.Equal(closedAt) {
		t.Errorf("%s: status %q terminated %v, want %q on %s", label, p.status, p.terminated, wantStatus, closedAt.Format("2006-01-02"))
	}
	ms, amts := snapshotMonths(t, pool, snapTable, fk, p.id)
	closeMonth := time.Date(closedAt.Year(), closedAt.Month(), 1, 0, 0, 0, 0, time.UTC)
	if len(ms) == 0 || !ms[len(ms)-1].Equal(closeMonth) {
		t.Errorf("%s: last snapshot is %v, want the close snapshot in %s (nothing after it)", label, ms, closeMonth.Format("2006-01"))
		return
	}
	if !amts[len(amts)-1].IsZero() {
		t.Errorf("%s: close snapshot is %s, want 0", label, amts[len(amts)-1])
	}
	if len(ms)-1 != before {
		t.Errorf("%s: %d snapshots before the close, want %d", label, len(ms)-1, before)
	}
	for i := 0; i < len(ms)-1; i++ {
		if !amts[i].IsPositive() {
			t.Errorf("%s: snapshot %s is %s before the close, want a value", label, ms[i].Format("2006-01"), amts[i])
		}
	}
}

type txnRow struct {
	typ                         string
	date                        time.Time
	amount, qty, price          *decimal.Decimal
	principal, interest         *decimal.Decimal
	principalDisp, interestDisp *string
}

func transactionsOf(t *testing.T, pool *pgxpool.Pool, id uuid.UUID, typ string) []txnRow {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		SELECT transaction_type, transaction_date, amount, quantity, price_per_unit,
		       principal_amount, interest_amount, principal_disposition, interest_disposition
		FROM investment_transactions
		WHERE investment_id = $1 AND transaction_type = $2 AND deleted_at IS NULL
		ORDER BY transaction_date`, id, typ)
	if err != nil {
		t.Fatalf("transactions: %v", err)
	}
	defer rows.Close()
	var out []txnRow
	for rows.Next() {
		var r txnRow
		if err := rows.Scan(&r.typ, &r.date, &r.amount, &r.qty, &r.price, &r.principal, &r.interest, &r.principalDisp, &r.interestDisp); err != nil {
			t.Fatalf("scan transaction: %v", err)
		}
		out = append(out, r)
	}
	return out
}

// checkSeededLifecycle holds the closed and matured positions the seeder drives
// through the app's real lifecycle paths: each carries its terminal state, a zero
// close snapshot and its settlement transaction; they leave the active totals; and
// the rollover chain links predecessor to successor with the amounts reconciling.
func checkSeededLifecycle(t *testing.T, pool *pgxpool.Pool, hid uuid.UUID, months []time.Time, current time.Time) {
	t.Helper()
	ctx := context.Background()
	dec := func(n int64) decimal.Decimal { return decimal.NewFromInt(n) }
	idxOf := func(w demoWhen) int {
		i, place := w.locate(months)
		if place != demoInWindow {
			t.Fatalf("%+v is not inside the window", w)
		}
		return i
	}

	// Old Hatchback: an asset sold through the lifecycle flip.
	hatch := positionByName(t, pool, "assets", hid, "Old Hatchback")
	hatchAt := demoOldHatchback.closing.when.date(months, demoOldHatchback.closing.day)
	assertClosedAt(t, pool, "assets", "asset_snapshots", "asset_id", hatch, "sold", hatchAt, idxOf(demoOldHatchback.closing.when))
	if hatch.note == nil || *hatch.note != demoOldHatchback.closing.note {
		t.Errorf("Old Hatchback termination note = %v, want %q", hatch.note, demoOldHatchback.closing.note)
	}

	// TLKM: a stock sold in full — the terminal Sell is the whole holding at the
	// sale month's price, so the ledger nets to zero.
	tlkm := positionByName(t, pool, "investments", hid, "Telkom Indonesia")
	tlkmAt := demoTLKM.closing.when.date(months, demoTLKM.closing.day)
	tlkmIdx := idxOf(demoTLKM.closing.when)
	assertClosedAt(t, pool, "investments", "investment_snapshots", "investment_id", tlkm, "sold", tlkmAt, tlkmIdx)
	held, _ := demoHoldings(months, demoTLKM.trades)
	sells := transactionsOf(t, pool, tlkm.id, repo.TxnTypeSell)
	wantPrice := demoTLKM.prices.money(months, tlkmIdx, demoCurrency)
	if len(sells) != 1 || !sells[0].date.Equal(tlkmAt) || sells[0].qty == nil || !sells[0].qty.Equal(dec(held[tlkmIdx])) ||
		sells[0].price == nil || !sells[0].price.Equal(wantPrice) || sells[0].amount == nil || !sells[0].amount.Equal(dec(held[tlkmIdx]).Mul(wantPrice)) {
		t.Errorf("TLKM has %d sells (see the ledger), want one on %s of %d at %s", len(sells), tlkmAt.Format("2006-01-02"), held[tlkmIdx], wantPrice)
	}

	// BNI: matured and rolled over.
	bni := positionByName(t, pool, "investments", hid, "BNI Time Deposit")
	bniPlaced := demoBNIDeposit.placed.date(months, 12)
	bniMaturity := bniPlaced.AddDate(0, demoBNIDeposit.termMonths, 0)
	// One snapshot per month of its term, placement month to the month before maturity.
	assertClosedAt(t, pool, "investments", "investment_snapshots", "investment_id", bni, "matured", bniMaturity, demoBNIDeposit.termMonths)
	mats := transactionsOf(t, pool, bni.id, repo.TxnTypeMaturity)
	if len(mats) != 1 || !mats[0].date.Equal(bniMaturity) ||
		mats[0].principal == nil || !mats[0].principal.Equal(dec(demoBNIDeposit.principal)) ||
		mats[0].interest == nil || !mats[0].interest.Equal(dec(demoBNIDeposit.interest)) ||
		mats[0].principalDisp == nil || *mats[0].principalDisp != repo.DispositionRolledToNew ||
		mats[0].interestDisp == nil || *mats[0].interestDisp != repo.DispositionRolledToNew {
		t.Errorf("BNI has %d maturity transactions (see the ledger), want %d + %d on %s, both rolled_to_new", len(mats), demoBNIDeposit.principal, demoBNIDeposit.interest, bniMaturity.Format("2006-01-02"))
	}

	// The successor: linked back, holding exactly what rolled, placed on the
	// maturity date, still running, and recorded from its placement month on.
	renewed := positionByName(t, pool, "investments", hid, "BNI Time Deposit (renewed)")
	if renewed.rolledFrom == nil || *renewed.rolledFrom != bni.id {
		t.Errorf("renewal rolled_from = %v, want the BNI deposit %s", renewed.rolledFrom, bni.id)
	}
	if renewed.status != repo.StatusActive || renewed.terminated != nil {
		t.Errorf("renewal status %q terminated %v, want an active deposit", renewed.status, renewed.terminated)
	}
	var principal decimal.Decimal
	var placement, maturity time.Time
	if err := pool.QueryRow(ctx, "SELECT principal, placement_date, maturity_date FROM time_deposit_details WHERE investment_id = $1", renewed.id).
		Scan(&principal, &placement, &maturity); err != nil {
		t.Fatalf("renewal details: %v", err)
	}
	if want := dec(demoBNIDeposit.principal + demoBNIDeposit.interest); !principal.Equal(want) {
		t.Errorf("renewal principal = %s, want the rolled principal + interest %s", principal, want)
	}
	if !placement.Equal(bniMaturity) || !maturity.After(current) {
		t.Errorf("renewal runs %s..%s, want it placed on %s and still running", placement.Format("2006-01-02"), maturity.Format("2006-01-02"), bniMaturity.Format("2006-01-02"))
	}
	ms, amts := snapshotMonths(t, pool, "investment_snapshots", "investment_id", renewed.id)
	if len(ms) == 0 || !ms[0].Equal(time.Date(bniMaturity.Year(), bniMaturity.Month(), 1, 0, 0, 0, 0, time.UTC)) || !ms[len(ms)-1].Equal(current) {
		t.Errorf("renewal is recorded %v, want from its placement month through the current month", ms)
	} else if !amts[0].GreaterThan(principal) || !amts[0].LessThan(principal.Mul(decimal.RequireFromString("1.005"))) {
		t.Errorf("renewal's first value %s is not the rolled principal %s plus about a month of interest", amts[0], principal)
	}
	// Both ends are live and the chain is 1:1: nothing else rolled from the source.
	var successors int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM investments WHERE household_id = $1 AND rolled_from_investment_id = $2 AND deleted_at IS NULL", hid, bni.id).Scan(&successors); err != nil {
		t.Fatalf("count successors: %v", err)
	}
	if successors != 1 {
		t.Errorf("%d successors of the BNI deposit, want exactly 1", successors)
	}

	// Danamon: matured and paid out — cash_out on both legs, and nothing rolled.
	dan := positionByName(t, pool, "investments", hid, "Danamon Time Deposit")
	danMaturity := demoDanamonDeposit.placed.date(months, 12).AddDate(0, demoDanamonDeposit.termMonths, 0)
	assertClosedAt(t, pool, "investments", "investment_snapshots", "investment_id", dan, "matured", danMaturity, demoDanamonDeposit.termMonths)
	mats = transactionsOf(t, pool, dan.id, repo.TxnTypeMaturity)
	if len(mats) != 1 || !mats[0].date.Equal(danMaturity) ||
		mats[0].principal == nil || !mats[0].principal.Equal(dec(demoDanamonDeposit.principal)) ||
		mats[0].interest == nil || !mats[0].interest.Equal(dec(demoDanamonDeposit.interest)) ||
		mats[0].principalDisp == nil || *mats[0].principalDisp != repo.DispositionCashOut ||
		mats[0].interestDisp == nil || *mats[0].interestDisp != repo.DispositionCashOut {
		t.Errorf("Danamon has %d maturity transactions (see the ledger), want %d + %d on %s, both cash_out", len(mats), demoDanamonDeposit.principal, demoDanamonDeposit.interest, danMaturity.Format("2006-01-02"))
	}
	var danSuccessors int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM investments WHERE household_id = $1 AND rolled_from_investment_id = $2", hid, dan.id).Scan(&danSuccessors); err != nil {
		t.Fatalf("count Danamon successors: %v", err)
	}
	if danSuccessors != 0 {
		t.Errorf("the paid-out Danamon deposit has %d successors, want none", danSuccessors)
	}

	// Closed positions show in history but not in the active totals: the current
	// month's asset and investment net worth are exactly the active positions'
	// latest values (USD converted at the month's rate), and no closed position
	// is stale or carried.
	for _, g := range []struct{ table, snap, fk, report string }{
		{"assets", "asset_snapshots", "asset_id", "nw_assets"},
		{"investments", "investment_snapshots", "investment_id", "nw_investments"},
	} {
		var active, reported decimal.Decimal
		if err := pool.QueryRow(ctx, `
			SELECT COALESCE(SUM(s.amount * CASE WHEN s.currency = 'USD' THEN fx.rate ELSE 1 END), 0)
			FROM `+g.table+` p
			JOIN LATERAL (SELECT amount, currency FROM `+g.snap+`
			               WHERE `+g.fk+` = p.id AND deleted_at IS NULL AND year_month <= $2
			               ORDER BY year_month DESC LIMIT 1) s ON true
			LEFT JOIN LATERAL (SELECT rate FROM fx_rates
			                    WHERE household_id = $1 AND currency = 'USD' AND deleted_at IS NULL AND year_month <= $2
			                    ORDER BY year_month DESC LIMIT 1) fx ON true
			WHERE p.household_id = $1 AND p.deleted_at IS NULL AND p.status = 'active'`, hid, current).Scan(&active); err != nil {
			t.Fatalf("active %s total: %v", g.table, err)
		}
		if err := pool.QueryRow(ctx, "SELECT "+g.report+" FROM monthly_reports WHERE household_id = $1 AND year_month = $2", hid, current).Scan(&reported); err != nil {
			t.Fatalf("reported %s: %v", g.report, err)
		}
		if reported.Sub(active).Abs().GreaterThan(decimal.NewFromInt(1)) {
			t.Errorf("%s at %s = %s, want the active positions' total %s — a closed position leaked into the active totals", g.report, current.Format("2006-01"), reported, active)
		}
	}
}
