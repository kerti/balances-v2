package email

import "testing"

// Layout is the shared branded shell both transactional senders render through.
// It must embed the caller's body verbatim and carry the inline "Balances"
// wordmark (the inline-branding decision — no image to block).
func TestLayout(t *testing.T) {
	out := Layout(`<p>hello body</p>`)

	for _, want := range []string{
		"<!DOCTYPE html>",
		">Balances<",        // inline wordmark header (no <img>)
		"<p>hello body</p>", // caller fragment embedded verbatim
		brandIndigo,         // brand accent applied
	} {
		if !contains(out, want) {
			t.Errorf("Layout output missing %q", want)
		}
	}

	// Inline branding by design: no remote/SVG/data-URI image to be blocked.
	if contains(out, "<img") {
		t.Error("Layout should carry no <img> (inline branding); found one")
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
