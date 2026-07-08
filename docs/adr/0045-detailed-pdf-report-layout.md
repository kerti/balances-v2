# Detailed PDF report layout: itemized positions and composition charts

Amends [[adr-0044]]. The exported monthly PDF grows from a one-page mirror of the on-screen
dashboard into a denser, deliberately different report: an itemized per-position breakdown,
composition charts (assets/investments/liabilities by subtype), and a 12-month
expense-vs-passive-income trend, alongside the existing net-worth trend and income-statement
lines. Rendering stays client-side via `@react-pdf/renderer` (0044's library/lazy-load decision is
unchanged; the separate bundle-size question is tracked independently in #394) — but this ADR adds
one new backend endpoint, reversing 0044's "no new dependency and no new endpoint" claim.

## Why

0044 shipped v1 scoped explicitly to mirror the dashboard, with "no per-position ledger" as a
stated non-goal ("the dashboard itself never enumerates individual positions, so nothing in the
PDF needs to either"). That was correct for a first cut, but a downloadable report has a different
job than the live dashboard: it's meant to be a point-in-time financial statement someone can
print, archive, or hand to another household member without opening the app — closer to the
household's old spreadsheet workbook than to a live UI. That calls for more detail, not a mirror.

A reference template (a household's prior monthly-report spreadsheet, not itself reused — layout
and section ideas only, no real data or copy) confirmed the shape: each position itemized under
its group, composition breakdowns per group, and a longer trend view. It also included a
ratios/statistics panel (cash-flow ratio, passive-income ratio, instant-liquidity ratio,
fund-resilience months) and a current-assets-vs-current-liabilities bar. Both are dropped from
this ADR's scope — see Considered alternatives.

"Ledger" was the wrong word for the itemized section during early scoping — it implies a
transaction log. What's wanted is simpler: each position's balance for the reported month, drilled
down from the group totals the dashboard already shows.

## Decision

- **New endpoint: `GET /api/reports/{yearMonth}/positions`.** Returns every active position's
  value *as of that month* — group, subtype, ownership, native + reporting-currency amount, and
  whether the value was carried forward from an earlier snapshot (stale). One request per PDF
  generation, correct for any month (not just the latest).

  The naive client-side approach — fetching the existing per-subtype list endpoints
  (`/api/investments/stocks` etc.) — was rejected: those return each position's *current* latest
  snapshot, not its value at an arbitrary past `year_month`. Since the dashboard lets a user select
  and export any past month (`DashboardScreen.tsx`'s `selectedMonth`), that would silently show
  today's balances stamped with a past month's report.

  The report engine already solves this. `generateMonthlyReports`
  (`backend/internal/repo/monthly_reports_engine.go:493-534`) computes exactly this — each
  position's carried-forward, FX-converted value for a given month index — then immediately sums
  it into `nwAssets`/`nwLiabilities`/etc. and discards the per-position value. This ADR extracts
  that resolution step into a new pure function, `generatePositionDetail(in reportEngineInput,
  targetMonth time.Time) []PositionDetail`, reusing `byPos`/`latestAtOrBefore`/`fx.convert`
  unchanged — no new SQL, no reimplementing carry-forward semantics, and no risk of the endpoint's
  numbers drifting from the aggregate report's.

  A new repo method, `MonthlyReportRepo.GetPositionDetail`, refreshes materialized reports first
  (same as `GetReport`) so both the aggregate totals and this itemized breakdown are backed by the
  same staleness/carry-forward pass, then validates the month exists (`ErrNotFound` otherwise,
  mirroring `GetReport`) before calling the pure function. Nothing here is persisted — computed
  fresh on every read, like the aggregate report is when stale.

- **Composition breakdowns.** Assets/investments/liabilities-by-subtype composition (for
  donut-style charts) is derived client-side from the same `/positions` response — no second
  endpoint. Income and investment-return composition (by category/instrument) reuses subtotals
  `MonthlyReport` already returns on the wire; `frontend/src/api/types.ts` currently types only the
  totals ("Per-category / per-subtype columns also exist on the wire; typed here only as totals
  until a drill-down needs them") — this is that drill-down, so the FE type gains the per-category
  fields already present server-side. Zero additional backend change beyond the one new endpoint.
- **12-month expense vs. passive-income trend.** `MonthlyReport` already carries
  `earned_income_total`, `investment_return_total`, and `derived_living_expenses` per row, and
  `reports[]` is already fetched in full by `useReports()`. `reportPdfData.ts` currently maps only
  `nw_total` into the trend series; it now also extracts these two lines. No new fetch, no backend
  change.
- **Charts.** A new donut/pie chart type joins `charts/LineChart.tsx` in
  `frontend/src/lib/pdf/charts/`, hand-rolled with `@react-pdf/renderer`'s `Svg`/`Path` primitives
  the same way — 0044 already anticipated this exact extension point ("a later chart type, e.g. a
  pie chart, expected as a likely follow-on ask, slots in beside it without restructuring"). No new
  bar chart (see Considered alternatives — the one candidate use for a bar chart is rejected).
- **Layout.** Not a literal port of the reference template's dense multi-column financial-statement
  layout. React-pdf's page-flow model and the app's brand system fit a cleaner,
  single-column-per-section flow better than replicating a spreadsheet-style grid; the template is
  a content checklist, not a wireframe.
- **Pagination.** Becomes real document flow instead of a non-issue: itemized positions scale with
  portfolio size, so the report is no longer roughly fixed-size regardless of household data.
  `@react-pdf/renderer`'s `Page` component paginates overflowing flex content automatically — no
  manual page-break logic needed, but layout choices (e.g. not letting a section start mid-page in
  a way that orphans its heading) now matter in a way 0044 didn't have to consider.
- **Server-side rendering remains out of scope.** The PDF itself still renders client-side (0044
  unchanged on that point); only the *data* for one section now comes from a dedicated endpoint
  instead of being assembled from data already on the client. Moving the whole export server-side
  was raised again while scoping this ADR and explicitly deferred — same trigger condition as
  0044's Consequences (the unattended-scheduled-email feature, if it's ever built) and #394's open
  question about the render library. Not resolved here.

## Considered alternatives

- **Mirror the reference template's literal 4-column dense layout.** Rejected — fighting
  react-pdf's flow model for a spreadsheet-grid layout costs more than it buys, and the app's
  brand/print identity is better served by a native layout than a faithful port of someone else's
  spreadsheet export.
- **Client-side itemized breakdown via existing per-subtype list endpoints.** Rejected — correct
  only for the latest month (see Decision); would need per-position snapshot-history fetches to
  generalize to past months, which reimplements the engine's carry-forward logic outside the
  backend and costs dozens of requests per export for a large portfolio.
- **Build the ratios/statistics panel now, from the template's formulas.** Rejected — those
  formulas encode household-specific assumptions (what counts as "instant liquidity," pension
  income as 80% of current salary, etc.) that need to be worked out deliberately, not lifted from a
  template built for a different purpose. Deferred to #412.
- **Add a current-assets-vs-current-liabilities bar chart.** Rejected for this pass — no
  liquid/current-vs-non-current split exists anywhere in the domain model (checked both the report
  engine and the DB layer); building it is new backend classification work, not a report layout
  change. Not filed as a follow-up issue yet; revisit if/when it's needed for something beyond this
  chart.

## Consequences

- One new endpoint, one new request per PDF generation — not the "up to 9 extra requests" an
  earlier draft of this ADR considered (client-side per-subtype fetching). Simpler for the frontend
  and correct for any month.
- The itemized breakdown makes the report's page count portfolio-size-dependent for the first time
  — still bounded in practice (a household's position count doesn't run away), but no longer the
  "roughly fixed-size" property 0044 relied on to wave off pagination.
- `generatePositionDetail` duplicates none of the aggregate engine's logic (it's extracted, not
  copied), so a future change to carry-forward/FX-conversion rules can't silently diverge between
  the aggregate report and this endpoint.
- Ratios/statistics panel and the current-vs-non-current split stay open questions: #412 (vague,
  deliberately) for the former; the latter has no issue yet.
- `docs/adr/README.md`'s one-line summary for 0044 should be read alongside this ADR — the
  "no per-position ledger," "mirrors dashboard," and "no new endpoint" clauses are superseded here.
