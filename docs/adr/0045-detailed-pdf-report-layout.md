---
status: supersedes ADR-0044 (render location; "no new endpoint")
---

# Detailed PDF report: server-side render, itemized financial statement

Supersedes [[adr-0044]] on where the report renders. The exported monthly PDF moves from a
client-side `@react-pdf/renderer` mirror of the dashboard to a **backend-rendered, portrait,
itemized financial statement** drawn natively in Go (`go-pdf/fpdf`): each Position itemized under
its group, composition donuts, a 12-month trend, and a prominent health-indicator panel — a
document closer to the household's old spreadsheet workbook than to the live UI.

## Why

0044 rendered the report client-side, reasoning that the data was already on the client and that
server-side rendering meant either headless Chromium (conflicts with the lean self-hostable image,
ADR-0030/0037) or a "hand-built native Go PDF layout" it lumped in as "meaningfully heavier." It
also kept the layout a light mirror of the dashboard, on the reasoning that a faithful, dense port
of the household's reference template wasn't worth it — that "fighting react-pdf's flow model for a
spreadsheet-grid layout costs more than it buys."

Both framings collapse once the goal is the dense, detailed, tabular statement the household
actually wants:

- **The layout that "costs more than it buys" in react-pdf is the layout we want.** A dense,
  itemized, right-aligned financial statement is the exact case react-pdf's flexbox-flow model
  fights hardest and a native cell/xy PDF library does trivially. That rejection reason inverts
  into a reason to change renderers, not to drop the layout.
- **Native Go PDF is *not* "meaningfully heavier."** 0044 conflated it with headless Chromium. The
  thing that violates the lean-image constraint is the *browser*; a pure-Go `fpdf` dependency is a
  few hundred KB of Go, no runtime binary, no browser payload — it *satisfies* ADR-0030/0037.
- **The data already lives on the backend.** The report engine computes every number. Rendering
  server-side stops shipping per-position JSON to the client only to re-derive layout there.
- **It removes the one stated blocker for a real future feature.** 0044 named a fully-unattended
  scheduled report email (no browser present) as the *only* thing that would force server-side
  rendering. Building it now makes that feature a straight compose-on-top, not a re-architecture.

The reference template (a household's prior monthly-report spreadsheet — layout and section ideas
only, no real data or copy reused) is a **content checklist, not a wireframe**: its value is the
density and per-Position itemization, not its literal landscape 4-column grid.

## Decision

- **Render location: backend, native Go via `go-pdf/fpdf`.** Supersedes 0044's client-side
  decision and its "no new dependency, no new endpoint" claim. `fpdf` (BSD-3, clean under the
  project's AGPL-3.0, ADR-0042) is pure Go — no browser, satisfies the lean-image constraint. The
  PDF itself now renders server-side; the client's job shrinks to triggering a download.

- **Layout: portrait, single-column section-flow.** Not the template's landscape 4-column grid.
  Fidelity is to the template's *content and tabular density*, not its grid. Sections stack
  top-to-bottom and paginate through natural document flow (`fpdf` page breaks), which also makes
  the portfolio-size-dependent page count a non-problem — single-column-per-section flow is the
  right shape regardless of renderer.

- **Section order:** Header → **Net Worth** headline → **Statistics** → **Assets** → **Liabilities**
  → **Investments** → **Cash Flow** → **Charts**. Statistics sits directly under the headline
  deliberately: it is the household's financial-health scorecard and should be immediately visible.
  It renders as a **placeholder block now** (#412) — reserving the prominent slot so the ratios drop
  in later with zero reflow. (Section headings render localized at runtime — English above, Indonesian
  in id-ID — from the `reportCopy` catalog; the domain terms are used here.)
  - **Assets**, itemized: *Current Assets* → Bank Accounts grouped **by owner**; *Non-current
    Assets* → Property, Vehicles. Subtotals + Total Assets. (Current vs non-current here is a fixed
    *presentational* mapping — Bank = current; Property, Vehicles = non-current — not a domain
    classification; see the dropped bar chart below, which needed a split that genuinely doesn't
    exist.)
  - **Liabilities**, itemized: Institutional Debt / Personal Debt + Total Liabilities.
  - **Investments**, itemized by subtype: Mutual Funds, Bonds, Gold, Stocks, Time Deposits + Total
    Investments. (The reference template's "Peer-to-Peer Lending" is *not* a domain subtype — the
    investment subtypes are exactly `stock`/`mutual_fund`/`bond`/`gold`/`time_deposit` — so it is
    dropped, not rendered as an empty group.)
  - **Receivables**, itemized (group only, no subtypes), rendered only when the household has any.
    Absent from the reference template but part of net worth (`nw = assets + investments +
    receivables − liabilities`); omitting it would leave the itemized sections not summing to the
    headline.
  - **Cash Flow**: Cash In (earned income, **by household member** — from the engine's existing
    per-user `earned_income`) − Cash Out (derived living expenses) = Net Cash Flow. Investment
    P/L is a statistic, not cash flow, and stays in the Statistics panel (deferred).

- **All currencies tracked in the reported month are surfaced.** Each itemized Position shows its
  **native** amount + currency where native ≠ reporting (e.g. a US stock: `$1,000` → `Rp
  15,500,000`), reporting-currency-only where they match; an **FX-rates-used** section lists every
  rate applied that month. Group subtotals and net worth stay in the household reporting currency —
  restating the *grand total* in every currency would be noise. This replaces 0044's single
  "secondary currency captured from the Q15c toggle at export": there is no secondary-currency
  concept and no currency parameter — the currency set is derived from the month's data.

- **Charts: hand-drawn vector via `fpdf` primitives** (arcs/polylines), not raster or SVG — crisp
  at any zoom, theme-independent, no image pipeline, no extra dependency. Three composition donuts
  (Assets / Investments / Liabilities by subtype) + a 12-month trend line. The branch's
  `pieChartMath` / `lineChartMath` (pure TS) port to Go as the geometry half. The reference
  template's current-assets-vs-current-liabilities bar stays dropped — no liquid/current-vs-non-
  current split exists across the domain (the Assets-section current/non-current grouping above is
  presentational only and does not generalize to a bar that needs the same split on the liabilities
  side).

- **Font: embed Geist (Regular + Bold)** via `go:embed` + `fpdf.AddUTF8Font`, matching the web UI
  (OFL, redistributable; Latin coverage suffices for Indonesian). Two TTFs compiled into the
  backend binary; ~50–100 KB subset per generated PDF. Replaces 0044's generic Helvetica.

- **Money/number formatting is server-side and matches the app, not the template.** A small
  `moneyfmt` package built on `golang.org/x/text/message`+`currency` (already an indirect dep;
  promoted to direct) reproduces `lib/format.ts`'s `Intl.NumberFormat` output — IDR shows with no
  decimals and locale grouping (`Rp 4.479.560.000`), *not* the template's 2-decimal comma-group
  spreadsheet convention. A **golden parity test** — `{en-GB, id-ID} × {tracked currencies}`
  asserting Go output equals captured `Intl.NumberFormat` strings — guards the one new drift surface
  (dashboard formats in JS, report formats in Go); any cell where x/text ≠ Intl gets a thin shim.

- **Endpoint: `GET /api/reports/{yearMonth}/pdf`** → `application/pdf`, `RequireAuth`, `Content-
  Disposition: attachment; filename="Balances_<YYYY-MM>.pdf"`. Reuses the same staleness-refresh
  path as `GetReport`/`GetPositionDetail`. **No currency param.** **Locale is derived from the
  authenticated user's stored language preference** (the same way transactional emails pick their
  recipient locale, ADR-0035) — not a query param. Report copy lives in a new **server-side Go
  catalog** (`reportCopy`, following `auth/email_i18n.go`'s hand-rolled per-locale pattern); the
  FE `dashboard.json` report keys are removed.

- **Reuse / discard partition:**
  - **Kept:** `generatePositionDetail` / `PositionDetail` / `buildPositionSnapshotIndex` (the
    extracted carry-forward + FX resolution) and `MonthlyReportRepo.GetPositionDetail` — they now
    feed the renderer directly. **INV-FINANCE-18** (per-position sum matches the aggregate report)
    is unchanged: it guards the extracted pure function, not any HTTP shape.
  - **Dropped:** `GET /api/reports/{yearMonth}/positions` (the JSON endpoint existed only to feed
    the client renderer — no consumer remains; re-add if one appears). All branch frontend PDF code
    (`ReportDocument.tsx`, `charts/*.tsx`, `reportPdfData.ts`, the `api/types.ts` position types,
    their tests). `ReportPdfButton.tsx` is rewritten to a plain authenticated download.
  - **Ported:** `pieChartMath` / `lineChartMath` (TS → Go); report copy strings (FE i18n → Go
    catalog).

- **Ratios / statistics stay deferred to #412.** Placeholder panel now (see section order); the
  formulas encode household-specific assumptions that need deliberate work, not lifting from a
  template built for a different purpose.

- **App version in the footer stays deferred to #414.** The per-page footer carries branding +
  page number now; the version isn't plumbed server-side yet (no ldflag/const/env) and gets its own
  PR (build-time `git describe` ldflag → config → footer, `dev` fallback).

## Considered alternatives

- **Keep client-side `@react-pdf/renderer` and build the dense layout there.** Rejected — react-pdf's
  flexbox-flow model is the wrong tool for a dense tabular statement; the "costs more than it buys"
  reasoning applies, and the fix is to change renderer, not to abandon the layout.
- **Headless Chromium (HTML → PDF).** Rejected — violates the lean self-hostable image (ADR-0030/
  0037). This is the constraint 0044 correctly cited; it applies to the *browser*, not to native
  Go PDF.
- **Faithful landscape 4-column port of the reference template.** Rejected — the template is a
  content checklist, not a wireframe; portrait single-column flow reads cleaner, paginates
  naturally, and was the chosen layout.
- **Raster (go-chart PNG) or SVG charts.** Rejected — hand-drawn vector is crisper at print/zoom,
  carries no chart-font-≠-report-font mismatch, and adds no dependency or image pipeline.
- **Hand-roll number formatting from scratch.** Rejected — `x/text` is already an indirect dep and
  is CLDR-backed (same family as `Intl`); a golden parity test handles the residual Intl-parity
  risk with far less code to own.
- **Keep the `/positions` JSON endpoint as a public data surface.** Rejected — YAGNI once the client
  renderer is gone; nothing consumes it.
- **Build the ratios/statistics panel now from the template's formulas.** Rejected — household-
  specific assumptions; deferred to #412.

## Consequences

- **A fully-unattended scheduled report email is no longer blocked.** 0044 named it as the sole
  trigger for server-side rendering; the structural barrier is now gone. Not built here —
  but it composes as a straight "render bytes → `Mailer.Send` with attachment," no browser needed.
- **The ~1.4 MB `@react-pdf/renderer` lazy chunk disappears from the frontend** — the bundle-size
  question tracked in #394 is resolved as a side effect of this pivot, not separately.
- **New backend dependency: `go-pdf/fpdf`** (BSD-3, clean under AGPL); `golang.org/x/text` promoted
  indirect → direct. Two Geist TTFs added to backend assets and compiled into the binary.
- **Formatting now has two implementations** (JS on the dashboard, Go in the report). The golden
  parity test is the guard; "the app changed its number format" becomes a fixture update the Go side
  is forced to match.
- **Report i18n is a new server-side surface** — a small, plural-free, two-locale Go catalog
  (`reportCopy`), matching the existing email-i18n precedent; the FE no longer carries report copy.
- **INV-FINANCE-18 is unchanged** — still guards the extracted per-position resolution against
  diverging from the aggregate report.
- `docs/adr/README.md`'s line for 0044 should be read alongside this ADR: its "renders client-side,"
  "mirrors the dashboard," and "no new endpoint" clauses are superseded here.
