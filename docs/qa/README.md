# QA coverage matrix

The rules this app must never violate, catalogued with stable IDs and joined
against the tests that verify them. Pick a zone and open only its two files —
the hand-authored catalog and its generated coverage — without reading the whole
matrix.

- **[How it works](how-it-works.md)** — the mechanism: IDs, the `covers:`
  annotation, `make qa-matrix` / `make qa-gaps`, how zones grow, the frontend/E2E
  story. Read once.
- **[Coverage index](coverage/README.md)** — generated rollup: the headline
  N/M number, per-zone counts, and any uncovered invariant or orphan annotation.

The catalog files are **hand-authored** (source of truth for *what must hold*);
everything under `coverage/` is **generated** by `make qa-matrix` — do not edit
it. Zones are ordered heaviest/riskiest first (ADR-0021); the numeric filename
prefix is that order.

| Zone | Guards | Severity | Catalog | Coverage |
|----|----|----|----|----|
| TENANCY | Per-household row isolation — no cross-tenant finance leak (ADR-0005) | Critical | [catalog](invariants/01-tenancy.md) | [coverage](coverage/01-tenancy.md) |
| FINANCE | Net-worth & comprehensive-income calculation correctness (ADR-0006/0008/0002) | Critical/High | [catalog](invariants/02-finance.md) | [coverage](coverage/02-finance.md) |
| LIFECYCLE | Position state machine the report engine assumes on read (ADR-0009) | Critical/High | [catalog](invariants/03-lifecycle.md) | [coverage](coverage/03-lifecycle.md) |
| AUTH | Who-you-are at the door + invitation binding (ADR-0017) | Critical/High | [catalog](invariants/04-auth.md) | [coverage](coverage/04-auth.md) |
| SNAPSHOTS | Temporal/value correctness of the snapshot store beneath FINANCE (ADR-0006/0007) | Critical/High | [catalog](invariants/05-snapshots.md) | [coverage](coverage/05-snapshots.md) |
| COST-BASIS | Avg-cost ledger replay (Go + frontend parity) beneath INV-FINANCE-08 (ADR-0023) | Critical/High | [catalog](invariants/06-cost-basis.md) | [coverage](coverage/06-cost-basis.md) |
| IMPORT | Bulk ingestion — preview/commit parity, all-or-nothing, fan-out atomicity (ADR-0022) | Critical/High | [catalog](invariants/07-import.md) | [coverage](coverage/07-import.md) |
| BONDS | _Seeded — bond/TD valuation: ledger-derived outstanding face + coupon (ADR-0003)_ | — | [catalog](invariants/08-bonds.md) | — |
