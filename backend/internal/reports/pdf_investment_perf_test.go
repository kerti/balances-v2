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

// No investments in the reported month → the whole block is suppressed.
// covers: INV-FINANCE-30
func TestBuildInvestmentPerformance_SuppressedWithoutInvestments(t *testing.T) {
	m := ym(2026, time.June)
	series := []db.MonthlyReport{{YearMonth: m, NwInvestments: dec("0")}}
	if perf := buildInvestmentPerformance(&series[0], series); perf.Defined {
		t.Errorf("perf.Defined = true, want false when no investments are held")
	}
}
