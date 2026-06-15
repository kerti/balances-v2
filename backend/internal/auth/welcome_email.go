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
	name := htmlEscape(user.DisplayName)

	html := email.Layout(fmt.Sprintf(`<p style="margin:0 0 16px;font-size:18px;font-weight:600;color:#0f172a;">Welcome, %s!</p>
<p style="margin:0 0 16px;">Balances helps your household see its <strong>net worth</strong> in one place — bank accounts, property, investments, and what you owe. Each month you enter your balances from your statements, and Balances tracks how your total moves over time.</p>
<p style="margin:0 0 20px;">It works best with everyone in. Invite the people you share finances with so you're all looking at the same picture.</p>
<p style="margin:0;"><a href="%s" style="display:inline-block;background:#6366F1;color:#ffffff;text-decoration:none;padding:10px 18px;border-radius:8px;font-weight:600;">Invite your household</a></p>`,
		name, inviteURL))

	text := fmt.Sprintf(`Welcome, %s!

Balances helps your household see its net worth in one place — bank accounts, property, investments, and what you owe. Each month you enter your balances from your statements, and Balances tracks how your total moves over time.

It works best with everyone in. Invite the people you share finances with so you're all looking at the same picture.

Invite your household:
%s

— the Balances team
`, user.DisplayName, inviteURL)

	return h.mailer.Send(ctx, email.Message{
		To:      user.Email,
		Subject: "Welcome to Balances",
		HTML:    html,
		Text:    text,
	})
}
