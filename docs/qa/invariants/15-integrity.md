# Zone: INTEGRITY

> _Seeded next — the **write-side shape & ownership constraints** that make a
> malformed row unrepresentable in the first place, the upstream guarantee every
> read-side zone (FINANCE, ATTRIBUTION, LIFECYCLE) silently assumes. The pattern
> is **two-layer**, called out repeatedly in HANDOFF: a DB `CHECK` enforces the
> hard shape, and a repo-layer validator rejects the same bad input early with a
> typed error (`ErrInvalidSnapshotShape`, the transaction validator) so the
> handler returns 422 rather than a constraint-violation 500. This zone
> catalogues the *guards*, not the calculations they protect. **Dedup discipline,
> read before writing rows:** LIFECYCLE already owns the status↔`terminated_at`
> biconditional (`<table>_lifecycle_chk`) — do **not** re-catalogue it; SNAPSHOTS
> owns the `as_of_in_month` pin (migration 00003) and BONDS owns TD term bounds /
> maturity-after-placement (migration 00004) — leave those there; ATTRIBUTION-04
> catalogues the *read-side degrade* when a malformed sole row somehow exists,
> whereas this zone catalogues the *write-side CHECK that stops it existing*
> (they are two ends of the same risk — cross-link, don't duplicate). Genuine new
> territory here:
>
> (1) **Ownership biconditional** — every owned-entity table (`assets`,
> `investments`, `liabilities`, `receivables`, `income`) carries
> `CHECK ((ownership_type='sole') = (sole_owner_user_id IS NOT NULL))`: a `sole`
> row **must** name an owner and a `joint` row **must not**. This is the exact
> constraint that makes ATTRIBUTION-04's "malformed sole" an *impossible* row in
> normal operation — pin that the DB rejects both halves (sole-without-owner and
> joint-with-owner). Plus the enum `CHECK`s that bound `ownership_type` to
> `{sole,joint}`.
>
> (2) **Snapshot shape is exactly-one** — `investment_snapshot_shape`:
> `(quantity AND price_per_unit AND NOT accrued_interest)` XOR
> `(NOT quantity AND NOT price_per_unit AND accrued_interest)`. A unit-priced
> holding and an interest-accruing holding are disjoint shapes; a row carrying
> both or neither is rejected. The repo mirror is `ErrInvalidSnapshotShape`
> (annotation targets: **`TestInvestmentRepo_SnapshotShapeValidation`** in
> `investments_tenancy_test.go`, and
> **`TestCreateStockWithSnapshotsAndLedger_RejectsMismatchedSnapshotShape`** in
> `investment_import_create_test.go` — both already exercise the reject path,
> unannotated).
>
> (3) **Transaction type→shape** — `investment_transaction_shape`: each
> `transaction_type` (`buy/sell/coupon/dividend/.../maturity`) admits only its
> own value-column combination, enforced both by the DB CHECK and the repo
> validator (annotation target: **`TestValidateSeedTransaction`** in
> `investment_import_create_test.go`). Pin that a wrong type→column combo is
> rejected at write, so the engine's per-type cash-flow branches never see an
> impossible row.
>
> Survey `investments_tenancy_test.go`, `investment_import_create_test.go`, and
> `investment_transactions_tenancy_test.go` before writing — much of the reject
> coverage already exists and needs only annotation; the likely *new* test is the
> ownership-biconditional reject (both directions) if no test asserts it today.
> Source: baseline migration `00001_baseline.sql` (the CHECKs), the repo
> validators, ADR-0004 (ownership model). CONTEXT.md is the de-facto spec for
> which shapes are legal._
