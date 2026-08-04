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
		InvestmentPerf: InvestmentPerf{
			Defined: true,
			Total:   PerfRow{Key: "total", Month: Rate{Defined: true, Percent: 1.2, Amount: "3400000"}, Trailing: Rate{Defined: true, Percent: 9.8}},
			ByRisk: []PerfRow{
				{Key: "low", Month: Rate{Defined: true, Percent: 0.5, Amount: "1100000"}, Trailing: Rate{Defined: true, Percent: 4.2}},
				{Key: "medium"}, // undefined → em-dash both cells
				{Key: "high", Month: Rate{Defined: true, Percent: -2.1, Amount: "-900000"}, Trailing: Rate{Defined: true, Percent: 6.0}},
			},
			ByType: []PerfRow{
				{Key: "stock", Month: Rate{Defined: true, Percent: 3.0, Amount: "2000000"}, Trailing: Rate{Defined: true, Percent: 12.0}},
				{Key: "mutual_fund", Month: Rate{Defined: true, Percent: 1.0, Amount: "800000"}, Trailing: Rate{Defined: true, Percent: 8.0}},
				{Key: "bond", Month: Rate{Defined: true, Percent: 0.4, Amount: "400000"}, Trailing: Rate{Defined: true, Percent: 5.0}},
				{Key: "gold"}, // month + trailing undefined (never held) → em-dash, no muted amount line
				{Key: "time_deposit", Month: Rate{Defined: true, Percent: 0.3, Amount: "300000"}, Trailing: Rate{Defined: true, Percent: 3.6}},
			},
			HasPlacement: true,
			Placement:    PerfRow{Key: "placement", Month: Rate{Defined: true, Percent: 0.8, Amount: "8000000"}, Trailing: Rate{Defined: true, Percent: 6.0, Amount: "5500000"}},
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

// The Write-Off section and the unsettled-termination footnote render only on
// the months that have them, so both need explicit exercise: the section body
// with a mixed-sign constituent list (the netting case the itemisation exists
// for), the advisory footnote, and both of their absent branches.
// covers: INV-FINANCE-33, INV-FINANCE-35
func TestRenderWriteOffsAndUnsettled(t *testing.T) {
	in := richInput()
	in.WriteOffs = &WriteOffs{
		Total: "-25000000",
		Items: []WriteOffItem{
			{Label: "Family loan", Amount: "50000000"},     // forgiven → positive
			{Label: "Owed by cousin", Amount: "-75000000"}, // uncollectible → negative
		},
	}
	in.Unsettled = []UnsettledTermination{{Label: "Index Fund"}}
	withSections, err := Render(in)
	if err != nil {
		t.Fatalf("write-offs Render: %v", err)
	}

	// Both branches of the section gate: nil, and non-nil-but-empty (a materialized
	// zero with nothing behind it must not print an empty section).
	in.WriteOffs = &WriteOffs{Total: "0"}
	in.Unsettled = nil
	without, err := Render(in)
	if err != nil {
		t.Fatalf("empty write-offs Render: %v", err)
	}
	if len(without) >= len(withSections) {
		t.Errorf("empty write-offs PDF (%d bytes) is not smaller than the itemized one (%d) — the section did not collapse",
			len(without), len(withSections))
	}

	in.WriteOffs = nil
	absent, err := Render(in)
	if err != nil {
		t.Fatalf("nil write-offs Render: %v", err)
	}
	// Smaller-than-itemized alone would also pass for a section that printed its
	// title, note and a bare 0 total, so pin the collapse to the no-line render.
	if len(without) != len(absent) {
		t.Errorf("materialized-zero PDF is %d bytes vs %d for no line at all — an empty section still printed",
			len(without), len(absent))
	}
}

// Tracking Changes renders on the same terms as Write-Offs and sits beside it in
// the same page group, so it needs the same three-way exercise: the itemised
// body on a month that nets toward zero in both directions, and both absent
// branches (nil, and a materialized zero with nothing behind it).
// covers: INV-FINANCE-38
func TestRenderTrackingChanges(t *testing.T) {
	in := richInput()
	in.TrackingChanges = &TrackingChanges{
		Total: "0",
		Items: []TrackingChangeItem{
			{Label: "Brokerage left the household", Amount: "-40000000"}, // untracked → negative
			{Label: "Old savings account", Amount: "40000000"},           // newly tracked → positive
		},
	}
	withSection, err := Render(in)
	if err != nil {
		t.Fatalf("tracking-changes Render: %v", err)
	}

	in.TrackingChanges = &TrackingChanges{Total: "0"}
	without, err := Render(in)
	if err != nil {
		t.Fatalf("empty tracking-changes Render: %v", err)
	}
	if len(without) >= len(withSection) {
		t.Errorf("empty tracking-changes PDF (%d bytes) is not smaller than the itemized one (%d) — the section did not collapse",
			len(without), len(withSection))
	}

	in.TrackingChanges = nil
	absent, err := Render(in)
	if err != nil {
		t.Fatalf("nil tracking-changes Render: %v", err)
	}
	// Smaller-than-itemized is too weak to prove the collapse: a section printing
	// its title, note and a bare 0 total also measures smaller. The month that
	// declared nothing has to render byte-for-byte like the month that has no line
	// at all — that is what "adds no visual noise to the common case" means.
	if len(without) != len(absent) {
		t.Errorf("materialized-zero PDF is %d bytes vs %d for no line at all — an empty section still printed",
			len(without), len(absent))
	}
}

func TestRenderRichReportEN(t *testing.T) {
	in := richInput()
	in.Locale = "en-GB"
	if _, err := Render(in); err != nil {
		t.Fatalf("en-GB rich Render: %v", err)
	}
}

// The investment-performance block's edge states must render without panicking:
// suppression (Defined=false), all-undefined rates (em-dash cells with no muted
// amount line), a skipped placement row (HasPlacement=false), and a placement row
// with a negative this-month rate + an undefined trailing cell / blank trailing
// amount (perfPct em-dash + moneyOrBlank("")).
func TestRenderInvestmentPerfEdges(t *testing.T) {
	// Defined=false but investments present → the block early-returns (suppressed).
	in := richInput()
	in.InvestmentPerf = InvestmentPerf{Defined: false}
	if _, err := Render(in); err != nil {
		t.Fatalf("suppressed perf Render: %v", err)
	}

	// All rates undefined (em-dash), placement row skipped.
	in = richInput()
	in.InvestmentPerf = InvestmentPerf{
		Defined: true,
		Total:   PerfRow{Key: "total"},
		ByRisk:  []PerfRow{{Key: "low"}, {Key: "medium"}, {Key: "high"}},
		ByType:  []PerfRow{{Key: "stock"}, {Key: "mutual_fund"}, {Key: "bond"}, {Key: "gold"}, {Key: "time_deposit"}},
		// HasPlacement false → placement row skipped.
	}
	if _, err := Render(in); err != nil {
		t.Fatalf("undefined perf Render: %v", err)
	}

	// Placement with a negative this-month rate and an undefined trailing cell +
	// blank trailing amount.
	in = richInput()
	in.InvestmentPerf.Placement = PerfRow{
		Key:      "placement",
		Month:    Rate{Defined: true, Percent: -1.5, Amount: "-2000000"},
		Trailing: Rate{}, // undefined %, blank amount
	}
	if _, err := Render(in); err != nil {
		t.Fatalf("edge placement Render: %v", err)
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
