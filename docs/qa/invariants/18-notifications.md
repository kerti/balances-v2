# Zone: NOTIFICATIONS

The **transactional email delivery** layer (`internal/email`, ADR-0020). The
Mailer is wired behind an interface so the concrete provider can be swapped
without touching call sites — SMTP / Mailpit in dev, Resend in prod. This zone
sits **beneath AUTH's invite-token invariants**: INV-AUTH-06/07/08 own the
token's randomness / single-use / email-match-on-acceptance; this zone owns the
**message that carries the token** — the side-effect path none of the token
tests exercise. Today the app's only transactional sender is the invitation
email (`Handlers.sendInvitationEmail`, `internal/auth/invitations.go`), so every
row below is anchored there; the zone grows as new senders land. The catalog bar
holds (a leaked/misaddressed token or a silently-lost invite, not mere
mechanics). `internal/email/smtp_test.go` covers only the SMTP *implementation*
(envelope/encoding), not the *call site* — these rows pin the call site. Source:
ADR-0020 (mailer interface), ADR-0017 (invitations).

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-NOTIFICATIONS-01 | Mailed to the right address only — `sendInvitationEmail` calls `mailer.Send` exactly once with `To: invite.InvitedEmail` (the lowercased/trimmed invitee), and the HTML body carries the accept URL embedding the *issued* token (`/api/auth/google/start?invite=<token>`). A misaddressed message would hand a working invite token to the wrong person — the privacy/security edge that clears the bar. The *delivery* side of the invite token; INV-AUTH-08 owns the email-match guard on *acceptance* | ADR-0020 / ADR-0017 / INV-AUTH-08 | High |
| INV-NOTIFICATIONS-02 | Best-effort, non-blocking — a `mailer.Send` error is logged (`slog.Error`) and the invitation **still persists and the 201 still returns the AcceptURL**, so the inviter can share the link manually. Email delivery must never lose the invite or block creation — the deliberate ADR-0020 "treat Send as best-effort" contract. The row most likely to regress into a blocking call that 500s the whole flow on a transient mail outage | ADR-0020 | Medium |
| INV-NOTIFICATIONS-03 | HTML-escaped interpolation — the inviter and household display names are `htmlEscape`d (`& < > "`) into the HTML body before interpolation; an injection guard on user-controlled display names rendered into an email. The plain-text part is raw by design (no markup to inject into) | ADR-0020 / invitations.go | Medium |

> _Next sender to catalogue when it lands: there is no second transactional
> email today. When one ships (e.g. a maturity / staleness digest), seed its
> rows here against its own call site — the Mailer interface and the
> best-effort contract (INV-NOTIFICATIONS-02) generalise, but the addressing and
> escaping rows are per-message and must be re-pinned. Until then this zone is
> complete at 3/3._
