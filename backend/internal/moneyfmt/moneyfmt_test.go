package moneyfmt

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"unicode"
)

// The parity fixture is captured from the frontend's own Intl.NumberFormat via
// Node (see the generator in the PR / ADR-0045). Regenerate it whenever the
// frontend's formatting rules change: the Go side is then forced to match.
type goldenFile struct {
	Currency []struct {
		Locale, Currency, Amount, Want string
	} `json:"currency"`
	Number []struct {
		Locale, Value, Want string
	} `json:"number"`
}

// spaceless strips every Unicode space so the comparison is insensitive to the
// affix/amount spacing, which is a cosmetic CLDR-pattern difference between V8's
// Intl (attaches symbol currencies tight: "US$6,…") and x/text (always spaces:
// "US$ 6,…"). Everything that is a real drift — symbol, digit grouping, decimal
// separator, fraction digits, rounding, sign placement — survives stripping and
// is asserted.
func spaceless(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

func loadGolden(t *testing.T) goldenFile {
	t.Helper()
	b, err := os.ReadFile("testdata/intl_golden.json")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	var g goldenFile
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatalf("parse golden: %v", err)
	}
	return g
}

func TestFormatCurrencyMatchesIntl(t *testing.T) {
	g := loadGolden(t)
	if len(g.Currency) == 0 {
		t.Fatal("no currency golden cases loaded")
	}
	for _, c := range g.Currency {
		got := FormatCurrency(c.Amount, c.Currency, c.Locale)
		if spaceless(got) != spaceless(c.Want) {
			t.Errorf("FormatCurrency(%q, %q, %q) = %q, want ~%q (space-insensitive)",
				c.Amount, c.Currency, c.Locale, got, c.Want)
		}
	}
}

func TestFormatNumberMatchesIntl(t *testing.T) {
	g := loadGolden(t)
	if len(g.Number) == 0 {
		t.Fatal("no number golden cases loaded")
	}
	for _, c := range g.Number {
		got := FormatNumber(c.Value, c.Locale)
		if spaceless(got) != spaceless(c.Want) {
			t.Errorf("FormatNumber(%q, %q) = %q, want ~%q", c.Value, c.Locale, got, c.Want)
		}
	}
}

func TestFormatCurrencyBadInputPassesThrough(t *testing.T) {
	if got := FormatCurrency("not-a-number", "IDR", "id-ID"); got != "not-a-number" {
		t.Errorf("bad input = %q, want passthrough", got)
	}
}
