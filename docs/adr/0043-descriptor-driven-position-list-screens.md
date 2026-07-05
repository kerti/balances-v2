# Descriptor-driven Position list screens

The ten per-type list screens (`BankAccountsScreen`, `GoldsScreen`, …) and their eleven matching
`*ListRow` components collapse into **one generic `PositionListScreen` core plus two cluster
presets**, each concrete type reduced to a small **descriptor** object. The core owns every
shared-surface concern — the table shell, sort wiring, empty/loading/error states, the show-inactive
toggle, and the row scaffold (click-to-select, terminated styling, the ⋮ → edit/delete menu with its
`ConfirmDialog`). A descriptor supplies only wiring (entity key, i18n namespace, the list/delete/import
hooks, default sort, which headline + aggregation, whether the risk filter shows) and **slots** for the
group-specific bits (extra columns, the create/edit dialogs). This activates [[adr-0021]]'s deferred
component-test tier (issue #69) as its safety net.

## Why now

The per-type screens are ~85–90% identical boilerplate — ~2.4k LOC across the screens, ~1.6k across
the rows — with a thin varying core. Two costs, not aesthetics, drive the change:

- **Change amplification.** A cross-cutting change (an empty-state redesign, a row a11y fix, a new
  sort behaviour) means editing ten screens plus eleven rows by hand, and they *have already drifted*
  (`{"—"}` vs `—`, divergent filter sets between siblings). The duplication is cheap to read but
  expensive and unsafe to change.
- **Test surface.** 11k LOC of per-type screens is not affordably testable; one generic core plus
  thin descriptors is. This is the concrete trigger for the RTL+MSW+jsdom harness [[adr-0021]]
  deferred until "we start writing component tests" (#69).

A planned divergence of the **mobile** UI from the web (dense tables on web; compact, essentials-only
cards on phones) makes this urgent: hand-written per-type rows would mean writing a *second*
per-type component for the card layout — twenty-two files instead of eleven. A descriptor lets two
renderers share one data spec.

## The decision

### The generic operates on the Position shared surface only

CONTEXT.md already treats **Position** as a first-class supertype across all four groups
(status/lifecycle span every group). The core touches only that shared surface — display name +
secondary line, status, latest-snapshot value, lifecycle, row actions — which the domain says *is*
uniform. **Every group-specific field is injected via a slot the core calls but never inspects; none
is ever absorbed into the core.** This is the load-bearing constraint of this ADR.

This is *not* in tension with [[adr-0022]]'s deliberate non-polymorphic, one-table-per-group storage.
That decision governs **storage** (typed per-group columns, CHECK constraints, no sparse polymorphic
table). Presentation genericity over the Position supertype's shared surface is a different layer and
does not reintroduce storage polymorphism — provided the shared-surface line holds. The moment the
core would need `bond_details.coupon_disposition` or a property address, that is a slot, not core
logic.

### Mechanism: core owns the markup; extras are injected render-props (option i)

To actually remove change amplification the *markup* must be centralized, so the core renders the
rows itself — the `*ListRow` files disappear. The four genuinely-shared columns (name+secondary,
status, latest value, actions) are **hard JSX in the core, never config** — that is the guardrail that
stops the descriptor becoming a god-config. The descriptor carries:

- **wiring data**: `entityKey`, i18n namespace, list/delete/import hooks, default sort, headline slot
  + aggregation fn, whether the risk filter shows;
- **extra columns**: `{ id, label, sortKey?, align?, render, mobile? }` descriptors whose `render`
  returns **presentation-neutral value content** (not a `<TableCell>`);
- **slots**: `renderCreateDialog`, `renderEditDialog`.

### Two cluster presets over one core, not two components

"Investment" and "Asset/Liability/Receivable" differ (risk filter + activity cell +
`InvestmentListHeadline`/`aggregateListPositions` vs ownership column +
`ListHeadline`/`activeCurrencyTotals`) but still share ~80%. The cluster is a **configuration
boundary, not a component boundary**: a single core, two presets. Two independent components would
merely relocate the amplification problem (the shared 80% duplicated between the two copies, free to
re-drift).

### Mobile: presentation-neutral descriptor, two renderers

The descriptor is presentation-neutral; two renderers consume it — `PositionListTable` (web, dense,
all columns) and `PositionListCards` (mobile, compact). Because the mobile card's essentials *are* the
Position shared surface (uniform across all types), the card renderer needs almost no per-type config:
its default content is the shared surface, group extras are **opt-in via a single `mobile:
"secondary"` hint** and otherwise hidden on mobile. The shared-surface boundary and mobile-minimalism
reinforce each other. Runtime switch via the existing `useIsMobile` (768px).

### Scope fence

- **In:** the `*Screen` and `*ListRow` families.
- **Out:** config-driven *form fields*. The Create/Edit form body (which fields, which validation,
  the type-specific selects) is the genuinely irreducible part and stays hand-written. A shared
  `<PositionFormDialog>` scaffold (owning submit → mutation → toast → error-envelope, dirty-guard) is
  a **separate later phase**, not this effort.
- **Out:** Income. Income is a flow event, not a Position (CONTEXT.md; not taggable), so
  `IncomeScreen`/`IncomeRow` are outside the Position generic by definition.

### Test strategy

Three tiers:

1. **Core, tested hard against a *synthetic* descriptor** (a fake position type, not a real domain
   type) — asserts the *abstraction contract*: sort toggle, show-inactive filter,
   empty/loading/error, the ⋮ → edit/delete flow, shared-surface-always-present, and
   extras-hidden-on-mobile. Per-PR; one suite covers list behaviour for all ten real types. Synthetic
   over a real type so the test asserts the abstraction, not one type's incidental behaviour.
2. **Per-type conformance** — one table-driven test over every real descriptor: it renders without
   crashing and surfaces its declared columns + wires its hooks. Cheap; no per-type re-testing of
   core behaviour.
3. **Playwright unchanged** — stays the behavioural net (not a coverage instrument, per #69), @smoke
   on one representative type per cluster. Not deleted; component tests add per-PR coverage beneath
   it.

Any list-behaviour invariant previously covered only by nightly Playwright gains a per-PR component
test as a side effect — a net `make qa-strict` improvement. New `// covers: INV-…` annotations land on
the synthetic-descriptor core tests.

### Rollout

Parallel-run, per-route cutover, never big-bang. A **fat tracer** slice (BankAccount — it already has
the import-roundtrip Playwright spec, INV-JOURNEYS-03) stands up the harness + core + both renderers +
both test tiers and migrates the first type, forcing every hard question through the whole stack once.
Remaining non-investment types follow as thin descriptor PRs; the investment preset lands with its
first type (Gold); remaining investment types follow. Each PR deletes the type's old Screen+Row in the
same commit, ships green, and is independently revertable. Closes #69 at the tracer.

## Considered alternatives

- **Leave the duplication.** Self-contained, greppable, AI-navigable — genuinely fine to *read*. Loses
  on change amplification and test surface, and blocks the mobile divergence (twenty-two hand-written
  components). Rejected on the two named costs; the tie-breaker survives as a rule — if a specific
  type's differences make the generic uglier than its duplicate, that type stays hand-written.
- **A shared `usePositionList` hook, per-type JSX.** Dedupes the logic but leaves the markup
  duplicated — and the markup is exactly what amplifies. Under-delivers on the primary driver.
- **Two fully independent cluster generics.** Relocates rather than removes the shared-80% duplication.
- **Config-driven form fields.** Reliably degrades into a `renderField(config)` mini-DSL less readable
  than the duplication it replaces; explicitly out of scope.
- **Test the core against a real type (Gold).** Less scaffolding, but bakes one type's quirks into the
  "generic" contract. Rejected for the synthetic descriptor.

## Consequences

- A per-type screen is **no longer a self-contained file** read top-to-bottom; it is a ~40-line
  descriptor plus the shared core. For this codebase's backend-leaning reader this is arguably more
  navigable (less noise), but it is a real change in how the code reads, and the shared-surface line
  must be defended in review — a group-specific branch creeping into the core is the failure mode.
- The RTL+MSW+jsdom harness lands for the first time (vitest gains a jsdom project); [[adr-0021]]'s
  deferred component tier is now active and #69 closes.
- New QA-matrix rows capture the generic's contract (shared surface always rendered; extras hidden on
  mobile / columns on web; sort/show-inactive/empty via the core), covered per-PR by the synthetic
  core tests.
- The `<PositionFormDialog>` scaffold and the full mobile card polish are acknowledged follow-on work,
  not part of this ADR's commitment.
- CONTEXT.md is unchanged: "Position" already carries the supertype; this is presentation architecture,
  not domain language.
