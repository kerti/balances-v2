# Zone: STALENESS

The **cache-coherence layer** over the materialized `monthly_reports` rows.
FINANCE owns the pure engine (`monthly_reports_engine.go`, fed pre-gathered
inputs) and already catalogues the **value** carry-forward and the stale-position
**flag** (the `stalePositions` drill-down from #50) — neither is re-catalogued
here. This zone is the layer the engine tests never touch:
`MonthlyReportRepo.refresh` in `internal/repo/monthly_reports.go`, which decides
**when** the cached rows must be recomputed and keeps them coherent with their
inputs. The defining risk is **serving a stale report**: a read returning a
net-worth number computed from inputs that have since changed (a new / edited /
soft-deleted snapshot, a txn, an fx rate) because the regeneration trigger
missed the mutation. The mechanism is `needsRegen(reports, existing, watermark)`
— a coarse-but-correct whole-household check driven by `MaxReportInputUpdatedAt`
(the input watermark) and a month-set comparison; ADR-0006's conservative rule
is to over-regenerate cheaply for one household rather than ever serve stale.
Reads are lazy (`ListReports` / `GetReport` refresh-then-fetch); `RebuildAll` /
`RebuildMonth` are the manual-override escapes that regenerate ignoring the
watermark (engine-code or FX changes the data-driven watermark can't see).
ADR-0006 (carry-forward + conservative staleness rule).

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-STALENESS-01 | Watermark freshness — a read never serves a value computed from stale inputs. Any input mutation (snapshot create / update / **soft-delete**, txn, fx-rate) bumps `MaxReportInputUpdatedAt`; a materialized row whose `generated_at` predates that watermark is regenerated on the next `GetReport` / `ListReports` (refresh-then-fetch), so a read after a write reflects the write. Conversely the check is conservative-but-correct in the other direction: with inputs unchanged the watermark hasn't moved, so no regen fires and `generated_at` is stable — the cache is not thrashed | ADR-0006 | Critical |
| INV-STALENESS-02 | Month-range coherence — the materialized month set exactly equals the engine's `[first..last]`. `len(existing) != len(reports)` forces regen, a new earliest snapshot extends the range, and rows that fall outside it are pruned (`DeleteMonthlyReportsOutsideRange`) inside the write transaction rather than left dangling as orphaned cache rows a later read could surface | ADR-0006 | High |
| INV-STALENESS-03 | Whole-household atomic regen — `writeReports` prunes out-of-range rows and upserts **every** generated month in one transaction, never committing a partial fresh+stale mix. The manual overrides bypass the watermark deliberately: `RebuildAll` force-regenerates the whole household (and is a clean no-op with no input data), while `RebuildMonth` rewrites a single month via `writeReport` **without** pruning, so neighbouring cached months stay intact (carry-forward still reads every input ≤ M) | ADR-0006 | High |
