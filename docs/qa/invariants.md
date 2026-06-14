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

The calculation correctness that *is* the product (ADR-0021's second heavy
zone). The materialized monthly report (ADR-0006) derives net worth, the
comprehensive-income statement (ADR-0008), and the multi-currency conversion
(ADR-0002) from snapshots + transactions. A wrong number here silently misstates
a household's wealth — the failure is invisible until someone reconciles by hand.
The compute core is the pure, DB-free engine `internal/repo/monthly_reports_engine.go`;
its rules are unit-tested without a container.

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-FINANCE-01 | Net worth = Assets + Receivables + Investments − Liabilities (liabilities subtract) | ADR-0001 | Critical |
| INV-FINANCE-02 | Per-user/Joint net-worth attribution reconciles with the total | ADR-0004, ADR-0012 | High |
| INV-FINANCE-03 | A month with no fresh snapshot carries the latest snapshot ≤ M and is flagged stale | ADR-0006 | High |
| INV-FINANCE-04 | A position contributes nothing before its first snapshot; the series starts at the first month with data | ADR-0006 | High |
| INV-FINANCE-05 | A terminated position contributes through its termination month, then drops out | ADR-0009 | Critical |
| INV-FINANCE-06 | Comprehensive-income identity closes: ΔNW = EarnedIncome + InvestmentReturn + AssetValueChange − LivingExpenses | ADR-0008 | Critical |
| INV-FINANCE-07 | The first reportable month suppresses the derived income-statement lines (return, asset-value-change, living-expenses NULL) | ADR-0006, ADR-0008 | High |
| INV-FINANCE-08 | Investment return per instrument per month = ΔSnapshot + cash_out − cash_in | ADR-0008 | Critical |
| INV-FINANCE-09 | Transaction→cash-flow mapping: buy=in; sell/coupon/dividend/distribution=out; cash fee=in; unit-deducting fee=none; maturity=full terminal value out | ADR-0008 | Critical |
| INV-FINANCE-10 | Only property + vehicle revaluation lands in asset-value-change; bank cash and investment marks stay out of it | ADR-0008 | High |
| INV-FINANCE-11 | A liquidation (maturity/sale) books gain only — terminal-value cash_out offsets the truthful 0-value close, leaving no net-worth bubble | ADR-0008, ADR-0009 | Critical |
| INV-FINANCE-12 | A rolled time deposit's terminal-value cash_out is offset by the successor's cash_in; combined return is interest only, no phantom loss/gain even when the close snapshot under-accrues | ADR-0008 | Critical |
| INV-FINANCE-13 | Deployed capital nets to zero in the placement month (TD synthetic placement cash_in; bond Buy, incl. multi-tranche) | ADR-0008, ADR-0009 | Critical |
| INV-FINANCE-14 | Every earned-income category and investment-return subtype accumulates to its own bucket and sums to the total | ADR-0012 | High |
| INV-FINANCE-15 | A foreign amount is converted at the latest rate ≤ M (carry-forward) and the rate is recorded in fx_rates_used | ADR-0002 | Critical |
| INV-FINANCE-16 | A foreign currency with no rate ≤ M is excluded from net worth and flagged in missing_fx — never summed 1:1 | ADR-0002 | Critical |
| INV-FINANCE-17 | With multi-currency off, amounts sum at face value — no conversion, missing_fx, or fx_rates_used | ADR-0002 | High |

## Zone: LIFECYCLE

> _Seeded next (the write-side twin of FINANCE): the position state machine of
> ADR-0009, whose guarantees the report engine **assumes** on read. INV-FINANCE-11
> /-12/-13 only hold if the repo actually writes them on mutation. Candidate
> invariants: the status/terminated_at biconditional (active ⟺ no date, any
> terminal status ⟺ a date); a terminal flip upserts a **truthful 0-value close
> snapshot** at the termination month (the assumption #25 restored); an
> un-terminate deletes that close snapshot; a rollover links the successor via
> `rolled_from_investment_id`; an unknown status is rejected (400) before the DB.
> Write-side code lives in `internal/repo/lifecycle.go` (`validatePositionLifecycle`,
> `Update*Lifecycle`, `upsertCloseSnapshot`/`deleteCloseSnapshot`); existing tests
> in `lifecycle_*_test.go` + `investment_maturity_edit_test.go` are the annotation
> targets. Fill this table when seeding the zone._
