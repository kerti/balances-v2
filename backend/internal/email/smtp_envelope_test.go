package email

import (
	"context"
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

// A malformed From must not crash construction (boot stays up — mail is
// best-effort), but the mailer must refuse to send rather than ship a bad
// reverse-path the relay silently 501s on (#195).
func TestNewSMTPMailer_RefusesUnparseableFrom(t *testing.T) {
	m, err := NewSMTPMailer(SMTPConfig{
		Host: "localhost",
		Port: 1025,
		From: "not-an-address",
	})
	if err != nil {
		t.Fatalf("NewSMTPMailer must not fail construction on an unparseable From: %v", err)
	}
	if m.fromErr == nil {
		t.Fatal("want fromErr recorded for an unparseable From, got nil")
	}
	if m.envelope != "" {
		t.Errorf("envelope = %q, want empty (no raw fallback)", m.envelope)
	}
	// Send must refuse without attempting delivery (no network needed).
	err = m.Send(context.Background(), Message{
		To:      "recipient@example.com",
		Subject: "hi",
		Text:    "hello",
	})
	if err == nil {
		t.Fatal("Send must refuse when From is unusable, got nil")
	}
}

// A secret with a stray trailing newline is the common real misconfiguration:
// stripping CR/LF rescues it to a clean, sendable address (#195).
func TestNewSMTPMailer_StripsTrailingNewline(t *testing.T) {
	m, err := NewSMTPMailer(SMTPConfig{
		Host: "localhost",
		Port: 1025,
		From: "noreply@example.com\n",
	})
	if err != nil {
		t.Fatalf("NewSMTPMailer: %v", err)
	}
	if m.fromErr != nil {
		t.Fatalf("want a usable mailer after stripping the newline, got fromErr=%v", m.fromErr)
	}
	if m.envelope != "noreply@example.com" {
		t.Errorf("envelope = %q, want %q", m.envelope, "noreply@example.com")
	}
}

// A CR/LF header-injection attempt is stripped; the remainder no longer parses,
// so the mailer refuses rather than emitting a From: header with smuggled
// extra headers (#195).
func TestNewSMTPMailer_RejectsHeaderInjection(t *testing.T) {
	m, err := NewSMTPMailer(SMTPConfig{
		Host: "localhost",
		Port: 1025,
		From: "noreply@example.com\r\nBcc: attacker@evil.example",
	})
	if err != nil {
		t.Fatalf("NewSMTPMailer must not fail construction: %v", err)
	}
	if m.fromErr == nil {
		t.Error("want fromErr for an injection attempt, got nil")
	}
	if strings.ContainsAny(m.cfg.From, "\r\n") {
		t.Errorf("cfg.From still carries CR/LF: %q", m.cfg.From)
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
