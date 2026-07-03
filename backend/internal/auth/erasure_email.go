package auth

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/email"
)

// NotifyErasure sends the best-effort, per-recipient-localized emails that
// follow a successful household erasure (#300, ADR-0040): a "deleted"
// confirmation to the founder who triggered it, and a notice to every other
// member whose account no longer exists. members is captured by the caller
// BEFORE the wipe runs — unlike NotifyRestore, there is nothing left in the
// database to query afterwards.
//
// Best-effort throughout (ADR-0020): every Send error is logged and
// swallowed. A mail outage must never reflect on the erasure that already
// committed.
func (h *Handlers) NotifyErasure(ctx context.Context, members []db.User, founderID uuid.UUID, householdName string) {
	for _, m := range members {
		var msg email.Message
		if m.ID == founderID {
			msg = h.erasureConfirmMessage(m, householdName)
		} else {
			msg = h.erasureNoticeMessage(m, householdName)
		}
		if err := h.mailer.Send(ctx, msg); err != nil {
			slog.Warn("erasure notify: send", "to", m.Email, "err", err)
		}
	}
}

// erasureConfirmMessage renders the founder's "erasure complete" confirmation
// in their own locale.
func (h *Handlers) erasureConfirmMessage(user db.User, householdName string) email.Message {
	c := localizedEmail(erasureConfirmCatalog, user.Locale)
	greetingHTML := fmt.Sprintf(c.greeting, htmlEscape(user.DisplayName))
	greetingText := fmt.Sprintf(c.greeting, user.DisplayName)
	bodyHTML := fmt.Sprintf(c.body, htmlEscape(householdName))
	bodyText := fmt.Sprintf(c.body, householdName)

	html := email.Layout(h.frontendURL, fmt.Sprintf(`<p style="margin:0 0 16px;font-size:18px;font-weight:600;color:#0f172a;">%s</p>
<p style="margin:0;">%s</p>`, greetingHTML, bodyHTML))
	text := fmt.Sprintf("%s\n\n%s\n\n%s\n", greetingText, bodyText, c.signoff)

	return email.Message{To: user.Email, Subject: c.subject, HTML: html, Text: text}
}

// erasureNoticeMessage renders a non-founder member's deletion notice in their
// own locale.
func (h *Handlers) erasureNoticeMessage(user db.User, householdName string) email.Message {
	c := localizedEmail(erasureNoticeCatalog, user.Locale)
	greetingHTML := fmt.Sprintf(c.greeting, htmlEscape(user.DisplayName))
	greetingText := fmt.Sprintf(c.greeting, user.DisplayName)
	bodyHTML := fmt.Sprintf(c.body, htmlEscape(householdName))
	bodyText := fmt.Sprintf(c.body, householdName)

	html := email.Layout(h.frontendURL, fmt.Sprintf(`<p style="margin:0 0 16px;font-size:18px;font-weight:600;color:#0f172a;">%s</p>
<p style="margin:0;">%s</p>`, greetingHTML, bodyHTML))
	text := fmt.Sprintf("%s\n\n%s\n\n%s\n", greetingText, bodyText, c.signoff)

	return email.Message{To: user.Email, Subject: c.subject, HTML: html, Text: text}
}
