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

- **Passive cash income** = `RentalIncome + Pension`. Cash that keeps arriving after active income
  stops → the Fund Resilience draw-offset. **Excludes** Investment Return, which the projection
  already models as the pool's own growth `g`; counting it as income too would double-count it.
- **Total passive income** = passive cash income **+ InvestmentReturn** → the Passive-Income Ratio
  numerator. Including unrealised returns makes this ratio market-sensitive and **occasionally
  negative** — a labelled, deliberate property (it matches the household's existing workbook).

Active vs passive is **category-derived**, not a per-event flag: Active = `Salary + BusinessIncome`;
Passive = `RentalIncome + Pension`; the rest (`Gift/TaxRefund/InsurancePayout/Other`) are neither. A
per-event override is deferred (see Considered alternatives).

### Fund Resilience: monthly depletion projection

```
P₀ = total investment value (NwInvestments)          — investments only
E₀ = trailing-12-avg living expenses (the residual)
PI₀ = trailing-12-avg passive cash income (Rental + Pension)
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
carried forward — **structurally identical to an FX rate**. It feeds *only* Fund Resilience. Because
the source is not yet wired and early months will be empty, an **`assumed_annual_inflation` Household
setting** (sensible default) drives the projection out of the box; stored monthly figures, once
present, refine it via the trailing-12 average. One reporting currency per Household ⇒ one inflation
series. Annual figures convert to a monthly rate as `(1 + a)^(1/12) − 1`.

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
themselves. Only the genuinely-new *inputs* (Pension category, inflation series, the setting) touch
the schema.

### Scope: an epic of sequenced slices

1. **This ADR.**
2. **Pension income category** — enum value + migration, `EarnedIncomePension` engine breakdown
   column, income form + i18n, backup-format bump. (Passive-Income Ratio depends on it.)
3. **Monthly inflation model** — FX-like `(household, year_month)` store + repo + entry surface +
   carry-forward + backup inclusion, and the `assumed_annual_inflation` setting fallback.
4. **Statistics panel** — the four-ratio computation package + PDF rendering replacing the ADR-0045
   placeholder block, plus `reportCopy` explanatory strings (en-GB / id-ID).

## Considered alternatives

- **Per-event active/passive flag instead of category-derived.** Rejected for v1 — puts an
  active/passive decision on every Income entry (friction for a non-technical audience) plus a column
  and a backfill rule, for flexibility (a passive business, a `Pension` mis-filed as `Other`) the
  category mapping already covers. Deferred as a future refinement.
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
  breakdown (`EarnedIncomePension`), the income form + i18n, and the **backup format version**
  (ADR-0036 immutability ⇒ a version bump, not an edit).
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
