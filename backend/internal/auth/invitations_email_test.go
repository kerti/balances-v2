package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/email"
)

// failingMailer always errors on Send — used to drive the best-effort path.
type failingMailer struct{}

func (failingMailer) Send(context.Context, email.Message) error {
	return errors.New("smtp: connection refused")
}

// covers: INV-NOTIFICATIONS-02
// covers: INV-NOTIFICATIONS-11
//
// Email delivery is best-effort (ADR-0020): a mailer.Send failure must be
// swallowed (logged, not surfaced) so the invitation still persists and the
// 201 still carries the AcceptURL the inviter can share by hand. Guards
// against a regression that makes Send a blocking dependency and 500s the
// whole flow on a transient mail outage. The 201 also reports the outcome via
// email_sent=false so the UI can nudge the inviter to share the link manually
// (INV-NOTIFICATIONS-11).
func TestHandleCreateInvitation_MailFailureIsBestEffort(t *testing.T) {
	h := newAuthHarness(t)
	h.h.mailer = failingMailer{}

	rec := h.do(t, "POST", "/invitations", map[string]any{
		"email": "guest@example.com",
	})

	// Creation succeeds despite the mail failure.
	requireStatus(t, rec, http.StatusCreated)
	body := decodeBody[createInvitationResp](t, rec)
	if !strings.Contains(body.AcceptURL, "invite=") {
		t.Errorf("accept_url missing invite token: %q", body.AcceptURL)
	}
	if body.EmailSent {
		t.Error("email_sent: want false when the mail send failed")
	}

	// The invitation row persisted — the failure cost nothing but the email.
	u, err := url.Parse(body.AcceptURL)
	if err != nil {
		t.Fatalf("parse accept_url: %v", err)
	}
	token := u.Query().Get("invite")
	if token == "" {
		t.Fatal("accept_url missing invite query param")
	}
	row, err := h.q.GetInvitationByToken(context.Background(), token)
	if err != nil {
		t.Fatalf("GetInvitationByToken: %v", err)
	}
	if row.ID != body.ID {
		t.Errorf("db row id: want %s, got %s", body.ID, row.ID)
	}
}
