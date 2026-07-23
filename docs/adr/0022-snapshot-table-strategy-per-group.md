## Snapshot table strategy: one table per position group

Monthly snapshots are stored in **four tables**, one per position group: `asset_snapshots`,
`liability_snapshots`, `receivable_snapshots`, and `investment_snapshots`. Each snapshot table has a
real foreign key to its parent group table; there is no polymorphic FK. The three CONTEXT.md shapes
(amount only; quantity + price; accrued interest + total) collapse onto these four tables with
`investment_snapshots` carrying subtype-conditional nullable columns plus a `CHECK` enforcing shape
integrity.

`asset_snapshots` (from ADR-0009 / M3.1) is the first such table and the template for the rest.

## Considered alternatives

- **Option A — single polymorphic `snapshots` table** with `position_type` enum and `position_id`
  referencing one of four group tables depending on type. Rejected — polymorphic FK loses DB-level
  referential integrity, and the nullable-column matrix grows fast as Investment subtypes pile on.
  Cross- position queries would be slightly simpler (no UNION), but at the cost of schema clarity
  we'd live with for years.
- **Option C — one table per *shape*** (`amount_snapshots`, `price_quantity_snapshots`,
  `interest_snapshots`). Rejected — shape doesn't map cleanly to user intent ("the user reads a bond
  statement" not "the user records an interest-shaped snapshot"), and `amount_snapshots` would still
  need polymorphic FK across Asset / Liability / Receivable. We'd also have to rename and migrate
  the existing `asset_snapshots` for no real win.
- **Option D — one table per Position subtype** (`bank_account_snapshots`, `property_snapshots`, …,
  ~11 tables). Rejected — table sprawl with little benefit; net-worth aggregation becomes long UNION
  ALLs across all snapshot tables; sqlc query duplication.

## Investment shape integrity

`investment_snapshots` will have:

| Column | Used by |
|---|---|
| `amount` (required) | all subtypes |
| `currency` (required) | all subtypes |
| `quantity` (nullable) | Stock, MutualFund, Gold |
| `price_per_unit` (nullable) | Stock, MutualFund, Gold |
| `accrued_interest` (nullable) | Bond, TimeDeposit |
| `year_month`, audit fields, soft-delete | all subtypes |

The required value column is named `amount` (not `total_value`) for cross-group consistency with
`asset_snapshots.amount`, `liability_snapshots.amount`, and `receivable_snapshots.amount`. This lets
the four snapshot tables present a uniform `(year_month, amount, currency)` shape to net-worth
aggregation and to the shared frontend snapshot components.

For the accrued-interest shape (Bond, TimeDeposit), `amount` is **dirty** — it is the total position
value including any accrued interest since the last coupon/interest payout. `accrued_interest` is
carried alongside as a *breakdown* column for income-tracking visibility (e.g., "of which IDR X is
unpaid accrual" on a detail view, or to separate capital from income in a future income statement).
It is never added to `amount` for aggregation. This convention keeps net-worth aggregation uniform:
every snapshot table's `SUM(amount)` is authoritative without needing per-shape adjustments.
Programmers reading the schema should not interpret `accrued_interest` as additive to `amount` — the
relationship is that `amount` *contains* it.

A `CHECK` constraint enforces the XOR shape:

```sql
CHECK (
    (quantity IS NOT NULL AND price_per_unit IS NOT NULL AND accrued_interest IS NULL)
    OR
    (quantity IS NULL AND price_per_unit IS NULL AND accrued_interest IS NOT NULL)
)
```

Postgres can't reference other tables in a `CHECK`, so we can't enforce "investment.subtype =
'stock' implies snapshot has quantity + price" at the DB level. That part lives in the repository
and is covered by integration tests. The XOR check still catches "rows that satisfy no real shape"
and "rows that try to satisfy both," which is the main programming-error class.

## Consequences

- Four snapshot tables, each with a real FK to its parent group table.
- Net-worth aggregation is a `UNION ALL` of four queries, all carrying `amount + currency +
  year_month` (subtraction for liabilities applied at the aggregate level).
- The three amount-shape tables (`asset_*`, `liability_*`, `receivable_*`) have identical column
  lists. This minor duplication is acceptable; collapsing them would force polymorphic FK or rename
  gymnastics.
- The existing `asset_snapshots` from M3.1 needs no schema change — it's already the right shape and
  FK.
- Future per-group leak tests (mirroring `assets_tenancy_test.go`) follow the same pattern: every
  snapshot mutation verifies the parent position belongs to the requesting household via JOIN or
  CTE.

## Presentation / UX

The three **group landing pages** — `AssetsHome`, `InvestmentsHome`, `LiabilitiesHome` (epic #204,
issue #14) — are the face of this per-group aggregation. Each reads its group's snapshot tables and
renders one card-set **per currency** (no FX, mirroring the list-screen convention): a total-value
headline, a value-over-time line, a 100%-stacked category-share area, and a category-mix pie
(`InvestmentsHome` adds cost/unrealized-P/L to the headline and a second risk-profile pie). The
charts are shared components (`SnapshotChart`, `CategoryStackChart` / `GroupCategoryStackChart`,
`InvestmentPieChart`) so all three hubs render identically-styled series from the same snapshot
shapes.

### Mobile (#510, [[adr-0050]])

The hubs are already the doctrine's target shape — a single-column `space-y-6` stack whose only
multi-column node is `InvestmentsHome`'s pie pair (already `md:grid-cols-2`, i.e. stacked below
768px). So the [[adr-0050]] **"multi-column dashboard/hub grid → single-column stack"** transform
lands here as **pure CSS reflow, not a forked renderer** (mirroring the dashboard, [[adr-0001]]): no
surface crosses the structural bar, so no `useIsMobile()` split is warranted and the shared chart
components are unchanged. The reflow closes the two squeeze failures a <768px audit found against the
[[adr-0050]] a11y floor:

- **Header toolbar** stacks below the title on phones (`flex-col` → `md:flex-row`), so the
  bulk-entry action(s) — two of them on `InvestmentsHome` — no longer crowd or overrun the title.
- **Tap targets** meet ≥44px at mobile width: each `size="sm"` bulk-entry `Button` sizes up
  (`h-11` → `md:h-8`, the `md` breakpoint being the same 768px boundary the doctrine's `useIsMobile`
  uses). On `InvestmentsHome`, whose two bulk-entry buttons (enter-interest + enter-prices) would
  otherwise cram against the right edge, the pair **splits the row evenly** on phones (`flex-1` →
  `md:flex-none`, staying on one line at half-width each) and returns to content-width on the right on
  desktop.
- **Multi-currency headlines** stack one figure per line on phones. A mixed household renders
  `Rp X · $ Y` inline on desktop; on mobile each currency drops onto its own line (`block md:inline`
  on the per-currency span, the `·` separator `hidden md:inline`) instead of running off the primary
  figure's right edge. This covers the total headline on all three hubs and the value / cost /
  unrealized-P/L lines of the shared `InvestmentListHeadline`. **Single-currency is unchanged** — the
  labelled cost/P/L rows keep label + figure on one line. The same fix rides the sibling
  `ListHeadline` (the shared headline of every flat [[adr-0043]] list screen — receivables, bank
  accounts, properties, vehicles, and each investment-subtype list), so those headlines stack
  identically on mobile. **Receivables** has no chart hub — it is a flat descriptor list whose row
  layout already diverges via [[adr-0043]]'s `PositionListCards` / `PositionListTable`
  (`use-mobile.ts`) — so the shared-headline stacking is the whole of its share of this pass.

Every chart is already `h-64 w-full`, so charts read within the viewport with no horizontal scroll.
This a11y floor is catalogued as INV-PRESENTATION-08 ([[adr-0034]] / [[adr-0050]]) and smoke-tested at
390px.
