# Bulk monthly-entry mode

The recurring monthly snapshot ritual becomes an **entry mode layered onto the existing
[[adr-0043]] descriptor-driven Position lists**, not a new screen: a list of the positions eligible
for a chosen month, each row's last value pre-filled and its shape-appropriate input inline, a single
batch-level "when" control, and one atomic Save that persists **only the rows the user changed** —
leaning on the existing net-worth carry-forward rule for everything untouched.

## Why now

Entering a month's balances today means opening each position's own detail dialog — as many as ~40
dialogs across a real household (Executive Assessment CF-31, "the most valuable UX investment
available"). This makes the core product promise ("low-effort monthly ritual") *implicit* — the app
technically supports it but the workflow actively fights it. This is distinct from
`ImportSnapshotsDialog` (CSV template up/download, aimed at backfilling years at once), which is a
bulk-*history* tool, not the recurring low-friction monthly path.

## The decision

### A per-type entry view launched from the [[adr-0043]] list

Bulk entry is reached by an **"Enter this month" action on each type's list**, which opens a **dedicated
per-type entry view** backed by the month-scoped entry endpoint. It stays **per-type**, so it still
**dissolves the Investment two-shape problem** — the Stock entry view is uniformly qty×price rows, the
accruing-bond view uniformly accrued rows — and there is no heterogeneous mega-screen and no per-group
`/enter` surface to keep in sync.

**Refinement (found during the S1 build, #421).** An earlier draft made this a *mode toggle that flips
the same list rows editable in place*. Two frictions killed that: (1) the entry data is a **different
endpoint** (`GET …/snapshots/entry`, month-scoped eligible positions + carry-forward prefill) with a
different shape than the list's own query (all positions + latest snapshot) — the same rows can't just
"become editable"; (2) `PositionListScreen`'s core is a **read-only, sortable component the [[adr-0043]]
decision explicitly fences against god-config** — injecting editable inputs, batch dirty-state, the
when-control, and per-row errors into it is exactly the creep that ADR guards. So the entry view is its
own component that **reuses the descriptor's presentation-neutral projections** (name, secondary line,
currency) but owns the editable/batch-save interaction; the read-only list core is untouched. The spirit
of "reuse the descriptor, stay per-type, no new top-level nav" holds — it is launched *from* the list.

The three snapshot input shapes ([[adr-0022]]) map to their entry views unchanged: **amount-only**
(Asset/Liability/Receivable), **qty×price** (Stock/MutualFund/Gold), **accrued** (Bond/TimeDeposit —
total value + accrued interest, with the `accrues` vs `pays_out` coupon-disposition default carrying
over from the per-position form).

**Correction (found during the S3 build, #423):** an earlier draft of this line put "Bond/TimeDeposit
total-value" under **qty×price**. That was wrong. The [[adr-0022]] shape XOR — enforced by both the
`investment_snapshot_shape` CHECK and `validateInvestmentSnapshotShape` — binds bond/time_deposit to
the **accrued** shape (accrued_interest required, quantity/price forbidden); there is no qty×price path
for them anywhere in the code, and a bond's total value is already the `amount` field of its accrued
snapshot. So **qty×price is exactly Stock/MutualFund/Gold**, and Bond/TimeDeposit belong wholly to the
accrued slice (S4, #424).

### Statement-aligned, per-type batching — not one omnibus save

Households do not receive one omnibus statement; bank emails arrive together, the brokerage statement
separately. Entering per-type mirrors that data-arrival batching: ~40 dialogs collapse to a handful of
homogeneous lists, and a household rarely fills them all in one sitting. Per-type also makes each Save
touch exactly one snapshot table, which makes atomicity trivial (below).

### Save persists only dirty rows; untouched positions rely on carry-forward

Net-worth aggregation already uses each position's most-recent snapshot for any month lacking one (the
carry-forward rule, [[adr-0001]], CONTEXT.md). Therefore an unchanged position needs **no** row for the
target month — carry-forward already reports it correctly. Save writes **only rows whose value the user
changed from the prefill**. Writing a row per position per month would be pure accumulation with no
informational gain, and would force upserts against the `(position_id, year_month)` unique index on
every re-open.

- **Prefill is display, not a write.** Each row shows its carried value ("carried from May") so the
  household sees the complete current picture without re-saving it.
- **Re-entry upserts.** Because `(position_id, year_month) WHERE deleted_at IS NULL` is unique on every
  snapshot table, a dirty edit to a month that *already* has a row updates that month's snapshot
  (soft-delete-then-insert, or update-in-place), never a second insert.
- **Accepted trade-off:** there is no positive "confirmed in June" stamp for a position whose value
  genuinely did not change. Carry-forward makes net worth correct regardless; a per-row confirm /
  write-all was rejected as redundant snapshot bloat that also defeats the unique index on re-open.

### One batch-level "when", seeded from existing preferences

A single control at the top of entry mode sets the whole batch's timing; the phone-first tab-through
would die if every row asked:

- **`year_month`** defaults to the most-recent not-yet-entered month for that list (else the just-closed
  calendar month); one selector, applied to every saved row.
- **`as_of_date`** defaults from the user's existing `carryover_date_mode` preference
  (`today | end_of_last_month | end_of_month_after_last_snapshot`, mig 00002 / #105) — the pref that
  already answers "what date do I stamp a carried value with." One batch value on all saved rows.

Per-row override of either is out of scope; an oddball single-position as-of-date is the per-position
dialog's job. **Currency is inherited/read-only** — a position redenominating is a rare deliberate act
for the full dialog, and keeping the row to a single tab-stop (the number) is the point.

### Month-aware eligibility

The list for a target month shows exactly the positions that may legitimately hold a snapshot for that
`year_month`: owned, not deleted, `status = active` **or** terminated in the target month or later (you
still enter the closing month of an account you closed this month). A position terminated *before* the
target month never appears — carry-forward already froze it. Changing the target month re-filters the
list.

**Correction (found during the S1 build, #421):** an earlier draft of this ADR also excluded
"positions created after the target month." That was dropped. `created_at` is a record timestamp, not
economic existence, and gating on it breaks two real flows — the importer backfilling years of
pre-creation history, and onboarding (entering *last* month for an account added today) — while the
per-position dialog imposes no such guard. Eligibility is therefore ownership + the termination bound
only. TimeDeposit's placement→maturity window (a genuine economic bound, unlike `created_at`) remains
in scope for the investment slices, enforced by the same CHECK the per-position form uses — that is a
real date, not a record timestamp.

### Atomic batch write + one coalesced report regen

Save is **all-or-nothing**: the server validates every dirty row, commits nothing on any failure, and
returns **per-row errors keyed by position id** so the UI marks the offending rows. Because a Save is
scoped to one type, it is one snapshot table and one transaction. The materialized monthly report
([[adr-0006]]) is regenerated **once after commit** for the affected month (and any later months whose
carry-forward it changes, via the existing regen reach) — never once per row.

## Considered alternatives

- **One screen, every position, heterogeneous rows.** The literal reading of the finding. Fights the
  three input shapes (1 vs 2 fields, computed value, forced-entry copy) in one tab-through and
  re-creates the mixed Investment screen. Rejected for per-type lists, which are shape-homogeneous and
  match how statements arrive.
- **A dedicated `/enter` screen per group.** Duplicates the list-rendering [[adr-0043]] just
  consolidated and rebuilds the mixed Investment surface. Rejected for entry-mode on the existing
  lists.
- **Write a snapshot for every row every month (or a per-row confirm).** Gives an explicit monthly
  "confirmed" stamp, but is redundant against carry-forward, bloats the snapshot tables, and forces
  upserts on every re-open. Rejected; dirty-only is correct-by-construction.
- **Per-row `as_of_date` / currency.** Maximum flexibility, but kills the phone-first single-tab-stop
  row for a case the per-position dialog already covers. Rejected for batch-level defaults.

## Consequences

- A new bulk-write path per snapshot group (endpoint + repo) lands alongside the existing single-write
  handlers; both share validation. The endpoint is inherently idempotent per `(position, year_month)`
  via upsert.
- New QA-matrix invariants under the snapshots zone: dirty-only (untouched rows write nothing),
  re-entry upserts rather than duplicates, batch atomicity (any failure ⇒ no writes), month-aware
  eligibility, single coalesced regen. Covered per-PR (Go handler/repo tests + a component test on the
  entry mode); one representative Playwright @smoke journey.
- **Delivery is sliced**, tracer-bullet first: amount-only on the Asset list stands up the whole
  DB→repo→handler→UI spine (batch when-control, dirty-only atomic save, coalesced regen, prefill
  display); Liability + Receivable follow as near-free same-shape descriptors; qty×price and then
  accrued layer on. Each slice ships green and leaves main releasable.
- **CONTEXT.md is unchanged** — "Snapshot", the carry-forward rule, and Position lifecycle already
  carry the domain language; this is presentation + write-path architecture, not new domain terms.
- No migration: reuses the four existing snapshot tables and `carryover_date_mode`.
