# Zone: NOTIFICATIONS

The **transactional email delivery** layer (`internal/email`, ADR-0020). The
Mailer is wired behind an interface so the concrete provider can be swapped
without touching call sites — SMTP / Mailpit in dev, Resend in prod. This zone
sits **beneath AUTH's invite-token invariants**: INV-AUTH-06/07/08 own the
token's randomness / single-use / email-match-on-acceptance; this zone owns the
**message that carries the token** — the side-effect path none of the token
tests exercise. The app has three transactional senders: the invitation email
(`Handlers.sendInvitationEmail`, `internal/auth/invitations.go`), the founder
**welcome email** (`Handlers.sendWelcomeEmail`, `internal/auth/welcome_email.go`),
and the **restore notifications** (`Handlers.NotifyRestore`,
`internal/auth/restore_email.go` — a restorer confirmation + a member
relocation/security notice, fired best-effort from the backup commit handler
after a successful restore). All render through the shared `email.Layout` shell.
The best-effort contract (INV-NOTIFICATIONS-02) is shared; addressing and
escaping are pinned per sender.
The zone grows as new senders land. The catalog bar
holds (a leaked/misaddressed token or a silently-lost invite, not mere
mechanics). `internal/email/smtp_test.go` covers only the SMTP *implementation*
(envelope/encoding), not the *call site* — these rows pin the call site. Source:
ADR-0020 (mailer interface), ADR-0017 (invitations).

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-NOTIFICATIONS-01 | Mailed to the right address only — `sendInvitationEmail` calls `mailer.Send` exactly once with `To: invite.InvitedEmail` (the lowercased/trimmed invitee), and the HTML body carries the accept URL embedding the *issued* token (`/api/auth/google/start?invite=<token>`). A misaddressed message would hand a working invite token to the wrong person — the privacy/security edge that clears the bar. The *delivery* side of the invite token; INV-AUTH-08 owns the email-match guard on *acceptance* | ADR-0020 / ADR-0017 / INV-AUTH-08 | High |
| INV-NOTIFICATIONS-02 | Best-effort, non-blocking — a `mailer.Send` error is logged (`slog.Error`) and the invitation **still persists and the 201 still returns the AcceptURL**, so the inviter can share the link manually. Email delivery must never lose the invite or block creation — the deliberate ADR-0020 "treat Send as best-effort" contract. The row most likely to regress into a blocking call that 500s the whole flow on a transient mail outage | ADR-0020 | Medium |
| INV-NOTIFICATIONS-03 | HTML-escaped interpolation — the inviter and household display names are `htmlEscape`d (`& < > "`) into the HTML body before interpolation; an injection guard on user-controlled display names rendered into an email. The plain-text part is raw by design (no markup to inject into) | ADR-0020 / invitations.go | Medium |
| INV-NOTIFICATIONS-04 | Mailed to the founder only — `sendWelcomeEmail` calls `mailer.Send` exactly once with `To:` the founder's own address (the just-created user), Subject `Welcome to balances`, and an HTML+text body carrying the invite CTA (`${FrontendURL}/settings`). Fires on `createFounder` only — never on invitation acceptance (an invited member already got the invite email). The per-message addressing row for the welcome sender; mirrors INV-NOTIFICATIONS-01 for the invitation sender | ADR-0020 / handlers.go (`createFounder`) | Medium |
| INV-NOTIFICATIONS-05 | HTML-escaped interpolation (welcome) — the founder display name (from Google OAuth claims) is `htmlEscape`d into the welcome HTML body before interpolation. The per-message re-pin of INV-NOTIFICATIONS-03 for the welcome sender; the plain-text part is raw by design | ADR-0020 / welcome_email.go | Medium |
| INV-NOTIFICATIONS-06 | Locale-rendered welcome — the welcome email's subject + body render in the recipient founder's `user.locale` via the per-locale catalog, falling back to en-GB for an unknown locale; the brand name "Balances" is left literal in every locale | ADR-0035 / email_i18n.go | Medium |
| INV-NOTIFICATIONS-07 | Locale-rendered invitation — the invitation email's subject + body render in the `inviter`'s locale (the only locale signal before the invitee exists), with the same en-GB fallback and literal brand name | ADR-0035 / email_i18n.go | Medium |
| INV-NOTIFICATIONS-08 | Restore notification addressing & roles — after a successful restore, `NotifyRestore` mails exactly the **live** members: the **restorer** gets the "restore complete" confirmation (carrying the sanity-check item count), every **other** live member gets the relocation/security notice (which names the restorer and doubles as a tamper tripwire), and a **soft-deleted member is not mailed at all** (`ListUsersForExport` with `include_deleted=false`). Misrouting a role or mailing a soft-deleted user is the bar this row guards; fires on commit success only, never on a failed/refused restore | ADR-0036 / restore_email.go | Medium |
| INV-NOTIFICATIONS-09 | Locale-rendered restore emails — each recipient's restore email (confirmation or notice) renders in **their own** `user.locale`, not the restorer's, with the en-GB fallback and literal brand name. The per-recipient re-pin of INV-NOTIFICATIONS-06/07 for the restore senders — a member reading the security notice in the wrong language is the bar | ADR-0035 / ADR-0036 / restore_email.go | Medium |
| INV-NOTIFICATIONS-10 | Mail is gated by `EMAIL_ENABLED` (default true) — when false, `main` wires `email.NoopMailer` and skips SMTP construction entirely, so the app boots with no SMTP config; `NoopMailer.Send` returns nil for any payload, so every best-effort sender (invitation, welcome, restore) no-ops cleanly and the invitation still persists + returns its AcceptURL, which the **copy-invite-link** UI affordance surfaces for manual sharing. The self-host no-mail path ([[adr-0037]]); the SMTP wiring is unchanged when the flag is true | ADR-0037 | Medium |

> _Next sender to catalogue when it lands: today's senders are the invitation
> email, the founder welcome email, and the restore notifications (restorer
> confirmation + member relocation/security notice). When a fourth ships (e.g. a
> maturity / staleness digest), seed its rows here against its own call site —
> the Mailer interface and the best-effort contract (INV-NOTIFICATIONS-02)
> generalise, but the addressing and escaping rows are per-message and must be
> re-pinned. INV-NOTIFICATIONS-10 is cross-cutting (the `EMAIL_ENABLED` gate over
> every sender), not a new sender. This zone is complete at 10/10._
