package auth

import (
	"context"
	"fmt"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/email"
)

// sendWelcomeEmail greets a freshly-created founder and nudges them to invite
// the rest of their household. Mirrors sendInvitationEmail and renders through
// the shared branded layout (email.Layout). Best-effort per ADR-0020: the
// caller (createFounder) logs a Send error and proceeds — a mail outage must
// never cost the founder their signup.
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
