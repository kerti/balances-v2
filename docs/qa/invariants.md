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

The write-side twin of FINANCE: the position state machine of ADR-0009, whose
guarantees the report engine **assumes** on read. INV-FINANCE-11/-12/-13 only
hold if the repo actually writes them on mutation — a terminated position must
carry a truthful 0-value close snapshot, a maturity must flip status, a rollover
must link its successor. A break here corrupts the derived return silently, the
same failure mode as the FINANCE zone but introduced at the mutation rather than
the calculation. Write-side code lives in `internal/repo/lifecycle.go` and the
maturity path of `internal/repo/investment_transactions.go`.

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-LIFECYCLE-01 | Lifecycle status is validated before the DB: group-defined enum + the status/terminated_at biconditional (active ⟺ no date; any terminal status ⟺ a date); violations are 400 | ADR-0009 | Critical |
| INV-LIFECYCLE-02 | A Maturity transaction flips the position to `matured` and sets terminated_at automatically | ADR-0009 | Critical |
| INV-LIFECYCLE-03 | An investment terminal flip writes a truthful 0-value close snapshot at the termination month (the INV-FINANCE-11/-13 read-side assumption) | ADR-0009, ADR-0008 | Critical |
| INV-LIFECYCLE-04 | Reactivating a terminated investment (back to active) drops that close snapshot, so it carries its last real value, not 0 | ADR-0009 | Critical |
| INV-LIFECYCLE-05 | Editing a Maturity transaction's date re-syncs terminated_at and relocates the close snapshot, leaving exactly one | ADR-0009 | High |
| INV-LIFECYCLE-06 | No further transaction is accepted on a terminal (matured) position — rejected with 409 | ADR-0009 | Critical |
| INV-LIFECYCLE-07 | Rollover successor linkage: linking sets `rolled_from_investment_id` / the source resolves `rolled_to`; self-link and unknown source are rejected (the INV-FINANCE-12 read-side assumption) | ADR-0009 | High |

## Zone: AUTH

The other half of the access-control threat model. TENANCY guards **which rows**
an authenticated household sees; AUTH guards **who you are** at the door, and
establishes the `household_id` every TENANCY filter then trusts. A break here is
the same finance leak TENANCY prevents, entered one layer earlier. Two security
hinges: the OAuth `state`/`session` cookies that authenticate a browser, and the
invitation flow that decides which household a brand-new user joins — a forwarded
invite link must never let an unintended Google account into someone else's
household. Code lives in `internal/auth/`: `session.go`
(`RequireAuth`/`SessionMiddleware`), `google.go` (OAuth + `randomState`),
`invitations.go`, `handlers.go` (callback + `bootstrapNewUser`/`createFounder`).

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-AUTH-01 | An unauthenticated request to a protected route is rejected with 401 by `RequireAuth` before the handler runs | ADR-0017, ADR-0005 | Critical |
| INV-AUTH-02 | The OAuth `state` is random and the callback rejects (400) any request whose `state` query param does not match the state cookie set at start (CSRF guard) | ADR-0017 | Critical |
| INV-AUTH-03 | A session is identified by a random opaque cookie value (HttpOnly, SameSite=Lax, Secure in prod); an unknown or expired session never authenticates, and a valid one attaches the user and slides the TTL | ADR-0017 | Critical |
| INV-AUTH-04 | Logout deletes the session row and clears the cookie, and is idempotent when no cookie is present | ADR-0017 | High |
| INV-AUTH-05 | First sign-in with no matching `google_sub` and no invitation bootstraps a brand-new household for the founder | ADR-0017 | High |
| INV-AUTH-06 | An invitation token is random, single-use, and expiring; an unknown, already-used, or expired token is rejected | ADR-0017 | Critical |
| INV-AUTH-07 | Accepting a valid invitation binds the new user to **only** the inviting household (not a new one) and marks the invitation used | ADR-0017, ADR-0005 | Critical |
| INV-AUTH-08 | Invitation acceptance requires the Google-supplied email to match `invited_email` (forwarded-link guard); a mismatch is rejected and leaves the invitation unconsumed | ADR-0017 | Critical |

## Zone: SNAPSHOTS

> _Seeded next — the valuation substrate beneath FINANCE. Every net-worth number
> the report engine derives traces back to a position **snapshot** (the dated
> value record, ADR-0006); FINANCE and LIFECYCLE both already *assume* its rules
> on read (INV-FINANCE-03's "latest snapshot ≤ M", INV-LIFECYCLE-03/04's close
> snapshot) — this zone guards them at the write/storage layer where they're
> actually enforced. Candidate invariants: a snapshot soft-deletes rather than
> hard-deletes (audit trail, ADR-0007); the engine's "latest value at-or-before
> M" selection ignores soft-deleted rows; a correction (re-snapshot at the same
> date) supersedes the prior value rather than double-counting; a snapshot's date
> can't precede the position's creation. Cross-household isolation is already
> INV-TENANCY-01/-06 — this zone is the **temporal/value** correctness, not the
> tenancy cut. Code lives in the snapshot write paths of `internal/repo/` (e.g.
> `assets.go`, `investments.go` value-update handlers) + the selection in
> `monthly_reports_engine.go`. Survey existing `*_test.go` in `internal/repo/`
> and `internal/assets|investments/` for annotation targets before writing new
> ones. Fill this table when seeding the zone._
