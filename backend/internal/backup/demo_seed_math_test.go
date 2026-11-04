package backup

import (
	"math"
	"testing"
	"time"
)

// These are container-free unit tests for the pure cash-flow / series maths the
// demo seeder is built on. The integration test (TestSeedDemoData_Reconciles)
// proves the whole thing wires together and reconciles, but it is blind to any
// helper whose output *cancels* in the engine's living-expenses identity —
// property/vehicle revaluation and the USD receivable step-down both do — so a
// wrong formula there would still pass it. These pin the maths directly.

const floatEps = 1e-6

func approxEqual(a, b float64) bool { return math.Abs(a-b) <= floatEps+1e-9*math.Abs(b) }

func TestCumulativeQty(t *testing.T) {
	trades := []demoTrade{{0, 10}, {5, 6}, {20, -2}}
	cases := []struct {
		upto int
		want float64
	}{
		{-1, 0},  // before the opening lot
		{0, 10},  // opening lot lands at month 0
		{4, 10},  // flat until the next trade
		{5, 16},  // top-up buy
		{19, 16}, // flat until the sell
		{20, 14}, // partial sell reduces the running total
		{99, 14}, // stays put past the last trade
	}
	for _, c := range cases {
		if got := cumulativeQty(trades, c.upto); !approxEqual(got, c.want) {
			t.Errorf("cumulativeQty(upto=%d) = %v, want %v", c.upto, got, c.want)
		}
	}
	// The net of the whole schedule is what the final snapshot must equal, so the
	// frontend reconcileQuantity check holds by construction.
	if got := cumulativeQty(trades, 26); !approxEqual(got, 14) {
		t.Errorf("final net qty = %v, want 14", got)
	}
}

func TestDemoStepDownSeries(t *testing.T) {
	got := demoStepDownSeries(1000, 50, []int{2, 5}, 8)
	want := []float64{1000, 1000, 950, 950, 950, 900, 900, 900}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if !approxEqual(got[i], want[i]) {
			t.Errorf("month %d = %v, want %v (should be flat between payments, stepping by the whole nominal)", i, got[i], want[i])
		}
	}

	// No payments → dead flat at base.
	for i, v := range demoStepDownSeries(500, 50, nil, 5) {
		if !approxEqual(v, 500) {
			t.Errorf("no-payment series month %d = %v, want flat 500", i, v)
		}
	}

	// Payment months at/after the series end are simply not reached.
	tail := demoStepDownSeries(100, 10, []int{1, 9}, 3)
	if !approxEqual(tail[2], 90) {
		t.Errorf("out-of-range payment leaked in: month 2 = %v, want 90", tail[2])
	}
}

func TestDemoRevaluationSeries(t *testing.T) {
	// Appreciation: month 0 is the base, a full year compounds by exactly the
	// annual rate, matching the app's own revaluation.ts: base*(1+r/100)^(i/12).
	appr := demoRevaluationSeries(250_000_000, 4, 25)
	if !approxEqual(appr[0], 250_000_000) {
		t.Errorf("month 0 = %v, want base 250000000", appr[0])
	}
	if !approxEqual(appr[12], 250_000_000*1.04) {
		t.Errorf("month 12 = %v, want 260000000 (one year at 4%%)", appr[12])
	}
	if !approxEqual(appr[24], 250_000_000*1.04*1.04) {
		t.Errorf("month 24 = %v, want 270400000 (two years at 4%%)", appr[24])
	}

	// Depreciation: a negative rate declines, and 12 months compounds to exactly
	// (1 − rate) — a vehicle passing -8 must lose 8% over the year.
	dep := demoRevaluationSeries(50_000_000, -8, 13)
	if !approxEqual(dep[12], 50_000_000*0.92) {
		t.Errorf("depreciation month 12 = %v, want 46000000 (one year at -8%%)", dep[12])
	}
	if dep[6] >= dep[0] {
		t.Errorf("depreciating series should decline: month 6 %v not < month 0 %v", dep[6], dep[0])
	}
}

func TestDemoExpenseSeries(t *testing.T) {
	e := demoExpenseSeries()
	if len(e) != demoMonthCount {
		t.Fatalf("len = %d, want demoMonthCount %d", len(e), demoMonthCount)
	}
	// Monthly income floor is salary(5M) + pension(2.5M) + interest(0.3M). This is
	// a *cash* floor, so it counts the interest regardless of its regularity.
	// Every expense must sit under it so the checking plug trends upward, and
	// stay comfortably positive so no month reads as negative spending.
	const monthlyIncomeFloor = 5_000_000 + 2_500_000 + 300_000
	for i, v := range e {
		if v <= 0 {
			t.Errorf("month %d expense %v is not positive", i, v)
		}
		if v >= monthlyIncomeFloor {
			t.Errorf("month %d expense %v >= monthly income floor %v — checking would not trend up", i, v, monthlyIncomeFloor)
		}
	}
}

func TestRecordSeriesDelta(t *testing.T) {
	dst := make([]float64, 4)
	recordSeriesDelta(dst, []float64{100, 130, 130, 90})
	// Index 0 is never written (there is no prior month to diff); the rest are
	// month-over-month changes.
	want := []float64{0, 30, 0, -40}
	for i := range want {
		if !approxEqual(dst[i], want[i]) {
			t.Errorf("delta[%d] = %v, want %v", i, dst[i], want[i])
		}
	}
	// A second series accumulates on top of the first (both feed the same bucket).
	recordSeriesDelta(dst, []float64{0, 5, 5, 5})
	if !approxEqual(dst[1], 35) {
		t.Errorf("accumulated delta[1] = %v, want 35 (30 + 5)", dst[1])
	}
}

func TestCheckingSeriesOpensAtBalance(t *testing.T) {
	l := newDemoCashLedger()
	c := l.checkingSeries()
	if len(c) != demoMonthCount {
		t.Fatalf("len = %d, want %d", len(c), demoMonthCount)
	}
	if !approxEqual(c[0], demoCheckingOpening) {
		t.Errorf("baseline balance = %v, want opening %v", c[0], demoCheckingOpening)
	}
}

// TestCheckingSeriesClosesTheIdentity is the unit-level statement of
// INV-FINANCE-28: the checking plug is defined so that, rearranged, the engine's
// comprehensive-income identity reproduces the chosen expense series exactly. If
// checkingSeries's recurrence ever drifts from that identity, the demo's derived
// Living Expenses would stop matching demoExpenseSeries — this catches it without
// a database.
//
// covers: INV-FINANCE-28
func TestCheckingSeriesClosesTheIdentity(t *testing.T) {
	l := newDemoCashLedger()
	// Populate every cash-flow bucket with distinct, non-trivial values so the
	// reconstruction can't pass by accident (e.g. from a dropped/zero term).
	for i := 1; i < demoMonthCount; i++ {
		l.income[i] = 7_800_000 + float64(i)*10_000
		l.investNetIn[i] = float64((i%4)-1) * 1_500_000 // mix of buys (+) and payouts (−)
		l.savingsDelta[i] = 300_000
		l.recvDelta[i] = float64((i%3)-1) * 40_000
		l.liabDelta[i] = -250_000 * float64(i%2) // debt paid down on alternating months
	}

	c := l.checkingSeries()
	for i := 1; i < demoMonthCount; i++ {
		deltaChecking := c[i] - c[i-1]
		// The identity solved for expenses:
		//   expenses = income − Δchecking − Δsavings − Δreceivables + Δliabilities − netInvestCashIn
		reconstructed := l.income[i] - deltaChecking - l.savingsDelta[i] - l.recvDelta[i] + l.liabDelta[i] - l.investNetIn[i]
		if !approxEqual(reconstructed, l.expenses[i]) {
			t.Errorf("month %d: identity does not close — reconstructed expenses %.4f != chosen %.4f", i, reconstructed, l.expenses[i])
		}
	}
}

func TestDemoCashLedgerAccumulates(t *testing.T) {
	l := newDemoCashLedger()
	l.addIncome(3, 5_000_000)
	l.addIncome(3, 2_500_000) // same month, different source — must sum
	if !approxEqual(l.income[3], 7_500_000) {
		t.Errorf("income[3] = %v, want 7500000", l.income[3])
	}
	// addInvest is signed: a Buy adds, a payout subtracts, into the same month.
	l.addInvest(6, 3_000_000)  // buy
	l.addInvest(6, -52_083.33) // coupon paid out that month
	if !approxEqual(l.investNetIn[6], 2_947_916.67) {
		t.Errorf("investNetIn[6] = %v, want 2947916.67", l.investNetIn[6])
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
