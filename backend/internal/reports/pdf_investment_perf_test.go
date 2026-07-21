package reports

import (
	"math"
	"testing"
	"time"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/reports/pdf"
)

func assertRate(t *testing.T, what string, r pdf.Rate, wantPct float64) {
	t.Helper()
	if !r.Defined {
		t.Errorf("%s: undefined, want %.2f%%", what, wantPct)
		return
	}
	if math.Abs(r.Percent-wantPct) > 1e-6 {
		t.Errorf("%s: got %.4f%%, want %.4f%%", what, r.Percent, wantPct)
	}
}

func assertUndefined(t *testing.T, what string, r pdf.Rate) {
	t.Helper()
	if r.Defined {
		t.Errorf("%s: defined %.4f%%, want undefined (em-dash)", what, r.Percent)
	}
}

func rowByKey(rows []pdf.PerfRow, key string) pdf.PerfRow {
	for _, r := range rows {
		if r.Key == key {
			return r
		}
	}
	return pdf.PerfRow{}
}

// buildInvestmentPerformance reads investment return as a rate — return over the
// prior-month opening invested value — and its trailing-12 figure as the
// geometric compound of monthly rates, never the arithmetic mean. A bucket whose
// opening base is zero or absent renders "—".
// covers: INV-FINANCE-30, INV-FINANCE-31
func TestBuildInvestmentPerformance_RatesAndCompound(t *testing.T) {
	jan, feb, mar := ym(2026, time.January), ym(2026, time.February), ym(2026, time.March)
	series := []db.MonthlyReport{
		// Baseline: values seed Feb's opening base; no return.
		{
			YearMonth:           jan,
			NwInvestments:       dec("1000"),
			InvestmentValueLow:  decp("1000"),
			InvestmentValueHigh: nil, // high bucket not yet held
		},
		// Feb: +100 on a 1000 opening base = +10%.
		{
			YearMonth:             feb,
			NwInvestments:         dec("1100"),
			InvestmentReturnTotal: decp("100"),
			InvestmentReturnLow:   decp("100"),
			InvestmentValueLow:    decp("1100"),
			InvestmentValueHigh:   decp("0"), // present but zero — a ÷0 opening base for Mar
		},
		// Mar (reported): +110 on a 1100 opening base = +10%.
		{
			YearMonth:             mar,
			NwInvestments:         dec("1210"),
			InvestmentReturnTotal: decp("110"),
			InvestmentReturnLow:   decp("110"),
			InvestmentReturnHigh:  decp("50"), // a return exists...
			InvestmentValueLow:    decp("1210"),
			InvestmentValueHigh:   decp("200"), // ...but its opening base (Feb) was 0
		},
	}
	row := &series[2]

	perf := buildInvestmentPerformance(row, series)
	if !perf.Defined {
		t.Fatal("perf.Defined = false, want true (investments held in the reported month)")
	}

	// Total: this month 10%, trailing-12 = (1.10)(1.10)−1 = 21% — the geometric
	// compound, NOT the arithmetic mean (which would be 10%). INV-FINANCE-31.
	assertRate(t, "total this-month", perf.Total.Month, 10)
	assertRate(t, "total trailing-12", perf.Total.Trailing, 21)
	if perf.Total.Month.Amount != "110" {
		t.Errorf("total this-month amount: got %q, want \"110\"", perf.Total.Month.Amount)
	}

	// Low risk: same single holding, same rates.
	low := rowByKey(perf.ByRisk, "low")
	assertRate(t, "low this-month", low.Month, 10)
	assertRate(t, "low trailing-12", low.Trailing, 21)

	// Medium risk: never held → base absent → undefined both cells. INV-FINANCE-30.
	med := rowByKey(perf.ByRisk, "medium")
	assertUndefined(t, "medium this-month", med.Month)
	assertUndefined(t, "medium trailing-12", med.Trailing)

	// High risk: a return exists in Mar, but the opening base (Feb) is zero → the
	// rate is undefined (÷0), rendered "—", never 0%. INV-FINANCE-30.
	high := rowByKey(perf.ByRisk, "high")
	assertUndefined(t, "high this-month (zero opening base)", high.Month)
	assertUndefined(t, "high trailing-12", high.Trailing)
	if high.Month.Amount != "50" {
		t.Errorf("high this-month amount: got %q, want \"50\" (amount shown even when rate undefined)", high.Month.Amount)
	}
}

// Placement is new money as a share of the opening pool; its trailing-12 figure
// is the arithmetic average (Σplacement/Σopening-pool for %, Σ/n for amount), not
// the geometric compound used for return.
// covers: INV-FINANCE-32
func TestBuildInvestmentPerformance_Placement(t *testing.T) {
	jan, feb, mar := ym(2026, time.January), ym(2026, time.February), ym(2026, time.March)
	series := []db.MonthlyReport{
		{YearMonth: jan, NwInvestments: dec("1000")}, // baseline: no placement
		// Feb: placed 100 on a 1000 opening pool = 10%.
		{YearMonth: feb, NwInvestments: dec("1200"), InvestmentPlacement: decp("100")},
		// Mar (reported): placed 300 on a 1200 opening pool = 25%.
		{YearMonth: mar, NwInvestments: dec("1600"), InvestmentPlacement: decp("300")},
	}
	row := &series[2]
	perf := buildInvestmentPerformance(row, series)
	if !perf.HasPlacement {
		t.Fatal("HasPlacement = false, want true")
	}
	// This month: 300 / 1200 = 25%.
	assertRate(t, "placement this-month", perf.Placement.Month, 25)
	if perf.Placement.Month.Amount != "300" {
		t.Errorf("placement this-month amount: got %q, want \"300\"", perf.Placement.Month.Amount)
	}
	// Trailing-12 = Σplacement / Σopening-pool = (100+300) / (1000+1200) = 400/2200
	// = 18.18% — arithmetic (avg/avg), NOT a compound. Amount = 400/2 = 200.
	assertRate(t, "placement trailing", perf.Placement.Trailing, 400.0/2200*100)
	if perf.Placement.Trailing.Amount != "200" {
		t.Errorf("placement trailing amount: got %q, want \"200\" (Σ/n = 400/2)", perf.Placement.Trailing.Amount)
	}
}

// Every bucket (all 5 subtypes + all 3 risk levels) carries a return and an
// opening base, so each bucketReturn/bucketValue key is exercised and every
// this-month rate resolves. Guards the full breakdown, not just the buckets a
// sparse fixture happens to populate.
// covers: INV-FINANCE-29, INV-FINANCE-30
func TestBuildInvestmentPerformance_AllBuckets(t *testing.T) {
	jan, feb := ym(2026, time.January), ym(2026, time.February)
	series := []db.MonthlyReport{
		{
			YearMonth:     jan,
			NwInvestments: dec("900"),
			// Opening bases: 5 subtypes × 180 = 900; 3 risks × 300 = 900.
			InvestmentValueStock: decp("180"), InvestmentValueMutualFund: decp("180"),
			InvestmentValueBond: decp("180"), InvestmentValueGold: decp("180"),
			InvestmentValueTimeDeposit: decp("180"),
			InvestmentValueLow:         decp("300"), InvestmentValueMedium: decp("300"), InvestmentValueHigh: decp("300"),
		},
		{
			YearMonth:             feb,
			NwInvestments:         dec("990"),
			InvestmentReturnTotal: decp("90"),
			// Each subtype return 18 (→10% on its 180 base); each risk return 30 (→10% on 300).
			InvestmentReturnStock: decp("18"), InvestmentReturnMutualFund: decp("18"),
			InvestmentReturnBond: decp("18"), InvestmentReturnGold: decp("18"),
			InvestmentReturnTimeDeposit: decp("18"),
			InvestmentReturnLow:         decp("30"), InvestmentReturnMedium: decp("30"), InvestmentReturnHigh: decp("30"),
		},
	}
	perf := buildInvestmentPerformance(&series[1], series)
	if !perf.Defined {
		t.Fatal("perf.Defined = false, want true")
	}
	assertRate(t, "total", perf.Total.Month, 10)
	for _, r := range perf.ByRisk {
		assertRate(t, "risk "+r.Key, r.Month, 10)
	}
	for _, r := range perf.ByType {
		assertRate(t, "type "+r.Key, r.Month, 10)
	}
}

// The trailing figures are bounded to the 12-month window (the reported month +
// 11 priors), and a month whose opening base is zero contributes no factor to
// either the return compound or the placement average.
// covers: INV-FINANCE-31, INV-FINANCE-32
func TestBuildInvestmentPerformance_WindowBoundedAndZeroBaseSkipped(t *testing.T) {
	// 14 months; reported = the last. The two earliest fall outside the window;
	// Feb-2025's pool is 0, so Mar-2025's opening base is 0 and is skipped.
	var series []db.MonthlyReport
	base := ym(2025, time.January)
	for i := 0; i < 14; i++ {
		pool := "1000"
		if i == 1 {
			pool = "0"
		}
		r := db.MonthlyReport{YearMonth: base.AddDate(0, i, 0), NwInvestments: dec(pool)}
		if i > 0 {
			r.InvestmentReturnTotal = decp("100")
			r.InvestmentPlacement = decp("50")
		}
		series = append(series, r)
	}
	perf := buildInvestmentPerformance(&series[13], series)
	if !perf.Total.Trailing.Defined {
		t.Error("total trailing should be defined (in-window months qualify)")
	}
	if !perf.HasPlacement || !perf.Placement.Trailing.Defined {
		t.Error("placement trailing should be defined")
	}
	// In-window qualifying months each rate 100/1000 = 10%; the zero-base month and
	// the out-of-window months are excluded, so the placement trailing % is a clean
	// 50/1000 = 5% average, not diluted by the skipped months.
	assertRate(t, "placement trailing (window-bounded, zero-base skipped)", perf.Placement.Trailing, 5)
}

// No investments in the reported month → the whole block is suppressed.
// covers: INV-FINANCE-30
func TestBuildInvestmentPerformance_SuppressedWithoutInvestments(t *testing.T) {
	m := ym(2026, time.June)
	series := []db.MonthlyReport{{YearMonth: m, NwInvestments: dec("0")}}
	if perf := buildInvestmentPerformance(&series[0], series); perf.Defined {
		t.Errorf("perf.Defined = true, want false when no investments are held")
	}
}
