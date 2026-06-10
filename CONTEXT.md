# Balances v2

A personal-finance app that tracks **net worth** via end-of-month balance snapshots across all of a
Household's positions. Investment instruments additionally keep a transaction ledger for cost-basis
and income reporting; snapshot tables remain the source of truth for net worth.
Transaction-by-transaction cash-flow tracking (Mint / YNAB style) is a deliberate non-feature.

## Language

### Top-level groups

**Asset**: A position with positive value that is not an investment instrument — bank account,
property, vehicle. _Avoid_: holding, account (reserved for the auth user identity).

**Liability**: A debt owed by the Household. Either **personal** (informal — owed to family,
friends) or **institutional** (formal — mortgage, bank loan, outstanding credit-card balance).

**Receivable**: Money owed *to* the Household by another party.

**Investment**: A position in a tradable or fixed-income instrument. Subtypes below.

### Investment subtypes

**Stock**: A position in an individual equity, tracked per ticker.

**MutualFund**: A position in a single mutual fund, tracked per fund.

**Bond**: A fixed-income instrument with face value, coupon rate, and maturity. Each Bond is either
**GovtPrimary** (issued directly by government, typically held to maturity) or **SecondaryMarket**
(purchased on the secondary market, may trade before maturity). Coupon frequency is one of `monthly
| quarterly | semi_annual | annual` — Indonesian retail (ORI/SBR/SR/ST) pays monthly; tradeable govt
FR series and most corporates pay semi-annually. For floating-rate instruments (SBR, ST) the stored
coupon rate is the *current* rate; the user edits it on each reset. **Coupon disposition** varies:
Indonesian govt-primary retail coupons pay out *directly to the user's bank account* each period
(tracked as a Coupon Transaction; the bond's accrued component at any snapshot is structurally 0).
Secondary-market and corporate bonds typically *accrue* between coupon dates (accrued > 0 between
coupons, resets at each coupon). The Snapshot model accommodates both: accrued is a breakdown column,
0 is valid. _Avoid_: Obligation, Obligasi.

**Gold**: A position in physical or paper gold, tracked by quantity (typically grams).

**TimeDeposit**: A locked principal at a bank with a fixed interest rate and maturity date.
*Indonesian: deposito.* _Avoid_: Deposit (ambiguous with cash held in a bank account).

### Snapshots and transactions

**Snapshot**: A monthly observation of a position's value. Asset / Liability / Receivable snapshots
record an amount in a currency. Stock / MutualFund / Gold snapshots record quantity, market price
per unit, and total value (= quantity × price). Bond / TimeDeposit snapshots record total value and
accrued interest, where **total value is dirty** — it already includes the accrued interest.
`accrued_interest` is carried as a breakdown column for income-tracking visibility, not as an
additive component. Net-worth aggregation sums `total value` uniformly across all snapshot tables.
All snapshot values are entered manually from statements; the system does not auto-compute interest
or market value. A Snapshot belongs to exactly one month: its `year_month` is immutable after
creation (to move a reading to another month, delete it and record a new one), and its optional
`as_of_date` (the statement date) must fall **within that same calendar month** — enforced by a DB
CHECK on every snapshot table and by bounded date inputs in the UI.

**Transaction**: An event in an Investment instrument's ledger. Types: **Buy**, **Sell**,
**Coupon**, **Dividend**, **Distribution**, **Fee**, **Maturity**.

- **Buy / Sell** — a trade; quantity changes hands, with a price per unit and a cash impact.
- **Coupon / Dividend / Distribution** — periodic income payments. The cash impact is *not*
  propagated to any bank-account snapshot; the user reads the resulting cash off their bank
  statement at the next month-end.
- **Fee** — a manager-imposed charge against the instrument. Records `fee_cash_amount`, `currency`,
  optional `fee_quantity_deducted` (for unit-settled instruments, e.g. gold or some mutual-fund
  classes), and optional `price_per_unit` for the conversion. NAV-embedded fees (typical for mutual
  funds) are not recorded — already reflected in the price snapshot.
- **Maturity** — an instrument's terminal event (Bond redemption, TimeDeposit completion). Carries
  `principal_amount`, `interest_amount`, and a disposition for each (`rolled_to_new` or `cash_out`),
  expressing whether the piece was reinvested or paid out. TimeDeposit auto-rollover bank policies:
  principal + interest both rolled (`auto_renew_with_interest`), principal rolled with interest paid
  out (`auto_renew_principal`), or both paid out (`no_rollover`). `cash_out` portions do not
  auto-update bank balances (ADR-0003) — they appear in the next bank statement. When principal or
  interest is rolled, a fresh TimeDeposit Position is created with its `principal` set to the
  rolled-over amount.

### Position lifecycle

Every Position carries a **status** (default `active`) and an optional `terminated_at` date marking
when it left the Household's portfolio, plus an optional free-text `termination_note`. Status values
vary per group:

- **Asset**: `active`, `closed` (bank account), `sold` (vehicle, property), `disposed`
- **Liability**: `active`, `paid_off`, `forgiven`, `written_off`
- **Receivable**: `active`, `collected`, `written_off`
- **Investment**: `active`, `sold` (fully exited), `matured` (Bond, TimeDeposit)

A non-active Position contributes to net worth only for months ending on or before `terminated_at`.
Its historical Snapshots and Transactions remain intact and queryable; only its forward contribution
is suppressed. Snapshot carry-forward does not extend a Position past its `terminated_at`.

Reactivation (a closed bank account reopened, a sold property bought back) creates a new Position
row rather than flipping `terminated_at` back to NULL — keeping termination periods unambiguous.
This applies uniformly, including to TimeDeposit auto-rollovers: each rollover creates a fresh
TimeDeposit Position with a new `placement_date`.

### Tags

**Tag**: A household-defined label attached to a Position to group it on a breakdown report. Each
Position carries **at most one** Tag (default none → an **Untagged** bucket). A Tag is free-form —
the Household names it and picks its colour from a fixed swatch palette — and carries no built-in
financial meaning: "by bank", "by goal", "by risk bucket" are all Tag values the Household chooses,
not a fixed taxonomy. Tags are orthogonal to every domain field (a Tag is *not* the bank-account
`bank_name`) and assert nothing about where a Position's value is held. **Income is not taggable** —
it is a flow event, not a Position. The Tag-breakdown report sums each Tag's Positions by
most-recent-snapshot value (the net-worth carry-forward rule), per currency with no FX conversion,
showing Liabilities as their own negative slice plus the Untagged bucket so proportions stay honest.
_Avoid_: Group (reserved for the four top-level groups), Category (reserved for Income categories),
Label.

### Identity and ownership

**Household**: The unit of access and aggregation — the people sharing economic life and tracking
net worth together. Every Position, Snapshot, Transaction, and Income event belongs to exactly one
Household. A Household carries a `display_name` and a `reporting_currency` (the currency net-worth
aggregates are computed in). _Avoid_: Family, Team, Tenant.

**User**: An individual member of a Household. A User belongs to exactly one Household (v1;
multi-household membership is deferred). All Users in a Household have full read/write access to all
its data. Users carry a `display_name`, `email`, `locale` (UI language, default `id-ID`), and
`time_zone` ("current month" interpretation, default `Asia/Jakarta`).

**Ownership** (Position attribute): Each Position carries an Ownership mode:

- **SoleOwner** — attributed to a specific User for net-worth-breakdown purposes (e.g., "her
  brokerage account").
- **Joint** — owned by the Household as a whole, no per-user attribution (e.g., a joint bank
  account).

Ownership reflects the Household's *intent* for attribution in reports — not the legal deed or
account-holder name. A property whose deed lists one spouse may still be tracked as Joint if the
Household considers it shared.

### Currency and net worth

**Native currency**: The currency a position is denominated in. Stored on every monetary value.

**Reporting currency**: The Household's chosen currency for net-worth aggregation (default IDR).
Non-native amounts are converted using a monthly FX rate, entered manually in v1.

**FX rate**: A per-`(Household, year_month, foreign currency)` rate giving units of reporting
currency per 1 unit of the foreign currency. By convention it is the **month-end** rate for that
`year_month` (matching the month-end Snapshot model); there is no exact as-of-date. The reporting
currency itself needs no rate (implicitly 1). For month `M`, a foreign currency converts at its most
recent rate with `year_month ≤ M` (carry-forward, mirroring Snapshots). A foreign currency held in
`M` with **no** rate at or before `M` cannot be converted: those Positions are excluded from the
converted totals and surfaced as a **missing-FX** warning (distinct from stale positions) rather
than silently treated as 1:1. An external feed (Frankfurter/ECB) is a deferred swap-in (ADR-0002).

**Multi-currency reporting** (Household setting): A toggle (`multi_currency_enabled`, default
**off**). When **off**, currency is pinned to the reporting currency everywhere — no currency picker
on monetary inputs, no FX-rate entry, no missing-FX path — and net worth sums native amounts
directly. When **on**, currency pickers, FX-rate entry, and conversion activate. The toggle gates UI
exposure and whether conversion runs; it does **not** change storage — every monetary value still
carries its `currency` (ADR-0002). Entering a foreign-currency Position requires the toggle on;
turning it off again is blocked while foreign-currency Positions exist.

**Net Worth**: For a given Household and month, `Σ(Asset, Receivable, Investment snapshots) −
Σ(Liability snapshots)`, all converted to the reporting currency. Reported per-Household (the total)
or broken down by User via each Position's Ownership — SoleOwner Positions contribute fully to their
User's column; Joint Positions go in a dedicated **Joint** column rather than split across members
(a split would imply a per-member division the Household hasn't asserted). The total is the sum of
every User column plus the Joint column.

For month `M`, each Position contributes its **most recent Snapshot with `year_month ≤ M`** (provided
the Position is active, or terminated after `M`). This single rule covers the in-progress month,
mid-history gaps, and never-yet-snapshotted Positions uniformly: no Snapshot at or before `M` → the
Position contributes nothing to `M`. Carry-forward is **unbounded** — a Position with no newer
Snapshot keeps contributing its last known value until a newer Snapshot or its `terminated_at`.
Whenever a Position's contributing Snapshot predates `M` (`year_month < M`), the Position is listed
in that month's **stale positions** as a data-entry nudge — not an error, does not suppress the
contribution.

### Income and cashflow

**Income**: A flow event recording earned cash entering the Household from outside — salary,
business income, rental income, gifts, refunds, payouts. Distinct from Investment cash events
(Coupon / Dividend / Distribution / Maturity), which are returns from positions already held.

Each Income event has: `date`, `amount`, `currency`, `category` (closed enum), `description` (free
text, optional), and `ownership` (`SoleOwner` or `Joint`). Categories: **Salary**,
**BusinessIncome**, **RentalIncome**, **Gift**, **TaxRefund**, **InsurancePayout**, **Other**.
`description` provides sub-categorisation within a category — e.g., base monthly pay and travel
per-diem are both `Salary`, distinguished by description in drill-downs but rolled up at the category
level for top-line reporting.

Like Investment cash events (ADR-0003), Income events do **not** auto-update bank-account snapshots.
The cash arrives via the next bank statement; the Income event supports the income-statement view and
cashflow approximation, not net worth (which the snapshot tables already capture).

**Comprehensive income identity**: `ΔNet Worth = Earned Income + Investment Return + Asset Value
Change − Living Expenses`. Earned Income is tracked explicitly. Investment Return is derived
per-instrument per-month as `ΔSnapshot_value + cash_paid_out_to_bank − cash_paid_in_from_bank`
(covering unrealised price movement, realised gains from Sells, and yield from Coupons / Dividends /
Distributions / Maturities; net of Fees). **Asset Value Change** is the non-cash mark change of
non-financial Assets — the signed sum of `ΔSnapshot` over property and vehicle Positions
(depreciation, leasehold amortization, or revaluation up); it moves net worth without any cash
changing hands. Living Expenses are the residual — itemised expense tracking is a non-feature
(ADR-0001). Because Investment Return already absorbs investment mark changes (they cancel in the
residual) and Asset Value Change pulls out property/vehicle marks, the residual is a genuine
**cash-spending** proxy, not a catch-all conflating non-cash depreciation with spending.

## Relationships

- A **Household** has 1..N **Users**; a **User** belongs to exactly one **Household**.
- Every **Asset / Liability / Receivable / Investment** belongs to exactly one **Household** and
  carries an **Ownership** mode.
- Each has zero or more **Snapshots** — typically one per month.
- An **Investment** additionally has zero or more **Transactions** — independent of its Snapshots.
- Each optionally carries **one Tag** (a household-defined grouping label); a Tag groups many
  Positions. **Income** is not taggable.
- A **TimeDeposit** auto-renewal terminates the old instrument and creates a new one; the chain is
  implicit, not modelled as a parent-child link.
- Every **Income** event belongs to exactly one **Household** and carries an **Ownership** mode;
  Income events stand alone — they don't link to any Position or Snapshot.

## Example dialogue

> **Q:** "Our house is registered in my name only — should I record it as SoleOwner?"
> **A:** Use whatever the Household *intends* for net-worth attribution. If both spouses consider the
> home shared, mark it Joint regardless of the deed. Ownership captures intent, not legal title.

> **Q:** "I get a base salary each month plus per-diem when I travel — both from the same employer."
> **A:** Two separate Income events, both category `Salary`, distinguished by description ("Base
> salary", "Per diem — Jakarta trip"). They roll up under the Salary line; descriptions surface in
> drill-downs.

> **Q:** "How do I know what I spent last month if I don't track expenses?"
> **A:** Living expenses are derived: `Earned Income + Investment Return − ΔNet Worth`. The app shows
> one residual number — itemised spending categories are not tracked.

## Flagged ambiguities

- **"Account"** meant both *bank account* (an Asset) and *user account* (auth identity). Resolved:
  "bank account" / "Asset" for the financial concept; "User" for the auth identity.
- **"Obligation"** was used for **Bond**. Resolved: Bond is canonical; Obligation / Obligasi are
  aliases to avoid in code and UI.
- **"Deposit"** was ambiguous between cash in a bank account and a fixed-term locked product.
  Resolved: TimeDeposit for the locked product; cash savings live under Asset → bank account.
- **"Holding" / "Position"** as an umbrella across all four groups was considered and rejected — the
  four groups are treated independently because their data shapes and lifecycles differ.
- **"Ownership"** means the Household's *intent* for attributing a Position in breakdowns, not the
  legal deed or registered account holder.
