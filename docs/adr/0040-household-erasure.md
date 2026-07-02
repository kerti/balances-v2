# Household erasure (hard delete)

**Status:** accepted

## Context

#222/#299/#300 (2026-07-02) settled prod SaaS data-protection on ordinary GDPR compliance rather
than zero-knowledge encryption. GDPR Art. 17 ("right to erasure") requires an honest hard-delete
path — the household's own **export** (epic #52, ADR-0036) already covers access/portability, so
erasure is the remaining piece. It also closes M7's "non-disposable environment" gate: any real
data, in preview or self-host, needs a real way out.

Restore (ADR-0036) already has an exact primitive for this: `wipeHousehold` deletes every
household-scoped row, children before parents, including sessions — driven by the same schema
walk this feature needs. Erasure is "restore with no load."

## Decision

- **Founder-only, single-step endpoint.** One request does the founder check + name-match +
  wipe. There is nothing meaningful to preview server-side (no incoming file to validate, unlike
  restore) — the frontend already renders the household's real name before the user types it — so
  the two-call preview/commit shape restore needs doesn't apply here.
- **Reuse `wipeHousehold` directly, no `loadHousehold` after it.** Lives in `internal/backup`
  alongside restore, not a new package — it's the same destructive-wipe primitive, one new call
  site. `local_credentials` and `password_reset_tokens` are already purged for free via
  `ON DELETE CASCADE` on `user_id` — no explicit delete statements needed for either.
- **Server-enforced confirm-by-name.** The request body carries the typed household name; the
  handler re-fetches the household and 400s on any mismatch before wiping. The frontend gates the
  submit button too, but the server is the real guard, matching the project's belt-and-suspenders
  habit for destructive actions.
- **No server-side export gate.** The confirm dialog offers a one-click export download as a
  strongly suggested first step with an explicit skip-and-delete path, but the backend has no way
  to verify a file was actually saved off-device, and no such proof-of-export concept exists
  anywhere else in the app. UI nudge only.
- **Best-effort peer notification.** Other members' accounts vanish with the household — reusing
  the `RestoreNotifier` shape's spirit but not its post-commit DB lookup (there is nothing left to
  query after the wipe): member email/locale is captured **before** the wipe runs, and the
  notification fires after a successful commit using that captured list. Fire-and-forget, doesn't
  fail the delete on send errors — matches ADR-0017's equal-peers model: peers who didn't choose
  this shouldn't just silently disappear.
- **No re-login after erasure.** Restore re-issues a session because there's a household to log
  back into; erasure has none. The response carries no new session; the frontend clears cached
  auth state and routes to a dedicated post-erasure screen rather than the login page, so the user
  isn't tempted to sign back in and re-trigger onboarding as if nothing happened.
- **Canonical term: Erasure.** Matches GDPR Art. 17 language directly and cross-references cleanly
  from the privacy policy (#299). Button/dialog copy can still read plainly ("Delete this
  household") — the domain term and the UI string don't have to match.
- **No migration.** Reuses existing schema and the existing `wipeHousehold` walk verbatim.

## Consequences

- `wipeHousehold` now has two callers with different post-conditions (restore reloads, erasure
  doesn't) — future household-scoped tables must still be added to the one `wipeDeletes` list to
  stay correct for both.
- The QA matrix gets new `INV-BACKUP-*` entries for the founder gate, name-match enforcement, and
  the wipe-with-no-load path (zone 20-backup, same zone as restore since it's the same mechanism).
