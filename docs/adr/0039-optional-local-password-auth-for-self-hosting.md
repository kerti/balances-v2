# Optional local password authentication for self-hosting

[[adr-0017]] picked **Google OAuth as the sole identity provider** and explicitly parked "password
fallback" as a deferred additive layer. Self-hosting ([[adr-0037]], #116) cashes that deferral in:
OAuth is an **external dependency** every self-hoster must satisfy by creating their own Google Cloud
project, configuring a consent screen, and minting client credentials — friction that is wholly
disproportionate for a household running Balances on a Raspberry Pi at home. A self-hoster should be
able to stand up an instance with **no third-party identity provider at all**.

We add **email + password** as an optional, deployment-time-selectable identity provider alongside
Google. This is purely additive: hosted Balances keeps Google-only; a self-hoster can run
local-only, Google-only, or both.

## Decision

### Identity model

A User is identified by **at least one** credential, of either kind:

- `users.google_sub` becomes **nullable** (was `NOT NULL`). Still the immutable key for
  Google-authenticated users; null for local-only users.
- `users.password_hash` (text, nullable) — Argon2id hash for local users; null for Google-only users.
- A row-level `CHECK (google_sub IS NOT NULL OR password_hash IS NOT NULL)` keeps every live User
  reachable by some method.

`users.email` stays the human-facing handle and the invitation-match key, exactly as in
[[adr-0017]]. A future account-linking flow (one User, both credentials) is non-breaking under this
shape; it is **out of scope** here.

### Password hashing

**Argon2id** via `golang.org/x/crypto/argon2`, with per-hash random salt and parameters encoded in
the stored string (PHC `$argon2id$...` format) so they can be tuned without a schema change.
Rejected bcrypt: Argon2id is the current OWASP first choice and `argon2` is already in the Go
extended-stdlib orbit. Parameters tuned for an SBC, not a server — memory cost is the knob that
matters on a Pi.

### Provider enablement is a boot-time config decision

Two flags, both defaulting to the **hosted** posture:

- `AUTH_GOOGLE_ENABLED` (default `true`)
- `AUTH_LOCAL_ENABLED` (default `false`)

At startup the server **fails fast** if neither is enabled, and only constructs the Google OAuth
client (the `newGoogleOAuth` discovery call) when Google is enabled — so a local-only self-host needs
**no** Google credentials and makes **no** outbound OIDC discovery call. The SPA learns which methods
are live from a small public endpoint (extending the existing pre-auth config surface) and renders
only the buttons/forms for enabled providers.

### Flows

The server-side session machinery (`sessions` table, cookie, `SessionMiddleware`, sliding TTL) is
**unchanged** and provider-agnostic — every flow below ends by minting a session row exactly as the
Google path does today.

**Founder, local.** Self-register with email + password → goes through the **same onboarding gate**
as Google ([[adr-0038]]): no `users`/`households` row until the person commits the founder choice.
Founder email is **not** independently verified — the operator controls the instance; requiring a
verification round-trip on the very first account of a fresh self-host is friction with no adversary.

The accepted risk: on an instance the operator exposes to a network, the **first** local registration
founds the household, so whoever reaches the sign-up page first becomes Founder — there is no
out-of-band proof that the registrant owns the email they typed. This is a deliberate trade for
zero-dependency bootstrap, and it is bounded: it is a *first-run* window (once a household exists,
further local accounts are invite-only, §"Invited user, local"), and on the canonical SBC posture the
instance is reachable only on the operator's LAN / VPN. **Operators who expose a fresh instance to an
untrusted network before founding it must be warned** to register immediately, or to keep the instance
private until the founder account exists. This warning is an operator-doc deliverable, not a code
behaviour ([[adr-0037]] self-host docs); see Consequences.

**Invited user, local.** The invitation already carries a single-use token emailed to
`invited_email` ([[adr-0017]]). For a local account, **possession of the invite link proves email
control** — the invitee follows the link and sets a password, and the account is created bound to
`invited_email`. This is the local mirror of Google's email-match check and closes the same
link-forwarding loophole the original ADR cared about.

**Subsequent sign-in, local.** Email + password → verify Argon2id hash → mint session. Login is
rate-limited (per-IP and per-email) to blunt online guessing; lockout policy is deliberately light
(backoff, not hard lock) to avoid a self-host footgun.

**Password reset, local.** Two-pronged, by mail posture:

- **`EMAIL_ENABLED=true`** — self-service: emailed single-use token → set new password.
- **`EMAIL_ENABLED=false`** ([[adr-0037]] `NoopMailer`) — no mail to send the token through, so reset
  is an **operator CLI** subcommand on the binary (e.g. `balances reset-password <email>`) that
  prints a one-time set-password link (or sets a temporary password). The operator owns the box, so
  an out-of-band, operator-mediated reset is the natural airgapped path — the same shape as the
  email-off **invite** flow, where the `AcceptURL` is copied from the UI panel and handed over by
  hand. The emailed token is the convenience layer, not the only door.

Either prong is a thin slice and may land after the core login path, but reset is in scope.

### Local-only with mail off is fully functional

A self-host running `AUTH_LOCAL_ENABLED=true`, `AUTH_GOOGLE_ENABLED=false`, `EMAIL_ENABLED=false` —
the minimal airgapped Pi posture — has **no remaining external dependency** and every auth path
works: founder register/login locally; add a member via the copy-link invite panel (no mail);
recover access via the operator CLI reset. Welcome and restore mails simply no-op. This is the
recommended SBC default and a tested configuration.

## Backup and restore (amends [[adr-0036]])

[[adr-0036]] re-links a backup's members to the new instance by Google's stable `sub` and explicitly
left the non-Google case open: *"A future non-Google IdP, where no `google_sub` exists, would add an
identity match scoped to that absent-sub case — not a parallel email key."* This ADR is that IdP, so
two things resolve:

- **Position ownership re-links automatically and is unaffected.** Ownership and audit references
  (`sole_owner_user_id`, `created_by`/`updated_by`, tag assignment) are FKs to the User **UUID**,
  which [[adr-0036]] preserves verbatim. They never keyed on the auth identity, so a local-only
  household round-trips with every owner/tag reference intact, exactly as a Google one does. No
  change.
- **The backup carries `password_hash` (alongside `google_sub`).** A restored local member must
  satisfy this ADR's `CHECK (google_sub IS NOT NULL OR password_hash IS NOT NULL)` at load; carrying
  the hash keeps the row valid and preserves the member's existing credential across instances. This
  is a backup-format addition to the Users section — permitted because backup-format immutability only
  begins at the first **production** release ([[adr-0036]]/[[adr-0033]]) and we are pre-`1.0`.

- **Membership guard for a local restorer.** The guard (who may trigger the destructive wipe-then-
  load) matches the caller to a backup User row. For Google users it stays `google_sub`-only — **no**
  email OR-fallback, per [[adr-0036]]. For a user with **null `google_sub`** (local), the match is
  scoped to that absent-sub case: the restorer authenticates by **email + verifying their password
  against the carried `password_hash`**. That is *possession of the credential*, the local analog of
  "presents the same stable sub" — not a coincidental-address match, which is the failure mode
  [[adr-0036]] guarded against. After commit, re-login re-issues the session by the same key.

**Consequence:** backup files now contain Argon2id hashes. They already carry the household's full
financial data and are sensitive; the hashes raise the stakes of a leaked file only marginally
(Argon2id is built to be stored), but self-host docs must reinforce treating the backup as a secret.

## Considered alternatives

- **Keep OAuth-only; document a "bring your own Google project" setup for self-hosters.** Rejected —
  it is exactly the friction this ADR removes; a household on a Pi should not need a Google Cloud
  console account.
- **Magic-link (email-only, no password) as the local method.** Tempting — no password storage at
  all — but every steady-state login waits on an email round-trip, and it hard-couples *login* (not
  just reset/invite) to mailer reliability on a self-host. Password keeps login local and instant;
  email is needed only for invite and reset. Magic-link remains a possible future additive provider.
- **Passkeys / WebAuthn as the local method.** The modern ideal, but higher implementation cost and
  an awkward bootstrap on a headless SBC accessed from varied devices. Still the right *eventual*
  additive layer ([[adr-0017]] already flagged it); not the minimum that unblocks self-host.
- **bcrypt instead of Argon2id.** Rejected — Argon2id is the current best-practice default and lets
  us tune memory cost for SBC hardware.
- **A single `auth_provider` enum column instead of nullable credential columns.** Rejected — an
  enum fights the (non-breaking, future) both-credentials-on-one-User case; nullable columns + a
  CHECK express "at least one method" directly.
- **Omit `password_hash` from the backup; provision restored local members via the operator CLI.**
  Rejected — a restored local row with neither credential violates the at-least-one-credential CHECK
  at load, and the member loses their existing password. Carrying the hash keeps the row valid and
  the round-trip exact.
- **Email OR-fallback in the membership guard for all users.** Rejected — reintroduces exactly the
  coincidental-address risk [[adr-0036]] rejected for Google users. Email-keyed matching is scoped
  strictly to the null-`google_sub` (local) case, and even there gated by password verification.

## Consequences

- **Migration** (additive, then a constraint change): add nullable `password_hash`; **drop the
  `NOT NULL` on `google_sub`**; add the `CHECK (google_sub IS NOT NULL OR password_hash IS NOT
  NULL)`; the existing soft-delete-aware unique index on `google_sub` must tolerate nulls (partial /
  `WHERE google_sub IS NOT NULL`). The email-uniqueness story for local accounts needs the same
  soft-delete-aware treatment. Labelled `needs-migration` / `migration:additive` (the `NOT NULL`
  drop is widening, not destructive).
- `internal/auth` grows local-auth handlers (`register`, `login`, `reset`) beside the Google ones;
  `Handlers.New` stops hard-failing when Google config is absent and instead branches on the enable
  flags. The `googleOAuthClient` seam is untouched.
- The binary gains an **operator CLI** subcommand for password reset (`reset-password <email>`), the
  email-off reset path; it shares the token-minting logic with the emailed-token handler.
- New config keys (`AUTH_GOOGLE_ENABLED`, `AUTH_LOCAL_ENABLED`, Argon2id cost params) join the env
  surface ([[adr-0020]]); self-host docs ([[adr-0037]]) document the local-only recipe as the
  default SBC path.
- **Operator-facing security note is a required deliverable** in the self-host docs ([[adr-0037]]),
  not just code: the first-run founder window (unverified first local registration founds the
  household), the guidance to found before exposing the instance to an untrusted network, and the
  reminder that backup files now contain password hashes and must be treated as secrets. Without
  this, the founder-verification trade-off is undocumented risk on the operator.
- Frontend gains an email/password form and conditional provider rendering driven by the public
  methods endpoint. Backend-owner's weak spot — AI-led, tracked in the issue.
- **Invariants:** new QA rows for "at least one credential per live User", "local-only boot needs no
  Google creds / makes no OIDC call", "invite link possession is the email proof for a local
  invitee", "local-only + `EMAIL_ENABLED=false` exercises every auth path (register / login / invite
  copy-link / CLI reset) with no outbound dependency", "a local-only household round-trips through
  backup→restore with ownership and the membership guard intact", and login rate-limiting. Annotated
  when the tests land.
- **Security surface we now own** (the cost [[adr-0017]] declined): password storage, reset,
  rate-limiting/lockout, and breach response — scoped to self-host, where the operator also owns the
  box. Hosted Balances stays Google-only and carries none of this unless `AUTH_LOCAL_ENABLED` is
  flipped.
- Not a `1.0.0` blocker on its own, but a self-host quality-of-life multiplier; if M7 closes before
  it lands it slips to M8 without reopening any decision here.
