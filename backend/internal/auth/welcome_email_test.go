package auth

import (
	"context"
	"strings"
	"testing"
)

// The welcome email is the second transactional sender (after the invitation
// email). It fires on founder creation only. These tests pin its call site:
// addressing + content, HTML-escaping of the founder name, and the best-effort
// contract that a mail failure must never cost the founder their signup.

// covers: INV-NOTIFICATIONS-04
//
// Founder creation sends exactly one welcome email to the founder's own
// address, carrying the /settings invite CTA. A misaddressed or missing welcome
// is the bar this row guards — the founder is the only intended recipient.
func TestCreateFounder_SendsWelcomeEmail(t *testing.T) {
	h := newAuthHarness(t)

	claims := &googleClaims{
		Sub:           "google-sub-welcome",
		Email:         "newfounder@example.com",
		EmailVerified: true,
		Name:          "New Founder",
	}
	user, err := h.h.createFounder(context.Background(), claims)
	if err != nil {
		t.Fatalf("createFounder: %v", err)
	}

	sent := h.mailer.sent()
	if len(sent) != 1 {
		t.Fatalf("want exactly 1 welcome email, got %d", len(sent))
	}
	msg := sent[0]
	if msg.To != user.Email {
		t.Errorf("To: want founder's own address %q, got %q", user.Email, msg.To)
	}
	if msg.Subject != "Welcome to balances" {
		t.Errorf("Subject: want %q, got %q", "Welcome to balances", msg.Subject)
	}
	// CTA deep-links to the Settings screen (hosts InviteForm) on the frontend.
	wantCTA := h.h.frontendURL + "/settings"
	if !strings.Contains(msg.HTML, wantCTA) {
		t.Errorf("HTML missing invite CTA %q", wantCTA)
	}
	if !strings.Contains(msg.HTML, "New Founder") {
		t.Errorf("HTML missing greeting with founder name; got %q", msg.HTML)
	}
	if strings.TrimSpace(msg.Text) == "" {
		t.Error("Text part is empty; a plain-text alternative is required")
	}
	if !strings.Contains(msg.Text, wantCTA) {
		t.Errorf("Text missing invite CTA %q", wantCTA)
	}
}

// covers: INV-NOTIFICATIONS-05
//
// Re-pin of the HTML-escape guard for this sender: the founder display name
// (from Google's OAuth claims) is escaped into the HTML body. The plain-text
// part is raw by design.
func TestCreateFounder_WelcomeEmailEscapesName(t *testing.T) {
	h := newAuthHarness(t)

	claims := &googleClaims{
		Sub:           "google-sub-xss",
		Email:         "xss@example.com",
		EmailVerified: true,
		Name:          `Mallory <script>alert(1)</script>`,
	}
	if _, err := h.h.createFounder(context.Background(), claims); err != nil {
		t.Fatalf("createFounder: %v", err)
	}

	sent := h.mailer.sent()
	if len(sent) != 1 {
		t.Fatalf("want 1 welcome email, got %d", len(sent))
	}
	html := sent[0].HTML
	if strings.Contains(html, "<script>") {
		t.Errorf("raw <script> leaked into HTML body: %q", html)
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Errorf("display name not HTML-escaped in body: %q", html)
	}
}

// covers: INV-NOTIFICATIONS-02
//
// Best-effort, non-blocking (ADR-0020) generalized to the welcome sender: a
// mailer.Send failure is swallowed (logged) and the founder + household still
// persist and createFounder still returns the user. A transient mail outage
// must never cost someone their account.
func TestCreateFounder_WelcomeMailFailureIsBestEffort(t *testing.T) {
	h := newAuthHarness(t)
	h.h.mailer = failingMailer{}

	claims := &googleClaims{
		Sub:           "google-sub-besteffort",
		Email:         "besteffort@example.com",
		EmailVerified: true,
		Name:          "Resilient Founder",
	}
	user, err := h.h.createFounder(context.Background(), claims)
	if err != nil {
		t.Fatalf("createFounder must not fail on mail error: %v", err)
	}

	// The founder + their household persisted despite the mail failure.
	got, err := h.q.GetHouseholdByID(context.Background(), user.HouseholdID)
	if err != nil {
		t.Fatalf("household should persist despite mail failure: %v", err)
	}
	if !strings.Contains(got.DisplayName, "Resilient Founder") {
		t.Errorf("household display_name should derive from claim; got %q", got.DisplayName)
	}
}
