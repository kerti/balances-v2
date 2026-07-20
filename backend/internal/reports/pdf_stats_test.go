package reports

import (
	"math"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/repo"
	"github.com/kerti/balances-v2/backend/internal/reports/pdf"
)

// flowMonth builds a report row carrying income-statement flows (a non-baseline
// month). NwInvestments is the reported stock; the *Total flows drive the
// trailing-12 averages.
// flowMonth's income is treated as fully routine (the routine subtotals mirror
// the totals) — the statistics read the *_routine columns (ADR-0048 amendment),
// so a fixture with no incidental income sets them equal. TestBuildStats_
// IncidentalIncomeExcluded exercises the routine < total case.
func flowMonth(m time.Time, income, expenses, rental, pension, invReturn, nwInv string) db.MonthlyReport {
	return db.MonthlyReport{
		YearMonth:                  m,
		NwInvestments:              dec(nwInv),
		EarnedIncomeTotal:          decp(income),
		EarnedIncomeRental:         decp(rental),
		EarnedIncomePension:        decp(pension),
		InvestmentReturnTotal:      decp(invReturn),
		DerivedLivingExpenses:      decp(expenses),
		EarnedIncomeTotalRoutine:   decp(income),
		EarnedIncomeRentalRoutine:  decp(rental),
		EarnedIncomePensionRoutine: decp(pension),
	}
}

func bank(amount string) repo.PositionDetail {
	return repo.PositionDetail{Group: "asset", Subtype: "bank_account", Amount: dec(amount)}
}

// covers: INV-FINANCE-19
func TestBuildStats_FourRatios_SingleMonth(t *testing.T) {
	m := ym(2026, time.June)
	series := []db.MonthlyReport{flowMonth(m, "1000", "600", "100", "50", "200", "10000")}
	row := &series[0]
	positions := []repo.PositionDetail{
		bank("500"),
		{Group: "asset", Subtype: "property", Amount: dec("999999")}, // not bank cash — excluded
	}

	st := buildStats(row, positions, series, nil, decimal.Zero)

	// Cash-Flow = (1000 − 600)/1000 = 40%.
	assertPct(t, "cash-flow", st.CashFlow, 40)
	// Passive-Income = TotalPassive(100+50+200) ÷ 600 = 58.33%.
	assertPct(t, "passive-income", st.PassiveIncome, 350.0/600*100)
	// Instant-Liquidity = bank 500 ÷ investments 10000 = 5%.
	assertPct(t, "instant-liquidity", st.InstantLiquidity, 5)
	// Fund Resilience is finite here (draw exceeds pool growth).
	if !st.Resilience.Defined || st.Resilience.Indefinite || st.Resilience.Months <= 0 {
		t.Errorf("resilience: got %+v, want a finite positive runway", st.Resilience)
	}
}

// The reproducibility block must let the reader recompute the two flow ratios
// by hand: Cash-Flow = (AvgIncome − AvgExpenses)/AvgIncome, Passive-Income =
// AvgPassive/AvgExpenses. The surfaced averages must be the exact operands.
// covers: INV-FINANCE-27
func TestBuildStats_InputsReproduceFlowRatios(t *testing.T) {
	// Two flow months so the averages are genuine means, not a single month.
	series := []db.MonthlyReport{
		flowMonth(ym(2026, time.May), "1000", "600", "100", "50", "200", "10000"),
		flowMonth(ym(2026, time.June), "1400", "800", "140", "60", "300", "10000"),
	}
	row := &series[1]

	st := buildStats(row, []repo.PositionDetail{bank("500")}, series, nil, decimal.Zero)

	in := st.Inputs
	if !in.Defined {
		t.Fatal("inputs: want defined")
	}
	if in.Months != 2 {
		t.Errorf("months: got %d, want 2", in.Months)
	}
	// Averages over the two months: income (1000+1400)/2=1200, expenses
	// (600+800)/2=700, total passive ((100+50+200)+(140+60+300))/2=425.
	if in.AvgIncome != "1200" || in.AvgExpenses != "700" || in.AvgPassive != "425" {
		t.Fatalf("avgs: got income=%q expenses=%q passive=%q, want 1200/700/425",
			in.AvgIncome, in.AvgExpenses, in.AvgPassive)
	}
	// Plugging the surfaced averages into the formulas reproduces the ratios.
	ai, ae, ap := dec(in.AvgIncome), dec(in.AvgExpenses), dec(in.AvgPassive)
	wantCash, _ := ai.Sub(ae).Div(ai).Mul(decimal.NewFromInt(100)).Float64()
	wantPass, _ := ap.Div(ae).Mul(decimal.NewFromInt(100)).Float64()
	assertPct(t, "cash-flow reproduced", st.CashFlow, wantCash)
	assertPct(t, "passive-income reproduced", st.PassiveIncome, wantPass)
}

// covers: INV-FINANCE-20
func TestBuildStats_UndefinedEdges(t *testing.T) {
	m := ym(2026, time.June)

	t.Run("baseline / no flow month → flow ratios undefined, liquidity still defined", func(t *testing.T) {
		// A baseline month has no derived expenses; it is skipped as a flow month.
		baseline := db.MonthlyReport{YearMonth: m, NwInvestments: dec("10000")}
		series := []db.MonthlyReport{baseline}
		st := buildStats(&baseline, []repo.PositionDetail{bank("500")}, series, nil, decimal.Zero)
		if st.CashFlow.Defined || st.PassiveIncome.Defined || st.Resilience.Defined {
			t.Errorf("flow ratios should be undefined with no flow month: %+v", st)
		}
		if !st.InstantLiquidity.Defined { // a stock — defined on the baseline
			t.Error("instant-liquidity should be defined from the reported stock")
		}
	})

	t.Run("income ≤ 0 → cash-flow undefined", func(t *testing.T) {
		series := []db.MonthlyReport{flowMonth(m, "0", "600", "0", "0", "0", "10000")}
		st := buildStats(&series[0], nil, series, nil, decimal.Zero)
		if st.CashFlow.Defined {
			t.Error("cash-flow should be undefined when income ≤ 0")
		}
	})

	t.Run("living expenses ≤ 0 → passive-income undefined", func(t *testing.T) {
		series := []db.MonthlyReport{flowMonth(m, "1000", "0", "100", "0", "0", "10000")}
		st := buildStats(&series[0], nil, series, nil, decimal.Zero)
		if st.PassiveIncome.Defined {
			t.Error("passive-income should be undefined when living expenses ≤ 0")
		}
	})

	t.Run("no investments → liquidity and resilience undefined", func(t *testing.T) {
		series := []db.MonthlyReport{flowMonth(m, "1000", "600", "0", "0", "0", "0")}
		st := buildStats(&series[0], []repo.PositionDetail{bank("500")}, series, nil, decimal.Zero)
		if st.InstantLiquidity.Defined || st.Resilience.Defined {
			t.Errorf("liquidity/resilience should be undefined with zero investments: %+v", st)
		}
	})
}

// covers: INV-FINANCE-20
func TestBuildStats_NullFlowColumnsTreatedAsZero(t *testing.T) {
	// A partially-populated flow month — the nullable income columns are NULL
	// (nil pointers) while expenses and the investment stock are present. decOr
	// must read the absent columns as zero rather than nil-deref, so the panel
	// still computes: income 0 leaves cash-flow undefined, passive income is a
	// defined 0% (no passive cash, no investment return), and the stock-based
	// gauges stay defined.
	m := ym(2026, time.June)
	partial := db.MonthlyReport{
		YearMonth:             m,
		NwInvestments:         dec("10000"),
		DerivedLivingExpenses: decp("600"),
		// EarnedIncomeTotal / Rental / Pension / InvestmentReturnTotal all nil.
	}
	series := []db.MonthlyReport{partial}

	st := buildStats(&partial, []repo.PositionDetail{bank("500")}, series, nil, decimal.Zero)

	if st.CashFlow.Defined {
		t.Error("cash-flow should be undefined when income column is NULL (read as 0)")
	}
	assertPct(t, "passive-income", st.PassiveIncome, 0)
	assertPct(t, "instant-liquidity", st.InstantLiquidity, 5)
	if !st.Resilience.Defined {
		t.Errorf("resilience should be defined from the stock + flow month, got %+v", st.Resilience)
	}
}

func TestBuildStats_NegativePassiveIncome(t *testing.T) {
	// A market loss (negative Investment Return) can drag total passive income —
	// and the ratio — negative. This is intended and labelled (ADR-0048).
	m := ym(2026, time.June)
	series := []db.MonthlyReport{flowMonth(m, "1000", "600", "50", "0", "-500", "10000")}
	st := buildStats(&series[0], nil, series, nil, decimal.Zero)
	if !st.PassiveIncome.Defined || st.PassiveIncome.Percent >= 0 {
		t.Errorf("passive-income should be defined and negative, got %+v", st.PassiveIncome)
	}
}

// covers: INV-FINANCE-21
func TestSimulateResilience(t *testing.T) {
	t.Run("flat pool depletes at pool/expenses", func(t *testing.T) {
		// P0=1000, draw=100, no growth, no inflation → exactly 10 months.
		r := simulateResilience(1000, 100, 0, 0, 0)
		if !r.Defined || r.Indefinite || r.Months != 10 {
			t.Errorf("got %+v, want finite 10 months", r)
		}
	})

	t.Run("passive income covering expenses → indefinite", func(t *testing.T) {
		r := simulateResilience(1000, 50, 100, 0, 0) // draw is negative, pool grows
		if !r.Defined || !r.Indefinite {
			t.Errorf("got %+v, want indefinite", r)
		}
	})

	t.Run("horizon cap reads as indefinite", func(t *testing.T) {
		// Tiny net draw against a large pool survives the ~100-year horizon.
		r := simulateResilience(1_000_000, 100, 0, 0, 0)
		if !r.Indefinite {
			t.Errorf("got %+v, want indefinite past the horizon", r)
		}
	})
}

// covers: INV-FINANCE-21
func TestBuildStats_ResilienceExcludesInvestmentReturnFromDraw(t *testing.T) {
	// The draw-offset is passive *cash* income (Rental + Pension) only — Investment
	// Return is already the pool's growth g, so it must not also reduce the draw
	// (the double-count the two-scope split guards). Pinning the exact runway
	// catches a regression that folds Investment Return back into the offset.
	m := ym(2026, time.June)

	// No passive cash, g = 10/1000 = 0.01, i = 0, expenses 200, pool 1000.
	noOffset := []db.MonthlyReport{flowMonth(m, "500", "200", "0", "0", "10", "1000")}
	base := buildStats(&noOffset[0], nil, noOffset, nil, decimal.Zero).Resilience
	if base.Months != 6 {
		t.Errorf("no-offset runway: got %+v, want 6 months", base)
	}

	// Same, but 50 of continuing Pension cash offsets the draw → longer runway.
	withPension := []db.MonthlyReport{flowMonth(m, "500", "200", "0", "50", "10", "1000")}
	ext := buildStats(&withPension[0], nil, withPension, nil, decimal.Zero).Resilience
	if ext.Months != 7 {
		t.Errorf("pension-offset runway: got %+v, want 7 months (pension extends it)", ext)
	}
}

// covers: INV-FINANCE-22
func TestBuildStats_InterestCountsAsPassiveCash(t *testing.T) {
	// Bank/deposit interest is passive *cash* income (external, not pool return),
	// so it offsets the resilience draw exactly like Rental/Pension and — because
	// it never appears in InvestmentReturn — carries no double-count. Mirrors the
	// Pension pin above (INV-FINANCE-21): a 50-cash offset extends 6 → 7 months.
	m := ym(2026, time.June)

	noOffset := []db.MonthlyReport{flowMonth(m, "500", "200", "0", "0", "10", "1000")}
	base := buildStats(&noOffset[0], nil, noOffset, nil, decimal.Zero).Resilience
	if base.Months != 6 {
		t.Errorf("no-offset runway: got %+v, want 6 months", base)
	}

	withInterest := noOffset[0]
	withInterest.EarnedIncomeInterest = decp("50")
	withInterest.EarnedIncomeInterestRoutine = decp("50") // routine passive cash — the stats read the routine column
	series := []db.MonthlyReport{withInterest}
	ext := buildStats(&series[0], nil, series, nil, decimal.Zero).Resilience
	if ext.Months != 7 {
		t.Errorf("interest-offset runway: got %+v, want 7 months (interest extends it)", ext)
	}
}

// covers: INV-FINANCE-25
func TestBuildStats_PaidOutCouponIsPassiveCashNotPoolGrowth(t *testing.T) {
	// A paid-out bond coupon is dependable external cash: it offsets the resilience
	// draw like Rental/Pension/Interest and must NOT also compound in the pool's
	// own-return g. The coupon slice lives inside InvestmentReturnTotal (the domain
	// keeps coupon yield in investment return), so buildStats adds PassiveCouponCash
	// to passive cash and subtracts it from own return. Pinning the exact runway
	// catches a regression that leaves the coupon in g or drops it from the offset.
	m := ym(2026, time.June)

	// Baseline: own return 10 on a 1000 pool → g = 0.01, draw 200 → 6-month runway.
	noCoupon := []db.MonthlyReport{flowMonth(m, "500", "200", "0", "0", "10", "1000")}
	base := buildStats(&noCoupon[0], nil, noCoupon, nil, decimal.Zero)
	if base.Resilience.Months != 6 {
		t.Fatalf("no-coupon runway: got %+v, want 6 months", base.Resilience)
	}
	// Passive-Income = (0 passive + 10 return) / 200 = 5%.
	assertPct(t, "passive-income no-coupon", base.PassiveIncome, 10.0/200*100)

	// Same own return (10), plus 50 of paid-out coupon. InvestmentReturnTotal
	// carries the coupon (60 = 10 own + 50 coupon); PassiveCouponCash carves it out.
	withCoupon := noCoupon[0]
	withCoupon.InvestmentReturnTotal = decp("60")
	withCoupon.PassiveCouponCash = decp("50")
	series := []db.MonthlyReport{withCoupon}
	st := buildStats(&series[0], nil, series, nil, decimal.Zero)

	// g still 0.01 (coupon removed from own return) AND the 50 offsets the draw →
	// runway extends 6 → 7 months, exactly like a 50-Pension offset.
	if st.Resilience.Months != 7 {
		t.Errorf("coupon-offset runway: got %+v, want 7 months (coupon offsets draw, not g)", st.Resilience)
	}
	// Passive-Income numerator is unchanged by the split: passiveCash gains 50,
	// own return loses 50, so (50 + 10) / 200 = 30% — identical to leaving the
	// full 60 in investment return with no carve-out.
	assertPct(t, "passive-income with-coupon", st.PassiveIncome, 60.0/200*100)
}

// Incidental income is excluded from every statistics income term: the ratios
// read the routine subtotals, so a windfall inflates the all-regularity totals
// but not the ratios (ADR-0048 amendment).
// covers: INV-FINANCE-19, INV-FINANCE-24
func TestBuildStats_IncidentalIncomeExcluded(t *testing.T) {
	m := ym(2026, time.June)
	row := db.MonthlyReport{
		YearMonth:                  m,
		NwInvestments:              dec("10000"),
		EarnedIncomeTotal:          decp("2000"), // 1000 routine + 1000 windfall
		EarnedIncomeTotalRoutine:   decp("1000"),
		EarnedIncomeRental:         decp("200"), // all incidental
		EarnedIncomeRentalRoutine:  decp("0"),
		EarnedIncomePension:        decp("100"),
		EarnedIncomePensionRoutine: decp("100"), // routine
		InvestmentReturnTotal:      decp("0"),
		DerivedLivingExpenses:      decp("600"),
	}
	series := []db.MonthlyReport{row}
	st := buildStats(&series[0], nil, series, nil, decimal.Zero)

	// Cash-Flow uses routine income (1000−600)/1000 = 40%, not (2000−600)/2000 = 70%.
	assertPct(t, "cash-flow", st.CashFlow, 40)
	// Passive-Income uses routine passive cash: (rentalRoutine 0 + pensionRoutine
	// 100 + IR 0) / 600 = 16.67%, not (200+100)/600 = 50%.
	assertPct(t, "passive-income", st.PassiveIncome, 100.0/600*100)
}

func TestBuildStats_TrailingWindowExcludesOlderMonths(t *testing.T) {
	report := ym(2026, time.June)
	series := []db.MonthlyReport{
		flowMonth(report.AddDate(0, -12, 0), "9999", "9999", "0", "0", "0", "10000"), // 12 months back — excluded
		flowMonth(report.AddDate(0, -11, 0), "1000", "600", "0", "0", "0", "10000"),  // in window
		flowMonth(report, "1000", "600", "0", "0", "0", "10000"),
	}
	row := &series[2]
	st := buildStats(row, nil, series, nil, decimal.Zero)
	// Only the two in-window months (identical 40% each) count; the lumpy
	// out-of-window month would have dragged cash-flow to 0 if included.
	assertPct(t, "cash-flow", st.CashFlow, 40)
}

func TestMonthlyInflation(t *testing.T) {
	lo := ym(2025, time.July)
	upto := ym(2026, time.June)

	t.Run("no stored rates → assumed setting", func(t *testing.T) {
		got := monthlyInflation(nil, dec("3.5"), lo, upto)
		want := math.Pow(1+3.5/100, 1.0/12) - 1
		assertFloat(t, got, want)
	})

	t.Run("stored rates in window averaged, setting ignored", func(t *testing.T) {
		rates := []db.InflationRate{
			{YearMonth: ym(2026, time.May), Rate: dec("2")},
			{YearMonth: ym(2026, time.June), Rate: dec("4")},
			{YearMonth: ym(2025, time.June), Rate: dec("99")}, // out of window — ignored
		}
		got := monthlyInflation(rates, dec("3.5"), lo, upto)
		want := math.Pow(1+3.0/100, 1.0/12) - 1 // avg(2,4) = 3
		assertFloat(t, got, want)
	})
}

func assertPct(t *testing.T, name string, r pdf.Ratio, want float64) {
	t.Helper()
	if !r.Defined {
		t.Fatalf("%s: want defined", name)
	}
	assertFloat(t, r.Percent, want)
}

func assertFloat(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("got %v, want %v", got, want)
	}
}
