package pdf

import (
	"bytes"
	"testing"
	"time"
)

func TestRenderProducesPDF(t *testing.T) {
	out, err := Render(Input{
		YearMonth:         time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
		ReportingCurrency: "IDR",
		Locale:            "id-ID",
		NetWorth:          "6274520495.58",
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("output is not a PDF (prefix %q)", firstN(out, 8))
	}
	if len(out) < 2000 {
		t.Fatalf("PDF suspiciously small: %d bytes (font embed / content likely missing)", len(out))
	}
}

func TestRenderLocaleFallback(t *testing.T) {
	// en-GB (or any non-id locale) must still render without panicking.
	out, err := Render(Input{
		YearMonth:         time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
		ReportingCurrency: "IDR",
		Locale:            "en-GB",
		NetWorth:          "6274520495.58",
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatal("en-GB render is not a PDF")
	}
}

func TestVersionSuffix(t *testing.T) {
	// A build tag renders after the wordmark; "dev" and any real tag are shown
	// verbatim. An empty version yields no suffix (the slot reads as just the
	// wordmark), so a caller that can't resolve a version doesn't print " · ".
	cases := []struct {
		in, want string
	}{
		{"v0.8.0-alpha.1", " · v0.8.0-alpha.1"},
		{"dev", " · dev"},
		{"", ""},
	}
	for _, c := range cases {
		if got := versionSuffix(c.in); got != c.want {
			t.Errorf("versionSuffix(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRenderWithVersion(t *testing.T) {
	// A version in the Input must not break the render (footer draws the suffix).
	out, err := Render(Input{
		YearMonth:         time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
		ReportingCurrency: "IDR",
		Locale:            "en-GB",
		Version:           "v0.8.0-alpha.1",
		NetWorth:          "6274520495.58",
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatal("versioned render is not a PDF")
	}
}

func firstN(b []byte, n int) []byte {
	if len(b) < n {
		return b
	}
	return b[:n]
}
