package reports

import (
	"testing"
	"time"
)

// Pure-helper coverage for the reports handler. The HTTP-level tests
// (reports_test.go) only ever feed parseYearMonth a "YYYY-MM" string and never
// exercise rawJSON's empty-input fallback, since a real report row always
// populates its JSON columns. These branches are deterministic and worth pinning
// directly rather than contriving handler inputs.

func TestParseYearMonth(t *testing.T) {
	t.Run("YYYY-MM", func(t *testing.T) {
		got, ok := parseYearMonth("2026-03")
		if !ok || got.Year() != 2026 || got.Month() != time.March {
			t.Errorf("parseYearMonth(2026-03) = %v, %v", got, ok)
		}
	})
	t.Run("YYYY-MM-DD", func(t *testing.T) {
		got, ok := parseYearMonth("2026-03-15")
		if !ok || got.Year() != 2026 || got.Month() != time.March {
			t.Errorf("parseYearMonth(2026-03-15) = %v, %v", got, ok)
		}
	})
	t.Run("unparseable", func(t *testing.T) {
		if _, ok := parseYearMonth("not-a-month"); ok {
			t.Error("parseYearMonth(not-a-month): want ok=false")
		}
	})
}

func TestRawJSON(t *testing.T) {
	t.Run("empty input uses the fallback", func(t *testing.T) {
		if got := string(rawJSON(nil, "[]")); got != "[]" {
			t.Errorf("rawJSON(nil, []) = %q, want []", got)
		}
		if got := string(rawJSON([]byte{}, "null")); got != "null" {
			t.Errorf("rawJSON({}, null) = %q, want null", got)
		}
	})
	t.Run("non-empty input passes through", func(t *testing.T) {
		if got := string(rawJSON([]byte(`{"a":1}`), "[]")); got != `{"a":1}` {
			t.Errorf("rawJSON({a:1}, []) = %q, want {\"a\":1}", got)
		}
	})
}
