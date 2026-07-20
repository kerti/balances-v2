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
