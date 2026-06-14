# QA invariant catalog

The **rows** of the coverage matrix: the rules this app must never violate, each
with a stable ID. This file is **hand-authored** — it is the source of truth for
*what must be true*. Which tests actually verify each invariant is **computed**,
not hand-typed: see [`COVERAGE.md`](./COVERAGE.md) (generated — do not edit).

Anchored on the de-facto spec: `CONTEXT.md` (domain invariants) and the
[ADRs](../adr/). This is *not* a list of features — it is a list of properties
whose failure would silently corrupt data or leak a household's finances.

## How it works

1. Every invariant here has an ID: `INV-<ZONE>-<NN>`.
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
   this catalog, regenerates `COVERAGE.md`, and prints any **uncovered**
   invariant (catalogued here, annotated nowhere) and any **orphan** annotation
   (an ID referenced by a test but absent here). It is **advisory** today
   (exit 0); the `-strict` flag (future CI gate) makes an uncovered invariant a
   build failure.

## Scope (v1)

Seeded with ADR-0021's two *heavy* priorities — **tenancy isolation** and the
**financial calculations** — where correctness *is* the product. Other zones
(snapshots, lifecycle, import/export, auth, tags) are added per-feature as the
mechanism proves out. A blank coverage cell is a finding, not a defect in this
doc.

---

## Zone: TENANCY

ADR-0005's threat model: every per-household repository must filter on
`household_id`; a request authenticated as one household must see **zero** rows
of another. One forgotten `WHERE` is a cross-tenant finance leak. Each row below
is the isolation guarantee for one resource. Severity: **Critical**.

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-TENANCY-01 | Bank-account/asset reads & mutations never cross households | ADR-0005 | Critical |
| INV-TENANCY-02 | Property reads & mutations never cross households | ADR-0005 | Critical |
| INV-TENANCY-03 | Vehicle reads & mutations never cross households | ADR-0005 | Critical |
| INV-TENANCY-04 | Liability reads & mutations never cross households | ADR-0005 | Critical |
| INV-TENANCY-05 | Receivable reads & mutations never cross households | ADR-0005 | Critical |
| INV-TENANCY-06 | Investment reads & mutations never cross households | ADR-0005 | Critical |
| INV-TENANCY-07 | Investment-transaction reads & mutations never cross households | ADR-0005 | Critical |
| INV-TENANCY-08 | Position-lifecycle mutations never cross households | ADR-0005, ADR-0009 | Critical |
| INV-TENANCY-09 | Monthly-report reads never expose another household's positions | ADR-0005 | Critical |
| INV-TENANCY-10 | Income reads & mutations never cross households | ADR-0005 | Critical |
| INV-TENANCY-11 | FX-rate reads & mutations never cross households | ADR-0005 | Critical |
| INV-TENANCY-12 | Tag reads, assignment & breakdown never cross households | ADR-0005, ADR-0028 | Critical |

## Zone: FINANCE

> _Seeded next (the second heavy zone): comprehensive-income identity (ADR-0008),
> investment-return formula, snapshot carry-forward, net-worth aggregation, FX
> conversion. Calcs live in `internal/repo/monthly_reports_engine.go`,
> `internal/reports/`, and `internal/income/` (ADR-0021's old `internal/finance/`
> reference was corrected to these when this catalog was seeded)._
