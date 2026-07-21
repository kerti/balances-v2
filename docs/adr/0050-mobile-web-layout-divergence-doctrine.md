---
status: proposed
---

# Mobile–web layout divergence doctrine

The app is one responsive layout that squeezes the web design onto phones. Where that squeeze
stops working — collisions on the [[adr-0046]] bulk-entry rows are the trigger (#428, #423) — we
**diverge the mobile layout from the web layout** for that surface, rather than keep tuning one
shared layout to satisfy both. This ADR is the standalone doctrine every per-surface divergence
cites: *when* to diverge, *how* the split is built, the mobile vocabulary, and the a11y/test bar.
It pins a pattern that already emerged un-recorded in the code ([[adr-0043]]'s
`PositionListScreen` → `PositionListCards`/`PositionListTable`, `use-mobile.ts`) and extends the
shell-level divergence [[adr-0025]] already ships (persistent sidebar on desktop, hamburger drawer
on phones).

Per [[adr-0034]], a cross-cutting UI-native philosophy with no single backend counterpart gets its
own ADR (like the non-technical-audience guardrail philosophy it names) — so this is standalone, and
each affected surface records *its* mobile presentation in a `## Presentation / UX` section of the
surface's own owning ADR, citing this doctrine.

## Why now

[[adr-0025]] already pinned the load-bearing constraint: the audience is **non-technical household
members on phones**, so a mobile usability failure is a correctness defect, not polish. The
responsive-squeeze approach has now visibly broken on the [[adr-0046]] entry rows — the qty×price
slice (#423) carries two tab-stops plus a computed value and cannot lay out next to the position
name; elements overlap and the row becomes unusable. A static audit (#428) found the squeeze also
degrades the history/data tables (they horizontally scroll to read a primary value) and the
dashboard/hub grids. Rather than fix each surface ad-hoc and let the *mechanism* drift (768 here,
640 there; cards here, accordions there), the doctrine is pinned once so per-surface slices apply it
mechanically.

## The decision

### Diverge on breakage or core-ritual degradation, not everywhere

The default stays **one responsive layout**. A surface earns a divergent mobile layout when the
squeeze crosses a concrete, testable bar — it:

1. forces **horizontal scroll to read a primary value**, or
2. causes **interactive-target collision / overlap**, or
3. drops a **tap target below 44px**.

Breakage *or* core-ritual degradation for the phone audience (a table that technically scrolls but
hides the number the user came for) qualifies. A surface that merely reflows or looks a little
cramped does **not** — it stays single-layout. This bar is the audit instrument and, restated, the
mobile renderer's a11y checklist (below). Rejected: *breakage-only* (too narrow — leaves the
h-scroll tables as permanent poor UX) and *mobile-first everywhere* (a second design per surface
forever, the exact cost [[adr-0043]] worried about).

### Runtime pick-one renderer, single 768px boundary

Divergence is **structural** (different DOM and interaction — table vs cards, cramped row vs stacked
fields), not cosmetic. So `useIsMobile()` (`use-mobile.ts`: `useSyncExternalStore` over a single
`768px` `matchMedia` boolean) picks **which** renderer mounts; only one tree is ever in the DOM.
Rejected: **render-both-and-CSS-hide** (`hidden md:block`) — it doubles the DOM, duplicates every
`data-testid` (breaking `getByTestId` and the `covers:` E2E annotations), duplicates ARIA, and for
entry would double the form state. One tree → one set of testids → one thing to assert. A **single
768px boolean**, no tablet middle-tier (YAGNI for this audience). Plain CSS media queries remain the
tool for surfaces that *don't* diverge — reflowing a grid, wrapping a toolbar — where the change is
cosmetic and no second component is warranted.

### The split lives at the renderer; the container is shared

Mobile and web differ **only in the leaf view**. The container owns data-fetching, interaction
state, validation, and the mutation as a **single source of truth**, and feeds *both* renderers one
**presentation-neutral projection** plus one callback surface — exactly [[adr-0043]]'s "two
renderers share one data spec." For [[adr-0046]] entry this is load-bearing: `EntryScreen` keeps all
dirty-tracking, per-row field state, batch validation, and the atomic Save, and delegates only the
row to `EntryRowMobile` vs `EntryRowDesktop`, both fed the same row projection and the same
`onFieldChange`.

Two shapes are explicitly rejected:

- **Fully separate top-to-bottom components** (`MobileEntryScreen` + `WebEntryScreen`, each owning
  its own data/state/save). This is [[adr-0043]]'s rejected "second per-type component" — full
  duplication, guaranteed drift, forked logic. For entry it would fork the Save into two copies that
  must stay bit-identical.
- **Inline `if (isMobile)` branches in the shared core** — the god-branch creep [[adr-0043]] fences
  against.

**Escape hatch.** If a surface needs a genuinely *different interaction model* on mobile — a
different *flow*, not a different layout of the same interaction (#428 floats "a mobile-first entry
flow") — that is a **separate container/route**, decided in that surface's own slice and flagged as
the deliberate exception with its own justification. The doctrine default is shared-container /
split-renderer.

### Canonical mobile vocabulary (provisional)

To keep divergent surfaces visually coherent and stop each slice reinventing a layout, three named
transforms are the mobile vocabulary; a surface needing something outside them flags it as a
one-off:

- **Wide table → stacked cards.** One card per row; label→value pairs stacked; the primary value
  promoted to the card headline. (details ×10, Income, Tags, Fx/Inflation.)
- **Cramped horizontal input row → stacked fields.** Each tab-stop on its own full-width line (its
  label shown when the shape names one); the computed/derived value and the row actions in a footer.
  (the three entry shapes.) **Proven** on the amount-only entry slice ([[adr-0046]] `## Presentation /
  UX`, #502): laid out cleanly as written, so the transform is no longer provisional; the qty×price
  and accrued shapes reuse the same `EntryRowMobile`/`EntryRowDesktop` split.
- **Multi-column dashboard/hub grid → single-column stack**, charts full-width, secondary cards
  below the fold. (Dashboard + three hubs.)

These are **provisional** — proven and refined on the first slice (entry amount-only); this ADR
stays editable on the epic branch for exactly that reason.

### A11y and test bar

- **Behavioural invariants are renderer-independent.** A `covers: INV-…` truth ("only dirty rows
  save", "terminated positions are visually flagged") is a property of the shared container and must
  hold in whichever renderer is active. The invariant catalog **does not fork** per form-factor.
- **Shared `data-testid` contract.** Both renderers expose the *same* testid for the same semantic
  element, so one spec asserts behaviour regardless of card-vs-table.
- **Divergence is smoke-tested at both widths, tiered.** One `@smoke` Playwright spec per divergent
  surface asserts the correct renderer mounts at mobile vs desktop width and the primary value is
  reachable; deep per-shape assertions stay in the nightly full suite (matching the existing
  Playwright tiering). vitest renderer tests set the `matchMedia`/width mock (`src/test/setup.ts`
  already stubs `matchMedia`) to exercise a specific renderer.
- **Mobile renderer a11y floor** = the Q1 bar restated: tap targets ≥44px, no horizontal scroll to
  read a primary value, focus order follows visual order.

New QA invariants for the divergence itself (correct renderer mounts; the a11y floor) are catalogued
**lazily as slices land**, not seeded up front.

### Scope and rollout

- No `CONTEXT.md` change — "renderer", "breakpoint", "card" are presentation vocabulary, not domain
  language ([[adr-0034]] / CONTEXT-FORMAT).
- No migration, no backend change — this is frontend presentation only.
- The list screens ([[adr-0043]]) already satisfy the doctrine and need no work. The epic (#428)
  runs per-surface slices, each amending the owning ADR's `## Presentation / UX` section. The
  **detail pages** are not descriptor-consolidated (ten hand-written pages, no shared shell); their
  mobile divergence is gated on a prerequisite consolidation (its own [[adr-0043]]-style ADR + epic)
  rather than ten hand-forked card layouts.

## Considered alternatives

- **Keep tuning one responsive layout.** Rejected — the trigger for this ADR is that it has already
  failed on the entry rows for the exact audience [[adr-0025]] centres.
- **Amend [[adr-0034]] instead of a standalone ADR.** Rejected — [[adr-0034]] itself routes
  cross-cutting UI-native philosophy to a standalone ADR; the mechanism is cited by every slice and
  wants one home.
- **Render-both-and-CSS-hide; fully-separate components; inline `isMobile` branches.** Rejected
  above.
