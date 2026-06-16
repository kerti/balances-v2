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
| BONDS | Bond/TD valuation — ledger-derived outstanding face + time-deposit term bounds (ADR-0003) | High | [catalog](invariants/08-bonds.md) | [coverage](coverage/08-bonds.md) |
| TAGS | User-defined position tag lifecycle + referential integrity — unique names, delete-detaches, household-validated assign, breakdown reconciliation (ADR-0028) | High/Medium | [catalog](invariants/09-tags.md) | [coverage](coverage/09-tags.md) |
| EXPORT | Per-position export workbook — owner-email/tag-name resolution privacy (joint ⇒ no owner identity, untagged ⇒ no tag) + household-scoped, subtype-checked gather (ADR-0005/0028) | Critical/High | [catalog](invariants/10-export.md) | [coverage](coverage/10-export.md) |
| FX | Multi-currency conversion beneath FINANCE — carry-forward latest-rate-at-or-before-month, reporting/multi-off passthrough, missing-rate surfaced (not zeroed), fx-rates-used audit gate (ADR-0002/0006) | Critical/High | [catalog](invariants/11-fx.md) | [coverage](coverage/11-fx.md) |
| SOFT-DELETE | Cross-cutting `deleted_at IS NULL` discipline — soft+idempotent delete, deleted rows invisible to every read path, report gather excludes tombstones, delete-then-recreate-same-month clean (ADR-0005/0006) | Critical/High | [catalog](invariants/12-soft-delete.md) | [coverage](coverage/12-soft-delete.md) |
| STALENESS | Cache-coherence over materialized `monthly_reports` — watermark-driven regen never serves a report computed from since-changed inputs, month-range pruning, whole-household atomic recompute. Distinct from FINANCE (the engine number) and its carry-forward/stale-flag (ADR-0006) | Critical/High | [catalog](invariants/13-staleness.md) | [coverage](coverage/13-staleness.md) |
| ATTRIBUTION | Per-owner routing beneath FINANCE's `userBreakdowns`: `ownerKey` sends sole→owner bucket, joint→whole `jointKey` bucket (never split), membership seeds the bucket set, malformed-sole degrades to joint. Intra-household misallocation that still reconciles — distinct from FINANCE-02 (the total) and TENANCY (cross-household leak) (ADR-0004/0012/0005) | High/Medium | [catalog](invariants/14-attribution.md) | [coverage](coverage/14-attribution.md) |
| INTEGRITY | Write-side shape & ownership CHECKs that make malformed rows unrepresentable, the upstream guarantee the read engine assumes: the ownership biconditional (`sole ⇔ owner present`) ATTRIBUTION-04 degrades against, the two-layer snapshot-shape (DB CHECK + `ErrInvalidSnapshotShape`) and txn type→shape guards. Distinct from LIFECYCLE (status machine) and SNAPSHOTS (as-of pin) (ADR-0004/baseline migration 00001) | Critical/High | [catalog](invariants/15-integrity.md) | [coverage](coverage/15-integrity.md) |
| PRESENTATION | Client presentation + input-guardrail layer for the non-technical audience (ADR-0021): money/date/ownership rendering that never misrepresents the backend truth (no `NaN`, correct locale, joint shows no member identity) + the form-side twins of backend CHECKs that stop bad input before the round-trip. The first frontend-native zone; mirrors, never re-owns, the backend truth (FINANCE / SNAPSHOTS-05 / EXPORT-02) (ADR-0021/0026) | High/Medium | [catalog](invariants/16-presentation.md) | [coverage](coverage/16-presentation.md) |
| JOURNEYS | E2E-native companion to PRESENTATION: whole-browser round-trips a handler unit test or a pure-fn vitest can't reach — OAuth sign-in loop (button→redirect→callback→session→authenticated, ADR-0024), carryover dialog seeding a submittable form (journey face of PRESENTATION-02), import preview→commit→list parity (browser face of IMPORT-01). Per-row **Tier** column records `@smoke` (per-PR gate) vs nightly-verified-by-design (#70) | High/Medium | [catalog](invariants/17-journeys.md) | [coverage](coverage/17-journeys.md) |
| NOTIFICATIONS | Transactional email delivery (`internal/email`, ADR-0020) — the message that carries an invite token, beneath AUTH's token invariants: mailed-to-the-right-address-only (misaddressed ⇒ leaked working token), best-effort/non-blocking (Send failure never loses the invite or blocks the 201), HTML-escaped interpolation of user-controlled display names (ADR-0020/0017) | High/Medium | [catalog](invariants/18-notifications.md) | [coverage](coverage/18-notifications.md) |
| CONTRACT | HTTP error-envelope contract (`internal/httperr`, ADR-0027). Every 4xx/5xx ships a typed `Envelope{Code, Args?}` with **no `message` field**, so a raw DB/internal error never reaches the wire: unknown-error→INTERNAL/500 + server-only log (info-disclosure guard), sentinel→stable-code+correct-status via `errors.Is` (false-state guard), code-only envelope (`Args` omitted when nil), VALIDATION reports the JSON field+rule not the Go name (ADR-0027/0026) | High/Medium | [catalog](invariants/19-contract.md) | [coverage](coverage/19-contract.md) |
| BACKUP | Whole-Household backup — versioned `.json.gz` export (ADR-0036). Household-scoped across ~22 tables (no cross-tenant leak), fidelity honored (full carries soft-deleted verbatim, compacted live-only, never resurrects), decimals-as-strings (no precision loss), frozen parents-before-children section order for stream-on-import. Restore + transform chain land later (#175/#177) | Critical/High | [catalog](invariants/20-backup.md) | [coverage](coverage/20-backup.md) |
