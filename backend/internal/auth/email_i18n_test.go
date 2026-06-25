package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/kerti/balances-v2/backend/internal/db"
)

// covers: INV-NOTIFICATIONS-06
// The welcome email renders in the recipient founder's locale. en-GB stays the
// existing English copy (pinned by TestCreateFounder_SendsWelcomeEmail); here we
// pin the id-ID rendering and that the brand name is left literal.
func TestWelcomeEmail_LocalizedByRecipientLocale(t *testing.T) {
	h := newAuthHarness(t)
	claims := &googleClaims{
		Sub:           "google-sub-id-welcome",
		Email:         "id-welcome@example.com",
		EmailVerified: true,
		Name:          "Budi",
	}
	if _, err := h.h.createFounder(context.Background(), claims, "id-ID", ""); err != nil {
		t.Fatalf("createFounder: %v", err)
	}

	sent := h.mailer.sent()
	if len(sent) != 1 {
		t.Fatalf("want 1 welcome email, got %d", len(sent))
	}
	msg := sent[0]
	if msg.Subject != "Selamat datang di Balances" {
		t.Errorf("Subject: want Indonesian welcome, got %q", msg.Subject)
	}
	if !strings.Contains(msg.HTML, "Selamat datang, Budi!") {
		t.Errorf("HTML missing Indonesian greeting; got %q", msg.HTML)
	}
	if !strings.Contains(msg.Text, "Selamat datang, Budi!") {
		t.Errorf("Text missing Indonesian greeting; got %q", msg.Text)
	}
	// The product name is a brand, not a translatable string.
	if !strings.Contains(msg.HTML, "Balances") {
		t.Errorf("brand name 'Balances' should stay literal in the id-ID email; got %q", msg.HTML)
	}
}

// covers: INV-NOTIFICATIONS-07
// The invitation email renders in the inviter's locale (the only locale signal
// available before the invitee exists).
func TestInvitationEmail_LocalizedByInviterLocale(t *testing.T) {
	h := newAuthHarness(t)
	inviter := db.User{DisplayName: "Budi", Email: "budi@example.com", Locale: "id-ID"}
	household := db.Household{DisplayName: "Rumah Budi"}
	invite := db.HouseholdInvitation{
		InvitedEmail: "guest@example.com",
		ExpiresAt:    pgtype.Timestamptz{Time: time.Now().Add(72 * time.Hour), Valid: true},
	}
	err := h.h.sendInvitationEmail(context.Background(), inviter, household, invite,
		"http://localhost:5173/accept?invite=tok")
	if err != nil {
		t.Fatalf("sendInvitationEmail: %v", err)
	}

	sent := h.mailer.sent()
	if len(sent) != 1 {
		t.Fatalf("want 1 invitation email, got %d", len(sent))
	}
	msg := sent[0]
	if !strings.Contains(msg.Subject, "mengundang Anda ke Balances") {
		t.Errorf("Subject: want Indonesian invitation, got %q", msg.Subject)
	}
	if !strings.Contains(msg.HTML, "Klik di sini") {
		t.Errorf("HTML missing Indonesian accept link copy; got %q", msg.HTML)
	}
	if !strings.Contains(msg.HTML, "Balances") {
		t.Errorf("brand name 'Balances' should stay literal in the id-ID email; got %q", msg.HTML)
	}
}

// covers: INV-NOTIFICATIONS-06
// An unknown / unsupported recipient locale falls back to en-GB rather than
// emitting raw keys or an empty body.
func TestEmailCatalog_FallsBackToEnGB(t *testing.T) {
	w := localizedEmail(welcomeCatalog, "fr-FR")
	if w.subject != welcomeCatalog["en-GB"].subject {
		t.Errorf("welcome fallback: want en-GB subject, got %q", w.subject)
	}
	inv := localizedEmail(invitationCatalog, "zz-ZZ")
	if inv.greeting != invitationCatalog["en-GB"].greeting {
		t.Errorf("invitation fallback: want en-GB greeting, got %q", inv.greeting)
	}
}
