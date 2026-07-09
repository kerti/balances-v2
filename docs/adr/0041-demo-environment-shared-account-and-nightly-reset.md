# Demo environment: shared account, gated nightly reset, and `DEMO_MODE`

**Status:** accepted

## Context

#215/#217 stood up a public demo at `demo.<personal-domain>` — a real, running instance visitors can
poke at with no signup friction. That collides with several invariants elsewhere in the app that
assume a durable, privacy-bearing Household: real OAuth would collect every visitor's Google identity
(#217 item 2); an ungated instance lets any visitor found their own Household indefinitely (noise, and
exactly what [[adr-0038]]'s onboarding gate exists to make deliberate); and Founder-only whole-Household
**Erasure** ([[adr-0040]]) is irreversible and, on a demo where every visitor shares one identity, would
lock out every subsequent visitor until the next scheduled reset. #299/#300 (2026-07-02) made demo's
PII-free design a hard constraint, not a nice-to-have: no real OAuth, no real email sends, nightly
wipe, toy seed data only — this is what keeps demo out of privacy-policy/PSE-registration scope
entirely.

## Decision

**One shared, pre-founded Household, not per-visitor sandboxes.** The maintainer founds a single demo
Household once via the ordinary local-auth founder flow ([[adr-0039]]), and sets `FOUNDING_DISABLED=true`
([[adr-0038]] hardening, #302) on the demo instance so no visitor can found another. Every visitor
authenticates as the same shared local-password User. This is pure reuse — no new domain concept, no
new backend auth flow — versus a per-visitor ephemeral-Household model, which was rejected as a
disproportionate build (anonymous provisioning + orphan cleanup at Household granularity) for what a
demo needs.

**Seed shape.** The demo Household seeds one credentialed shared User plus a second, credential-less
member (**dormant**, in the [[adr-0039]] sense — no `local_credentials` row, never logged into) purely
so SoleOwner-vs-Joint ownership attribution ([[adr-0004]]) has someone to attribute to in the UI. Toy
positions span multiple position groups so the dashboard/net-worth report isn't empty on first visit.

**One umbrella `DEMO_MODE` flag** gates three demo-exclusive capabilities together, kept separate from
the general-purpose `FOUNDING_DISABLED`:

- **Erasure is blocked.** A Founder-only irreversible hard-delete ([[adr-0040]]) is unacceptable on an
  identity every visitor shares — one click would lock out every subsequent visitor until the next
  nightly reset. Every other mutation stays live; it's recoverable by the reset job.
- **`/api/auth/methods` gains a `demo_mode` field**, consumed by the SPA to pre-fill the local-login
  form with the shared credentials and show a caption stating them again in plain text (a deliberate
  redundancy against the visitor clobbering the pre-filled fields — there is no incremental
  confidentiality cost to displaying them, since possessing them grants nothing beyond what the
  pre-fill already grants).
- **The nightly reset endpoint only mounts when `DEMO_MODE` is on** — off-demo (self-host, preview,
  prod) it's a 404, not a wrong-token 403, matching the existing pattern of `AUTH_GOOGLE_ENABLED` /
  `AUTH_LOCAL_ENABLED` gating whether their respective routes/clients are constructed at all.

These three always travel together in practice; a single flag removes the footgun of an operator
setting the reset endpoint live while forgetting to also block Erasure.

**Nightly reset.** A GitHub Actions scheduled workflow (`schedule:` cron, using the `demo` GitHub
Environment's existing secrets, same shape as `deploy.yml` already driving Fly from CI) calls a new
admin endpoint that reuses `wipeHousehold` ([[adr-0036]]/[[adr-0040]]) followed by re-seeding, rather
than an in-process ticker inside the running server. The endpoint is bearer-authed by a dedicated
`DEMO_RESET_TOKEN` (constant-time compare) — deliberately not `FLY_API_TOKEN`, which is Fly's own
control-plane credential and shouldn't double as an app-level secret.

**Login rate limiting is unaffected.** [[adr-0039]]'s per-email/per-IP backoff is failure-count-based
only — every successful login clears both counters — so many visitors successfully authenticating
against the same shared email across a day trips nothing. Confirmed in `backend/internal/auth/ratelimit.go`.

**Mid-reset session collision is accepted, unhandled.** `wipeHousehold` deletes sessions too
([[adr-0040]]), so a visitor active when the nightly reset fires gets a 401 on their next request. The
existing session-expiry→login-redirect path handles this with no demo-specific code; building a grace
window or warning banner was rejected as disproportionate for a rare, cheap-to-recover-from edge case
at a fixed low-traffic hour.

**The writable-sandbox posture is disclosed, not fenced off (#346/CF-03).** The demo is a public,
writable sandbox behind one shared login, so any visitor can wipe-and-replace the shared Household with
offensive or personal content that stays visible until the next nightly reset. Two capabilities were
weighed: the sign-in notice and whether to demo-block `restore/commit`.

- **A one-screen sign-in notice is added**, shown only when `demo_mode` is on (same gate as the
  credential pre-fill): shared account, no real data, nightly wipe, no warranty, and a stated
  out-of-cycle reset the maintainer runs on request (via the footer project link — visitors have no
  reset button, and shouldn't, since it's `DEMO_RESET_TOKEN`-authed). This is the honest disclosure of
  what the shared sandbox is.
- **`restore/commit` stays live on demo — the defacement-until-reset window is an accepted, documented
  risk, not a blocked endpoint.** Erasure is blocked (above) for one specific reason: it strands the
  shared identity, locking out every later visitor until the reset. That reason does **not** apply to
  restore — the commit carries the caller's local credential across the wipe and re-issues a session
  ([[adr-0039]]), so the shared login keeps working afterwards. And a malicious restore defaces nothing
  an ordinary edit couldn't already deface (rename accounts, add offensively-named positions) — both are
  live mutations recoverable by the same nightly reset. Blocking restore would remove a headline feature
  ([[adr-0036]]) from the demo while closing none of the defacement window it purports to. So restore is
  treated as any other mutation, and the notice's disclosure is what covers the residual window.

## Considered alternatives

- **Per-visitor ephemeral Household sandboxes.** Rejected — needs new anonymous-provisioning and
  Household-granularity teardown machinery; disproportionate for a demo, and stretches "Household" past
  its durable-unit intent ([[adr-0004]]).
- **Dedicated one-click "Try Demo" auto-submit button** instead of pre-filling the existing local-login
  form. Rejected — identical security profile to pre-filling (either way, any visitor authenticates as
  the shared identity), for a new frontend code path with no offsetting gain.
- **In-process reset ticker** inside the running server. Rejected — couples reset timing to process
  uptime/restarts and introduces an always-on background-job pattern the codebase doesn't otherwise
  have; the GitHub-Actions-drives-Fly pattern already exists for deploys.
- **Leaning on the nightly reset as Erasure's only safety net** (i.e. leave Erasure enabled). Rejected —
  up to ~24h of "demo is down" from a single click is worse than the cost of one more gated endpoint.
- **Demo-blocking `restore/commit` alongside Erasure (#346/CF-03).** Rejected — restore doesn't strand
  the shared login (credential carried across the wipe, [[adr-0039]]) the way Erasure does, and it
  enables no defacement an ordinary edit doesn't; blocking it would cost a headline feature on the demo
  and close none of the window. The sign-in notice + nightly reset cover the residual risk instead.

## Consequences

- Standing up demo requires setting `AUTH_LOCAL_ENABLED=true`, `FOUNDING_DISABLED=true`, `DEMO_MODE=true`,
  and `DEMO_RESET_TOKEN=<secret>` on the `balances-demo` Fly app, plus the matching token as a
  `demo` GitHub Environment secret for the reset workflow. **Correction (2026-07-09):** moved to a
  plain repo secret instead — any job referencing a GitHub Environment auto-registers a Deployment
  against it, so the nightly cron was stamping `demo`'s deployment history with a bogus entry (pointing
  at `main` HEAD, not the live tag) on every reset, making demo look newer than the code actually
  running on it. `FLY_API_TOKEN` stays Environment-scoped for real deploys; `DEMO_RESET_TOKEN` doesn't
  need that protection (worst case: wipes/reseeds demo data, recoverable by re-running the reset).
- A future non-demo use of `FOUNDING_DISABLED` (e.g. a self-hoster freezing their instance) is
  unaffected — it isn't bundled into `DEMO_MODE`.
- `wipeHousehold` gains a third caller (reset) alongside restore and Erasure — same "every
  household-scoped table must stay in the one `wipeDeletes` list" obligation [[adr-0040]] already
  notes.
