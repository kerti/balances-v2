# Pre-auth language picker, locale precedence, and backend email i18n

Adds a **language picker to the pre-auth sign-in surface**, seeds a new User's `locale`
**server-side at account birth** from that pick, retires the navigator-driven account mutation
introduced by [[adr-0026]], translates the **transactional emails** by recipient locale, and flips
the app's default language from `id-ID` to **`en-GB`**. Ships EN (`en-GB`) + ID (`id-ID`); the
frontend UI catalogs already exist (ADR-0026) — this closes the pre-auth and backend gaps.

Status: accepted. **Supersedes the first-login navigator-flip behaviour of [[adr-0026]]** (the
paragraph that read `navigator.language` on first login and PATCHed the User row to match).

## The decision

### The pre-auth picker is display-only

A language control on `SignInScreen` sets the i18next language + localStorage **for the
unauthenticated UI only**. It never PATCHes an account and never decides persistence — because
pre-auth we cannot know whether the visitor is a brand-new founder or a returning User. Persistence
is decided entirely *post-auth*.

Consequence accepted: a returning `id-ID` User who picks EN at the login screen sees EN on that
screen, then the UI reverts to their saved `id-ID` after sign-in. The toggle was only ever to read
the login page; a real change happens in Settings (the post-auth picker that ADR-0026 already
shipped). This keeps **one writable source of truth** and stops a transient toggle from silently
overwriting a saved preference.

### Locale is seeded server-side at account birth, not flipped client-side

The pre-auth pick rides the OAuth round-trip in a short-lived `oauth_locale` cookie — the same
pattern the flow already uses for `oauth_state` and `oauth_invite`. Both birth paths
(`createFounder` and the invited-member branch of `bootstrapNewUser`) seed
`user.locale = oauth_locale ?? "en-GB"` — **identical code**. This replaces the hard-coded `id-ID`
seed in both paths. A returning User's row already exists, so the hint is ignored and the DB wins.

This is why the client no longer needs a new-vs-returning heuristic: the DB is already correct when
the User row first loads. The old localStorage-presence heuristic was poisoned anyway — the pre-auth
picker now always writes localStorage, so "localStorage populated ⇒ returning user" no longer holds.

### Precedence chain

| Priority | Source | Effect |
|---|---|---|
| 1 | Saved `user.locale` (DB) | Wins for any returning User, always |
| 2 | Pre-auth pick → `oauth_locale` cookie → seed at account birth | Sets a fresh User's locale once |
| 3 | `navigator.language` | **Display-only** pre-fill of the pre-auth picker; never PATCHes |
| 4 | `en-GB` | Final fallback when navigator signals nothing supported |

`navigator.language` drops from "writes your account" (ADR-0026) to "pre-selects the login picker."
An Indonesian browser still lands on a pre-selected `id-ID` login screen, but the guess can never
silently mutate an account. `useLocaleReconcile` shrinks to: *User loads → sync UI to `user.locale`,
prime localStorage.* The navigator-flip + PATCH branch is deleted.

### Default flips to `en-GB`

The terminal fallback becomes `en-GB` (lingua-franca for an unknown visitor / broader-audience
discoverability) rather than `id-ID`. Applied in three places for a single coherent answer to "what
is the default language":

1. App seed fallback in both birth paths.
2. DB column default — migration `00005` (`ALTER COLUMN locale SET DEFAULT 'en-GB'`; additive).
3. Docs — CONTEXT.md User term; this ADR.

ID-reading Users are unaffected: the navigator pre-fill (priority 3) routes an Indonesian browser to
`id-ID` before the fallback ever fires. `en-GB` only wins when the browser signals nothing supported.

### Invite locale: inheritance via email language + accept-link, no schema change

A household usually shares a language, so an invitation inherits the **inviter's** locale — realized
without touching the `household_invitations` table:

- **Invitation email language** = `inviter.Locale` (already in scope in `sendInvitationEmail`).
- **Invitee picker pre-fill** = the accept link carries `?lng=<inviter.Locale>`, pre-selecting the
  language the email was written in (falls back to navigator if absent).
- **Override** = the invitee touches the same pre-auth picker before continuing.
- **Invitee account seed** = the `oauth_locale` cookie carries whatever the picker showed at
  click-time, so `bootstrapNewUser` runs the *same* `oauth_locale ?? "en-GB"` seed as the founder.

### Backend email i18n: a hand-rolled per-locale catalog

Emails render outside react-i18next's reach (Go `fmt.Sprintf` through `email.Layout`). A
`map[string]emailStrings` keyed by BCP47 holds subject + HTML fragment + text per email, looked up
by recipient locale with an `en-GB` fallback on a missing locale. Chosen over **`nicksnyder/go-i18n`**
(its ICU/plural engine buys nothing for ~3 plural-free emails × 2 locales, and it adds a dependency +
message-file format) and over **sharing the frontend JSON catalogs** (email copy ≠ UI copy, HTML
structure lives in Go, and coupling the Go build to `src/locales/*` paths is fragile). Subject lines
are localized; the product name **"Balances" stays literal in every locale** (it is the brand, not a
translatable string).

## Considered alternatives

- **Persist the pre-auth pick for existing Users** (literal reading of "explicit choice wins"). A
  transient login-screen toggle silently overwriting a saved account preference; rejected — explicit
  choice means Settings or the first-sign-in seed, not the display toggle.
- **Keep the navigator-flip auto-PATCH from ADR-0026.** Redundant once an explicit picker exists, and
  exactly the silent mutation we rejected above. Retired.
- **`go-i18n` / shared FE catalogs for emails.** Covered above. Rejected.
- **Carry the invite locale on the `household_invitations` row.** A migration for what
  `inviter.Locale` + an accept-link query param already deliver. Rejected.

## Consequences

- **Migration `00005`** flips the `users.locale` column default to `en-GB` (additive; the CHECK set
  is unchanged). Atomic with this feature per the migration-batching policy.
- **`useLocaleReconcile` loses its navigator-flip + PATCH branch** and its localStorage-presence
  heuristic; it only syncs UI to `user.locale`.
- **`createFounder` / `bootstrapNewUser`** stop hard-coding `id-ID`; both seed from the
  `oauth_locale` cookie. `/login` sets the cookie from the SPA's chosen display locale.
- **Transactional emails are localized** by recipient locale via the per-locale catalog; the welcome
  email keys on the just-seeded `user.Locale`, the invitation on `inviter.Locale`.
- **ADR-0026's first-login navigator-flip paragraph is superseded** by the precedence chain above.
