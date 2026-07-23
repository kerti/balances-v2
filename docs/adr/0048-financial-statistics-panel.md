# Financial statistics panel: four ratios, derived rates, stored inflation

Fills the **Statistics** slot [[adr-0045]] reserved on the monthly PDF report (`#412`) with four
household-health ratios — **Cash-Flow**, **Passive-Income**, **Instant-Liquidity**, and **Fund
Resilience** — worked out deliberately against the report engine's actual data rather than lifted
from a spreadsheet template. Along the way it adds a **Pension** Income category and a monthly
**inflation** figure (modelled like an FX rate), because the ratios need inputs the domain did not
yet have.

## Why

[[adr-0045]] deferred the ratios precisely because they "encode household-specific assumptions that
need deliberate work." Working them out surfaced three properties of the existing data that shape
every formula:

- **Living Expenses is a *residual plug*, not observed spending** — `earned_income + investment_return
  + asset_value_change − ΔNW` (the comprehensive-income identity, ADR-0006). It is a legitimate
  cash-spending proxy but is **noisy month-to-month and can go negative**.
- **Investment Return is unrealised mark-to-market** (plus realised + yield, inseparable in the
  monthly aggregate) — it **swings with the market and can be negative**.
- **No liquidity classification exists** across the position subtypes, and there is no inflation or
  expected-return input anywhere.

A ratio built naively on a single month of these would flap wildly and mislead a non-technical
reader. So the deliberate decisions below are mostly about **smoothing, classification, and where
new assumptions live** — not arithmetic.

## Decision

### Smoothing convention

All flow inputs — income, living expenses, passive income, investment-return rate — are a
**trailing-12-month average** (fewer months when the history is shorter). A single lumpy month (a
car bought via a snapshot, a bonus) must not dominate a "health" signal. Stocks (net worth,
investment value, cash) are read at the reported month as-is.

### The four ratios

| Ratio | Formula (flows = trailing-12 avg) | Unit | Reading |
|---|---|---|---|
| **Cash-Flow** (savings rate) | `(Income − LivingExpenses) / Income` | % | share of earned income kept |
| **Passive-Income** | `TotalPassiveIncome / LivingExpenses` | % | `≥100%` = financially independent |
| **Instant-Liquidity** | `InstantLiquidity / InvestmentValue` | % | a **cap**: above target ⇒ idle cash |
| **Fund Resilience** | depletion projection (below) | months / "indefinite" | runway if active income stops |

- **Income** = total earned income (`EarnedIncomeTotal`); the Cash-Flow ratio is cash-only, so it
  excludes Investment Return (mostly non-cash).
- **Instant-Liquidity is a ceiling gauge, not a floor.** Denominator is *total investment value*, not
  expenses: the household reads it to catch **over-liquidity** ("I'm not investing enough"), default
  target intent ~5%. This is deliberately *not* the textbook "emergency-fund months" reading —
  Fund Resilience already covers runway.

### Instant Liquidity = cash-equivalent only

`bank_account` Assets only — the sole same-day-accessible class. Investments (including gold, whose
sale settles in days) are the denominator, not the numerator.

### Passive income has two named scopes (to avoid double-counting)

- **Passive cash income** = `RentalIncome + Pension + Interest`. Cash that keeps arriving after active
  income stops → the Fund Resilience draw-offset. `Interest` is bank/deposit interest — an external
  cash stream, distinct from Investment Return (bank cash is not in the investment pool), so it
  carries no double-count. **Excludes** Investment Return, which the projection already models as the
  pool's own growth `g`; counting it as income too would double-count it.
- **Total passive income** = passive cash income **+ InvestmentReturn** → the Passive-Income Ratio
  numerator. Including unrealised returns makes this ratio market-sensitive and **occasionally
  negative** — a labelled, deliberate property (it matches the household's existing workbook).

Active vs passive is **category-derived**, not a per-event flag: Active = `Salary + BusinessIncome`;
Passive = `RentalIncome + Pension + Interest`; the rest (`Gift/TaxRefund/InsurancePayout/Other`) are
neither. A per-event override is deferred (see Considered alternatives).

> The realized-cash boundary is the axis that keeps passive cash non-double-counting: rent, pension
> and bank interest are cash that has *left* any instrument, so they offset the draw without also
> being counted in `g`. A future slice extends this to bond coupons that **pay out** (vs. accrue),
> the one remaining realized-cash stream currently folded into Investment Return.

### Fund Resilience: monthly depletion projection

```
P₀ = total investment value (NwInvestments)          — investments only
E₀ = trailing-12-avg living expenses (the residual)
PI₀ = trailing-12-avg passive cash income (Rental + Pension + Interest)
g  = trailing-12-avg monthly investment-return rate  (InvestmentReturnTotal / NwInvestments)
i  = monthly inflation rate (see below)

each month m ≥ 1:
    Pₘ  = Pₘ₋₁ · (1 + g) − (Eₘ₋₁ − PIₘ₋₁)
    Eₘ  = Eₘ₋₁ · (1 + i)
    PIₘ = PIₘ₋₁ · (1 + i)          — rents/pensions track inflation

N (Fund Resilience) = first m where Pₘ ≤ 0
```

- Pool is **investments only** — keeps Instant-Liquidity (cash) and Fund Resilience (investments)
  non-overlapping and separately interpretable, and matches the household's framing ("live off the
  investments").
- The draw is **net of continuing passive cash income**, which inflates alongside expenses — the
  whole point of the active/passive split (passive income survives "active income ceases").
- **Never depletes → "indefinite"** (financial independence reached). Detected by simulating to a
  horizon cap (~100 years); a household whose passive cash income already covers expenses is
  indefinite from month 0.
- `g` is **derived** (household's own trailing return), taxes folded into it (no separate tax model);
  it can be negative after a bad year, which correctly shortens runway.

### Inflation: stored monthly like an FX rate, with a setting fallback

`i` is a per-`(Household, year_month)` figure entered manually from a reputable source (source TBD),
**structurally identical to an FX rate** (household-scoped, month-stamped, soft-deleted, no currency
dimension — one series per household). It feeds *only* Fund Resilience. Each monthly entry is an
**annualized (YoY) percentage** — the headline figure sources publish and the same basis as the
setting below — *not* a month-over-month change; the two would otherwise be different
representations to reconcile. The projection averages the annualized figures over the trailing-12
window to an effective annual rate, then converts once to a monthly rate as `(1 + a)^(1/12) − 1`.
Because the source is not yet wired and early months will be empty, an **`assumed_annual_inflation`
Household setting** (annual %, **default 3.5** — slightly conservative vs recent Indonesian CPI, so
the runway estimate errs short/safe) drives the projection out of the box; stored monthly figures,
once present, override it via that trailing-12 average. Deflation months are allowed (the value may
be negative — no positivity constraint, unlike the FX rate).

#### Presentation / UX: monthly-rates page mobile layout

The manual monthly series is edited on the **Settings ▸ Inflation Rates** subpage
(`InflationRatesCard`); the `assumed_annual_inflation` fallback lives on the Settings home page
(Household section) instead — it's a single preference, not a row-based lookup. A shared add form
(month · annual %) sits above the list of entered figures.

Per [[adr-0050]] (mobile–web layout divergence doctrine), the **entered-rates list diverges its
mobile layout** exactly as the structurally-identical FX table does ([[adr-0002]] → Presentation /
UX): the wide month · rate · delete table horizontally scrolls on a phone, hiding the figure the
user opened the page to check. The add form is single-layout; the list splits at the renderer, picked
at runtime by `useIsMobile()` (the single 768px boolean) — one tree in the DOM, both leaves fed the
same rows under the shared `inflation-rate-row` / `inflation-rate-value` testids. **<768px** applies
the doctrine's **"wide table → stacked cards"** transform: one card per figure with the **rate
promoted to the headline** (`tabular-nums`, a trailing "%" making the unit explicit without the
column header, the month below) and a delete icon button at the a11y floor (`size-11`, ≥44px —
INV-PRESENTATION-08); **≥768px** keeps the wide table.

### Presentation: numbers + plain-language explanation, no colour/targets yet

Each ratio renders its value plus one short explanatory sentence (audience is non-technical). **No
per-household target settings and no good/bad colour-coding in v1** — targets are themselves
household assumptions and would each add a setting; a colour-coded scorecard is a later iteration.
The Fund Resilience and Passive-Income explanations state that living expenses are a **derived
residual**, and the Passive-Income explanation notes it moves with the market.

### Undefined / edge states → "—"

A ratio renders an em-dash with a short note when its inputs are unavailable: no flow history
(baseline / <1 month), `Income ≤ 0`, `LivingExpenses ≤ 0`, or `InvestmentValue = 0`. Fund Resilience
needs investments > 0 and at least one flow month.

### Computed at render time, not materialized

The ratios are **pure functions** of the already-materialized report series + positions + inflation
+ setting, computed in Go when the PDF is built (alongside the existing `buildDelta`/`buildYoY` in
`pdf_input`), **not** stored as new `monthly_reports` columns. They depend on a trailing window and
on inflation that may be entered *after* a report is generated, so materializing them would
immediately stale; re-deriving on render is always fresh and needs no migration for the ratios
themselves. Only the genuinely-new *inputs* (Pension and Interest categories, inflation series, the
setting) touch the schema.

### Scope: an epic of sequenced slices

1. **This ADR.**
2. **Pension income category** — enum value + migration (extend `income_category_check`),
   `EarnedIncomePension` engine breakdown column, income form + i18n. **No backup change**: category
   is a plain `text` value (not a shape change per `format.go`'s bump rule) and restore relies on the
   DB CHECK, so backups carry `pension` automatically once the CHECK allows it. (Passive-Income Ratio
   depends on it.)
3. **Monthly inflation model** — FX-like `(household, year_month)` store + repo + entry surface +
   carry-forward + backup inclusion, and the `assumed_annual_inflation` setting fallback.
4. **Statistics panel** — the four-ratio computation package + PDF rendering replacing the ADR-0045
   placeholder block, plus `reportCopy` explanatory strings (en-GB / id-ID).
5. **Interest income category** — bank/deposit interest as a passive *cash* stream. Same shape as
   the Pension slice (enum value folded into the 00012 CHECK + `EarnedIncomeInterest` breakdown
   column + engine bucket + income form + i18n, no backup change) plus one line folding it into the
   passive-cash scope. Untracked today, so it never overlaps Investment Return — no double-count.
6. **Coupon-payout passive-cash extension** — bond coupons with `coupon_disposition = 'pays_out'`
   (migration 00006) are realized cash that has left the instrument, so they belong in passive cash
   alongside rent/pension/interest; accruing coupons stay in `g`. Touches the investment-return
   engine (bigger than an enum slice); sequenced after slice 5.

## Considered alternatives

- **Per-event active/passive flag instead of category-derived.** Rejected for v1 — puts an
  active/passive decision on every Income entry (friction for a non-technical audience) plus a column
  and a backfill rule, for flexibility (a passive business, a `Pension` mis-filed as `Other`) the
  category mapping already covers. Deferred as a future refinement.
- **Reclassify "safe / low-volatility" investment returns as passive cash.** Rejected — volatility is
  the wrong axis. For the Passive-Income Ratio those returns are *already* in the numerator (total
  passive = passive cash + all InvestmentReturn), so splitting a "safe" subset out is a no-op relabel.
  For Fund Resilience they *are* the pool's growth `g`, so counting them as a draw-offset double-counts.
  The distinction that actually matters is **realized cash that left the instrument** (rent, pension,
  bank interest, paid-out coupons) vs. **mark-to-market pool growth** (accruing coupons, equity
  markup) — which is what slices 5–6 implement.
- **Two-tier liquidity (cash + marketable investments).** Rejected — "instant" means same-day, which
  is `bank_account` only; a broader pool blurs the ceiling-gauge reading and overlaps Fund Resilience.
- **Instant-Liquidity ÷ monthly expenses (emergency-fund months).** Rejected — that is Fund
  Resilience's job; the household wants the *over-liquidity* signal (cash ÷ investments).
- **Derive inflation, or hardcode it.** Derived-from-history is noisy and circular (expense growth
  ≈ inflation is self-referential); a hidden constant contradicts "household-specific assumptions".
  A setting-with-default + stored-monthly-override gives a working default now and precision later.
- **Include Investment Return in the resilience draw-offset.** Rejected — double-counts it against
  the pool's growth `g`.
- **Materialize ratios as report columns.** Rejected — trailing-window + late-entered inflation make
  stored values stale on arrival; render-time derivation is always fresh and migration-free.
- **Single reported month (no smoothing).** Rejected — the residual living-expenses figure is too
  noisy (and can be negative) to anchor a "health" signal on one month.
- **Colour-coded targets in v1.** Deferred — every target is another household setting; ship the
  numbers + explanations first.

## Consequences

- **New Income category `Pension`** ripples through the enum, a migration, the engine's earned-income
  breakdown (`EarnedIncomePension`), and the income form + i18n. It needs **no backup-format bump**:
  a new allowed value for the existing `category` `text` column is not a *shape* change (the
  criterion in `backup/format.go`), and restore re-inserts income against the live DB CHECK — so a
  backup carries `pension` with no envelope change. An older build importing a file that contains
  `pension` fails cleanly at the DB CHECK, which is acceptable and inherent to any additive enum
  value; bumping the version would instead reject *every* newer backup, including pension-free ones.
- **A new stored data class (monthly inflation)** plus one Household setting — the first
  report-input the household enters that is neither a Position nor an FX rate. Included in backups.
- **"Passive income" is now a two-scoped domain term** — the CONTEXT glossary names both scopes to
  stop them being conflated in code or copy.
- **The Passive-Income Ratio can print a negative percentage** in a market downturn; this is
  intended and labelled, not a bug.
- **Report copy gains a statistics block** in the server-side `reportCopy` catalog (en-GB / id-ID),
  including the residual-expense and market-sensitivity caveats.
- **No new HTTP surface and no new report columns** — the panel rides the existing
  `GET /api/reports/{yearMonth}/pdf` render path; only inputs touch the schema.
- New QA invariants are warranted (e.g. the passive-income double-count guard: resilience draw-offset
  excludes Investment Return; the "indefinite" detection; undefined-state em-dash rendering).

## Amendment — 2026-07-15: statistics compute on *routine* income only

Slice 4 shipped reading `EarnedIncome*` / `InvestmentReturnTotal` from the materialized report,
which bucket income by **category only** — so the panel averages **all** income, routine and
incidental alike. A one-off (severance, THR, insurance payout, a windfall gift) therefore flatters
every income-based ratio and, worse, is projected by **Fund Resilience as a recurring draw-offset it
will never actually pay**. That is a correctness bug in the survival projection, not a preference.

### Decision

Every income-derived statistics input is filtered to `regularity = 'routine'`:

- **Cash-Flow** `Income` = routine `EarnedIncomeTotal`.
- **Passive-Income** and **Fund Resilience** passive cash = routine `Rental + Pension + Interest`.
- **`LivingExpenses` is unchanged (all-income)** — spending is observed reality; a spent windfall was
  still spent. Only the *income baseline* the ratios judge against becomes "income you rely on". This
  asymmetry is deliberate: expenses are observed, the income baseline is chosen.
- `InvestmentReturn` and all **stocks** (net worth, investments, cash) are untouched — no regularity
  dimension. Instant-Liquidity has no income term and is unaffected.

Applies to **all four ratios**, not resilience alone, so "income" means one thing across the panel.

### Routine vs incidental is the lever — no global toggle

Considered a per-household "include incidental income" setting so gig/informal-income households
could opt one-off income in. **Rejected.** The `regularity` flag already on every income row *is*
that lever, at finer grain than a global switch, and the two would fight — in an "include-all" mode
the per-row tag means nothing. Instead we **sharpen what the flag means**:

> **Routine** = income you *rely on* for planning, even if lumpy or variable (a freelancer's project
> fees, commission, irregular gig pay). **Incidental** = a windfall you should *not* build survival
> on (severance, inheritance, one-off gift, insurance payout, a bonus you won't bank on).

Under that definition one lever serves both households the toggle was meant for: the gig worker tags
relied-upon income **routine** (counted); the conservative planner leaves windfalls **incidental**
(excluded). A global setting would be redundant, add density for a non-technical audience, and
contradict this ADR's no-new-per-household-settings-in-v1 stance. A full docs site is likewise the
wrong tool — a non-technical member won't leave the app to read a manual for a two-option field.

### Legibility: make the choice self-evident at the point of entry

Because `regularity` is now load-bearing for a non-technical audience, the income form earns its keep
without new screens:

1. **Reframe the control as the question, not the jargon** — "Routine / Incidental" becomes *"Can
   you count on this income regularly?"* → **Yes, it's regular** / **No, it's a one-off**, each with a
   one-line example set (en-GB / id-ID).
2. **Smart default from the chosen category** — Salary / Pension / Rental / Interest / Business →
   default **routine**; Gift / TaxRefund / InsurancePayout → default **incidental**; user can flip.
   The category already encodes most of the answer, so the common case needs no decision and the
   sharpened definition only bites on the genuine edge (lumpy-but-relied-upon gig income).
3. **Teach at the point of consumption** — the Fund-Resilience / Passive-Income explanatory strings
   gain a clause: *"Counts income you marked as regular — one-offs like bonuses or severance are left
   out."* The number and the entry form then teach the same rule.

An on-control ⓘ popover is a deferred fast-follow, only if usage testing shows confusion.

### Mechanism: materialize routine subtotals

`regularity` is discarded when the engine buckets income by category, and the ratios cannot recompute
it from the raw `income` table because those amounts are native-currency while the report columns are
FX-converted — recomputing FX outside the engine reimplements it (against INV-FINANCE-18). So the
routine split is **materialized like the category breakdown already is**:

- `reportIncome` carries `regularity`; `loadEngineInput` selects it.
- `earnedIncomeAmounts` accumulates routine subtotals alongside the category totals.
- Additive migration: `earned_income_total_routine`, `earned_income_rental_routine`,
  `earned_income_pension_routine`, `earned_income_interest_routine` on `monthly_reports`.
- `buildStats` reads the `_routine` columns for the income terms above.

This revises the "no new report columns" line only for **inputs** — exactly as the Pension and
Interest slices already added category columns; the *ratios* themselves stay render-time-derived and
unmaterialized. No backup-format bump (additive numeric columns, same rule as the category columns).

### Invariants

- **INV-FINANCE-19** amended — Cash-Flow / Passive-Income income terms are the trailing-12 average of
  **routine** income; LivingExpenses stays all-income.
- **INV-FINANCE-21** amended — the Fund Resilience draw-offset is **routine** passive cash income.
- **INV-FINANCE-24** (new) — `incidental` income is excluded from every statistics income term while
  still counting in full toward net worth, the income statement, and LivingExpenses.

### Slice

7. **Routine-aware statistics** — engine routine subtotals + additive migration + `buildStats` read +
   the three legibility changes (reframed control, category-driven default, panel caption) + i18n.
   Existing reports regenerate on next read; no data migration. The staleness watermark alone cannot
   see this (pure-DDL migration, no input row changes), so each report row carries an
   `engine_version` stamp and `needsRegen` forces regeneration on mismatch — pre-migration rows read
   as version-NULL and regenerate (INV-STALENESS-04).

## Amendment — 2026-07-20: paid-out bond coupons are passive cash (slice 6, #476)

Slice 6 of the scope above. The **open question** it flagged — *do `pays_out` coupons surface as
`InvestmentReturn`, or vanish into the bank balance?* — resolves to the former: a paid-out coupon is
recorded as a Coupon Transaction, and the engine's per-position return (`ΔSnapshot + cash_out −
cash_in`, INV-FINANCE-08) already books its `cash_out` as bond `InvestmentReturn`. So it lands in the
pool's own-return `g` today, **not** in passive cash — the exact miscount slice 6 exists to fix. This
is therefore a **reclassification for the resilience projection**, not a new capture.

### Decision

The realized-cash axis (see Considered alternatives) puts a paid-out coupon in **passive cash**: it is
external cash that left the pool, dependable like rent/pension/interest. But the **domain keeps coupon
yield inside Investment Return** (CONTEXT: investment return covers yield from
Coupons/Dividends/Distributions), and the income statement + `investment_return_total` must not lose
it. The two requirements are reconciled by a two-scope split at render time, not by moving the coupon
out of investment return:

- The engine **materializes** the paid-out slice on its own — `passive_coupon_cash` on
  `monthly_reports`, summed from `pays_out`-disposition Coupon Transactions — leaving
  `investment_return_total` (coupon included) untouched.
- `buildStats` **adds** `passive_coupon_cash` to the passive-cash scope (Passive-Income numerator +
  Fund Resilience draw-offset) **and subtracts** it from own-return `g` (`ownReturn =
  InvestmentReturnTotal − passiveCouponCash`). The Passive-Income **numerator is unchanged** by the
  split (passive cash gains the coupon, own return loses it — the "no-op relabel" the rejected
  low-volatility alternative would have been *for the ratio*); the substantive change is in **Fund
  Resilience**, where the coupon stops compounding on the whole pool as `g` and becomes a fixed,
  inflating draw-offset. That is the real fix — a paid-out coupon shouldn't grow the pool it left.

**Accruing** coupons (`coupon_disposition = 'accrues'`) record no Coupon Transaction — their yield is
snapshot growth — so they never enter `passive_coupon_cash` and stay mark-to-market in `g`, unchanged.
The disposition is read into the engine via a `LEFT JOIN bond_details` on `ListInvestmentsForReport`
and gates the tally, so a mis-entered coupon on an accruing bond is not swept into passive cash.

### Mechanism

- `reportPosition` carries `couponDisposition`; the engine builds a `pays_out` bond set and tallies
  each month's paid-out coupon `cash_out` into `couponCashByMonth`, surfaced as
  `monthlyReportData.passiveCouponCash` (nil on the baseline, mirroring `investmentReturn`).
- Additive migration `00013`: `passive_coupon_cash numeric(20,4)` on `monthly_reports`. No
  backup-format bump (a report column is rematerialized from inputs on restore — reports aren't
  backed up). `reportEngineVersion → 2` trips `needsRegen` so pre-existing rows recompute the column
  (INV-STALENESS-04), same pattern as slice 7.

### Invariants

- **INV-FINANCE-25** (new) — a paid-out coupon is passive cash: added to the passive-cash scope and
  removed from own-return `g`, while its yield stays in `investment_return_total`; accruing coupons
  stay in `g`. Guards the resilience double-count from the coupon angle, extending the INV-FINANCE-21/
  -22 family.

## Amendment — 2026-07-21: earned-income drill-down under cash flow (PDF report PR2)

The downloadable monthly PDF report (ADR-0045) shows a Cash Flow section — earned income in (by
household member) minus living expenses out. It gave no answer to *where* the income came from: the
only active/passive signal in the app was the statistics **passive-income ratio**, a percentage with
no rupiah behind it. This adds a by-source drill-down of the month's earned income, directly under
the existing member breakdown. Second of two PDF-report PRs (PR1 was the page-group layout reorder,
#495); the layout is unchanged here — this is net-new content in the Cash In block.

### Decision

Under the Income total, the report prints the same month's earned income split two ways that must
reconcile:

- **Active** = `salary + business + gift + tax_refund + insurance + other`.
- **Passive** = `rental + pension + interest`.
- **Active + Passive == Income** exactly — one income list, no line shown twice. The buckets are the
  engine's own single-month source columns on `monthly_reports` (`earned_income_*`, **not** the
  `_routine` subtotals), so the split is a decomposition of `earned_income_total`, not a re-read.

**Single-month total basis, incl. one-offs.** This deliberately differs from the statistics
passive-income ratio, which is trailing-12 and **routine-only** (INV-FINANCE-24). The two answer
different questions — the drill-down says "where did *this month's* cash come from", the ratio judges
"how much income does the household *rely on*". The mismatch is intentional; the code and this ADR
say so, so nobody "fixes" it into agreement.

**Paid-out bond coupons ride as a separate additive line.** Coupon cash is passive cash but lives
inside `investment_return_total`, not `earned_income_total` (INV-FINANCE-25). Folding it into Passive
would break `Active + Passive == Income` and double-count it against the cash-flow Net. So
`passive_coupon_cash` prints as its own "Bond coupons paid out" line below the split, informational
only — never summed into Income or Net.

**Member rows kept.** The existing by-member Cash In breakdown stays; the by-source split is added
below it as a second lens on the same total (two decompositions of one Income figure), not a
replacement. Considered replacing member rows with the source split — rejected: "who earned it" and
"active vs passive" are both wanted, and the section has room.

### Mechanism

- No schema change, no migration, no engine change — every input already materialized on
  `monthly_reports` (the `earned_income_*` source columns since the stats epic, `passive_coupon_cash`
  since slice 6 / migration `00013`).
- `buildCashFlow` (`reports/pdf_input.go`) sums the source columns into `CashFlow.Active` /
  `.Passive` and copies `passive_coupon_cash` into `.Coupons` (blank string when zero, so the line is
  suppressed). `cashFlow()` (`reports/pdf/render.go`) renders a "By source" sub-group + the coupon
  line. Copy strings added for `en` + `id`.

### Stats reproducibility inputs + expenses-are-estimated note

Two companion clarity changes shipped alongside the drill-down (owner-facing, `en` + `id`):

- **Reproducibility inputs.** Under the four ratios the panel now prints a muted "Inputs — 12-mo avg,
  regular income only" block: the trailing-12 *routine-income* per-month averages the two **flow**
  ratios divide (`AvgIncome`, `AvgExpenses`, `AvgPassive`), plus the two formulas in words. The reader
  plugs them back in: `Cash-Flow = (AvgIncome − AvgExpenses)/AvgIncome`, `Passive-Income =
  AvgPassive/AvgExpenses`. Averages, not sums — `sum/sum == avg/avg`, so the ratio is identical while
  the figures read as a typical month. Scope is deliberate: only the two flow ratios are
  formula-reproducible. **Instant-Liquidity** is point-in-time stocks (bank cash ÷ investments,
  readable off the balance sheet), **Fund Resilience** is a depletion simulation — neither is a
  one-line plug-in, so neither is expanded. Surfaced with the stats (not in the all-income Cash Flow
  section) so the routine/all-income basis clash never lands inside a section where
  `Active + Passive == Income` must hold. `buildStats` fills `Stats.Inputs` (`StatInputs`) from the
  sums it already accumulates; `statInputs()` renders it, collapsing on the baseline.
- **Expenses are an estimate, stated.** Living expenses are never recorded — the engine derives them
  as a residual (`earned_income + investment_return + asset_value_change − Δnet_worth`). The Cash Flow
  row label is now "Living Expenses (estimated)" and the stats convention note spells out the
  derivation, so the estimate is explicit at the point of reading.

### Invariants

- **INV-FINANCE-26** (new) — the report's earned-income drill-down decomposes the month's income by
  source with `Active + Passive == earned_income_total` (single-month, all-income basis, deliberately
  unlike the trailing-12 routine passive ratio); paid-out coupon cash surfaces as a separate additive
  line and is never folded into the earned-income total or the cash-flow Net.
- **INV-FINANCE-27** (new) — the stats reproducibility inputs are the exact trailing-12 routine
  operands the flow ratios divide (per-month averages), so `Cash-Flow == (AvgIncome −
  AvgExpenses)/AvgIncome` and `Passive-Income == AvgPassive/AvgExpenses` recompute by hand; defined
  exactly when the flow ratios are.

## Amendment — 2026-07-21: investment-performance rates on the PDF report

The report reports investment **return** only as a rupiah figure folded into the comprehensive-income
identity (`ΔNet Worth = Earned Income + Investment Return + Asset Value Change − Living Expenses`), and
the only rate the panel carries about investments is the Instant-Liquidity **cap** gauge. Neither
answers "how are the investments *performing*". This adds a dedicated investment-performance block to
the PDF report: the month's investment return expressed as a **rate**, three ways (total, by **risk
profile**, by **instrument type**), each paired with its **trailing-12-month** figure so a single
lumpy month does not read as the trend. The objective is a read on performance, not another rupiah
tally — so the headline is a rate, with the underlying amount shown muted alongside for context.

### Why a rate, not an amount

A return *amount* ("investments earned Rp 12M this month") is tangible but says nothing about
performance: Rp 12M is excellent on a Rp 100M pool and dismal on a Rp 3B one. Performance is the
amount **over the capital it was earned on**. The numerator already exists — `investment_return_total`
and the per-subtype `investment_return_*` columns are the domain's genuine return
(`ΔSnapshot_value + cash_paid_out − cash_paid_in`, contributions already netted out, ADR-0003 /
INV-FINANCE-08/23). The rate is that numerator over an invested-capital **base**.

### Decision

- **Rate = bucket return ÷ bucket _opening_ invested value**, where the opening value is the bucket's
  invested value at the **prior** month-end (its most-recent snapshot value carried forward, the same
  net-worth carry-forward rule). Opening (not average or closing) capital: "return on what you started
  the month holding". Big mid-month contributions distort a period return slightly — accepted for a
  household read; we do **not** compute a money-weighted IRR (rejected below).
- **Three cuts, each `this month` and `trailing-12`:**
  - **Total** — `investment_return_total ÷ opening total investment value`.
  - **By risk profile** — one row each for `low` / `medium` / `high`. `risk_profile` is a `NOT NULL`
    forced-choice attribute on every Investment (baseline schema, CHECK `low|medium|high`), so this is
    a clean partition with no residual bucket.
  - **By instrument type** — one row each for `stock` / `mutual_fund` / `bond` / `gold` /
    `time_deposit`. `subtype` is likewise single-valued and total on every Investment.
- **Trailing-12 is the geometric compound, not the arithmetic mean.** The trailing figure is
  `Π(1 + rₘ) − 1` over the in-window months (the reported month and the eleven preceding, fewer when
  history is shorter), where `rₘ` is that month's bucket rate. Averaging monthly rates arithmetically
  overstates a compounding series (a `+50% / −50%` pair means `−25%`, not `0%`); the panel would lie.
  A month whose opening base is zero contributes **no factor** (it is skipped, like a zero-flow month
  in the ratio window), rather than forcing the product to zero.
- **Amounts add, rates do not.** Reconciliation guards are on the amounts: `Σ_risk return =
  Σ_subtype return = investment_return_total`. The three *rate* headlines do **not** sum to the total
  rate (each bucket divides by its own base) — that is correct and expected, and the copy does not
  imply otherwise.
- **Zero / absent opening base → "—".** A bucket held for the first month (no prior snapshot → opening
  base 0), or fully exited, has an undefined rate; it renders an em-dash, never `÷0` or a misleading
  `0%`. The muted amount still prints when a return exists. Follows the existing undefined-state
  convention (INV-FINANCE-22).

### Rejected

- **Money-weighted (IRR) or true time-weighted return.** Correct-to-the-textbook but needs
  intra-month dated cash flows and sub-period linking the month-granularity snapshot model does not
  carry; opening-base return is the honest approximation at this data resolution, and the ADR labels
  it as such.
- **Arithmetic mean of monthly rates** — overstates, see above.
- **Amount-led with rate secondary.** Reversed after grilling: the stated objective is a performance
  read, which is the rate; the amount is the context number.

### Mechanism

- **Materialize the amounts + bases, derive the rates at render** — the same boundary as the four
  ratios (rates are never stored, ADR-0048 "Computed at render time"). Migration `00014` adds, all
  `numeric(20,4)` nullable (nil on the baseline month, like the existing per-subtype returns):
  - per-risk return — `investment_return_{low,medium,high}`;
  - per-instrument-type opening-base source — `investment_value_{stock,mutual_fund,bond,gold,time_deposit}`;
  - per-risk opening-base source — `investment_value_{low,medium,high}`.

  The total opening base is the existing `nw_investments` (no duplicate column). The `investment_value_*`
  columns are each month's **closing** invested value per bucket; the render reads the *prior* month's
  column as the current month's opening base (carry-forward already applied by the engine).
- **Engine** (`repo/monthly_reports_engine.go`): `reportPosition` gains `riskProfile`; the net-worth
  pass accumulates per-subtype and per-risk closing value alongside `nw_investments`; the
  income-statement pass adds a per-risk return tally beside the existing per-subtype one. `engine_version`
  bumps **2 → 4** (v3 = this breakdown; v4 folds in the placement column below), so the staleness
  watermark rebuilds every month on deploy (no manual backfill).
- **Render** (`reports/pdf_input.go` + `pdf/render.go` + `pdf/reportcopy.go`): a new
  `buildInvestmentPerformance` walks the report series, computing each bucket's this-month rate
  (return ÷ prior-month base) and trailing-12 compound; a new performance block renders the three
  small tables (`this month | 12-mo`). Copy strings for `en` + `id`.
- **No new route, no API-shape change** — the PDF handler renders from the materialized row; the JSON
  `reportResponse` is left untouched (the per-risk columns are render-only for now).

### Invariants

- **INV-FINANCE-29** (new) — investment return and closing value each reconcile across **both**
  partitions of the Investment group: `Σ investment_return_{low,medium,high} == investment_return_total`
  and `Σ investment_value_{low,medium,high} == Σ investment_value_{stock,mutual_fund,bond,gold,time_deposit}
  == nw_investments`. Both `risk_profile` and `subtype` are `NOT NULL`, single-valued, and total on
  every Investment, so each is a complete partition with no residual bucket.
- **INV-FINANCE-30** (new) — a bucket's this-month investment-return rate is `bucket return ÷ bucket
  opening (prior-month) invested value`; it is **undefined** (renders "—", never `÷0` or `0%`) when
  the opening base is zero or absent. Extends the undefined-state convention (INV-FINANCE-22) to the
  performance block.
- **INV-FINANCE-31** (new) — the trailing-12 investment-return rate is the geometric compound
  `Π(1 + rₘ) − 1` over in-window months with a defined base (months with a zero/absent opening base
  contribute no factor), **not** the arithmetic mean of the monthly rates.

### Placement — new money as a share of the pool

The performance block answers "how did the investments *perform*"; it does not say "how much did we
*put in*". A pool can grow because it appreciated (return) or because the household deployed new money
(placement) — two different stories the report should keep apart. This adds a **placement** line below
the performance tables: new money deployed into investments, as a share of the opening pool, so the
reader sees `pool growth ≈ return% + placement%` for the month.

**Definition — NET new money into the pool.** Placement is a **net monthly capital flow**:

```
placement = (Buys + fresh TD placements)  −  (Sells + cash_out maturity principal/interest)
```

all FX-converted. It **excludes**:
- **TD rollover funding + rolled-to-new maturities** (`rolled_to_new`, issue #27). A rolled TD is funded
  by its predecessor's maturity — the money **never touches the bank**, it recycles internally. It falls
  out of *both* legs (the rollover cash_in is not a Buy; the rolled maturity's disposition isn't
  `cash_out`), so an `auto_renew` TD nets to 0, not a recurring phantom placement.
- **Cash `fee` inflows** — a cost, not deployment.
- **Coupons / dividends / distributions** — income *yield*, not principal returning; they do **not** net
  against placement.

**Net, not gross — corrected from the first cut.** The first version was gross (Buys + placements only)
and **double-counted recycled capital**: a matured bond that pays out to the bank (`cash_out`) and is
reinvested the same month into a new bond read as a full fresh placement, even though it is the *same*
capital cycling through — counted once when the old bond was bought and again on reinvestment. (Illustration:
a bond of 100 matures paying out to the bank and 100 is reinvested into a new bond the same month,
alongside a genuine 25 of fresh deployment — gross reads 125, the correct net is 25.) Netting principal
returns fixes it and is the same rule that makes a **rebalance** (sell A → buy B) net
to ~0. A **net-withdrawal month** (matured/sold more than deployed) legitimately goes **negative** —
a real, labelled signal that the household drew down its investments; the rate then reads negative, not
"—". Because the flow is *monthly net*, a maturity in one month and its reinvestment in the next show as
a negative then a positive that wash out over the trailing window — the correct behaviour.

**Rate base = the opening pool**, the same denominator as the return rate (chosen so the two compose:
`return% + placement% ≈ this-month pool growth`, ex-withdrawals). Undefined → "—" on a zero/absent
opening pool (INV-FINANCE-30's convention).

**Trailing-12 is an arithmetic average, NOT the geometric compound used for return.** Placement is a
*flow* you average to a "typical month" (`Σplacement / Σopening-pool` for the %, `Σplacement / n` for
the amount) — it is not a compounding growth rate, so compounding it would be meaningless. This is a
deliberate, documented asymmetry with the return trailing figure (INV-FINANCE-31): return *compounds*
because it is growth; placement *averages* because it is a contribution. The code and this ADR say so,
so nobody "fixes" them into agreement.

- **Mechanism.** Migration `00014` adds one more column, `investment_placement numeric(20,4)` (nil on
  the baseline). The engine nets two per-month accumulators — `placementByMonth` (Buy cash_in + the
  synthetic fresh-TD placement cash_in) minus `returnedByMonth` (Sell proceeds + `cash_out`-disposition
  maturity principal/interest); the rollover-funding loop and cash fees touch neither, coupons/dividends
  are not netted. `buildInvestmentPerformance` computes the this-month and trailing figures at render;
  `perfPlacementRow` renders the line (% in both columns, amount muted beneath each). `engine_version →
  4` (the placement leg is part of v4; its net correction is pre-release, so no separate bump).

### Invariants (placement)

- **INV-FINANCE-32** (new) — `investment_placement` is the month's **net new money into investments**:
  `(Buys + fresh TD placements) − (Sells + cash_out maturity principal/interest)`, FX-converted. It
  **excludes** TD rollover funding and rolled-to-new maturities (recycled capital that never touches the
  bank — off both legs, so an `auto_renew` TD nets to 0), cash `fee` inflows, and coupons/dividends/
  distributions (income yield, not principal, so they do not net). Reinvesting a matured bond nets to
  ~0; a net-withdrawal month is **negative** (not "—"). Rendered as a share of the **opening** invested
  pool (undefined → "—" only on a zero/absent pool), whose trailing-12 figure is the **arithmetic
  average** (`Σplacement / Σopening-pool`; amount `Σ / n`) — deliberately **not** the geometric compound
  of the return rate (INV-FINANCE-31), because placement is a flow, not a compounding growth rate.
