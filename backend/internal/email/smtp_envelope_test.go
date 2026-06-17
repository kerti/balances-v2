package email

import (
	"strings"
	"testing"
)

// Internal (package email) tests for the SMTP envelope reverse-path derivation
// (#192). A display-name From must keep its form in the From: header but be
// reduced to a bare address for MAIL FROM, or the relay returns 501 "Bad sender
// address syntax". These pin the implementation layer the NOTIFICATIONS zone
// explicitly scopes out of the call-site invariant catalog.

func TestNewSMTPMailer_EnvelopeFromDisplayName(t *testing.T) {
	m, err := NewSMTPMailer(SMTPConfig{
		Host: "localhost",
		Port: 1025,
		From: "Balances <noreply@example.com>",
	})
	if err != nil {
		t.Fatalf("NewSMTPMailer: %v", err)
	}
	if m.envelope != "noreply@example.com" {
		t.Errorf("envelope = %q, want bare %q", m.envelope, "noreply@example.com")
	}
}

func TestNewSMTPMailer_EnvelopeBareUnchanged(t *testing.T) {
	m, err := NewSMTPMailer(SMTPConfig{
		Host: "localhost",
		Port: 1025,
		From: "noreply@example.com",
	})
	if err != nil {
		t.Fatalf("NewSMTPMailer: %v", err)
	}
	if m.envelope != "noreply@example.com" {
		t.Errorf("envelope = %q, want %q", m.envelope, "noreply@example.com")
	}
}

// A malformed From must not crash construction — it degrades to the raw value
// (the send may still fail, but boot must not).
func TestNewSMTPMailer_EnvelopeFallbackOnUnparseable(t *testing.T) {
	m, err := NewSMTPMailer(SMTPConfig{
		Host: "localhost",
		Port: 1025,
		From: "not-an-address",
	})
	if err != nil {
		t.Fatalf("NewSMTPMailer must not fail on an unparseable From: %v", err)
	}
	if m.envelope != "not-an-address" {
		t.Errorf("envelope = %q, want raw fallback %q", m.envelope, "not-an-address")
	}
}

// The From: header keeps the full display-name form even though the envelope is
// reduced — the two uses are deliberately split.
func TestBuildMultipartMessage_KeepsDisplayNameInHeader(t *testing.T) {
	from := "Balances <noreply@example.com>"
	body, err := buildMultipartMessage(from, Message{
		To:      "recipient@example.com",
		Subject: "hi",
		Text:    "hello",
	})
	if err != nil {
		t.Fatalf("buildMultipartMessage: %v", err)
	}
	if !strings.Contains(string(body), "From: "+from+"\r\n") {
		t.Errorf("From: header missing display-name form %q in:\n%s", from, body)
	}
}
