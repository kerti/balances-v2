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
			Expenses: "90000000",
			Net:      "-15000000", // negative → exercises the red net-cash-flow path
		},
		FxRates: []FxRate{{Currency: "USD", Rate: "15500"}},
		Trend: []TrendPoint{
			{Label: "Jan 26", NetWorth: 6000000000},
			{Label: "Feb 26", NetWorth: 6100000000},
			{Label: "Jun 26", NetWorth: 6274520495.58},
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
