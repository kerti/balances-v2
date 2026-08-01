# Zone: SOFT-DELETE

The cross-cutting `deleted_at IS NULL` discipline. Nothing is ever hard-deleted
on the write path: a delete stamps `deleted_at = now()` and the deleting user,
and the row stays in the table. Correctness then depends on **every read path
filtering it back out** — the `WHERE … AND deleted_at IS NULL` clause repeated
across the `backend/queries/*.sql` files (assets, liabilities, receivables,
investments, the four `*_snapshots`, `investment_transactions`, income, tags,
fx_rates, households, users, monthly_reports). The defining risk is a **leaked
tombstone**: a single read query missing the filter resurfaces a deleted
position/snapshot/transaction — in a list, an export, or (worst) the
monthly-report gather, where it silently re-enters net worth or comprehensive
income long after the user "removed" it. This is its own zone rather than a
corner of TENANCY because the failure is orthogonal to household scoping: a
correctly household-scoped query can still leak a tombstone, and the report
engine's gather (`monthly_reports.sql`) is the highest-stakes consumer. The
write path lives in the `SoftDelete*` queries (each repo wrapper —
`DeleteAssetSnapshot`, `softDeleteAsset`, etc. — maps `RowsAffected == 0` to
`ErrNotFound`); the partial unique indexes (`… WHERE deleted_at IS NULL`) let a
tombstone and a fresh row coexist in the same `(parent_id, year_month)`.
Deleting a **position** additionally cascades to its children in the same
transaction (`CascadeSoftDelete*`, #575) — before that, a tombstoned parent kept
live snapshots and transactions underneath it, invisible only because every
named read path re-joins the parent and re-checks its tombstone. That made the
governing rule correctness-by-repetition; the cascade makes it
correctness-by-construction, and the read-path filters stay as defence in depth.
ADR-0005 (the tenancy boundary this rides alongside), ADR-0006 (the report
gather it protects).

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-SOFT-DELETE-01 | Delete is soft + idempotent: a delete stamps `deleted_at`/`updated_by` and the row **persists** in the table — never a hard `DELETE`. Because the `SoftDelete*` query itself carries `AND deleted_at IS NULL`, a second delete (or any mutation) against the same id matches zero rows and returns `ErrNotFound` — the tombstone is never double-stamped and a missing/already-deleted id is never a silent success | ADR-0005 | Critical |
| INV-SOFT-DELETE-02 | Deleted rows are invisible to every read path: once `deleted_at` is set, Get returns `ErrNotFound` and List / snapshot-history exclude the row, so a deleted position or snapshot can never resurface in a read response. The `deleted_at IS NULL` filter is not optional per-query polish — a single read missing it leaks a tombstone the user believes is gone | ADR-0005 | Critical |
| INV-SOFT-DELETE-03 | The report engine's gather excludes tombstones: a snapshot (or position) soft-deleted contributes **zero** to the next net-worth / comprehensive-income computation for its month — the gather feeding `monthly_reports` re-reads through the `deleted_at IS NULL` filter and the refreshed report drops the deleted holding's value. A leaked tombstone here silently overstates net worth long after deletion, the worst-case failure for this zone | ADR-0006 | Critical |
| INV-SOFT-DELETE-04 | Delete-then-recreate-same-month is clean: the partial unique index (`… WHERE deleted_at IS NULL`) means a soft-deleted snapshot no longer collides, so re-recording the **same** `(asset_id, year_month)` succeeds and produces a genuinely new row rather than failing the duplicate check or resurrecting the tombstone. This is the #57 "delete and re-record a misplaced snapshot" UX | ADR-0005, ADR-0006 | High |
| INV-SOFT-DELETE-05 | Deleting a Position cascades the tombstone to every child it owns (snapshots for all four groups; transactions additionally for Investment) in the same transaction, so no child row outlives its parent as a live row. Child visibility is a **write-path** guarantee, not a filter each read query must remember to re-derive — the ID-array child queries that carry no parent join are safe even when fed a stale id. A child already deleted keeps its original `deleted_at` (never re-stamped, per -01), and a failure anywhere in the cascade rolls the parent tombstone back with it — no half-deleted position | ADR-0005 | Critical |
