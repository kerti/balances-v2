# Post-auth onboarding gate: resolve invited-vs-founder by verified email

A brand-new Google identity's first sign-in must resolve **which Household the person belongs to** —
join an existing Household they were invited to, or found their own. [[adr-0017]] branched this on the
*presence of an invite link* (a `?invite=token` cookie carried through the OAuth round-trip): empty
token → `createFounder`, silently and with no email check. That is irreversible — a User belongs to
exactly one Household with no leave/switch — and it mis-fires in two ways:

- **Invited user signs in without the link.** They click "Sign in with Google" on the landing page
  instead of the email link → empty token → they **found their own empty Household**; the real
  invitation rots until expiry. The welcome email (#160) then cheerfully confirms the wrong outcome.
- **Two people invite the same person.** No precedence rule for 0 / 1 / N pending invitations, and
  the non-chosen invites dangle.

We move the founder-vs-join decision from *before* authentication (link presence) to **after** Google
verifies the identity, and make it an **explicit, deliberate choice** rather than a side effect of
which button was clicked.

## Decision

Branch on the **verified email**, not the link. After the OAuth callback, when no `users` row matches
the Google `sub`, the backend creates **nothing** and issues **no session**. Instead it records a
short-lived **onboarding handshake** and redirects the SPA to an onboarding gate that presents the
choice. The `users` (and, for founders, `households`) row is created only when the person commits a
choice. Founding is now a deliberate act; the `?invite=` link degrades from *the decision* to a
*pre-selection hint*.

### Onboarding handshake

A new table holding the transient, pre-account verified identity — modelled on `sessions` (opaque
token in an httpOnly/secure/samesite=lax cookie → DB lookup → identify), **not** a signed cookie:

| Field | Notes |
|---|---|
| `id` | random opaque token — the cookie value (as `sessions.id`) |
| `google_sub` | verified Google subject |
| `email` | verified Google email — the key for the pending-invite lookup |
| `display_name`, `picture_url` | carried Google claims, used at deferred account creation |
| `seed_locale` | the [[adr-0035]] pre-auth locale-picker value, captured here so it survives to the deferred creation |
| `hint_invitation_id` | nullable — the invitation a clicked `?invite=` link refers to, pre-highlighted on the gate |
| `created_at` | timestamp |
| `expires_at` | timestamp; short (≈15 min) |

The handshake is **deleted on resolution** and swept by the same expiry path as `sessions`. It is
transient auth state, **not** Household data: it has no `household_id`, is never part of a backup, and
restore's per-Household wipe ([[adr-0036]]) does not touch it.

### The gate (0 / 1 / N pending invitations)

The gate lists pending invitations for the verified email
(`invited_email = handshake.email AND used_at IS NULL AND expires_at > now()`):

- **0** → only *Start your own household* (display-name field, pre-filled `"{Name}'s Household"`,
  optional override). No special confirmation — it is the sole path.
- **1** → *Join {Household} (invited by {Inviter})* **and** *Start your own instead*.
- **N** → one row per **distinct Household** (same-Household double-invites deduped, showing the most
  recent inviter; ordered most-recently-invited first), each joinable, plus *Start your own instead*.

The two-option screen is shown uniformly — a single pending invite is **never** auto-joined, because
the "invited but signed in directly" case produces exactly a 1-invite-no-link gate where the chooser
is the entire point. When pending invitations exist, choosing to found requires an explicit
confirmation ("You were invited to {Household}. Start your own instead? This can't be undone.").

### Commit (TOCTOU re-validation)

The choice `POST` re-validates server-side against the handshake; it does not trust the client's claim
of which invitation:

- **Join** → re-query the chosen invitation `WHERE id = ? AND invited_email = handshake.email AND
  used_at IS NULL AND expires_at > now()`. Valid → `CreateUser` into that Household, `MarkInvitationUsed`.
  No longer valid (expired/used between the gate's read and this write) → bounce to a refreshed gate.
- **Found** → `createFounder` (new Household + User from the handshake claims; welcome email #160 fires
  here, now only on the deliberate founder outcome).

On success: delete the handshake, issue the real session, redirect to the dashboard.

### Dangling and late invitations

Non-chosen pending invitations are **left to expire** (≈72h, [[adr-0017]]) — no schema marker, no
cross-Household write. Once the person has a `google_sub` row, every later callback takes the
existing-user branch *before* any bootstrap, so stray tokens are inert; and no UI lists pending
invitations, so a dangling row is invisible. An **already-onboarded** user arriving via a fresh invite
link is shown a gentle, non-blocking notice ("You're already in {Household} — invitations can't move
you, one Household per person") instead of the previous silent ignore.

## Considered alternatives

- **Keep link-based branching; look up by email only when the token is empty** (smaller change).
  Rejected — leaves two divergent code paths and still can't *decline* a clicked-link join; the
  irreversible founder mis-fire survives for the no-link case only, which is the very case that needs
  the chooser.
- **Eager-found, then offer "convert to join."** Rejected — violates the irreversible
  one-Household-per-User rule ([[adr-0017]]) and fires the welcome email on the to-be-undone outcome.
- **Hold the pending identity in a stateless signed (HMAC/JWT) cookie** instead of a DB row. Rejected
  — the app deliberately uses opaque DB-backed tokens, not signed cookies ([[adr-0017]]: instant
  revocation, auditability), and a signing key would become **a new required secret for every
  self-host operator** ([[adr-0037]]). The DB handshake needs zero new configuration.
- **Proactively void the non-chosen invitations on resolution.** Rejected — an honest marker
  ("superseded" ≠ "used" ≠ "time-expired") wants a new column and a write reaching across Household
  boundaries, all to tidy rows nobody can see and that the existing-user short-circuit already neuters.

## Consequences

- A new intermediate auth state exists between "Google verified" and "has an account." Nothing is
  written until the choice commits, so an abandoned/expired handshake leaves no partial state — the
  person simply re-authenticates and sees the gate again (idempotent).
- The SPA gains an onboarding route and the backend two endpoints (list options, commit choice). The
  route shares the first-sign-in surface with the [[adr-0035]] pre-auth language picker.
- `createFounder` no longer runs on an accidental empty-token sign-in, which un-mis-fires the welcome
  email (#160).
- Editing a Household's `display_name` after founding is **out of scope** here (a reversible Settings
  affordance, any member) and tracked separately (#265).

## Hardening: FOUNDING_DISABLED (#302)

An operator-only `.env` boolean, default open, gates the gate's **found** commit specifically — not
identity verification, so it applies uniformly to both providers ([[adr-0039]]). When set, `Found`
answers 403 and the options response carries `founding_disabled: true` so the SPA hides the "start
your own household" affordance before it's ever offered. The **join** commit and every invite-flow
invariant above are untouched: an operator can still invite members into an existing Household after
flipping the flag. Lets a preview/self-host instance freeze its household population once its known
users are onboarded, without inventing a new "public vs private instance mode" — see SELF-HOSTING.md's
"found the household before exposing the instance" for the sequencing this slots into.
