# Zone: SNAPSHOTS

> _Seeded next — the valuation substrate beneath FINANCE. Every net-worth number
> the report engine derives traces back to a position **snapshot** (the dated
> value record, ADR-0006); FINANCE and LIFECYCLE both already *assume* its rules
> on read (INV-FINANCE-03's "latest snapshot ≤ M", INV-LIFECYCLE-03/04's close
> snapshot) — this zone guards them at the write/storage layer where they're
> actually enforced. Candidate invariants: a snapshot soft-deletes rather than
> hard-deletes (audit trail, ADR-0007); the engine's "latest value at-or-before
> M" selection ignores soft-deleted rows; a correction (re-snapshot at the same
> date) supersedes the prior value rather than double-counting; a snapshot's date
> can't precede the position's creation. Cross-household isolation is already
> INV-TENANCY-01/-06 — this zone is the **temporal/value** correctness, not the
> tenancy cut. Code lives in the snapshot write paths of `internal/repo/` (e.g.
> `assets.go`, `investments.go` value-update handlers) + the selection in
> `monthly_reports_engine.go`. Survey existing `*_test.go` in `internal/repo/`
> and `internal/assets|investments/` for annotation targets before writing new
> ones. Fill this table when seeding the zone._
