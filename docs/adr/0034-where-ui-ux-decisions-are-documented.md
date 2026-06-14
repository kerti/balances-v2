# Where UI/UX decisions are documented

We have frontend ADRs for the *stack* and *plumbing* — React/Vite ([[adr-0015]]), routing + the
sidebar shell ([[adr-0025]]), i18n ([[adr-0026]]) — and one narrow UX widget decision, toast feedback
for buttonless autosaves ([[adr-0032]]). What we have never had is a **convention** for two things:
where the *presentation* of a behavior that already has a backend ADR gets recorded, and how UI
behaviour enters the QA coverage matrix as catalogued invariants. This ADR sets that convention.

## Why now

The coverage matrix ([[adr-0021]], `docs/qa/`) is growing a frontend/E2E story — the
`covers:` token is language-agnostic and we will seed UI/E2E annotations per-invariant. Backend
invariants derive from behavior ADRs (TENANCY from [[adr-0005]], FINANCE from [[adr-0008]], and so
on); UI invariants need an equivalent documented basis to derive from, or they will be ad-hoc. And
because the audience is non-technical, presentation is a
correctness concern, not polish — a missing guardrail or a silent failure is a defect, exactly the
kind of thing the matrix exists to guard. So the place UI decisions live, and the place UI invariants
come from, want to be settled before the frontend zones land rather than reverse-engineered after.

## The decision

### Presentation sections on the behavior ADR

Where a UI surface is the **face of a decision that already has an ADR**, document how that behaviour
is presented in a `## Presentation / UX` section *of that same ADR* — not a new one. Examples: the
multi-currency toggle and the foreign-positions block belong in [[adr-0002]]; how a position's
status/lifecycle is surfaced and guarded belongs in [[adr-0009]]; the Tag dropdown's placement and
affordances belong in [[adr-0028]]. One decision, one record. The UI description then can't drift
from the backend rule it depends on, because they sit in the same file and move together.

### Standalone ADRs for UI-native concerns

Decisions with **no backend counterpart** get their own ADR: information architecture and navigation,
empty / error / loading states, and the guardrail philosophy for the non-technical audience. This is
not new — [[adr-0032]] is the precedent (a UX decision that stands alone because no backend rule
underlies it), and [[adr-0025]] / [[adr-0026]] are UI-native in the same sense. This ADR just names
the category so the next one knows it has a home.

### How UI behaviour enters the matrix

A Presentation section or a UI-native ADR is the source of UI invariants exactly as a behavior ADR is
the source of backend ones: catalogue them in the appropriate zone file under `docs/qa/invariants/`, and
verify them with Playwright or vitest per the "Frontend & E2E coverage" rules in `docs/qa/how-it-works.md` —
minding the two wrinkles documented there (Playwright is tiered: `@smoke` gates per-PR, the full
suite runs nightly; vitest runs per-PR but isn't scanned by `qa-matrix` until its file filter is
widened to `.test.ts`).

## Considered alternatives

- **A parallel UI ADR series mirroring each backend ADR.** Rejected — doubles the record count and
  guarantees drift: two files describing one feature, updated at different times.
- **Presentation in `CONTEXT.md` prose only.** Rejected — `CONTEXT.md` is the living domain model, not
  a decision log. A decision with trade-offs and a rationale belongs in a dated, linkable ADR; the
  domain doc states *what is*, not *why we chose it*.
- **Do nothing; keep writing UI ADRs ad-hoc.** Rejected — it leaves UI invariants without a documented
  basis right as the frontend matrix starts to grow, which is the failure this ADR exists to prevent.

## Consequences

- Behavior ADRs may grow a `## Presentation / UX` section retroactively, done **per-zone as the
  frontend matrix advances** — not a bulk backfill. An ADR with no UI consequence (e.g. a pure data-type
  or migration decision) never needs one.
- [[adr-0032]]'s shape is blessed as the template for future UI-native ADRs.
- No tooling change — this is a documentation convention. The matrix mechanism already supports
  frontend annotations; this ADR governs where the invariants behind them are decided.
