# Earned income tracking with derived returns and residual expenses

To approximate the Household's cashflow without contradicting the snapshot-based net-worth model
(ADR-0001), only **earned income** is tracked explicitly. Investment returns are **derived** from
snapshots and transactions already in the system. Living expenses are **derived** as the residual
that closes the comprehensive-income identity.

```
ΔNet Worth = Earned Income + Investment Return + Asset Value Change − Living Expenses
```

This decomposes the per-month `ΔNet Worth` (which the materialized monthly report already produces —
ADR-0006) into named line items the user can read as an income statement.

The **Asset Value Change** term (added during the M5 grilling) isolates the non-cash mark change of
non-financial Assets — property + vehicle — so that depreciation/amortization no longer inflates the
residual. See "M5 implementation notes" below for the derivation.

## What's tracked vs. derived

- **Earned Income** is a new flow-event entity. Each event records `date`, `amount`, `currency`,
  `category` (closed enum: Salary / BusinessIncome / RentalIncome / Gift / TaxRefund /
  InsurancePayout / Other), `description` (free-text sub-label), and `ownership` (`SoleOwner` or
  `Joint`). The description field allows fine-grained labelling within a category (e.g., base salary
  vs. per-diem under `Salary`) for drill-downs without polluting top-line categories.
- **Investment Return** per instrument per month is computed as `ΔSnapshot_value +
  cash_paid_out_to_bank − cash_paid_in_from_bank`. Cash paid out covers Sell proceeds, Coupons,
  Dividends, Distributions, and Maturities; cash paid in covers Buys. Fees are absorbed into the
  snapshot delta (since they reduce quantity, not require external cash). The formula gives the full
  economic return — realised + unrealised + yield − fees — for the period.
- **Living Expenses** are derived as `Earned Income + Investment Return − ΔNet Worth`. The user sees
  one number per month, which captures everything not explained by income or investment activity:
  living costs, taxes, interest on debt, charitable giving, etc.

## Decoupling from bank balances

Per ADR-0003's principle, Income events do not auto-update bank-account snapshots. The bank
statement is the source of truth for cash balances; Income events feed the income-statement view
only. Cross-validation between the two happens via the identity above, not via direct linkage.

This means the materialized monthly report (ADR-0006) gains additional columns — earned income
total, investment return total, derived living expenses — alongside the existing net worth and
breakdowns. The staleness check (ADR-0006) gains the `Income` table as an input: an Income write
bumps `updated_at`, which invalidates the affected month's report.

## Considered alternatives

- **Track expenses as events too.** Rejected — re-introduces transaction-level cashflow tracking,
  which ADR-0001 explicitly rules out. The residual approach gives a single useful number per month
  with zero data-entry burden.
- **Don't track Income; infer it from snapshot deltas + transactions.** Rejected — there's no way to
  distinguish salary from a market rally without an explicit signal. The "unexplained NW increase"
  residual is too noisy to be useful as a cashflow proxy.
- **Auto-link Income to a specific bank account.** Rejected — same drift-risk reasoning as ADR-0003.
  Reconciliation between manually-entered income and bank statements is exactly the bug class we're
  avoiding.
- **User-defined income categories.** Deferred — the closed enum keeps reports comparable and
  prevents category sprawl. Migration to user-defined later is non-breaking.

## M5 implementation notes — return formula made precise

Per-instrument per-month: `Return(i, M) = [value(i, M) − value(i, M−1)] + Σ cash_out(M) − Σ
cash_in(M)`, where `value` is the carry-forward snapshot value converted to reporting currency
(CONTEXT → Net Worth), and transaction cash flows bucket by `transaction_date`'s year-month. Sum
per-instrument returns into the five `investment_return_*` subtype columns (ADR-0012) and the total.

Transaction → cash-flow mapping (columns per migration 00010):

| Type | cash_in (bank→instrument) | cash_out (instrument→bank) |
|---|---|---|
| Buy | `amount` | — |
| Sell | — | `amount` |
| Coupon / Dividend / Distribution | — | `amount` |
| Fee | `amount` **only if `quantity IS NULL`** | — |
| Maturity | — (rolled portions re-enter the *successor* TD — see below) | `principal_amount + interest_amount` (**full terminal value, regardless of disposition**) |

- **Unit-deducting fees are NOT a cash flow** (`quantity` set): the deducted units lower the next
  snapshot, so the fee is already in `ΔSnapshot`. Only pure cash fees hit `cash_in`. This makes
  "fees absorbed into the snapshot delta" precise for both fee flavors.
- **Maturity books the full terminal value as `cash_out`, and a rollover re-enters the successor as
  `cash_in` (M6 correction).** Both legs run *regardless of disposition*: the matured TD's terminal
  value leaves it (drop-to-0 offset), and for a `rolled_to_new` portion the same amount enters the
  successor TD (linked by `investments.rolled_from_investment_id`) as a `cash_in` at the maturity
  month. The two legs cancel **across** the rollover, leaving only genuine new interest as return.
  *Original (broken) model:* a rolled maturity booked **no** cash flow at all, on the theory that
  snapshots alone capture it (old → 0, the successor's first snapshot carries the rolled amount). That
  holds **only** when the matured TD's closing snapshot equals the full terminal value. Real
  statement snapshots under-accrue the final period's interest, so the close-to-0 drop exceeded the
  (absent) offset and the matured principal read as a **phantom loss** — a real case surfaced a large
  negative investment-return headline for a month when net worth had actually risen. A rolled TD
  therefore also takes **no** synthetic placement `cash_in` (see the birth-month bullet) — its
  funding is the rollover `cash_in` above, not `td_principal`; using `td_principal` would cancel only
  the principal and leave the rolled-in interest as a phantom *gain*. See the pre-alpha changelog
  (`docs/history/CHANGELOG-pre-alpha.md`) "Time-deposit rollover return-continuity fix (M6)".
- **Birth month**: `value(M−1) = 0` when no snapshot ≤ M−1 — correct under the expected workflow
  (buy during the month, snapshot the position at month-end): `ΔSnapshot − cash_in` = unrealised
  gain since purchase. **This depends on placement being a `cash_in` to cancel the `0 → principal`
  jump (issue #27).** Stocks, mutual funds, gold, and secondary-market bonds always recorded a Buy.
  The two that historically did not — `govt_primary` bonds and time deposits — would otherwise read
  their entire deployed principal as return in the placement month (the entry-side twin of the #25
  exit-side bug). Both are now fixed: a primary bond records a **Buy at placement** (ADR-0009), and a
  time deposit's placement `cash_in` is **synthesized by the engine** from
  `time_deposit_details.principal` at `placement_date` (a TD records no Buy). With that `cash_in` the
  placement month nets to `0` and only later yield (coupon / accrued interest) is booked as return.
  **Exception: a rolled TD takes no synthetic placement** — it is funded by its predecessor's
  rollover `cash_in` instead (see the maturity/rollover bullet above), so synthesizing a
  `td_principal` placement on top would double-count.
- **Termination/maturity month — the liquidation-to-0 assumption (made explicit, issue #25).** The
  formula's correctness at the end of a position's life *depends on the close-month snapshot being
  `0`*. A matured/sold position holds nothing at month-end — the principal and any interest/proceeds
  have left it for the bank (recorded as the `cash_out` transaction; decoupled from bank balances
  per ADR-0003). With `value(M) = 0`, `prev ≈ principal`, and `cash_out = principal + return`, the
  formula collapses to `(0 − principal) + (principal + return) = return` — interest/gain only, no
  return *of* capital. This is the same mechanism the "rolled maturity portions" bullet above relies
  on (old position → 0). A non-zero close (e.g. a snapshot of `principal + interest`) leaves
  `cash_out` with nothing to cancel and **double-counts the entire payout as investment return**.
  The truthful `0` close snapshot is written by the repo on maturity and on any manual termination
  (ADR-0009); the engine needs no special case. *(This corrects a regression: issue #17 briefly
  wrote a `principal + interest` close snapshot to fix a display glitch, silently violating this
  assumption until #25 restored it.)*

**Timing noise & its mitigation.** A transaction in a month with no snapshot for that instrument
produces a per-month return that is off (e.g. `−buy_cost` that month, `+value` the next) but
**cumulatively correct** — and the residual `living_expenses` line absorbs the exact opposite noise,
so the comprehensive-income identity always closes. We keep the correct formula rather than
engineering phantom-snapshot alignment. Because the audience is non-technical, the mitigation is a
**UX guardrail, not interpretation**: when an instrument has transactions in a month it wasn't
snapshotted, the dashboard nudges the user to record that snapshot (surfaced alongside
`stale_positions`). The fix is complete data, driven by the UI.

**Asset Value Change — isolating non-cash depreciation/amortization.** Property and vehicle values
are entered as manually-declining snapshots (the per-subtype amortization/depreciation rate is only
a UI helper). Their decline lands in `ΔNW`, and expanding the residual shows it leaking in as
phantom spending:

```
LivingExpenses = EarnedIncome + InvestmentReturn − ΔNW
ΔNW            = ΔBank + ΔProp + ΔVeh + ΔInv + ΔRecv − ΔLiab
⇒ LivingExpenses = EarnedIncome + cash_out − cash_in − ΔBank − ΔProp − ΔVeh − ΔRecv + ΔLiab
```

The `ΔInv` terms cancel (investment marks are already in `InvestmentReturn`), so **property +
vehicle revaluation is the only major non-cash distortion left** — making its isolation a *complete*
fix, not a partial one. We add a line:

```
AssetValueChange     = Σ ΔSnapshot over property + vehicle positions  (signed; usually negative)
LivingExpenses(cash) = EarnedIncome + InvestmentReturn + AssetValueChange − ΔNW
```

Scope is **property + vehicle only** — bank accounts are cash (stay in the residual), investments
have their own line, and receivable write-downs are genuine wealth losses left in the residual
(revisit if material). Display: a non-cash **"Property & vehicle value change"** line in the income
statement / waterfall; the residual is relabeled a cash-spending estimate. Stored on the report row
per ADR-0012.

## Presentation / UX

Per [[adr-0050]] (mobile–web layout divergence doctrine), the Income screen **diverges its mobile
layout** from the web layout: the wide history table (date · category · amount · description ·
ownership · actions) horizontally scrolls on a phone, hiding the **amount** — the value the
non-technical household member [[adr-0025]] centres came for. `IncomeScreen` stays the single source
of truth — it owns the query, the month picker, the routine/incidental filter, the per-currency
headline stats and the pagination — and delegates only the **row list** to one of two renderers,
picked at runtime by `useIsMobile()` (the single 768px boolean); one tree is ever in the DOM. Both
consume the same page of rows and the same per-row projection + handlers (`useIncomeRow` — the
derived labels, the edit/duplicate/delete dialog state, the delete mutation), and expose the **same
`income-*` data-testid`s** (`income-row`, `income-amount`, `regularity-{routine|incidental}`), so the
income assertions run against either renderer unchanged. No logic forks into a renderer.

- **`IncomeRowDesktop`** (≥768px) keeps the original wide table row, wrapped by `IncomeList` in the
  `Table` + header + paginated `Card`, with the **amount column right-aligned** (`tabular-nums`) as a
  numeric column.
- **`IncomeCard`** (<768px) applies the doctrine's **"wide table → stacked cards"** transform: one
  card per row with the **amount promoted to the card headline** (`tabular-nums`, the largest element,
  readable with no horizontal scroll) on the top line, the category chip + regularity icon clustered
  to its **right, next to the actions button**, and the remaining fields (date, description when
  present, ownership) stacked below as label→value pairs across the **full card width** so their
  **right-aligned values** sit against the same `p-4` edge as the actions button. The ⋮ actions
  trigger is a pronounced **outline** button (not bare ghost dots) sized to the a11y floor
  (`size-11`, ≥44px), so the action reads as a real menu button; focus order follows visual order.

The `IncomeScreen` header and the per-currency **summary panel** stay single-layout and reflow by
plain CSS / a runtime placement pick per the doctrine's cosmetic path (no renderer split):

- The primary **create action** lives top-right on desktop; on mobile it moves onto the filter
  toolbar as a compact **"New"** (`+` icon + primary fill, right-aligned), reclaiming the vertical
  space under the page copy without reading as another filter pill.
- The **summary panel** is styled **distinct from the row cards** with a **primary tint + primary
  border** (`bg-primary/5 dark:bg-primary/10`, `border-primary/30`) — a muted-grey fill washed out
  against the near-identical card/background token and vanished in dark mode, so the panel keys off the
  brand colour to stay discernible in both themes — so it reads as a header, not another income entry.
  Its **regularity / by-person / by-category** breakdowns are peer blocks listed **vertically** (label
  → right-aligned amount) rather than as a dense wrapped one-liner, and sit **side-by-side on desktop**
  (md+, up to three columns — by-person appears only when the household has more than one earner)
  separated by a **muted vertical divider** (`md:divide-x`), stacking on phones.

The `IncomeList` renderer is the split point and owns pagination placement in both variants. One
`@smoke` Playwright spec (`income-mobile.spec.ts`) asserts the correct renderer mounts at mobile vs
desktop width and the amount stays reachable; the desktop write-flow round-trip stays in
`income.spec.ts`, and renderer conformance in `IncomeList.test.tsx`.

## Consequences

- Schema gains one new table (`income`) — flow events with no FK to Positions.
- The materialized monthly report (ADR-0006) extends to carry an income-statement view; the report's
  staleness check must include the Income table.
- "Where did my money go?" is answerable in aggregate (the residual) but not in detail (no expense
  items). This is a deliberate scope choice consistent with the snapshot philosophy.
- Recurring income (monthly salary) is handled at the frontend by a "duplicate last month" helper
  rather than a backend recurrence engine. Adding backend recurrence later is additive.
