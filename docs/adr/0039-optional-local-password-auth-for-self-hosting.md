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

**Identity and credential are separated.** The `users` row is *who a household member is* (domain
data, portable across instances); a *credential* is *how they prove it on this instance* — and a
password is a **secret that must never leave the box**. The two live apart:

- `users.google_sub` becomes **nullable** (was `NOT NULL`). It stays **on `users`** because it is an
  *identifier, not a secret*: authentication happens at Google, so the stored value grants nothing on
  its own, and carrying it is what lets a Google member re-link automatically across instances
  ([[adr-0036]]). Null for local-only users.
- **Local password credentials live in a separate `local_credentials` table** (`user_id` FK,
  `password_hash`, salt/params in the PHC string), **not** as a column on `users`. This table is
  *instance-local auth state* — grouped with `sessions`, and like them **excluded from backup**
  (next section). Putting the hash in its own table makes "never serialize the secret" **structural**:
  the whole table is out of the export, so there is no per-column "remember to omit" footgun (e.g. a
  `SELECT *` in `backup.sql`).

A User is *reachable* when they have a `google_sub` **or** a `local_credentials` row. This is an
application-layer invariant, **not** a single-row CHECK — because a User can legitimately exist with
neither and be **dormant**: present in the data, owning Positions, but unable to authenticate until a
credential is (re)established. [[adr-0036]] already accepts exactly this state for a Google member who
has never signed into a new instance; local members post-restore reuse it (next section).

`users.email` stays the human-facing handle and the invitation-match key, exactly as in
[[adr-0017]]. A future account-linking flow (one User, both credentials) is non-breaking under this
shape — a second `local_credentials` row beside a `google_sub`; it is **out of scope** here.

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

**Password reset / member reactivation, local.** Three paths, by mail posture and who acts:

- **`EMAIL_ENABLED=true`, self-service** — emailed single-use token → set new password.
- **`EMAIL_ENABLED=false`, in-app, founder-assisted** — the founder/first account reactivates a member
  from the UI **without the CLI**. This is the friction-reducer for the no-mail home deploy: stand up
  the instance, restore, then help each household member back in from a screen. **Strictly scoped to
  reactivation:** it acts **only on a dormant member** (one with no `local_credentials` row), and it
  mints a **per-member random one-time set-password secret/link**, shown to the founder once to relay
  out-of-band — the same shape as the copy-link invite. It is **not** a standing "reset anyone"
  power: once a member holds their own credential, no member can silently re-set it in-app. This keeps
  the peer model ([[adr-0017]]: *Founder is lineage only, never a privilege*) — reactivating a
  credential-less row is operator bring-up, not impersonation of an active account.
- **Operator CLI** (`balances reset-password <email>`) — the out-of-band escape hatch, and the **only**
  way to reset an **active** member (one who already has a credential), since the in-app path
  deliberately refuses that. Available regardless of mail.

Rejected: a **shared/known default password** for reactivated members. A public default leaves each
account open until the member first logs in and changes it — an account-takeover window that
"home = LAN-only" wrongly assumes away (self-host includes VPN/Tailscale and occasionally exposed
setups). A per-member random one-time secret is no more friction and has no open window. A
force-change-on-first-login may be layered on but is not the safety mechanism.

Any path is a thin slice and may land after the core login path, but reset/reactivation is in scope.

### Local-only with mail off is fully functional

A self-host running `AUTH_LOCAL_ENABLED=true`, `AUTH_GOOGLE_ENABLED=false`, `EMAIL_ENABLED=false` —
the minimal airgapped Pi posture — has **no remaining external dependency** and every auth path
works: founder register/login locally; add a member via the copy-link invite panel (no mail);
reactivate or recover a member in-app, founder-assisted, with no CLI (the operator CLI remains the
escape hatch). Welcome and restore mails simply no-op. This is the recommended SBC default and a
tested configuration.

## Backup and restore (amends [[adr-0036]])

[[adr-0036]] re-links a backup's members to the new instance by Google's stable `sub` and explicitly
left the non-Google case open: *"A future non-Google IdP, where no `google_sub` exists, would add an
identity match scoped to that absent-sub case — not a parallel email key."* This ADR is that IdP. The
design goal here: do the restore dance for local-only households **without OAuth and without ever
serializing a password hash into the backup file.**

- **Position ownership re-links automatically and is unaffected.** Ownership and audit references
  (`sole_owner_user_id`, `created_by`/`updated_by`, tag assignment) are FKs to the User **UUID**,
  which [[adr-0036]] preserves verbatim. They never keyed on the auth identity, so a local-only
  household round-trips with every owner/tag reference intact, exactly as a Google one does. No
  change.
- **The backup carries no credential secret — `local_credentials` is excluded**, exactly as
  `sessions` and pending invitations are ([[adr-0036]]). The export is unchanged: identity (`users`,
  including the non-secret `google_sub`) goes in the file; the secret stays on the old box. **The
  backup format does not change**, and there is no at-load CHECK to satisfy because the credential
  invariant is dormancy-tolerant (see Identity model).
- **Restored local members are dormant, then (re)activated — symmetric with Google.** A Google member
  re-links on next sign-in (carried `google_sub`); a local member, having no carried secret, lands
  **dormant** (row present, owns data, no `local_credentials`) and is reactivated by the
  **founder-assisted in-app** flow (per-member one-time secret, no CLI — see "Password reset / member
  reactivation"), the operator CLI, or a self-served emailed reset when `EMAIL_ENABLED=true`. For a
  household of a few people this is a couple of reactivations after a disaster-recovery restore — the
  price of keeping secrets out of the file.
- **Membership guard.** The wipe-then-load is destructive, so the caller must be a member of the
  backup. The restorer **authenticates first** on the target instance (registering the bootstrap
  account on a fresh box, or already signed in), then the guard matches them to a backup User row by
  **stable identity**: `google_sub` for Google users (no email OR-fallback, per [[adr-0036]]); for the
  **null-`google_sub`** case, by **email** — scoped exactly to the absent-sub branch [[adr-0036]]
  foresaw. Confidentiality still rests where it already does: **possession of the backup file is the
  boundary** (the file *is* the data; matching by email grants nothing to someone who lacks it).

- **Reconciling the bootstrap UUID — the fresh one is discarded, not merged.** To authenticate, the
  restorer must exist as a User before the restore, so they get a **fresh UUID** (and, on a fresh
  deployment, a just-founded empty Household). The backup carries the member's **original UUID**. These
  are never reconciled by id; reconciliation is by the stable key above, exactly as [[adr-0036]]
  already does for Google. The restore is a single transaction that **deletes the bootstrap rows
  before inserting the backup rows**, so the backup's original UUID lands intact and the
  delete-before-insert dodges the unique-key collision (`google_sub` for Google; `email` for local) —
  the same collision-dodge [[adr-0036]] notes for the Google bootstrap. This is lossless: a
  just-created bootstrap account **owns no domain data** (zero Positions/snapshots — it is seconds
  old), so its fresh UUID and all FKs vanish with the wipe with nothing to carry. Every restored FK
  (`sole_owner_user_id`, tags, snapshots, txns) references **backup** UUIDs, which load verbatim, so
  the graph stays internally consistent. After load, the caller's session is re-issued against the
  **restored (original) UUID**.

- **Restorer continuity, no hash from the file.** For Google the re-issued session re-links by
  `google_sub` automatically. For local, the caller's credential is **carried across the wipe inside
  the transaction**: stash the bootstrap row's `local_credentials.password_hash` before the wipe, then
  re-insert it against the restored UUID at commit. The hash moves DB-row→DB-row on the same box and
  is **never read from or written to the file**; no plaintext is retained and the restorer stays
  logged in without a self-reset. (A simpler variant — re-prompt for the password at the destructive
  confirm and hash it fresh — is equivalent and also requires nothing from the file; the stash avoids
  the extra prompt.) Other local members remain dormant → `reset-password`, as above.

**Net:** local-only backup→restore needs no OAuth and puts **no password hash in the file**. The fresh
bootstrap UUID is throwaway; the backup's original UUID always wins; the only secret a backup holds
remains the household's financial data itself — unchanged from [[adr-0036]].

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
- **A single `auth_provider` enum column.** Rejected — fights the (non-breaking, future)
  both-credentials-on-one-User case; the identity/credential split expresses "any number of methods"
  directly.
- **`password_hash` as a column on `users` with a row-level `CHECK (google_sub OR password_hash)`.**
  Rejected — it forces the backup to either carry the hash (serialize a secret) or emit rows that
  fail the CHECK at load. The separate, backup-excluded `local_credentials` table sidesteps both: no
  secret in the file, and a dormancy-tolerant invariant instead of a hard CHECK.
- **Carry `password_hash` in the backup so members keep their password across instances.** Rejected —
  it puts an (offline-crackable) secret into a file users copy around, for the marginal convenience of
  skipping a post-restore reset. Disaster recovery for a few-person household tolerates a couple of
  reactivations; keeping the secret on the box is the better trade.
- **Reactivate members to a shared/known default password (members change it on first login).**
  Rejected — a public default leaves every reactivated account open until first login, an
  account-takeover window that the "home = LAN-only" assumption wrongly dismisses (self-host includes
  VPN/Tailscale and occasionally exposed instances). A per-member random one-time secret is no more
  friction and has no open window.
- **In-app "reset any member's password" as a founder power.** Rejected — setting an *active*
  member's credential is impersonation and breaks the peer model ([[adr-0017]]). The in-app path is
  scoped to **dormant** (credential-less) members only — bring-up, not takeover; the CLI is the
  escape hatch for an active member.
- **Email OR-fallback in the membership guard for all users.** Rejected — reintroduces the
  coincidental-address risk [[adr-0036]] rejected for Google users. Email matching is scoped strictly
  to the null-`google_sub` (local) case; confidentiality rests on possession of the backup file, not
  on the email.

## Consequences

- **Migration** (additive + one widening): **drop the `NOT NULL` on `users.google_sub`**; the
  soft-delete-aware unique index on `google_sub` must tolerate nulls (partial / `WHERE google_sub IS
  NOT NULL`); add the `local_credentials` table (`user_id` FK, `password_hash`, timestamps); give
  local accounts a soft-delete-aware unique on `users.email`. No `password_hash` column on `users` and
  no at-least-one-credential CHECK (replaced by the dormancy-tolerant app invariant). Labelled
  `needs-migration` / `migration:additive` (the `NOT NULL` drop is widening, not destructive).
- `internal/auth` grows local-auth handlers (`register`, `login`, `reset`) beside the Google ones;
  `Handlers.New` stops hard-failing when Google config is absent and instead branches on the enable
  flags. The `googleOAuthClient` seam is untouched.
- `internal/auth` gains a **founder-assisted in-app reactivation** handler scoped to dormant members
  (mints a per-member one-time set-password secret; refuses members who already hold a credential),
  plus an **operator CLI** subcommand (`reset-password <email>`) that is the email-off escape hatch
  and the only path to reset an **active** member. All three reset paths share the token-minting core.
- Backup export/restore is **unchanged** by this ADR — `local_credentials` is excluded like
  `sessions`, so no new sensitive field enters the file. Restore gains two small steps for the local
  case: match the caller by email (not just `google_sub`), and carry their credential across the wipe
  in-transaction (stash the bootstrap row's `password_hash`, re-insert it against the restored UUID at
  commit) so the throwaway bootstrap UUID is cleanly discarded and the caller stays logged in.
- New config keys (`AUTH_GOOGLE_ENABLED`, `AUTH_LOCAL_ENABLED`, Argon2id cost params) join the env
  surface ([[adr-0020]]); self-host docs ([[adr-0037]]) document the local-only recipe as the
  default SBC path.
- **Operator-facing security note is a required deliverable** in the self-host docs ([[adr-0037]]),
  not just code: the first-run founder window (unverified first local registration founds the
  household), the guidance to found before exposing the instance to an untrusted network, and the
  post-restore step that local members are dormant until reactivated (in-app founder-assisted, or
  CLI). Without this, the founder-verification trade-off is undocumented risk on the operator.
- Frontend gains an email/password form and conditional provider rendering driven by the public
  methods endpoint. Backend-owner's weak spot — AI-led, tracked in the issue.
- **Invariants:** new QA rows for "a reachable User has a `google_sub` or a `local_credentials` row
  (else dormant)", "local-only boot needs no Google creds / makes no OIDC call", "invite link
  possession is the email proof for a local invitee", "local-only + `EMAIL_ENABLED=false` exercises
  every auth path (register / login / invite copy-link / in-app reactivation) with no outbound
  dependency", "in-app reactivation acts only on a dormant member and refuses one who already holds a
  credential (no in-app reset of an active account)", "a backup file never contains a credential
  secret (`local_credentials` excluded)", "a local-only household round-trips through backup→restore
  with ownership intact, the bootstrap UUID discarded (backup's original UUID wins, no unique-key
  collision), and the caller re-bound to the restored row", and login rate-limiting. Annotated when
  the tests land.
- **Security surface we now own** (the cost [[adr-0017]] declined): password storage, reset,
  rate-limiting/lockout, and breach response — scoped to self-host, where the operator also owns the
  box. Hosted Balances stays Google-only and carries none of this unless `AUTH_LOCAL_ENABLED` is
  flipped.
- Not a `1.0.0` blocker on its own, but a self-host quality-of-life multiplier; if M7 closes before
  it lands it slips to M8 without reopening any decision here.
