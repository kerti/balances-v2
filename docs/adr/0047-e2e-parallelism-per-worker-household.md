# E2E parallelism: per-worker Household isolation, triggered by suite growth

The Playwright suite ([[adr-0024]]) runs **fully serial** — `fullyParallel: false`, `workers: 1`.
This ADR records *why* it is serial today, the concrete **trigger** for making it parallel, and the
**chosen lever** when that trigger fires — so the decision is made now, while it's cheap to reason
about, rather than scrambled together the week the suite becomes the bottleneck. No code changes
now; `workers` stays `1`.

## Why serial today

The suite runs against **one** dedicated `balances_e2e` database seeded to **one** fixture Household
(the Alice + Bob rows `seed-e2e` writes, [[adr-0024]]). Every spec authenticates as that Household
and mutates its shared state — positions, snapshots, settings, the theme row. Specs are written
order-independent and self-cleaning (e.g. `theme.spec.ts` drives to each value and ends on the
seed-pinned one), and a CI retry re-runs a spec against that same live DB. Two workers hitting the
one Household concurrently would race each other's rows non-deterministically, so `workers: 1` is
not a default left unexamined — it is load-bearing.

At the current size (~40 spec files) serial is fine: the `@smoke` subset gates PRs in well under the
per-PR budget, and the full suite runs nightly where wall-clock doesn't gate anyone.

## The trigger

Parallelize when **either**:

- the **`@smoke` gate** — the tier every contributor waits on per PR — approaches its budget (say,
  half of the e2e job's slice of the PR wall-clock), **or**
- the **nightly full suite** wall-clock grows enough to threaten the nightly window or delay a
  same-day regression signal.

As CF-27 framed it, this is roughly when the suite **doubles**. Recording the trigger keeps us from
paying the isolation complexity before it buys anything, and from discovering the wall only once
we've hit it.

## The chosen lever: per-worker Household, not per-worker database

Parallelism needs data isolation between workers. The app already has exactly that guarantee at the
data layer — **tenancy isolation** ([[adr-0005]]): a request authenticated as Household A cannot see
Household B's rows, enforced by the `household_id` filter on every per-Household query and guarded by
the `INV-TENANCY-*` leak tests. So *W* workers each authenticated as their **own** seeded Household
are genuinely isolated over **one** backend and **one** database — no per-worker schema, no per-worker
server.

Mechanics, when triggered:

1. Parameterize `seed-e2e` to mint *W* fixture Households, each with its own users + active
   `sessions` row (it already mints the one; this generalizes the count).
2. A Playwright **worker-scoped fixture** binds `storageState` / session to the worker's
   `test.info().parallelIndex`, so worker *i* always drives Household *i*.
3. Flip `fullyParallel: true` and raise `workers`.

The tenancy leak tests double as the correctness net: if per-worker isolation ever failed, they —
and the parallel specs themselves — would see foreign rows.

## What still shares process state

Isolation is per-Household, so anything **not** Household-scoped must be keyed per-worker or kept
serial:

- **The login / reset rate limiter** — shared in-process state keyed by client IP + email
  ([[adr-0039]]). Parallel auth specs from one runner share an IP and would cross-trip each other's
  backoff. Fix: give each worker a synthetic per-worker client IP (a header the e2e backend trusts),
  or pin the auth specs to a single worker.
- **Demo-mode singletons** — the `DEMO_MODE` reset endpoint and its shared account ([[adr-0041]])
  are process-global; the e2e backend doesn't run in demo mode, so this is a note, not a live
  hazard.
- **The en-GB locale pin** (`#12`) is process-global but read-only — parallel-safe as is.

## Alternatives considered

- **Per-worker database** (`balances_e2e_<n>`). Maximal isolation, but costs *W* migration runs and
  either *W* backends or DB-routing per worker — more machinery than needed, since tenancy already
  isolates at the row level. Kept as the fallback if a future test needs *schema*-level isolation
  (e.g. exercising a migration), not the first lever.
- **Cross-runner sharding only** (`--shard=i/n` across a CI matrix). Cuts CI wall-clock but not the
  local loop, and consumes CI minutes roughly linearly. It's **complementary** — a later,
  orthogonal multiplier once in-process worker parallelism is in place, best aimed first at the
  non-gating nightly full suite — not a substitute for worker isolation.
- **Accept serial indefinitely.** Correct until the trigger; this ADR exists so crossing the trigger
  is a config flip plus a seed tweak, not a redesign under time pressure.

## Consequences

- No change now — `playwright.config.ts` stays `workers: 1`; its "single household" comment points
  here for the path forward.
- When triggered, the work is bounded and known: seed parameterization + a worker-scoped session
  fixture + rate-limiter keying. Not a rewrite.
- The safety net (tenancy leak tests, [[adr-0005]]) already exists, so parallelism doesn't ship
  blind.
