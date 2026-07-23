# Descriptor-driven detail-page consolidation

The ten per-type detail pages (`BankAccountDetail` … `GoldDetail`, `LiabilityDetail`,
`ReceivableDetail`) collapse into **one generic `PositionDetailScreen` core plus ten small
descriptors**, mirroring what [[adr-0043]] did for the list screens. The core owns every shared-shell
concern — back/title, `DetailTagControl`, the actions row (help/edit-trigger/terminate/delete +
`ConfirmDialog`/export), the `SnapshotChart` card, loading/error/not-found states, the page scaffold —
and hosts the two variable regions through **presentation-neutral primitives** (`InfoGrid`,
`HistorySection`). A descriptor supplies only wiring (entity key, i18n namespace, load/delete hooks,
`exportUrl`, tour copy) and slots for the group-specific bits. This is the [[adr-0050]] **D0**
prerequisite: consolidating the detail pages first means **one** mobile card renderer flips all ten,
rather than ten hand-forked card layouts.

## Why now

The detail pages are the last app surface still on ten hand-written ~300–700 LOC pages with no shared
shell — the [[adr-0043]] list screens, Dashboard, Income, Tags, hubs and Settings all diverged on
mobile during epic #428, but [[adr-0050]] explicitly **gated** the detail pages on this consolidation
rather than forking ten card layouts. Same two costs as 0043 drive it, not aesthetics:

- **Change amplification.** The shared shell is dead-consistent across all ten (every page imports the
  same eight shell pieces), so any cross-cutting change — the mobile header-stack a11y fix, a new
  action, an empty-state tweak — means editing ten pages by hand, and they have already drifted.
- **The mobile mandate.** [[adr-0050]]'s a11y floor (INV-PRESENTATION-08) has to land on every detail
  page. Hand-written pages would mean writing a *second* per-type layout for the card view — twenty
  files instead of ten. A descriptor lets two renderers share one data spec, exactly as 0043 argued.

## The decision

### The core operates on the Position shared surface only

CONTEXT.md treats **Position** as a first-class supertype across all four groups. The core touches
only that shared surface — `display_name`, `status`, `terminated_at`, `native_currency`, `tag_id`,
lifecycle, and the snapshot stream — which every group carries uniformly. **Every `details.*` field
reaches the page only through a slot the core calls but never inspects.** This is the load-bearing
constraint, restated from [[adr-0043]] for the detail layer: the moment the core would read
`bond_details.coupon_disposition` or a property address, that is a slot, not core logic. This is not
in tension with [[adr-0022]]'s non-polymorphic per-group storage — presentation genericity over the
supertype is a different layer and does not reintroduce storage polymorphism.

### Two orthogonal variation axes, not one cluster

The list screens ([[adr-0043]]) varied on a **2-cluster** boundary (Investment vs
Asset/Liability/Receivable) driven by risk-filter + headline/aggregation. The detail pages do **not**
vary on that axis. They vary on two *independent* axes:

- **snapshot-row shape** — amount-only / qty×price / accrued. This is the **S1/S2/S3** taxonomy the
  bulk-entry work (#502/#505/#506, [[adr-0046]]/[[adr-0050]]) already built split-renderers for; the
  detail history table **reuses those exact snapshot renderers** (`SnapshotRow`,
  `QuantityPriceSnapshotRow`, `AccruedInterestSnapshotRow`) rather than forking a fourth.
- **has-transactions** — the five investment types add a headline + transaction-history sections
  (trades, dividends/cash-income, fees, accrued-interest, maturity/rollover, coupons); the five
  amount-only types have none.

The descriptor keys its slots off these two axes. There is deliberately **no** detail-specific cluster
preset — the detail cut is not the 0043 cut, and forcing the 2-cluster shape onto it would misfit.

### The info section: shared grid fed a neutral `{ label, value }[]`

The details/info card body consolidates into a shared **`InfoGrid`** primitive the descriptor feeds a
presentation-neutral `infoFields: { label, value }[]`, where `value` is **already-formatted content**
(a string or neutral node). The core lays out label/value pairs and owns the responsive reflow
(grid → stacked pairs on mobile — the single reflow that justifies this whole consolidation); it never
inspects *which* fields or their domain meaning.

This is the region where "the descriptor becomes a god-config" risk lives, so the guardrail is hard,
and identical in spirit to 0043's "`render` returns presentation-neutral value content, not a
`<TableCell>`":

- Alignment/formatting/currency ride on the **value node itself** (`ml-auto`, `text-right`,
  `tabular-nums`, `formatCurrency` applied before hand-off). The core gives every value cell the same
  value column and makes it a flex container, so per-node right-align produces a lined-up column
  without the core learning what "numeric" means.
- **Banned:** a `{ align }` / `{ kind }` field **the core switches on**
  (`className={f.align === "right" && …}`). Core reading the knob is the god-config slide; node-level
  styling the core stays blind to is not.
- A genuine cross-row numeric-stack alignment need (rare in a detail card — most values are "BCA",
  "IDR", "5.5%") graduates to a **second neutral primitive** (`NumericInfoGrid`) the descriptor opts
  into wholesale, never a core-read field flag.

### One repeatable `HistorySection` primitive for all history tables

Snapshot-history and transaction-history collapse into **one** `HistorySection` primitive. Every
detail page renders ≥1 section (snapshots); investments render more (trades/dividends/fees/etc.). Each
section is `{ title, rows, renderRow, createDialog, pagination }`; the primitive owns the
card + table + pagination + **mobile table→cards reflow** and never inspects columns. `renderRow`
returns presentation-neutral row content — snapshot rows reuse the S1/S2/S3 renderers, transaction
rows reuse `TransactionRow` and friends; the primitive doesn't care which. Hosting the *transaction*
tables here (not just snapshots) is what makes [[adr-0050]]'s "one card renderer flips all the tables"
actually true — otherwise every transaction table would re-solve its own mobile reflow.

### The investment headline is an optional slot, already a shared component

The headline block *looks* like the high-variance region but its **markup is already a shared
`InvestmentHeadline`** with a uniform prop shape across all five investment types. The only per-type
variance is the **input**: `totalCost` — ledger replay via `lib/costBasis` (stock/MF/gold/bond) vs
flat `principal` (time deposit). So it folds in as `renderHeadline?` — an optional slot on the
has-transactions axis, present for investments, absent for amount-only, wrapping the existing shared
component with the per-type cost-basis computation as **wiring**. Core never computes cost basis — that
subtype quirk stays in the descriptor, the same line as core never touching `coupon_disposition`.

### Core / slot inventory

- **Core (hard JSX, never config):** back button, title (`display_name`), `DetailTagControl`, the
  actions row (`HelpTourButton` / edit trigger / `TerminatePositionDialog` / delete + `ConfirmDialog`
  / export), the `SnapshotChart` card + its ≥2-snapshot guard, loading/error/not-found, the page
  scaffold, and the mobile header-stack a11y treatment (solved once, like 0043's screen header).
- **Slots (core calls, never inspects):** `headerSecondary` (neutral node), `renderHeadline?`
  (investment-only), `infoFields`, `historySections`, `renderEditDialog` (opaque).
- **Wiring data:** `entityKey`, i18n namespace, load/delete hooks, `exportUrl`, `tourSteps` copy
  (the `data-testid` anchors stay core so a step can't point at a nonexistent region), `listKey` for
  terminate.

### Scope fence

- **In:** the `*Detail` family and the shell/primitives above.
- **Out — Create/Edit form bodies + dialogs.** Hand-written, reached via the opaque `renderEditDialog`
  slot. A shared `PositionFormDialog` scaffold is a **separate later phase**, exactly as [[adr-0043]]
  fenced it — the irreducible per-type part.
- **Out — the per-shape create-dialogs** (`CreateSnapshotDialog`,
  `CreateQuantityPriceSnapshotDialog`, `CreateAccruedInterestSnapshotDialog`, trade/fee/dividend/
  maturity dialogs). Referenced as `HistorySection.createDialog` **wiring**, not rewritten — they
  already exist per-shape; the shell mounts them and never touches their internals.
- **Out — `SnapshotChart` internals.** The shell mounts the existing chart; its mobile behaviour is
  the chart's own concern (already handled in #507).
- **Out — Income & Tags.** Not Positions (CONTEXT.md); they have their own renderers from #508/#509.

### Test strategy

Same safety net as [[adr-0043]]: this activates / extends the RTL+MSW+jsdom component-test tier
([[adr-0021]], #69). Consolidation is verified at **web parity** first — component tests assert each
descriptor renders the same regions the hand-written page did. Per [[adr-0050]], the QA invariants are
**renderer-independent** (`// covers:` tokens hold across web table and mobile card) with shared
`data-testid` anchors; a tiered `@smoke` spec asserts the correct renderer mounts at mobile vs desktop
width and the primary value is reachable, with deep per-shape assertions in the nightly suite.
INV-PRESENTATION-08 (≥44px tap targets, no h-scroll to read a primary value, focus order follows
visual order) lands on all ten pages at once via the shared shell.

## Rollout — one epic, two phases

Run as a **single epic on one integration branch**, with the ADR mutable on the branch (the #428 /
[[adr-0050]] model — slices amend it as they land, so a forced backtrack moves the ADR with the code).
The two phases are sequencing *within* the epic, not two epics:

- **Phase A — consolidate to shell + descriptor on web, at visual parity.** Ten pages → one
  `PositionDetailScreen` core + ten descriptors, no mobile work, no visual change. Sliced
  amount-only-first as the linchpin (like #502 S1), then the investment headline + transaction
  sections, then TimeDeposit's rollover/maturity tail.
- **Phase B — detail-mobile slices.** With all ten on the shared shell, `InfoGrid` / `HistorySection`
  / header get their mobile reflow **once** in the core primitives, flipping all ten. Sliced by
  snapshot shape (amount-only / qty×price / accrued), mirroring S1/S2/S3, plus the shared a11y floor.

The epic branches off `main` **after** the #428 landing (PR #523) merges — Phase A reuses the S1/S2/S3
snapshot renderers, `headlineSurface`, and `lib/format` extensions that #523 brings to `main`. Only
Phase A issues are created up front; Phase B slices derive from Phase A's shell landing.

## Considered alternatives

- **A detail-specific cluster preset (like 0043's two clusters).** Rejected — the detail page varies
  on snapshot-shape × has-transactions, not on 0043's risk-filter/aggregation axis. A cluster preset
  would misfit the actual cut and re-introduce a shape the data doesn't have.
- **Opaque `renderInfo` slot instead of a shared `InfoGrid`.** Rejected — maximally faithful to the
  0043 boundary but every type re-solves its own mobile reflow, defeating "one card renderer flips all
  ten." The neutral-`{label,value}[]` grid keeps the boundary (core never reads field identity) while
  centralising the reflow.
- **Snapshot table in core, transaction sections an opaque slot.** Rejected — relocates the
  ten-hand-forked-card-layout problem from detail pages onto transaction tables. One `HistorySection`
  primitive gives all history tables the mobile split at once.
- **Consolidate + diverge each cluster in a single pass.** Rejected — couples the risk of the
  web-parity refactor with the mobile visual change; the two-phase split lets Phase A land invisibly
  behind component tests before any pixel moves.
