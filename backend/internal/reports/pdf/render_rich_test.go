package pdf

import (
	"bytes"
	"testing"
	"time"
)

// richInput exercises every section, subtype, owner grouping, native-currency
// row, cash flow, FX table, trend line, and both deltas — the paths the minimal
// TestRenderProducesPDF input leaves untouched (charts, itemization, legends).
func richInput() Input {
	return Input{
		YearMonth:         time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
		Locale:            "id-ID",
		ReportingCurrency: "IDR",
		NetWorth:          "6274520495.58",
		Delta:             &Delta{Amount: "110000000", Percent: 1.8, Prev: time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC)},
		YoY:               &Delta{Amount: "-250000000", Percent: -3.5, Prev: time.Date(2025, time.June, 1, 0, 0, 0, 0, time.UTC)},
		Positions: []Position{
			{Group: "asset", Subtype: "bank_account", Name: "Everyday", OwnerLabel: "Alice", Amount: "500000000"},
			{Group: "asset", Subtype: "bank_account", Name: "Savings", OwnerLabel: "Bob", Amount: "300000000"},
			{Group: "asset", Subtype: "bank_account", Name: "Brokerage USD", OwnerLabel: "Alice",
				NativeCurrency: "USD", NativeAmount: "1000", Amount: "15500000"},
			{Group: "asset", Subtype: "property", Name: "House", OwnerLabel: "Bersama", Amount: "4000000000"},
			{Group: "asset", Subtype: "vehicle", Name: "Car", OwnerLabel: "Bersama", Amount: "200000000", Stale: true},
			{Group: "liability", Subtype: "institutional", Name: "Mortgage", OwnerLabel: "Bersama", Amount: "1500000000"},
			{Group: "liability", Subtype: "personal", Name: "Family loan", OwnerLabel: "Alice", Amount: "50000000"},
			{Group: "investment", Subtype: "mutual_fund", Name: "Index Fund", OwnerLabel: "Alice", Amount: "800000000"},
			{Group: "investment", Subtype: "bond", Name: "Govt Bond", OwnerLabel: "Bob", Amount: "400000000"},
			{Group: "investment", Subtype: "gold", Name: "Bullion", OwnerLabel: "Bersama", Amount: "150000000"},
			{Group: "investment", Subtype: "stock", Name: "US Stock", OwnerLabel: "Alice",
				NativeCurrency: "USD", NativeAmount: "5000", Amount: "77500000"},
			{Group: "investment", Subtype: "time_deposit", Name: "12mo TD", OwnerLabel: "Bob", Amount: "100000000"},
			{Group: "receivable", Name: "Owed by cousin", OwnerLabel: "Alice", Amount: "25000000"},
		},
		CashFlow: &CashFlow{
			Members: []CashMember{
				{Label: "Alice", Amount: "40000000"},
				{Label: "Bob", Amount: "35000000"},
			},
			Income:   "75000000",
			Active:   "60000000", // 60M active + 15M passive == 75M income
			Passive:  "15000000",
			Coupons:  "2000000", // additive coupon-cash line
			Expenses: "90000000",
			Net:      "-15000000", // negative → exercises the red net-cash-flow path
		},
		FxRates: []FxRate{{Currency: "USD", Rate: "15500"}},
		Trend: []TrendPoint{
			{Label: "Jan 26", NetWorth: 6000000000},
			{Label: "Feb 26", NetWorth: 6100000000},
			{Label: "Jun 26", NetWorth: 6274520495.58},
		},
		Stats: Stats{
			CashFlow:         Ratio{Defined: true, Percent: 18.4},
			PassiveIncome:    Ratio{Defined: true, Percent: -4.2}, // market-sensitive, can print negative
			InstantLiquidity: Ratio{Defined: true, Percent: 5.3},
			Resilience:       Resilience{Defined: true, Months: 137},
			Inputs: StatInputs{
				Defined:     true,
				AvgIncome:   "68000000",
				AvgExpenses: "55000000",
				AvgPassive:  "3950000",
				Months:      12,
			},
		},
	}
}

func TestRenderRichReport(t *testing.T) {
	out, err := Render(richInput())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("output is not a PDF (prefix %q)", firstN(out, 8))
	}
	// A fully itemized statement with charts is far larger than the near-empty
	// baseline; this guards against sections silently no-op'ing.
	if len(out) < 8000 {
		t.Fatalf("rich PDF suspiciously small: %d bytes", len(out))
	}
}

// A statement with only assets (no liabilities, receivables, or investments)
// must render without panicking — exercising the false branch of each page-group
// presence gate (hasLiabilities/hasReceivables/hasInvestments) so an absent
// section skips its page break rather than leaving a blank page.
func TestRenderSparsePageGroups(t *testing.T) {
	in := richInput()
	in.Positions = []Position{
		{Group: "asset", Subtype: "bank_account", Name: "Everyday", OwnerLabel: "Alice", Amount: "500000000"},
	}
	if _, err := Render(in); err != nil {
		t.Fatalf("sparse Render: %v", err)
	}

	// And the mirror case: only investments, no own-book sections — exercises the
	// false branches of hasAssets/hasLiabilities/hasReceivables.
	in.Positions = []Position{
		{Group: "investment", Subtype: "stock", Name: "US Stock", OwnerLabel: "Alice", Amount: "77500000"},
	}
	if _, err := Render(in); err != nil {
		t.Fatalf("investments-only Render: %v", err)
	}
}

func TestRenderRichReportEN(t *testing.T) {
	in := richInput()
	in.Locale = "en-GB"
	if _, err := Render(in); err != nil {
		t.Fatalf("en-GB rich Render: %v", err)
	}
}

// A baseline month has no CashFlow (nil) — the section must render its
// "first reported month" note rather than panic on the nil.
func TestRenderBaselineCashFlow(t *testing.T) {
	in := richInput()
	in.CashFlow = nil
	in.Delta = nil
	in.YoY = nil
	if _, err := Render(in); err != nil {
		t.Fatalf("baseline Render: %v", err)
	}
}

// The zero-value Stats (all ratios undefined) must render the reserved em-dash
// panel with the "not enough history" note, and the Indefinite resilience value,
// rather than panic.
func TestRenderStatsPanelStates(t *testing.T) {
	in := richInput()
	in.Stats = Stats{} // all undefined
	if _, err := Render(in); err != nil {
		t.Fatalf("undefined stats Render: %v", err)
	}

	in.Stats = Stats{
		CashFlow:         Ratio{Defined: true, Percent: 42},
		PassiveIncome:    Ratio{Defined: true, Percent: 120},
		InstantLiquidity: Ratio{Defined: true, Percent: 3.1},
		Resilience:       Resilience{Defined: true, Indefinite: true},
	}
	if _, err := Render(in); err != nil {
		t.Fatalf("indefinite stats Render: %v", err)
	}

	// A one-month runway takes the singular unit branch ("%s month", not
	// "%s months") in resilienceValue — the plural/indefinite cases above leave
	// it untested.
	in.Stats.Resilience = Resilience{Defined: true, Months: 1}
	if _, err := Render(in); err != nil {
		t.Fatalf("singular-month resilience Render: %v", err)
	}
}

// resilienceValue renders the runway as "Y years M months", each part
// singular-aware and dropped when zero — a sub-year runway is months only, an
// exact multiple of 12 is years only.
func TestResilienceValue_YearsMonths(t *testing.T) {
	cases := []struct {
		months     int
		indefinite bool
		want       string
	}{
		{months: 5, want: "5 months"},
		{months: 1, want: "1 month"},
		{months: 0, want: "0 months"},
		{months: 12, want: "1 year"},
		{months: 13, want: "1 year 1 month"},
		{months: 24, want: "2 years"},
		{months: 137, want: "11 years 5 months"}, // was "137 months"
		{indefinite: true, want: "Indefinite"},
	}
	for _, c := range cases {
		d := &doc{c: copyFor("en-GB"), in: Input{Locale: "en-GB",
			Stats: Stats{Resilience: Resilience{Defined: true, Months: c.months, Indefinite: c.indefinite}}}}
		if got := d.resilienceValue(); got != c.want {
			t.Errorf("months=%d indefinite=%v: got %q, want %q", c.months, c.indefinite, got, c.want)
		}
	}
}
