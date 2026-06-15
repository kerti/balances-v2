# Zone: SOFT-DELETE

> _Seeded next — the cross-cutting `deleted_at IS NULL` discipline. Nothing is
> ever hard-deleted on the write path: a delete stamps `deleted_at = now()`
> (and a deleting user) and the row stays in the table. Correctness then depends
> on **every read path filtering it back out** — the `WHERE … AND deleted_at IS
> NULL` clause appears in ~15 query files (`backend/queries/*.sql`: assets,
> liabilities, receivables, investments, the four `*_snapshots`,
> `investment_transactions`, income, tags, fx_rates, households, users,
> monthly_reports). The defining risk is a **leaked tombstone**: a single read
> query missing the filter resurfaces a deleted position/snapshot/transaction —
> in a list, an export, or (worst) the monthly-report gather, where it silently
> re-enters net worth or comprehensive income long after the user "removed" it.
> This is its own zone rather than a corner of TENANCY because the failure is
> orthogonal to household scoping: a correctly household-scoped query can still
> leak a tombstone, and the report engine's gather (`monthly_reports.sql`) is the
> highest-stakes consumer. Candidate invariants: (1) delete is soft + idempotent —
> the row persists with `deleted_at` set and a second delete (or any mutation)
> against the same id is `ErrNotFound`/404, never a hard row removal or
> double-stamp; (2) deleted rows are invisible to every read path — Get/List/Export
> and the snapshot/transaction history queries all exclude them (the handler
> delete-then-GET ⇒ 404 tests already assert the Get half; List/Export/history
> are the gaps); (3) the report engine excludes deleted positions and snapshots
> from the gather, so a deleted holding contributes zero to net worth / income
> the month after deletion (annotation target likely in the repo-level monthly
> report integration test, not the pure engine unit test — the engine takes
> already-gathered rows); (4) delete-then-recreate-same-month is clean — a tombstone
> does not collide with a fresh row in the same `year_month` (see
> `internal/repo/snapshot_as_of_month_test.go:TestAssetSnapshot_DeleteThenRecreateSameMonth`,
> already written, just unannotated). Annotation targets: the per-handler
> `Test*Handlers_Delete` / `*DeleteSnapshot` suites across `internal/{assets,
> liabilities,receivables,investments,income}` (the 204-then-404 contract), plus
> the as-of-month recreate test. Survey those before writing new tests; the List/
> Export/report-gather exclusion is where genuine new coverage is likely needed.
> ADR-0005 (tenancy boundary this rides alongside), ADR-0006 (the report gather it
> protects)._
