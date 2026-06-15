# Zone: STALENESS

> _Seeded next — the **cache-coherence layer** over the materialized
> `monthly_reports` rows. FINANCE owns the pure engine
> (`monthly_reports_engine.go`, fed pre-gathered inputs) and already catalogues
> the **value** carry-forward + the stale-position **flag** (`TestEngine_CarryForward`,
> the `stalePositions` drill-down from #50) — do NOT re-catalog those here. This
> zone is the layer FINANCE's engine tests never touch: `MonthlyReportRepo.refresh`
> in `internal/repo/monthly_reports.go`, which decides **when** the cached rows
> must be recomputed and keeps them coherent with their inputs. The defining risk
> is **serving a stale report**: a read returning a net-worth number computed from
> inputs that have since changed (a new/edited/soft-deleted snapshot, a txn, an fx
> rate) because the regeneration trigger missed the mutation. The mechanism is
> `needsRegen(reports, existing, watermark)` — a coarse-but-correct whole-household
> check (ADR-0006 conservative rule: over-regenerate cheaply, never serve stale)
> driven by `MaxReportInputUpdatedAt` (the input watermark) and a month-set/range
> comparison. Candidate invariants (all distinct from FINANCE's number): (1) **watermark
> freshness** — any input mutation bumps `MaxReportInputUpdatedAt`, so a materialized
> row whose `generated_at` predates the watermark forces regen on the next
> `GetReport`/`ListReports` (refresh-then-fetch); a read after a write never serves a
> value computed from stale inputs. Mutation kinds to exercise: snapshot
> create/update/**soft-delete** (the soft-delete half is already shown indirectly by
> `soft_delete_test.go:TestMonthlyReport_GatherExcludesDeletedSnapshot` — the report
> drops after a delete only because the watermark moved), txn, fx-rate; (2) **month-range
> coherence** — the materialized month set exactly equals the engine's `[first..last]`:
> `len(existing) != len(reports)` ⇒ regen, a new earliest snapshot extends the range,
> and rows that fall out are pruned (`DeleteMonthlyReportsOutsideRange`) rather than
> left dangling; (3) **whole-household atomic regen** — `writeReports` recomputes and
> upserts every month in one transaction, never leaving a partial mix of fresh + stale
> rows; `RebuildMonth` is the manual-override escape that forces regen ignoring the
> watermark (ADR-0006 manual rebuild). Annotation targets: the repo-level integration
> tests — `monthly_reports_read_test.go` (`TestMonthlyReportRepo_ReadPaths` already
> exercises the refresh-then-fetch path) and whatever proves full generation/staleness
> wiring; survey those before writing. Genuine new coverage is likely a per-mutation
> "write then re-read reflects it" test and a range-prune test. **Dedup discipline is
> the whole game here** (zone-fill routine #2): every row must pin the cache layer, not
> restate a FINANCE engine property. ADR-0006 (carry-forward + conservative staleness
> rule)._
