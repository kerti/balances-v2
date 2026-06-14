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
   (catalogued but annotated nowhere) and any **orphan** annotation (an ID
   referenced by a test but absent from the catalog). It is **advisory** today
   (exit 0); the `-strict` flag (future CI gate) makes an uncovered invariant a
   build failure.

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
Go; we'll seed frontend ones for invariants whose verification genuinely lives
in the UI — client-side guardrails with no backend equivalent (the
non-technical-audience design, ADR-0021's audience), or end-to-end flows a
handler unit test can't reach (e.g. the full OAuth
button→redirect→callback→session round-trip via the mock-OIDC server, ADR-0024,
of which only the callback half is unit-tested today).

Two wrinkles stand between here and that working:

- **Playwright (`e2e/*.spec.ts`)** — *scanned by `make qa-matrix` and run in CI,
  but tiered*: only `@smoke`-tagged specs gate per-PR; the full suite runs
  nightly (`e2e.yml`, #70). So if an invariant is covered *only* by a non-smoke
  spec, a per-PR `-strict` gate would credit coverage that didn't run in that PR
  — it runs nightly instead. Before E2E annotations count toward a per-PR strict
  gate, either tag the covering spec `@smoke` so it runs in the gate, or have
  strict accept nightly-verified coverage by design.
- **Vitest (`src/**/*.test.ts`)** — *run in CI on every PR (`frontend-checks` /
  `make check`), but not scanned*: the tool's file filter matches `.spec.ts`,
  not `.test.ts`. A one-line change to `tools/qa-matrix` (add the `.test.ts` /
  `.test.tsx` suffixes) makes vitest annotations count. Since vitest already
  runs in every PR, these are the **safe ones to plug in first** — no
  strict-gate hazard.
