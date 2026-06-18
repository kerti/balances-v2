package email

import (
	"context"
	"log/slog"
)

// NoopMailer is a Mailer that discards every message. It is wired when
// EMAIL_ENABLED=false (ADR-0037) so a self-hoster can run with no SMTP
// dependency: Send always succeeds, every best-effort sender (invitation,
// welcome, restore) no-ops cleanly, and the invitation flow falls back to the
// "copy invite link" affordance since the create endpoint still returns the
// AcceptURL. Send returning nil (not an error) keeps the no-op invisible to
// call sites — an error would surface as a logged best-effort failure on every
// send, which is noise, not signal, when mail is deliberately off.
type NoopMailer struct{}

func NewNoopMailer() *NoopMailer { return &NoopMailer{} }

func (NoopMailer) Send(_ context.Context, msg Message) error {
	slog.Debug("email disabled (EMAIL_ENABLED=false); dropping message", "to", msg.To, "subject", msg.Subject)
	return nil
}
