package auth

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/email"
)

// dispatchWelcomeEmail sends the founder's welcome email off the request
// goroutine (mirrors issueResetToken's h.dispatch use in reset.go): the caller
// has already committed the new Household+User by the time this is called, so
// nothing about signup depends on the send completing, only best-effort
// delivery (ADR-0020).
func (h *Handlers) dispatchWelcomeEmail(user db.User) {
	h.dispatch(func() {
		// The request goroutine is gone; use a fresh bounded context, not the
		// caller's.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := h.sendWelcomeEmail(ctx, user); err != nil {
			slog.Error("send welcome email", "err", err, "user_id", user.ID)
		}
	})
}

// sendWelcomeEmail greets a freshly-created founder and nudges them to invite
// the rest of their household. Mirrors sendInvitationEmail and renders through
// the shared branded layout (email.Layout). Best-effort per ADR-0020: the
// caller (dispatchWelcomeEmail) logs a Send error; a mail outage must never
// cost the founder their signup.
func (h *Handlers) sendWelcomeEmail(ctx context.Context, user db.User) error {
	inviteURL := h.frontendURL + "/settings"
	c := localizedEmail(welcomeCatalog, user.Locale)
	greetingHTML := fmt.Sprintf(c.greeting, htmlEscape(user.DisplayName))
	greetingText := fmt.Sprintf(c.greeting, user.DisplayName)

	html := email.Layout(h.frontendURL, fmt.Sprintf(`<p style="margin:0 0 16px;font-size:18px;font-weight:600;color:#0f172a;">%s</p>
<p style="margin:0 0 16px;">%s</p>
<p style="margin:0 0 20px;">%s</p>
<p style="margin:0;"><a href="%s" style="display:inline-block;background:#6366F1;color:#ffffff;text-decoration:none;padding:10px 18px;border-radius:8px;font-weight:600;">%s</a></p>`,
		greetingHTML, c.intro, c.invite, inviteURL, c.cta))

	text := fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s:\n%s\n\n%s\n",
		greetingText, c.intro, c.invite, c.cta, inviteURL, c.signoff)

	return h.mailer.Send(ctx, email.Message{
		To:      user.Email,
		Subject: c.subject,
		HTML:    html,
		Text:    text,
	})
}
