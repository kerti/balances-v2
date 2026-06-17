# How the QA coverage matrix works

The mechanism behind the [invariant catalog](invariants/) and its generated
[coverage](coverage/). Read this once to understand the system; per-zone tasks
need only the zone's catalog file + its coverage file, not this page.

The catalog is **hand-authored** — the source of truth for *what must be true*.
Which tests actually verify each invariant is **computed**, not hand-typed.
Anchored on the de-facto spec: `CONTEXT.md` (domain invariants) and the
[ADRs](../adr/). The catalog is *not* a list of features — it is a list of
properties whose failure would silently corrupt data or leak a household's
finances.

## How it works

1. Every invariant has an ID: `INV-<ZONE>-<NN>`, catalogued in
   `docs/qa/invariants/<NN>-<zone>.md`.
2. A test declares which invariants it verifies with a `covers:` annotation —
   the same token in Go and TypeScript:

   ```go
   // covers: INV-TENANCY-01
   func TestAssetRepo_TenancyIsolation(t *testing.T) { ... }
   ```
   ```ts
   // covers: INV-TD-03, INV-LIFECYCLE-02
   test('time deposit maturity flips status', async ({ page }) => { ... })
   ```

3. `make qa-matrix` greps the suite for those annotations, joins them against
   the catalog, regenerates the per-zone files under `docs/qa/coverage/` plus
   their [index](coverage/README.md), and prints any **uncovered** invariant
   (catalogued but annotated nowhere), any **nightly-only** invariant (covered,
   but only by a non-smoke Playwright spec — see below), and any **orphan**
   annotation (an ID referenced by a test but absent from the catalog). `make
   qa-matrix` itself is **advisory** (exit 0); the **`-strict`** flag is the CI
   gate (`make qa-strict`, wired into `ci.yml` + `make check`) — it exits
   non-zero if any invariant is uncovered *or* nightly-only.

`make qa-gaps` (the `-gaps` flag) answers the inverse question — *which tests
aren't in the matrix?* — without the noise. A blanket list would be nearly every
test, since most legitimately verify mechanics, not catalogued invariants. So it
reports only **within-zone stragglers**: a test file with no `covers:`
annotation that sits in a directory where another test *does* carry one. Those
are the files most likely to be silently guarding a catalogued invariant.
Wholly-unannotated directories (an uncatalogued zone — an expected blank) are
excluded on purpose. Advisory only; it never rewrites the coverage files.

## How zones grow

Zones are added one at a time, heaviest/riskiest first (ADR-0021 put tenancy and
the financial calculations ahead of everything — correctness *is* the product
there), as a new `docs/qa/invariants/<NN>-<zone>.md` file. The numeric filename
prefix is the matrix's display order, so a new zone slots in at its risk rank.
A zone starts life as a `> _Seeded next…_` blockquote hint — candidate
invariants plus where the code and the annotation-target tests live — and becomes
a table when someone fills it. That hint convention is load-bearing, not
decoration: it's how the *next* session knows where to pick up. A blank coverage
cell is a finding, not a defect in the catalog.

## Frontend & E2E coverage

The `covers:` token is deliberately language-agnostic, so a browser or component
test can verify an invariant exactly like a Go test. Every annotation today is
still Go, but the tool now scans three test kinds — Go (`_test.go`), Playwright
(`.spec.ts`), and vitest (`.test.ts` / `.test.tsx`) — so a frontend annotation
counts the moment it's written. We'll seed frontend ones for invariants whose
verification genuinely lives in the UI — client-side guardrails with no backend
equivalent (the non-technical-audience design, ADR-0021's audience), or
end-to-end flows a handler unit test can't reach (e.g. the full OAuth
button→redirect→callback→session round-trip via the mock-OIDC server, ADR-0024,
of which only the callback half is unit-tested today).

## Tiering: what the per-PR gate credits

`-strict` enforces **per-PR** coverage, because the per-PR gate is what actually
protects a merge. Not every test kind runs per-PR, so coverage is tiered:

- **Go (`_test.go`)** and **vitest (`src/**/*.test.ts`, `.test.tsx`)** — run on
  every PR (`backend-test` / `frontend-checks`, i.e. `make check`). Always
  **per-PR**.
- **Playwright (`e2e/*.spec.ts`)** — *tiered* (#70): only `@smoke`-tagged tests
  gate per-PR; the full suite runs nightly (`e2e.yml`). So a spec test is
  **per-PR** iff it carries `{ tag: '@smoke' }`, else **nightly**.

The tool reads the `@smoke` tag of the `test()` a `covers:` annotation sits above
(the catalog convention is that the annotation is the line directly before its
`test()`), so it knows each location's tier. An invariant is **per-PR-covered**
if at least one of its covering tests is per-PR. If every covering test is
nightly-only, `-strict` reports it as **nightly-only** and fails the gate — the
same as uncovered — so the per-PR gate never credits coverage that didn't run in
the PR. To clear a nightly-only finding: `@smoke`-tag the covering spec so it
runs in the gate, or add a Go/vitest backstop.

This makes the gate honest by construction: a future non-smoke-only annotation
can't silently inflate the per-PR number. Vitest annotations remain the
friction-free ones — they run in the same gate that credits them.
