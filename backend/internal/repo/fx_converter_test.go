package repo

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// Pure unit tests for the fxConverter contract — the multi-currency layer the
// report engine applies before any net-worth figure is summed. The engine-level
// net-worth assertions live in monthly_reports_engine_test.go (INV-FINANCE-15..17);
// these pin the converter's own contract directly, so a regression that does not
// move a headline number (wrong ok/rate semantics, the foreign() audit gate)
// still trips a test. native × rate, latest-rate-at-or-before-month, missing
// surfaced not zeroed. ADR-0002 (multi-currency), ADR-0006 (carry-forward).

func usdConverter(reporting string, multi bool, rates ...reportFxRate) fxConverter {
	return newFxConverter(reportEngineInput{
		reportingCurrency: reporting,
		multiCurrency:     multi,
		fxRates:           rates,
	})
}

func janIdx() int { return monthIndex(ym(2026, time.January)) }

// Passthrough: convert returns the amount unchanged with rate=1 and ok=true —
// and consults no rate at all — when the currency is empty, equals the reporting
// currency, or multi-currency is off. A stray USD rate in the table must not
// leak into any of these branches.
// covers: INV-FX-01
func TestFx_Passthrough(t *testing.T) {
	rate := reportFxRate{currency: "USD", yearMonth: ym(2026, time.January), rate: dec("16000")}
	amt := dec("100")
	idx := janIdx()

	cases := []struct {
		name     string
		fx       fxConverter
		currency string
	}{
		{"empty currency", usdConverter("IDR", true, rate), ""},
		{"equals reporting", usdConverter("IDR", true, rate), "IDR"},
		{"multi off, foreign currency", usdConverter("IDR", false, rate), "USD"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, r, ok := c.fx.convert(amt, c.currency, idx)
			if !ok || !got.Equal(amt) || !r.Equal(decimal.NewFromInt(1)) {
				t.Errorf("convert(%s, %q): got (%s, rate %s, ok %v), want (100, rate 1, ok true)",
					amt, c.currency, got, r, ok)
			}
		})
	}
}

// Carry-forward: a foreign amount converts at native × the latest rate at or
// before its month — never a future one. With rates at Jan and Mar, February
// picks January's; March picks its own.
// covers: INV-FX-02
func TestFx_CarryForwardLatestAtOrBefore(t *testing.T) {
	fx := usdConverter("IDR", true,
		reportFxRate{currency: "USD", yearMonth: ym(2026, time.January), rate: dec("16000")},
		reportFxRate{currency: "USD", yearMonth: ym(2026, time.March), rate: dec("17000")},
	)
	amt := dec("100")

	feb, febRate, ok := fx.convert(amt, "USD", monthIndex(ym(2026, time.February)))
	if !ok || !feb.Equal(dec("1600000")) || !febRate.Equal(dec("16000")) {
		t.Errorf("Feb convert: got (%s, rate %s, ok %v), want (1600000, 16000, true) — Jan rate carried, not Mar's future rate", feb, febRate, ok)
	}
	mar, marRate, ok := fx.convert(amt, "USD", monthIndex(ym(2026, time.March)))
	if !ok || !mar.Equal(dec("1700000")) || !marRate.Equal(dec("17000")) {
		t.Errorf("Mar convert: got (%s, rate %s, ok %v), want (1700000, 17000, true)", mar, marRate, ok)
	}
}

// Missing surfaced, not zeroed: a foreign amount with no rate at or before its
// month is unconvertible (ok=false, zero out) so the engine can exclude it and
// record a missingFxEntry — never silently counted 1:1. The contrast assertion
// is that the amount is NOT returned at face value.
// covers: INV-FX-03
func TestFx_MissingRateUnconvertible(t *testing.T) {
	fx := usdConverter("IDR", true,
		reportFxRate{currency: "USD", yearMonth: ym(2026, time.March), rate: dec("17000")},
	)
	got, rate, ok := fx.convert(dec("100"), "USD", janIdx()) // January is before the only (March) rate
	if ok {
		t.Fatalf("convert with no rate ≤ month: ok=true, want false (must not count 1:1)")
	}
	if !got.IsZero() || !rate.IsZero() {
		t.Errorf("unconvertible amount: got (%s, rate %s), want (0, 0)", got, rate)
	}
}

// Audit gate: foreign() is true exactly when multi-currency is on and the
// currency is neither empty nor the reporting currency. This predicate gates
// recordRate, so fx_rates_used carries a row for — and only for — a genuinely
// converted currency.
// covers: INV-FX-04
func TestFx_ForeignAuditGate(t *testing.T) {
	on := usdConverter("IDR", true)
	off := usdConverter("IDR", false)

	cases := []struct {
		fx       fxConverter
		currency string
		want     bool
	}{
		{on, "USD", true},
		{on, "IDR", false},  // reporting currency
		{on, "", false},     // unset
		{off, "USD", false}, // multi off
	}
	for _, c := range cases {
		if got := c.fx.foreign(c.currency); got != c.want {
			t.Errorf("foreign(multi=%v, %q): got %v, want %v", c.fx.multi, c.currency, got, c.want)
		}
	}
}
