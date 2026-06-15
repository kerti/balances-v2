# Zone: NOTIFICATIONS

> _Seeded next — **transactional email delivery** (`internal/email`, ADR-0020).
> The Mailer is wired behind an interface so the provider can be swapped without
> touching call sites (SMTP / Mailpit in dev, Resend in prod). This zone sits
> **beneath AUTH's invite-token invariants**: INV-AUTH-06/07/08 own the token's
> randomness / single-use / email-match; this zone owns the **message that
> carries the token** — the side-effect path none of the token tests exercise.
> The catalog bar applies (a leaked/misaddressed token or a lost invite, not mere
> mechanics)._
>
> **Candidate rows** (today's only sender is the invitation email —
> `Handlers.sendInvitationEmail` in `internal/auth/invitations.go`):
> - **Mailed to the right address only** — `Send` goes to `invite.InvitedEmail`
>   with an accept URL embedding the *issued* token (`?invite=<token>`). A
>   misaddressed message would hand a working invite token to the wrong person —
>   the privacy/security edge that clears the bar. Cross-link INV-AUTH-08
>   (email-match guard on *acceptance*); this is the *delivery* side.
> - **Best-effort, non-blocking** — a `mailer.Send` error is logged
>   (`slog.Error`) and the invitation **still persists and the 201 still returns
>   the AcceptURL**, so the inviter can share the link manually. Email failure
>   must never lose the invite or block creation — the deliberate ADR-0020
>   "treat Send as best-effort" contract. This is the highest-value row: it's the
>   one most likely to regress into a blocking call.
> - **HTML-escaped interpolation** — inviter + household display names are
>   `htmlEscape`d into the HTML body (the plain-text part is raw by design). An
>   injection guard on user-controlled display names rendered into an email.
>
> **Likely new work**: a **fake Mailer** capturing the `email.Message` to assert
> `To` / token-in-URL / the best-effort path (drop a `Send` error → invitation
> still created, 201 still carries AcceptURL). `internal/email/smtp_test.go`
> covers only the SMTP *implementation* (envelope/encoding), not the invitation
> *call site*, so this is genuinely uncovered. Targets:
> `internal/auth/invitations.go`, a new `invitations_email_test.go`. Survey the
> existing invitation tests first — the token invariants are already covered by
> AUTH, so only the email side-effect needs new annotations. Source: ADR-0020
> (mailer interface), ADR-0017 (invitations)._
