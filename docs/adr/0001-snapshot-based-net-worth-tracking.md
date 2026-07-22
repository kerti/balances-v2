# Snapshot-based net worth tracking

Most personal-finance apps (Mint, YNAB) track every individual cash-flow transaction and reconstruct
balances by summing them. Balances v2 instead records end-of-month balance **snapshots** per
position as the primary data; net worth is computed by summing snapshots, not by integrating cash
flows.

The user's goal is net-worth tracking, not budgeting or cash-flow analysis. A monthly cadence with
one number per position is low-friction enough to sustain manually, and it works for positions that
have no transactions to record (properties, vehicles, illiquid debts). Investment instruments
additionally maintain a transaction ledger for cost-basis and income reporting, but snapshots remain
the source of truth for net worth.

Per-transaction expense tracking and budgeting/category breakdowns of spending are deliberate
non-features. A future change to a transaction-based model would require schema changes and a
rethink of what the app is for.

## Presentation / UX

The dashboard (the home tab) is the face of this decision. Net worth is presented headline-first —
the big current number and its trend — followed by the time-series chart. The chart's primary series
is net worth itself, drawn as an emphasized area.

Net worth is a composition, not an atom: the materialized report ([[adr-0006]]) already carries it
decomposed as `assets + receivables + investments − liabilities`. The chart surfaces that composition
as three **secondary lines** beneath the net-worth area, so a household can see *what moved* — an
investment run-up, a shrinking loan — not just the net result:

- **Assets** — the asset group, with **receivables folded in**. Receivables is a small, often-empty
  group; a fourth line for it would clutter the chart while rarely carrying signal, and folding it
  into assets keeps the three visible lines a clean, legible decomposition. The cost is that the
  visible lines then reconcile to net worth as `assets(+receivables) + investments − liabilities`,
  which is stated in-chart via the legend rather than left implicit.
- **Liabilities** — drawn as a **positive magnitude** (above the zero axis), matching how the engine
  stores it. Dipping the line below zero to signal "this subtracts" is more technically faithful but
  less readable for a non-technical household ([[adr-0038]] audience framing); a positive line that
  the legend labels as a debt is clearer. Liabilities reduce net worth even while drawn positive —
  the net-worth line already reflects the subtraction.
- **Investments** — the investment group at total closing value.

The three secondary lines are thinner and unfilled so the net-worth area reads first; they are
context for the headline, not co-equal to it. The same `SnapshotChartImpl` component renders both
this multi-line dashboard view and the single-series area on each position-group detail screen — the
detail screens pass one series and are visually unchanged.

**Invariant.** For every snapshot month, the charted composition (derived by `lib/netWorthComposition`)
reconciles to the net-worth line:
`nw_assets + nw_receivables + nw_investments − nw_liabilities = nw_total` (the fold and the
positive-magnitude choices are presentation-only and do not change this identity). Catalogued in the
PRESENTATION zone (INV-PRESENTATION-07) per [[adr-0034]] — the client-render-mirrors-backend-truth
zone, since this is a display faithfulness rule over the FINANCE number, not a new behaviour.

### Mobile (#507, [[adr-0050]])

The home tab is already the doctrine's target shape — a single-column `space-y-6` stack, not a
multi-column grid — so the [[adr-0050]] **"multi-column dashboard/hub grid → single-column stack"**
transform lands here as **pure CSS reflow, not a forked renderer**: no surface crosses the structural
bar (different DOM / different interaction), so no `useIsMobile()` split is warranted and the shared
`DashboardScreen` container is unchanged. The reflow closes the three squeeze failures a <768px audit
found against the [[adr-0050]] a11y floor:

- **Header toolbar** stacks below the title on phones (`flex-col` → `md:flex-row`), and its controls
  wrap — otherwise the month-picker + second-currency + PDF actions overrun the viewport and collide.
- **Group-breakdown rows** lift the label onto its own line above the bar+amount (`flex-col` →
  `md:grid`), so the primary figure stays readable instead of the fixed label/amount columns crushing
  the bar to nothing.
- **Tap targets** meet ≥44px at mobile width — the second-currency `<select>`, the shared
  `MonthPickerPopover` prev/trigger/next, the `ReportPdfButton`, and the rebuild footer links all size
  up on phones (`h-11`/`size-11` → `md:` back to the dense desktop height, the `md` breakpoint being
  the same 768px boundary the doctrine's `useIsMobile` uses). Sizing the shared
  toolbar controls responsively pre-satisfies the same floor on the Reports surface that reuses them.

The chart is already `w-full` and needs no change. This a11y floor (primary value reachable with no
horizontal scroll; tap targets ≥44px on a reflowed or diverged mobile surface) is catalogued lazily
as INV-PRESENTATION-08 and smoke-tested at 390px.
