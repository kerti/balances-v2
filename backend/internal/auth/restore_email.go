package auth

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/kerti/balances-v2/backend/internal/db"
	"github.com/kerti/balances-v2/backend/internal/email"
)

// NotifyRestore sends the best-effort, per-recipient-localized emails that
// follow a successful restore (#176, ADR-0036): a "restore complete"
// confirmation to the restorer, and a relocation + security notice to every
// other live member (which doubles as a tamper tripwire). It is invoked from
// the backup commit handler only after the wipe+load transaction has committed,
// so it never fires on a failed restore.
//
// Best-effort throughout (ADR-0020): every Send error — and a failure to even
// list the members — is logged and swallowed. A mail outage (a fresh self-host
// may have no SMTP at all) must never reflect on the restore that already
// succeeded, so this returns nothing for the caller to fail on. Each recipient's
// message renders in their own user.locale; soft-deleted members are not mailed
// (ListUsersForExport with include_deleted=false drops them).
func (h *Handlers) NotifyRestore(ctx context.Context, householdID, restorerID uuid.UUID, itemCount int) {
	members, err := h.q.ListUsersForExport(ctx, db.ListUsersForExportParams{
		HouseholdID:    householdID,
		IncludeDeleted: false, // never notify a soft-deleted member
	})
	if err != nil {
		slog.Warn("restore notify: list members", "err", err)
		return
	}

	var restorer db.User
	for _, m := range members {
		if m.ID == restorerID {
			restorer = m
			break
		}
	}
	restorerName := restorer.DisplayName // empty only if the restorer is somehow absent
	now := time.Now()                    // formatted per-recipient (locale + time zone) below

	for _, m := range members {
		var msg email.Message
		if m.ID == restorerID {
			msg = h.restoreConfirmMessage(m, itemCount)
		} else {
			msg = h.restoreNoticeMessage(m, restorerName, now)
		}
		if err := h.mailer.Send(ctx, msg); err != nil {
			slog.Warn("restore notify: send", "to", m.Email, "err", err)
		}
	}
}

// restoreConfirmMessage renders the restorer's confirmation in their own locale.
func (h *Handlers) restoreConfirmMessage(user db.User, itemCount int) email.Message {
	openURL := h.frontendURL
	c := localizedEmail(restoreConfirmCatalog, user.Locale)
	greetingHTML := fmt.Sprintf(c.greeting, htmlEscape(user.DisplayName))
	greetingText := fmt.Sprintf(c.greeting, user.DisplayName)
	intro := fmt.Sprintf(c.intro, itemCount)

	html := email.Layout(fmt.Sprintf(`<p style="margin:0 0 16px;font-size:18px;font-weight:600;color:#0f172a;">%s</p>
<p style="margin:0 0 20px;">%s</p>
<p style="margin:0;"><a href="%s" style="display:inline-block;background:#6366F1;color:#ffffff;text-decoration:none;padding:10px 18px;border-radius:8px;font-weight:600;">%s</a></p>`,
		greetingHTML, intro, openURL, c.cta))

	text := fmt.Sprintf("%s\n\n%s\n\n%s:\n%s\n\n%s\n",
		greetingText, intro, c.cta, openURL, c.signoff)

	return email.Message{To: user.Email, Subject: c.subject, HTML: html, Text: text}
}

// restoreNoticeMessage renders the relocation + security notice for a non-restorer
// member in their own locale and time zone. restorerName is interpolated raw into
// the plain-text part and HTML-escaped into the HTML part; the restore instant is
// rendered as a human-readable date in the recipient's locale + time zone.
func (h *Handlers) restoreNoticeMessage(user db.User, restorerName string, when time.Time) email.Message {
	signInURL := h.frontendURL
	c := localizedEmail(restoreNoticeCatalog, user.Locale)
	date := localizedDate(when, user.Locale, user.TimeZone)
	greetingHTML := fmt.Sprintf(c.greeting, htmlEscape(user.DisplayName))
	greetingText := fmt.Sprintf(c.greeting, user.DisplayName)
	bodyHTML := fmt.Sprintf(c.body, htmlEscape(restorerName), date)
	bodyText := fmt.Sprintf(c.body, restorerName, date)

	html := email.Layout(fmt.Sprintf(`<p style="margin:0 0 16px;font-size:18px;font-weight:600;color:#0f172a;">%s</p>
<p style="margin:0 0 16px;">%s</p>
<p style="margin:0 0 20px;">%s</p>
<p style="margin:0 0 20px;"><a href="%s" style="display:inline-block;background:#6366F1;color:#ffffff;text-decoration:none;padding:10px 18px;border-radius:8px;font-weight:600;">%s</a></p>
<p style="margin:0;color:#64748b;font-size:14px;">%s</p>`,
		greetingHTML, bodyHTML, c.action, signInURL, c.cta, c.security))

	text := fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s:\n%s\n\n%s\n\n%s\n",
		greetingText, bodyText, c.action, c.cta, signInURL, c.security, c.signoff)

	return email.Message{To: user.Email, Subject: c.subject, HTML: html, Text: text}
}
