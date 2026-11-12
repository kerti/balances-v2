package backup

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/kerti/balances-v2/backend/internal/repo"
)

// These are container-free tests for the demo seeder's calendar machinery and
// the hand-written tables (demo_seed_data.go). The integration test
// (TestSeedDemoData_Reconciles) proves a seeded household reconciles through the
// real report engine; these pin what it cannot see — every table is the right
// shape, a malformed row fails here rather than in the nightly reset, and each
// seasonal event lands on its calendar month for every "now" the window can take.

// nowIn returns a mid-month instant in the given Jakarta-local month, so tests
// state the month they mean without tripping the UTC/Jakarta month-end offset.
func nowIn(year int, month time.Month) time.Time {
	return time.Date(year, month, 15, 12, 0, 0, 0, time.UTC)
}

// everyMonthNow is one "now" per calendar month: the window can end in any of
// them, and each aligns the year rows differently.
func everyMonthNow() []time.Time {
	var nows []time.Time
	for m := time.January; m <= time.December; m++ {
		nows = append(nows, nowIn(2027, m))
	}
	return nows
}

// covers: INV-FINANCE-28
//
// The window is exactly demoMonthCount consecutive first-of-month dates ending
// at the current month, for every month the cron can run in.
func TestDemoMonthsAt_WindowShape(t *testing.T) {
	if demoMonthCount != 72 {
		t.Fatalf("demoMonthCount = %d, want 72 (6 years, so a 5Y range chip shows)", demoMonthCount)
	}
	for _, now := range everyMonthNow() {
		months := demoMonthsAt(now)
		if len(months) != 72 {
			t.Fatalf("now=%s: %d months, want 72", now.Format("2006-01"), len(months))
		}
		last := months[len(months)-1]
		if last.Year() != now.Year() || last.Month() != now.Month() {
			t.Errorf("now=%s: window ends %s", now.Format("2006-01"), last.Format("2006-01"))
		}
		for i, ym := range months {
			if ym.Day() != 1 || ym.Location() != time.UTC {
				t.Errorf("now=%s: month %d = %v is not a UTC first-of-month", now.Format("2006-01"), i, ym)
			}
			if i > 0 && !ym.Equal(months[i-1].AddDate(0, 1, 0)) {
				t.Errorf("now=%s: month %d (%s) does not follow %s", now.Format("2006-01"), i, ym.Format("2006-01"), months[i-1].Format("2006-01"))
			}
		}
		if want := last.AddDate(0, -71, 0); !months[0].Equal(want) {
			t.Errorf("now=%s: window starts %s, want %s", now.Format("2006-01"), months[0].Format("2006-01"), want.Format("2006-01"))
		}
	}
}

// covers: INV-FINANCE-28
//
// The report engine buckets "now" into the demo users' local month, and the
// demo-reset cron fires at 19:00 UTC — already the next month in Asia/Jakarta on
// every month end. The seeder's window must end on that same local month, or
// the demo shows an empty current month with zero living expenses.
func TestDemoMonthsAt_EndsOnLocalMonth(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{"cron run on UTC month end is next month in Jakarta",
			time.Date(2026, time.October, 31, 19, 0, 0, 0, time.UTC),
			time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)},
		{"UTC year end rolls the local year",
			time.Date(2026, time.December, 31, 20, 0, 0, 0, time.UTC),
			time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)},
		{"mid-month agrees with UTC",
			time.Date(2026, time.June, 15, 12, 0, 0, 0, time.UTC),
			time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			months := demoMonthsAt(tc.now)
			if len(months) != demoMonthCount {
				t.Fatalf("got %d months, want %d", len(months), demoMonthCount)
			}
			if got := months[len(months)-1]; !got.Equal(tc.want) {
				t.Errorf("window ends %s, want %s", got.Format("2006-01"), tc.want.Format("2006-01"))
			}
		})
	}
}

// covers: INV-FINANCE-28
//
// A window month maps onto "N years ago, in <calendar month>" — across a January
// boundary, at fixed "now" dates, and for every month the window can end in.
func TestDemoSlot_CalendarMapping(t *testing.T) {
	type slot struct{ row, mon int }
	cases := []struct {
		name          string
		now           time.Time
		first, last   slot
		beforeLast    slot // the month before "now": crosses a year boundary in January
		firstReadable string
	}{
		// Window Dec 2020 .. Nov 2026: starts in the oldest row's December.
		{"November", nowIn(2026, time.November), slot{0, 11}, slot{6, 10}, slot{6, 9}, "2020-12"},
		// Window Feb 2021 .. Jan 2027: the oldest row is used February..December,
		// and the month before January is the previous year's December.
		{"January boundary", nowIn(2027, time.January), slot{0, 1}, slot{6, 0}, slot{5, 11}, "2021-02"},
		// Window Jan 2022 .. Dec 2027: the oldest row is not used at all, so the
		// first month is row 1's January.
		{"December", nowIn(2027, time.December), slot{1, 0}, slot{6, 11}, slot{6, 10}, "2022-01"},
		{"April", nowIn(2027, time.April), slot{0, 4}, slot{6, 3}, slot{6, 2}, "2021-05"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			months := demoMonthsAt(tc.now)
			if got := months[0].Format("2006-01"); got != tc.firstReadable {
				t.Fatalf("window starts %s, want %s", got, tc.firstReadable)
			}
			check := func(label string, i int, want slot) {
				row, mon := demoSlot(months, i)
				if row != want.row || mon != want.mon {
					t.Errorf("%s: slot(%d) = (%d,%d), want (%d,%d)", label, i, row, mon, want.row, want.mon)
				}
			}
			check("first", 0, tc.first)
			check("last", len(months)-1, tc.last)
			check("before last", len(months)-2, tc.beforeLast)
		})
	}

	// Every month of every window sits on its own calendar month, in a row that
	// exists, and consecutive months step one calendar month at a time.
	for _, now := range everyMonthNow() {
		months := demoMonthsAt(now)
		for i, ym := range months {
			row, mon := demoSlot(months, i)
			if row < 0 || row >= demoYearRows {
				t.Fatalf("now=%s: month %s maps to row %d outside 0..%d", now.Format("2006-01"), ym.Format("2006-01"), row, demoYearRows-1)
			}
			if mon != int(ym.Month())-1 {
				t.Errorf("now=%s: month %s maps to calendar month index %d", now.Format("2006-01"), ym.Format("2006-01"), mon)
			}
			if i == 0 {
				continue
			}
			pr, pm := demoSlot(months, i-1)
			wantRow, wantMon := pr, pm+1
			if pm == 11 {
				wantRow, wantMon = pr+1, 0
			}
			if row != wantRow || mon != wantMon {
				t.Errorf("now=%s: slot after (%d,%d) is (%d,%d), want (%d,%d)", now.Format("2006-01"), pr, pm, row, mon, wantRow, wantMon)
			}
		}
		// The current month is always the newest row.
		if row, _ := demoSlot(months, len(months)-1); row != demoYearRows-1 {
			t.Errorf("now=%s: current month is in row %d, want the newest (%d)", now.Format("2006-01"), row, demoYearRows-1)
		}
	}
}

// covers: INV-FINANCE-28
//
// A grid read goes through the slot map: the same calendar cell is "this year"
// when now is in November and "1 year ago" once the year has turned.
func TestDemoGrid_ReadsTheMappedCell(t *testing.T) {
	var g demoGrid
	for r := range g {
		for m := range g[r] {
			g[r][m] = int64(r*100 + m + 1) // row 3, March = 302
		}
	}
	nov := demoMonthsAt(nowIn(2026, time.November))
	// November 2026 is this year's (row 6) November; March 2026 is row 6 too.
	if got := g.at(nov, len(nov)-1); got != 611 {
		t.Errorf("current month cell = %d, want 611 (row 6, November)", got)
	}
	// Dec 2025 is last year's December; March 2024 is 2 years back.
	idx := func(months []time.Time, y int, m time.Month) int {
		for i, ym := range months {
			if ym.Year() == y && ym.Month() == m {
				return i
			}
		}
		t.Fatalf("%d-%d not in window", y, m)
		return -1
	}
	if got := g.at(nov, idx(nov, 2025, time.December)); got != 512 {
		t.Errorf("Dec 2025 cell = %d, want 512 (row 5, December)", got)
	}
	if got := g.at(nov, idx(nov, 2024, time.March)); got != 403 {
		t.Errorf("Mar 2024 cell = %d, want 403 (row 4, March)", got)
	}
	// Past New Year the same calendar months slide one row down.
	jan := demoMonthsAt(nowIn(2027, time.January))
	if got := g.at(jan, idx(jan, 2026, time.December)); got != 512 {
		t.Errorf("Dec 2026 cell (now Jan 2027) = %d, want 512 (row 5, December)", got)
	}
	if got := g.at(jan, idx(jan, 2027, time.January)); got != 601 {
		t.Errorf("Jan 2027 cell = %d, want 601 (row 6, January)", got)
	}
	if got := g.at(jan, idx(jan, 2021, time.February)); got != 2 {
		t.Errorf("Feb 2021 cell (now Jan 2027) = %d, want 2 (row 0, February)", got)
	}
}

// covers: INV-FINANCE-28
func TestDemoWhen_LocateAndDate(t *testing.T) {
	nov := demoMonthsAt(nowIn(2026, time.November)) // Dec 2020 .. Nov 2026
	jan := demoMonthsAt(nowIn(2027, time.January))  // Feb 2021 .. Jan 2027
	cases := []struct {
		name    string
		months  []time.Time
		w       demoWhen
		wantIdx int
		want    demoPlace
	}{
		{"this November is the last month", nov, at(0, time.November), 71, demoInWindow},
		{"this December is not seeded yet", nov, at(0, time.December), 0, demoAfterNow},
		{"oldest December is the first month", nov, at(6, time.December), 0, demoInWindow},
		{"oldest November is before the window", nov, at(6, time.November), 0, demoBeforeWindow},
		{"January 2025", nov, at(1, time.January), 49, demoInWindow},
		{"this January is the last month", jan, at(0, time.January), 71, demoInWindow},
		{"this February is not seeded yet", jan, at(0, time.February), 0, demoAfterNow},
		{"oldest January is before the window", jan, at(6, time.January), 0, demoBeforeWindow},
		{"oldest February is the first month", jan, at(6, time.February), 0, demoInWindow},
		{"a year before the oldest row is before", jan, at(7, time.June), 0, demoBeforeWindow},
		{"a maturity next year is after now", jan, at(-1, time.April), 0, demoAfterNow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			idx, place := tc.w.locate(tc.months)
			if place != tc.want || (place == demoInWindow && idx != tc.wantIdx) {
				t.Errorf("locate = (%d, %d), want (%d, %d)", idx, place, tc.wantIdx, tc.want)
			}
		})
	}

	// date ignores the window: a loan's start is years before it, a maturity after.
	if got, want := at(7, time.June).date(nov, 5), time.Date(2019, time.June, 5, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("start date = %s, want %s", got, want)
	}
	if got, want := at(-2, time.June).date(nov, 1), time.Date(2028, time.June, 1, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("maturity date = %s, want %s", got, want)
	}
	if got, want := at(0, time.January).date(jan, 20), time.Date(2027, time.January, 20, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("this-year date across New Year = %s, want %s", got, want)
	}
}

// covers: INV-FINANCE-28
//
// Trades before the window fold into one opening lot, a trade after "now" is not
// seeded, and the snapshot quantity is the running sum of the deals recorded — so
// the snapshot always equals the ledger's net Buy/Sell quantity (INV-FINANCE-08).
func TestDemoHoldings(t *testing.T) {
	trades := []demoTrade{
		{at(6, time.January), 10}, // opening lot, before every window
		{at(2, time.March), 5},
		{at(1, time.June), -3},
		{at(0, time.December), 100}, // future for any "now" before December
	}
	nov := demoMonthsAt(nowIn(2026, time.November))
	held, deals := demoHoldings(nov, trades)
	if len(held) != len(nov) {
		t.Fatalf("held has %d months, want %d", len(held), len(nov))
	}
	if held[0] != 10 {
		t.Errorf("opening quantity = %d, want 10 (the pre-window lot)", held[0])
	}
	if got := held[len(held)-1]; got != 12 {
		t.Errorf("current quantity = %d, want 12 (10 + 5 - 3, the December Buy is not seeded yet)", got)
	}
	// Mar 2024 and Jun 2025 are months 39 and 54 of a window starting Dec 2020.
	wantDeals := []demoDeal{{0, 10}, {39, 5}, {54, -3}}
	if fmt.Sprint(deals) != fmt.Sprint(wantDeals) {
		t.Errorf("deals = %v, want %v", deals, wantDeals)
	}
	// The running sum of deals equals the held quantity at every month.
	var run int64
	next := 0
	for i := range held {
		for next < len(deals) && deals[next].idx == i {
			run += deals[next].qty
			next++
		}
		if run != held[i] {
			t.Fatalf("month %d: deals sum to %d but snapshot quantity is %d", i, run, held[i])
		}
	}

	// In December the December Buy is the current month.
	dec := demoMonthsAt(nowIn(2026, time.December))
	heldDec, _ := demoHoldings(dec, trades)
	if got := heldDec[len(heldDec)-1]; got != 112 {
		t.Errorf("December quantity = %d, want 112", got)
	}

	// A trade in the window's first month merges with the folded opening lot.
	first := demoMonthsAt(nowIn(2027, time.January)) // starts Feb 2021
	heldFirst, dealsFirst := demoHoldings(first, []demoTrade{{at(6, time.January), 4}, {at(6, time.February), 2}})
	if heldFirst[0] != 6 || len(dealsFirst) != 1 || dealsFirst[0].qty != 6 {
		t.Errorf("baseline merge: held[0]=%d deals=%v, want one opening deal of 6", heldFirst[0], dealsFirst)
	}

	// A position bought mid-history has nothing before its first Buy.
	mid, _ := demoHoldings(nov, []demoTrade{{at(2, time.March), 2}})
	if mid[38] != 0 || mid[39] != 2 {
		t.Errorf("mid-history holding: month 38 = %d, month 39 = %d, want 0 then 2", mid[38], mid[39])
	}
}

// slotOf is the (row, month-index) a demoWhen addresses in the tables.
func slotOf(w demoWhen) (int, int) { return demoYearRows - 1 - w.yearsBack, int(w.month) - 1 }

func firstNonZero(g *demoGrid) (int, int, bool) {
	for r := range g {
		for m := range g[r] {
			if g[r][m] != 0 {
				return r, m, true
			}
		}
	}
	return 0, 0, false
}

// covers: INV-FINANCE-28
//
// Every table that has a value in every month has one in every cell. The row
// width is the array type, so a row with too many entries does not compile and a
// row with too few leaves zeros behind — which this catches, here rather than in
// the nightly reset.
func TestDemoTables_DenseGridsAreComplete(t *testing.T) {
	dense := map[string]*demoGrid{
		"demoSalary":           &demoSalary,
		"demoPension":          &demoPension,
		"demoSavingsInterest":  &demoSavingsInterest,
		"demoChecking":         &demoChecking,
		"demoJointSavings":     &demoJointSavings,
		"demoUSDSavings":       &demoUSDSavings,
		"demoFamilyHome":       &demoFamilyHome,
		"demoRentalApartment":  &demoRentalApartment,
		"demoFamilyCar":        &demoFamilyCar,
		"demoMotorbike":        &demoMotorbike,
		"demoMortgage":         &demoMortgage,
		"demoParentsLoan":      &demoParentsLoan,
		"demoFriendLoan":       &demoFriendLoan,
		"demoFreelanceInvoice": &demoFreelanceInvoice,
		"demoUSDRate":          &demoUSDRate,
		"demoBBCA.prices":      &demoBBCA.prices,
		"demoASII.prices":      &demoASII.prices,
		"demoEquityGrowthFund": &demoEquityGrowthFund.prices,
		"demoMoneyMarketFund":  &demoMoneyMarketFund.prices,
		"demoGoldBar.prices":   &demoGoldBar.prices,
		"demoTLKM.prices":      &demoTLKM.prices,
	}
	for name, g := range dense {
		for r := range g {
			for m := range g[r] {
				if g[r][m] <= 0 {
					t.Errorf("%s: %s of %d years ago is %d, want a positive value (a short or malformed row?)",
						name, time.Month(m+1), demoYearRows-1-r, g[r][m])
				}
			}
		}
	}
}

// covers: INV-FINANCE-28
//
// Tables for something that starts mid-history are zero before its first month
// and non-zero from it on (a hole would drop a month from the position's trail),
// and that first month is the one its placement names.
func TestDemoTables_PartialGridsStartWhereTheyPlace(t *testing.T) {
	type partial struct {
		name   string
		grid   *demoGrid
		placed demoWhen
	}
	partials := []partial{
		{"demoDigitalGold.prices", &demoDigitalGold.prices, demoDigitalGold.trades[0].when},
		{"demoAstraBond.value", &demoAstraBond.value, demoAstraBond.placed},
		{"demoMandiriTD.value", &demoMandiriTD.value, demoMandiriTD.placed},
		{"demoBCATD.value", &demoBCATD.value, demoBCATD.placed},
	}
	for _, p := range partials {
		wr, wm := slotOf(p.placed)
		fr, fm, ok := firstNonZero(p.grid)
		if !ok || fr != wr || fm != wm {
			t.Errorf("%s: first value at (row %d, month %d), want its placement (row %d, month %d)", p.name, fr, fm, wr, wm)
			continue
		}
		for r := range p.grid {
			for m := range p.grid[r] {
				before := r < wr || (r == wr && m < wm)
				if before && p.grid[r][m] != 0 {
					t.Errorf("%s: value before placement at (row %d, month %d)", p.name, r, m)
				}
				if !before && p.grid[r][m] <= 0 {
					t.Errorf("%s: no value at (row %d, month %d) after placement", p.name, r, m)
				}
			}
		}
	}

	// Coupon tables: ORI pays monthly from the month after placement; the Astra
	// bond pays in June and December after purchase.
	oriR, oriM := slotOf(demoORI.placed)
	astraR, astraM := slotOf(demoAstraBond.placed)
	for r := range demoORI.coupons {
		for m := range demoORI.coupons[r] {
			wantORI := r > oriR || (r == oriR && m > oriM)
			if got := demoORI.coupons[r][m] > 0; got != wantORI {
				t.Errorf("demoORI.coupons (row %d, month %d): paid=%v, want %v", r, m, got, wantORI)
			}
			after := r > astraR || (r == astraR && m > astraM)
			wantAstra := after && (m == int(time.June)-1 || m == int(time.December)-1)
			if got := demoAstraBond.coupons[r][m] > 0; got != wantAstra {
				t.Errorf("demoAstraBond.coupons (row %d, month %d): paid=%v, want %v", r, m, got, wantAstra)
			}
			// Accrued resets to nothing exactly when the coupon is paid.
			if after || (r == astraR && m == astraM) {
				coupon := m == int(time.June)-1 || m == int(time.December)-1
				if reset := demoAstraBond.accrued[r][m] == 0; reset != coupon {
					t.Errorf("demoAstraBond.accrued (row %d, month %d): zero=%v, want %v", r, m, reset, coupon)
				}
			}
		}
	}

	// Inflation is entered for the last three years only; older rows are empty
	// so the assumed-inflation fallback is exercised too.
	for r := range demoInflation {
		for m := range demoInflation[r] {
			entered := r >= demoYearRows-3
			if got := demoInflation[r][m] > 0; got != entered {
				t.Errorf("demoInflation (row %d, month %d): entered=%v, want %v", r, m, got, entered)
			}
		}
	}
}

var (
	validIncomeCategories = map[string]bool{
		"salary": true, "business_income": true, "rental_income": true, "pension": true, "interest": true,
		"gift": true, "tax_refund": true, "insurance_payout": true, "other": true,
	}
	validCashTxns = map[string]bool{
		repo.TxnTypeDividend: true, repo.TxnTypeDistribution: true, repo.TxnTypeFee: true, repo.TxnTypeCoupon: true,
	}
)

func validWhen(w demoWhen) bool {
	return w.yearsBack >= 0 && w.yearsBack < demoYearRows && w.month >= time.January && w.month <= time.December
}

// covers: INV-FINANCE-28
//
// Every dated event names a real calendar slot, a plausible day, a positive
// amount and a value the database will accept.
func TestDemoTables_EventsAreWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for i, ev := range demoIncomeEvents {
		where := fmt.Sprintf("demoIncomeEvents[%d] (%s)", i, ev.description)
		if !validWhen(ev.when) {
			t.Errorf("%s: slot %+v is outside the table calendar", where, ev.when)
		}
		if ev.day < 1 || ev.day > 28 {
			t.Errorf("%s: day %d is not valid in every month", where, ev.day)
		}
		if ev.amount <= 0 {
			t.Errorf("%s: amount %d is not positive", where, ev.amount)
		}
		if ev.description == "" || !validIncomeCategories[ev.category] {
			t.Errorf("%s: bad description or category %q", where, ev.category)
		}
		if ev.regularity != "routine" && ev.regularity != "incidental" {
			t.Errorf("%s: regularity %q", where, ev.regularity)
		}
		if ev.owner < demoOwnerSole || ev.owner > demoOwnerJoint {
			t.Errorf("%s: unknown owner %d", where, ev.owner)
		}
		key := fmt.Sprintf("%v/%s/%d", ev.when, ev.description, ev.day)
		if seen[key] {
			t.Errorf("%s: duplicated", where)
		}
		seen[key] = true
	}

	markets := map[string]*demoMarket{
		"BBCA": &demoBBCA, "ASII": &demoASII, "EGF": &demoEquityGrowthFund, "TLKM": &demoTLKM,
		"MMF": &demoMoneyMarketFund, "gold bar": &demoGoldBar, "digital gold": &demoDigitalGold,
	}
	for name, p := range markets {
		for i, c := range p.cash {
			if !validWhen(c.when) || c.day < 1 || c.day > 28 || c.amount <= 0 || !validCashTxns[c.txn] {
				t.Errorf("%s cash[%d] is malformed: %+v", name, i, c)
			}
		}
		// Walk the trades oldest first: quantities are whole non-zero units, the
		// holding never goes negative, and wherever a holding exists the price
		// table has a price for the month.
		trades := append([]demoTrade(nil), p.trades...)
		sort.SliceStable(trades, func(a, b int) bool {
			ra, ma := slotOf(trades[a].when)
			rb, mb := slotOf(trades[b].when)
			return ra*12+ma < rb*12+mb
		})
		var held int64
		next := 0
		for slot := 0; slot < demoYearRows*12; slot++ {
			for next < len(trades) {
				r, m := slotOf(trades[next].when)
				if r*12+m != slot {
					break
				}
				if !validWhen(trades[next].when) || trades[next].qty == 0 {
					t.Errorf("%s trade %+v is malformed", name, trades[next])
				}
				held += trades[next].qty
				next++
			}
			if held < 0 {
				t.Errorf("%s: holding goes negative (%d) at (row %d, month %d)", name, held, slot/12, slot%12)
			}
			if held > 0 && p.prices[slot/12][slot%12] <= 0 {
				t.Errorf("%s: holds %d at (row %d, month %d) but has no price", name, held, slot/12, slot%12)
			}
		}
		if next != len(trades) {
			t.Errorf("%s: %d trades never reached", name, len(trades)-next)
		}
	}
}

// covers: INV-FINANCE-28
//
// Whatever month it is, everything bought mid-history is already placed (inside
// the window, not in the future) and everything with a term is still running —
// a time deposit's snapshots must fall inside its term, and a matured bond would
// need a settlement the demo does not carry.
func TestDemoTables_PlacementsAndTermsHoldForEveryNow(t *testing.T) {
	for _, now := range everyMonthNow() {
		months := demoMonthsAt(now)
		last := months[len(months)-1]
		label := now.Format("2006-01")
		placed := map[string]demoWhen{
			"ORI024":             demoORI.placed,
			"Astra Sedaya":       demoAstraBond.placed,
			"Mandiri TD":         demoMandiriTD.placed,
			"BCA TD":             demoBCATD.placed,
			"Digital Gold":       demoDigitalGold.trades[0].when,
			"BNI TD":             demoBNIDeposit.placed,
			"BNI renewal":        demoBNIRenewal.placed,
			"Danamon TD":         demoDanamonDeposit.placed,
			"Old Hatchback sale": demoOldHatchback.closing.when,
			"TLKM sale":          demoTLKM.closing.when,
		}
		for name, w := range placed {
			if _, place := w.locate(months); place != demoInWindow {
				t.Errorf("now=%s: %s placement is not inside the window (place %d)", label, name, place)
			}
		}
		ends := map[string]time.Time{
			"ORI024":       demoORI.matures.date(months, 10),
			"Astra Sedaya": demoAstraBond.matures.date(months, 1),
			"Mandiri TD":   demoMandiriTD.placed.date(months, 12).AddDate(0, demoMandiriTD.termMonths, 0),
			"BCA TD":       demoBCATD.placed.date(months, 12).AddDate(0, demoBCATD.termMonths, 0),
			"BNI renewal":  demoBNIRenewal.placed.date(months, 12).AddDate(0, demoBNIRenewal.termMonths, 0),
		}
		// What matured or was sold must already be over: inside the window and
		// before the current month, so a closed position never reads as future.
		for name, w := range map[string]demoWhen{
			"BNI maturity":     {demoBNIDeposit.placed.yearsBack - demoBNIDeposit.termMonths/12, demoBNIDeposit.placed.month},
			"Danamon maturity": {demoDanamonDeposit.placed.yearsBack - demoDanamonDeposit.termMonths/12, demoDanamonDeposit.placed.month},
			"hatchback sale":   demoOldHatchback.closing.when,
			"TLKM sale":        demoTLKM.closing.when,
		} {
			if idx, place := w.locate(months); place != demoInWindow || idx >= len(months)-1 {
				t.Errorf("now=%s: %s is not safely in the past (idx %d, place %d)", label, name, idx, place)
			}
		}
		for name, end := range ends {
			if !end.After(last.AddDate(0, 1, 0)) {
				t.Errorf("now=%s: %s matures %s, not after the current month", label, name, end.Format("2006-01-02"))
			}
		}
	}
	// ORI024 matures exactly three years after it is placed.
	months := demoMonthsAt(nowIn(2026, time.November))
	if got, want := demoORI.matures.date(months, 10), demoORI.placed.date(months, 10).AddDate(3, 0, 0); !got.Equal(want) {
		t.Errorf("ORI024 matures %s, want three years after placement (%s)", got, want)
	}
}

// covers: INV-FINANCE-28
//
// Seasonal anchors land on their calendar month for any "now": THR (the Eid
// bonus) in April, the year-end bonus in December, at a different size each year.
func TestDemoIncome_SeasonalAnchors(t *testing.T) {
	var thr, bonus []demoIncomeEvent
	for _, ev := range demoIncomeEvents {
		switch ev.description {
		case "THR (religious holiday bonus)":
			thr = append(thr, ev)
		case "Year-end bonus":
			bonus = append(bonus, ev)
		}
	}
	if len(thr) != demoYearRows {
		t.Fatalf("%d THR events, want one per year row (%d)", len(thr), demoYearRows)
	}
	sizes := map[int64]bool{}
	for _, ev := range thr {
		if ev.when.month != time.April {
			t.Errorf("THR in %s, want April", ev.when.month)
		}
		sizes[ev.amount] = true
	}
	if len(sizes) != len(thr) {
		t.Errorf("THR has %d distinct sizes across %d years, want a different size each year", len(sizes), len(thr))
	}
	for _, ev := range bonus {
		if ev.when.month != time.December {
			t.Errorf("year-end bonus in %s, want December", ev.when.month)
		}
	}

	cases := []struct {
		name      string
		now       time.Time
		wantYears []int // calendar years of the April THRs seeded
	}{
		{"November", nowIn(2026, time.November), []int{2021, 2022, 2023, 2024, 2025, 2026}},
		{"January: this April is not seeded yet", nowIn(2027, time.January), []int{2021, 2022, 2023, 2024, 2025, 2026}},
		{"March: this April is not seeded yet, the oldest April is the first month", nowIn(2027, time.March), []int{2021, 2022, 2023, 2024, 2025, 2026}},
		{"April: this April is the current month", nowIn(2027, time.April), []int{2022, 2023, 2024, 2025, 2026, 2027}},
		{"December: the oldest row is unused", nowIn(2027, time.December), []int{2022, 2023, 2024, 2025, 2026, 2027}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			months := demoMonthsAt(tc.now)
			var years []int
			for _, ev := range thr {
				idx, place := ev.when.locate(months)
				if place != demoInWindow {
					continue
				}
				if months[idx].Month() != time.April {
					t.Errorf("THR resolved to %s, want April", months[idx].Format("2006-01"))
				}
				years = append(years, months[idx].Year())
			}
			sort.Ints(years)
			if fmt.Sprint(years) != fmt.Sprint(tc.wantYears) {
				t.Errorf("THR years = %v, want %v", years, tc.wantYears)
			}
		})
	}
}

// covers: INV-FINANCE-28
//
// The story the tables tell: the Family Home is revalued only in January (once
// by a lot), the Rental Apartment only in July, debts amortise down and never
// below zero, the market slid early "1 year ago" and was back by August, and the
// cash and savings balances stay solvent in every month.
func TestDemoTables_StoryHolds(t *testing.T) {
	flat := func(name string, g *demoGrid, onlyChangesIn int) (jumps []float64) {
		for r := range g {
			for m := range g[r] {
				pr, pm := r, m-1
				if m == 0 {
					pr, pm = r-1, 11
				}
				if pr < 0 || g[r][m] == g[pr][pm] {
					continue
				}
				if m != onlyChangesIn {
					t.Errorf("%s changes in %s of %d years ago, want only month index %d", name, time.Month(m+1), demoYearRows-1-r, onlyChangesIn)
				}
				jumps = append(jumps, float64(g[r][m])/float64(g[pr][pm])-1)
			}
		}
		return jumps
	}
	biggest := 0.0
	for _, j := range flat("demoFamilyHome", &demoFamilyHome, 0) {
		if j > biggest {
			biggest = j
		}
	}
	if biggest < 0.08 {
		t.Errorf("largest January revaluation is %.1f%%, want one big one (>= 8%%)", biggest*100)
	}
	flat("demoRentalApartment", &demoRentalApartment, 6)

	// Liabilities and receivables only ever step down, never below zero; the
	// mortgage amortises every single month.
	steps := map[string]*demoGrid{
		"demoMortgage": &demoMortgage, "demoParentsLoan": &demoParentsLoan,
		"demoFriendLoan": &demoFriendLoan, "demoFreelanceInvoice": &demoFreelanceInvoice,
	}
	for name, g := range steps {
		prev := int64(1 << 62)
		for r := range g {
			for m := range g[r] {
				v := g[r][m]
				if v < 0 || v > prev {
					t.Errorf("%s: %d at (row %d, month %d) after %d", name, v, r, m, prev)
				}
				if name == "demoMortgage" && v == prev {
					t.Errorf("demoMortgage did not amortise at (row %d, month %d)", r, m)
				}
				prev = v
			}
		}
	}

	// Solvent cash: everyday checking never runs dry, savings never thin out.
	for r := range demoChecking {
		for m := range demoChecking[r] {
			if v := demoChecking[r][m]; v < 5_000_000 || v > 40_000_000 {
				t.Errorf("demoChecking (row %d, month %d) = %d, want 5M..40M", r, m, v)
			}
			if v := demoJointSavings[r][m]; v < 10_000_000 {
				t.Errorf("demoJointSavings (row %d, month %d) = %d, want at least 10M", r, m, v)
			}
		}
	}

	// The slide "1 year ago": BBCA, ASII and the equity fund all fall more than
	// 10% from January into April and are back within 10% of January by August.
	slide := map[string]*demoGrid{"BBCA": &demoBBCA.prices, "ASII": &demoASII.prices, "EGF": &demoEquityGrowthFund.prices}
	row := demoYearRows - 2
	for name, g := range slide {
		jan, apr, aug := float64(g[row][0]), float64(g[row][3]), float64(g[row][7])
		if apr > 0.90*jan {
			t.Errorf("%s: April (%v) is not more than 10%% below January (%v)", name, apr, jan)
		}
		if aug < 0.90*jan {
			t.Errorf("%s: August (%v) has not recovered to within 10%% of January (%v)", name, aug, jan)
		}
	}
}

// slotIndex is a demoWhen's position in the flattened table calendar (row*12+month).
func slotIndex(w demoWhen) int {
	r, m := slotOf(w)
	return r*12 + m
}

// covers: INV-FINANCE-28
//
// Closed and matured positions are recorded up to the month before they closed
// and are zero from then on (the lifecycle flip writes the close snapshot, not the
// table); the rollover successor starts on the predecessor's maturity month with
// what rolled; and the interest paid at maturity matches what accrued.
func TestDemoTables_ClosedAndMaturedPositions(t *testing.T) {
	flat := func(g *demoGrid) []int64 {
		var out []int64
		for r := range g {
			out = append(out, g[r][:]...)
		}
		return out
	}
	assertSpan := func(name string, g *demoGrid, first, end int) { // values in [first, end), zero elsewhere
		for slot, v := range flat(g) {
			inside := slot >= first && slot < end
			if inside && v <= 0 {
				t.Errorf("%s: no value at slot %d inside its life [%d, %d)", name, slot, first, end)
			}
			if !inside && v != 0 {
				t.Errorf("%s: value %d at slot %d outside its life [%d, %d)", name, v, slot, first, end)
			}
		}
	}

	// The sold hatchback is recorded from the first cell to the month before its
	// sale: row 0's January is the first cell, and it is zero from the sale on.
	assertSpan("demoOldHatchback", &demoOldHatchback.value, 0, slotIndex(demoOldHatchback.closing.when))

	bniEnd := slotIndex(demoBNIDeposit.placed) + demoBNIDeposit.termMonths
	danEnd := slotIndex(demoDanamonDeposit.placed) + demoDanamonDeposit.termMonths
	assertSpan("demoBNIDeposit", &demoBNIDeposit.value, slotIndex(demoBNIDeposit.placed), bniEnd)
	assertSpan("demoDanamonDeposit", &demoDanamonDeposit.value, slotIndex(demoDanamonDeposit.placed), danEnd)
	assertSpan("demoBNIRenewal", &demoBNIRenewal.value, slotIndex(demoBNIRenewal.placed), demoYearRows*12)

	// The successor is placed in the predecessor's maturity month, with a term
	// that is still running.
	if got := slotIndex(demoBNIRenewal.placed); got != bniEnd {
		t.Errorf("renewal is placed at slot %d, want the predecessor's maturity slot %d", got, bniEnd)
	}

	// Interest at maturity covers what had accrued by the month before and is no
	// more than one more month of it (5.0% on 15M is 62.5k a month; 5.25% on 10M is
	// 43.75k), so the Maturity transaction's interest is believable against the
	// snapshots. The successor opens with the rolled amount plus under a month of
	// interest.
	for _, c := range []struct {
		name         string
		d            *demoMaturedDeposit
		end          int
		monthlyAccru int64
	}{
		{"BNI", &demoBNIDeposit, bniEnd, 62_500},
		{"Danamon", &demoDanamonDeposit, danEnd, 43_750},
	} {
		last := flat(&c.d.value)[c.end-1]
		accrued := last - c.d.principal
		if c.d.interest < accrued || c.d.interest > accrued+c.monthlyAccru {
			t.Errorf("%s: maturity interest %d vs %d accrued a month before — want within one month (%d) above it", c.name, c.d.interest, accrued, c.monthlyAccru)
		}
	}
	rolled := demoBNIDeposit.principal + demoBNIDeposit.interest
	if first := flat(&demoBNIRenewal.value)[slotIndex(demoBNIRenewal.placed)]; first <= rolled || first > rolled+72_000 {
		t.Errorf("renewal opens at %d, want the rolled %d plus under a month of 5%% interest", first, rolled)
	}
}

// covers: INV-FINANCE-28
//
// Positions left stale are named by display name, behind by a few trailing months
// (not the whole history), and investments among them have no trade or payout in
// any month they could be stale in — a carried-forward value is only harmless to
// the derived expenses when no cash moved in it.
func TestDemoNotUpdated(t *testing.T) {
	want := map[string]bool{"Family Car": true, "Motorbike": true, "Digital Gold Savings": true, "Money Lent to a Friend": true}
	for name, behind := range demoNotUpdated {
		if !want[name] {
			t.Errorf("unexpected stale position %q", name)
		}
		if behind < 1 || behind > 6 {
			t.Errorf("%q is %d months behind, want 1..6", name, behind)
		}
	}
	if len(demoNotUpdated) != len(want) {
		t.Errorf("%d positions not updated, want %d", len(demoNotUpdated), len(want))
	}

	// The window always ends in the newest row; the trailing months it can be
	// stale in are the newest row's first months and the previous row's last ones.
	behind := demoNotUpdated["Digital Gold Savings"]
	for _, tr := range demoDigitalGold.trades {
		if tr.when.yearsBack == 0 || (tr.when.yearsBack == 1 && int(tr.when.month) > 12-behind) {
			t.Errorf("digital gold trade %+v falls in a month it can be stale in", tr)
		}
	}
	for _, c := range demoDigitalGold.cash {
		if c.when.yearsBack == 0 || (c.when.yearsBack == 1 && int(c.when.month) > 12-behind) {
			t.Errorf("digital gold cash event %+v falls in a month it can be stale in", c)
		}
	}

	// demoRecorded drops exactly that many trailing months.
	months := demoMonthsAt(nowIn(2026, time.November))
	if got := demoRecorded(months, "Motorbike"); got != demoMonthCount-3 {
		t.Errorf("Motorbike recorded %d months, want %d", got, demoMonthCount-3)
	}
	if got := demoRecorded(months, "Everyday Checking"); got != demoMonthCount {
		t.Errorf("an up-to-date position records %d months, want all %d", got, demoMonthCount)
	}
}
