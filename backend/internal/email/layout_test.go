package email

import "testing"

// Layout is the shared branded shell every transactional sender renders through.
// It must embed the caller's body verbatim and carry the hosted "Balances"
// wordmark image with a styled-text alt fallback (#163): best case the real
// letterforms, worst case (blocked/failed image) the brand name in plain text.
func TestLayout(t *testing.T) {
	const frontendURL = "https://app.example.test"
	out := Layout(frontendURL, `<p>hello body</p>`)

	for _, want := range []string{
		"<!DOCTYPE html>",
		"<img ",                               // hosted wordmark raster
		frontendURL + "/brand/email-logo.png", // served by the single-origin app (ADR-0030)
		`alt="Balances"`,                      // alt is the styled wordmark name — the fallback
		"<p>hello body</p>",                   // caller fragment embedded verbatim
	} {
		if !contains(out, want) {
			t.Errorf("Layout output missing %q", want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
