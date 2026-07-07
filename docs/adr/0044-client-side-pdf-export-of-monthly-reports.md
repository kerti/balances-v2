# Client-side PDF export of monthly reports

Exporting a monthly net-worth report (#187, Q22) as a PDF renders entirely client-side, via
`@react-pdf/renderer`, triggered by a "Download PDF" button on `DashboardScreen`. The backend adds
no new dependency and no new endpoint for this feature.

## Why

The report data (`MonthlyReport`) is already materialized and shipped to the client as JSON (ADR-0006),
and locale-aware formatting (`lib/format.ts`, `react-i18next`) already runs client-side. Server-side
rendering would require either a headless-Chromium dependency or a hand-built native Go PDF layout —
both meaningfully heavier than reusing what already renders the dashboard, and the former conflicts
with keeping the self-hostable image lean (ADR-0030/0037: one pull-based image, no browser payload).

We seriously considered server-side rendering because of a **planned future feature**: a button to
email the report directly to household members. That doesn't actually require server-side PDF
generation — the client generates the bytes and POSTs them to a new endpoint that attaches them to a
`Mailer.Send` call (needs an `Attachments` field on `email.Message`, a small addition regardless of
render path). The *only* thing that would force server-side rendering is a fully **unattended**
scheduled send (no browser present) — a bigger, structurally different feature, deliberately deferred
(see Consequences).

## Decision

- **Library:** `@react-pdf/renderer` (MIT; compatible with the AGPL-3.0 project license, ADR-0042).
  Lazy-loaded via a plain dynamic `import()` **inside the button's click handler**, not
  `React.lazy`/`Suspense` on mount. Its dependency graph (~1.4MB — fonts, layout engine, pdfkit) is
  far larger than any other lazy chunk in this app (`recharts`, mount-loaded for `SnapshotChart`, is
  ~18KB) and is only needed for a comparatively rare, explicitly-triggered action — eagerly fetching
  it on every dashboard view would be bloat disproportionate to how often it's used. (First shipped
  mount-loaded, mirroring `SnapshotChart`'s pattern without weighing that difference; corrected after
  it caused e2e flakiness under CI's cold Vite cache — see Consequences.) Reuses
  `lazyWithReload.ts`'s `importWithReloadGuard` directly (the same post-deploy chunk-reload recovery,
  applicable to any dynamic import, not just `React.lazy` components) rather than the `lazyWithReload`
  wrapper itself. This is still a PWA with an offline angle (ADR-0016) — the chunk isn't part of the
  main bundle either way.
- **Scope (v1):** mirrors the on-screen dashboard exactly — headline net worth, time-series chart,
  4-row group breakdown, FX rates used, income-statement lines, by-person split. **No per-position
  ledger.** The dashboard itself never enumerates individual positions, so nothing in the PDF needs
  to either; this also means pagination is a non-issue — content is bounded and roughly fixed-size
  regardless of portfolio size, and multi-page happens via ordinary document flow if it ever
  overflows one page. Future feature requests (itemized position appendix, etc.) get filed as their
  own issues against the shipped export, not bundled in here.
- **Chart:** the one time-series chart (`SnapshotChart`) is redrawn natively using
  `@react-pdf/renderer`'s own `Svg`/`Path` primitives (vector, print-crisp, theme-independent) rather
  than rasterizing the on-screen `recharts` SVG. Lives in its own module
  (`frontend/src/lib/pdf/charts/LineChart.tsx`) so a later chart type (e.g. a pie chart, expected as
  a likely follow-on ask) slots in beside it without restructuring — not built now.
- **Currency display:** the PDF captures whatever the Q15c secondary-currency toggle is set to at
  the moment of export — no separate PDF-only currency setting.
- **Branding:** the vector wordmark (`wordmark-light.svg`), not the rasterized email PNG from #163 —
  that raster exists only because mail clients strip `@font-face`/SVG, a constraint that doesn't
  apply here.
- **Filename:** `Balances_<YYYY-MM>.pdf`.

## Considered alternatives

- **Server-side rendering (headless Chromium or native Go PDF lib).** Rejected — see Why. Revisit
  only if the unattended-email feature below actually gets built.
- **Browser-native print-to-PDF (`window.print()`).** Rejected — worse UX than a single download
  button for a non-technical audience; no real implementation savings over a dedicated lib.
- **Rasterize the on-screen chart** instead of redrawing it. Rejected for print fidelity and
  theme-independence (see Decision).

## Consequences

- Emailing the report to household members (a real, stated future want) is *not* blocked by this
  decision — it composes as: client generates PDF → new endpoint accepts bytes + recipient list →
  `Mailer.Send` with an attachment. Not built as part of #187.
- A **fully unattended, scheduled** report delivery (no user session, e.g. auto-email on the 1st of
  the month) is explicitly out of scope and would need a separate design decision — client-side
  generation cannot run without a browser. Note: issue #371 (reminder email) does not currently ask
  for this — it's scoped as a stale-data nudge, not a report attachment.
- Click-triggered loading means the button itself must render as a plain, always-visible element —
  no `Suspense` fallback state to design for, and the one-time chunk-fetch latency now lands after
  the click (where a "Preparing…" busy state already covers it) rather than before the button even
  appears. The e2e smoke spec (`dashboard-pdf-export.spec.ts`) asserts the button is visible with the
  *default* locator timeout — if that ever needs widening again, that's a signal the button stopped
  being a plain synchronous render, not that the timeout needs raising further.
