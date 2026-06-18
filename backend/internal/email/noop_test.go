package email_test

import (
	"context"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/email"
)

// NoopMailer satisfies the Mailer interface — a compile-time guard that the
// EMAIL_ENABLED=false wiring path stays substitutable for the SMTP one.
var _ email.Mailer = email.NoopMailer{}

// covers: INV-NOTIFICATIONS-10
// TestNoopMailer_SendSucceeds: with mail disabled every send is a clean no-op —
// Send returns nil regardless of payload (even an empty one that SMTPMailer
// would reject), so best-effort senders never log a spurious failure and the
// invitation flow still persists + returns its AcceptURL (ADR-0037).
func TestNoopMailer_SendSucceeds(t *testing.T) {
	m := email.NewNoopMailer()

	cases := []email.Message{
		{To: "someone@example.com", Subject: "Invitation", HTML: "<p>link</p>", Text: "link"},
		{}, // empty payload SMTPMailer would 4xx — the no-op must still succeed
	}
	for _, msg := range cases {
		if err := m.Send(context.Background(), msg); err != nil {
			t.Errorf("Send(%+v) = %v, want nil", msg, err)
		}
	}
}
