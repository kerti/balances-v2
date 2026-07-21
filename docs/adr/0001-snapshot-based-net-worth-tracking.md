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
