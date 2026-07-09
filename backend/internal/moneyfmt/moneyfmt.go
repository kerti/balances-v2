// Package moneyfmt formats currency and decimal values to match the frontend's
// lib/format.ts (Intl.NumberFormat) so the server-rendered PDF report (ADR-0045)
// shows the same numbers as the on-screen dashboard. Built on golang.org/x/text
// (CLDR-backed, the same data family V8's Intl uses); a golden parity test
// (moneyfmt_test.go) pins the output against captured Intl.NumberFormat strings
// so the two implementations can't silently drift.
//
// Amounts arrive as decimal strings (report values are decimals) and are parsed
// through float64 — the same Number(amount) path the frontend uses — so rounding
// matches.
package moneyfmt

import (
	"math"
	"strconv"

	"golang.org/x/text/currency"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/number"
)

func localeOf(locale string) language.Tag {
	if locale == "" {
		return language.MustParse("en-GB")
	}
	return language.Make(locale)
}

// FormatCurrency renders amount (a decimal string) in the given ISO-4217
// currency and BCP-47 locale — the server-side counterpart of the frontend's
// formatCurrency. Falls back to the raw string if the amount doesn't parse.
//
// Fraction digits follow x/text's CLDR default (0 for IDR/JPY/KRW/VND, 2 for
// USD/EUR/SGD/…), which coincides with lib/format.ts's NO_DECIMAL_CURRENCIES
// rule for every currency this app realistically handles; the golden matrix
// enforces that coincidence. A currency whose CLDR fraction disagrees with the
// frontend rule (e.g. CLP=0, BHD=3) would need an explicit override — none is in
// use, and the parity test flags it the moment one is added to the matrix.
func FormatCurrency(amount, cur, locale string) string {
	f, err := strconv.ParseFloat(amount, 64)
	if err != nil {
		return amount
	}
	// Intl places the minus sign *outside* the currency affix ("-IDR 70,000"),
	// where x/text places it inside ("IDR -70,000"); Intl also signs values that
	// round to zero from below (math.Signbit semantics: -0.004 -> "-$0.00"). We
	// format the magnitude and re-attach the sign to match.
	neg := math.Signbit(f)
	mag := math.Abs(f)
	p := message.NewPrinter(localeOf(locale))
	c, err := currency.ParseISO(cur)
	var s string
	if err != nil {
		// Unknown currency code: ISO code + locale-grouped number, matching
		// Intl's own no-symbol fallback shape.
		s = cur + " " + p.Sprint(number.Decimal(mag))
	} else {
		s = p.Sprint(currency.Symbol(c.Amount(mag)))
	}
	if neg {
		return "-" + s
	}
	return s
}

// FormatNumber renders a decimal string with locale grouping and no currency —
// the server-side counterpart of the frontend's formatNumber (used for FX rates).
func FormatNumber(value, locale string) string {
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return value
	}
	p := message.NewPrinter(localeOf(locale))
	return p.Sprint(number.Decimal(f))
}
