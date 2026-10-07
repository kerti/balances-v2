package pdf

import (
	"bytes"
	"testing"
	"time"
)

// fpdf's UTF-8 CID font map stops at U+FFFF, so a rune above it (nearly every
// emoji) panicked inside Output. Every user-typed label reaching the PDF must
// survive one (#703).
//
// covers: INV-PRESENTATION-10
func TestRenderSurvivesEmojiInUserText(t *testing.T) {
	const e = "🏦👨‍👩‍👧❤️"
	in := Input{
		YearMonth:         time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
		ReportingCurrency: "IDR",
		Locale:            "en-GB",
		NetWorth:          "1500",
		Positions: []Position{
			{Group: "asset", Subtype: "bank_account", Name: "Savings " + e, OwnerLabel: "Alice " + e, NativeCurrency: "IDR", NativeAmount: "1000", Amount: "1000"},
			{Group: "investment", Subtype: "stock", Name: e + " Stocks", OwnerLabel: "Joint", NativeCurrency: "IDR", NativeAmount: "500", Amount: "500"},
		},
		CashFlow:        &CashFlow{Members: []CashMember{{Label: "Bob " + e, Amount: "10"}}, Income: "10", Expenses: "0", Net: "10"},
		WriteOffs:       &WriteOffs{Total: "-5", Items: []WriteOffItem{{Label: "Loan " + e, Amount: "-5"}}},
		TrackingChanges: &TrackingChanges{Total: "5", Items: []TrackingChangeItem{{Label: "Gift " + e, Amount: "5"}}},
		Unsettled:       []UnsettledTermination{{Label: "Deposit " + e}},
	}

	var out []byte
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Render panicked on emoji: %v", r)
			}
		}()
		out, err = Render(in)
	}()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatal("emoji render is not a PDF")
	}
}

func TestPDFSafeText(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Savings", "Savings"},
		{"Tabungan Rp — 12,5 %", "Tabungan Rp — 12,5 %"}, // BMP punctuation kept
		{"Savings 🏦", "Savings"},
		{"🏦 Savings", "Savings"},
		{"Kids 👨‍👩‍👧 fund", "Kids fund"}, // ZWJ sequence collapses, no double space
		{"Love ❤️", "Love ❤"},            // U+2764 is BMP and kept; U+FE0F selector dropped
		{"🏦", ""},
	}
	for _, c := range cases {
		if got := pdfSafeText(c.in); got != c.want {
			t.Errorf("pdfSafeText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
