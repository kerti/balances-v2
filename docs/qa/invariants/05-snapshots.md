# Zone: SNAPSHOTS

The valuation substrate beneath FINANCE. Every net-worth number the report engine
derives traces back to a position **snapshot** — the dated value record of
ADR-0006. FINANCE and LIFECYCLE both *assume* a snapshot's rules on read
(INV-FINANCE-03's "latest snapshot ≤ M", INV-LIFECYCLE-03/04's 0-value close);
this zone guards them at the write/storage layer where they are actually enforced.
The cut here is **temporal/value** correctness — a snapshot's date pinned to its
month, a correction superseding (not double-counting) the prior value, a deleted
reading staying out of every read. Cross-household isolation is INV-TENANCY-01/-06,
not repeated here. Code lives in the snapshot write paths of `internal/repo/`
(`snapshot_as_of_month`, `import_snapshots.go`) and the handler validation in
`internal/assets/snapshots.go` / `internal/investments/snapshots.go`; the
constraints themselves are migration 00003 plus the per-table partial unique index.

A snapshot's date is allowed to *precede* its position's creation — historical
backfill (a 2026 account importing 2015 readings) is intentional, so there is no
"date ≥ creation" invariant here.

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-SNAPSHOTS-01 | A snapshot's `as_of_date` is pinned to its `year_month` month — the `<table>_as_of_in_month` CHECK (migration 00003) fires on insert and update across all four snapshot tables; an out-of-month date is `ErrSnapshotDateOutsideMonth` (400), a NULL or in-month date passes | ADR-0006 | High |
| INV-SNAPSHOTS-02 | At most one live snapshot per (position, year_month) — the unique index is partial (`WHERE deleted_at IS NULL`), so a soft-deleted row is superseded by a fresh re-record of the same month as a genuinely new row, not a resurrection (#57) | ADR-0006, ADR-0007 | High |
| INV-SNAPSHOTS-03 | A soft-deleted snapshot is excluded from every value read — the live list, the latest-per-position selection, and the report-feeding query all carry `deleted_at IS NULL`, so a corrected or removed reading never re-enters net worth | ADR-0007 | Critical |
| INV-SNAPSHOTS-04 | Bulk import is an upsert keyed by (position, year_month): last-write-wins overwrites an existing month in place rather than double-counting, dry-run writes nothing, and the whole batch is one transaction | ADR-0006 | High |
| INV-SNAPSHOTS-05 | A snapshot is a past observation — a future `year_month` (`CodeFutureYearMonth`) or future `as_of_date` (`CodeSnapshotFutureDate`) is rejected 400 on both the create and update paths | ADR-0006 | Medium |
