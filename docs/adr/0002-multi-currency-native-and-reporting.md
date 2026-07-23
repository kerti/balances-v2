# Multi-currency: native amount + reporting currency

Every monetary value is stored as `(amount, currency)` in its **native currency**. Each user has a
`reporting_currency` setting (default IDR). Net-worth aggregation looks up a per-month FX rate for
each non-reporting currency — entered manually in v1; an external rate-feed API can replace manual
entry later without a schema change.

Storing native amounts preserves auditability against the user's bank statements (their source of
truth) and avoids irrecoverable loss of original-currency information. A separate reporting layer
keeps aggregation simple and lets historical FX assumptions be revised without rewriting source
data.

## Considered alternatives

- **Single currency (IDR everywhere).** Rejected — converting at entry time loses the original
  number forever, breaking auditability against statements.
- **Store both native and IDR-equivalent on every row.** Rejected — couples FX assumptions to
  historical rows; rate corrections require migrations of all snapshots.

## M5 implementation notes

- **`fx_rates` table** is household-scoped: `(household_id, year_month, currency, rate)` with `rate`
  = reporting-currency units per 1 unit of the foreign `currency`, `DECIMAL(20,8)` per ADR-0011,
  plus audit + soft-delete (so edits feed the report staleness check, ADR-0006). Partial unique on
  `(household_id, year_month, currency) WHERE deleted_at IS NULL`. No `as_of_date` — the rate is the
  month-end rate for `year_month` by convention; `created_at` is audit-only (when entered, not the
  rate's month). Reporting currency stores no row (rate ≡ 1).
- **Rate resolution carries forward**: month `M` uses the most recent rate with `year_month ≤ M`,
  mirroring snapshot carry-forward (CONTEXT → Net Worth).
- **Missing rate → exclude + warn**, never treat as 1:1. A foreign currency held in `M` with no rate
  ≤ `M` excludes those positions from converted totals and lists them in a `missing_fx` warning on
  the report, distinct from `stale_positions`. Carry-forward of a stale *rate* was rejected — a
  year-old FX rate distorts materially and the user can't eyeball the error the way they can a stale
  balance.
- **Multi-currency toggle** (`households.multi_currency_enabled`, default off) gates UI exposure +
  whether conversion runs, not storage. Off = single-currency household, pinned to reporting
  currency, FX machinery dormant. See CONTEXT → Multi-currency reporting.
- **Manual entry in v1**; auto-fetch is deferred post-M5. Planned provider is **Frankfurter**
  (frankfurter.app — free, no key, ECB-sourced, historical-by-date, covers IDR), not Google Finance
  (no usable public API). When it lands: an on-demand "fetch this month's rates" button (not a cron)
  plus a `source` column (`manual`/`auto`) so manual corrections stay authoritative and are never
  overwritten by the fetcher. Column added with the fetcher, not speculatively now.

## Presentation / UX

The manual rate table lives on the **Settings ▸ Exchange Rates** subpage (`FxRatesCard`), gated by
the multi-currency toggle — a single-currency household gets a pointer back to the Currency toggle
rather than a dead-end CRUD form. A shared add form (month · currency · rate) sits above the list of
entered rates.

**Naming the direction (both the code and the number).** `rate` is reporting-currency units per 1
unit of the foreign currency, so a bare "USD" hides both the base and the direction — a real point of
confusion for the non-technical household audience. Two surfaces express it differently, matched to
their shape, and the reporting currency (the counterpart) comes from the session, falling back
gracefully until it loads:

- The desktop **Currency column** names the whole **pair**: `USD → IDR`. The rate number lives in its
  own labelled **Rate** column, so there's no adjacency to misread.
- The mobile **card** and the add form's **live hint** spell a full **equation**: `1 USD = 15600 IDR`.
  On a stacked card the promoted bare number would sit flush against `USD → IDR` and misread as
  "15600 USD"; the equation binds the number to the *reporting* currency it actually is. The add hint
  renders as soon as a 3-letter code is typed (`1 USD = ? IDR` before a rate is entered), so the
  foreign-only field is never a clueless single input. The `→` and `=` are built as expressions to
  stay clear of the [[adr-0026]] bare-JSX-text rule.

Per [[adr-0050]] (mobile–web layout divergence doctrine), the **entered-rates list diverges its
mobile layout**: the wide month · currency · rate · delete table horizontally scrolls on a phone,
hiding the rate the user opened the page to check. The add form is single-layout (it already reflows
via `flex-wrap`), but the list splits at the renderer, picked at runtime by `useIsMobile()` (the
single 768px boolean) — one tree is ever in the DOM, both leaves fed the same rows under the shared
`fx-rate-row` / `fx-rate-value` testids:

- **≥768px** keeps the wide table (Month · Currency pair · Rate · delete).
- **<768px** applies the doctrine's **"wide table → stacked cards"** transform: one card per rate
  with the **rate promoted to the headline as the equation** (`tabular-nums`, the month below),
  readable with no horizontal scroll. The desktop's ghost text "Delete" becomes an icon button sized
  to the a11y floor (`size-11`, ≥44px — INV-PRESENTATION-08).
