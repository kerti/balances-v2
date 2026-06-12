# Investment transaction table strategy: single polymorphic table

Investment transactions (Buy, Sell, Coupon, Dividend, Distribution, Fee, Maturity) are stored in
**one table**, `investment_transactions`, with a `transaction_type` enum and nullable per-shape
value columns. A type-driven DB `CHECK` enforces shape integrity (which columns must be populated
for which type); a repository-layer helper enforces the subtype→type compatibility matrix (which
transaction types each Investment subtype is allowed to record).

This deliberately differs from ADR-0022, which splits **snapshots** into four tables (one per
position group). The two design choices are consistent with their respective concerns: snapshots are
owned by distinct groups with substantively different value semantics (asset/ liability/receivable
amount-only; investment quantity+price or accrued interest) and aggregate through a `UNION ALL` for
net-worth roll-up. Transactions live entirely inside the Investment group, with seven types sharing
a small handful of shape combos and being queried *together* (per-instrument chronological ledger,
future per-type aggregates for yield reports). One table fits how the data is read.

## Shape integrity

`investment_transactions` columns and which transaction types populate each:

| Column | Used by |
|---|---|
| `amount` | buy, sell, coupon, dividend, distribution, fee |
| `quantity` | buy, sell (required); fee (optional, paired with `price_per_unit`) |
| `price_per_unit` | buy, sell (required); fee (optional, paired with `quantity`) |
| `principal_amount` | maturity |
| `interest_amount` | maturity |
| `principal_disposition` | maturity |
| `interest_disposition` | maturity |

Plus shared columns (`id`, `investment_id`, `transaction_type`, `transaction_date`, `currency`,
`description`, audit + soft-delete) and the disposition CHECK on `(principal|interest)_disposition
IN ('rolled_to_new', 'cash_out')`.

A single `CASE`-driven CHECK constraint enforces type→shape:

```sql
CHECK (
    CASE
        WHEN transaction_type IN ('buy', 'sell') THEN
            amount IS NOT NULL AND quantity IS NOT NULL AND price_per_unit IS NOT NULL
            AND principal_amount IS NULL AND interest_amount IS NULL
            AND principal_disposition IS NULL AND interest_disposition IS NULL
        WHEN transaction_type IN ('coupon', 'dividend', 'distribution') THEN
            amount IS NOT NULL
            AND quantity IS NULL AND price_per_unit IS NULL
            AND principal_amount IS NULL AND interest_amount IS NULL
            AND principal_disposition IS NULL AND interest_disposition IS NULL
        WHEN transaction_type = 'fee' THEN
            amount IS NOT NULL
            AND ((quantity IS NULL AND price_per_unit IS NULL)
                 OR (quantity IS NOT NULL AND price_per_unit IS NOT NULL))
            AND principal_amount IS NULL AND interest_amount IS NULL
            AND principal_disposition IS NULL AND interest_disposition IS NULL
        WHEN transaction_type = 'maturity' THEN
            principal_amount IS NOT NULL AND interest_amount IS NOT NULL
            AND principal_disposition IS NOT NULL AND interest_disposition IS NOT NULL
            AND amount IS NULL AND quantity IS NULL AND price_per_unit IS NULL
        ELSE FALSE
    END
)
```

The CHECK catches "rows that satisfy no real shape" and "rows that pick the wrong column combo for
their declared type" — the main programming-error class.

## Subtype→type validation lives in the repo

Postgres CHECK can't reference another table, so it can't enforce "Stock → Buy/Sell/Dividend/Fee
only" or "TimeDeposit → Maturity only". `validateInvestmentTransactionType(subtype, txnType)` in the
repo carries the matrix:

| Subtype | Allowed types |
|---|---|
| Stock | Buy, Sell, Dividend, Fee |
| MutualFund | Buy, Sell, Distribution, Fee |
| Bond | Buy, Sell, Coupon, Fee, Maturity |
| Gold | Buy, Sell, Fee |
| TimeDeposit | Maturity |

Mismatches return `repo.ErrInvalidTransactionType`, mapped to HTTP 400. A second helper,
`validateInvestmentTransactionShape`, re-checks the column combo against the declared type with
friendlier error messages than the DB CHECK would surface (the DB error is precise but cryptic for
end users). Mismatches return `repo.ErrInvalidTransactionShape`, also 400.

This two-layer validation pattern mirrors ADR-0022's snapshot strategy where
`validateInvestmentSnapshotShape` handles the subtype→shape mapping that the DB column-level CHECK
can't see.

## transaction_type is immutable

Once a row is created, its `transaction_type` cannot be updated. The update query column list
deliberately omits it. Changing the type would invalidate the shape (a Buy with no quantity, etc.).
The user-facing recovery path is delete-and-recreate; soft-delete preserves the original row in
history per ADR-0007.

## Considered alternatives

- **One table per transaction type** (~7 tables: `buys`, `sells`, `coupons`, etc., each with its own
  column set). Rejected — every per-instrument query becomes a 7-way UNION ALL across tables with
  near-identical shape; sqlc query duplication; cross-type aggregates (income-statement views, the
  "all transactions for this instrument chronologically" detail page) become awkward; the cost of a
  polymorphic CHECK constraint is much lower than the cost of UNION proliferation.

- **One table per shape** (~4 tables: `trade_transactions`, `cash_income_transactions`,
  `fee_transactions`, `maturity_transactions`). Rejected — shape is an implementation detail, not a
  user-facing concept. The user reasons in terms of "record a Coupon" not "record a
  cash-income-shape transaction." The per-shape split would also force two layers of indirection for
  any cross-shape query.

- **Polymorphic transactions table spanning all four position groups** (Asset, Liability,
  Receivable, Investment). Rejected — the other three groups have no concept of a transaction;
  CONTEXT.md is explicit that "Transaction is an event in an Investment instrument's ledger."
  Forcing a polymorphic FK across groups would invite future drift ("could we also log asset
  transactions here?") that ADR-0001's snapshot-based net-worth premise specifically rejects.

- **Core `investment_transactions` table + per-type extension tables** (e.g., `buy_details`,
  `maturity_details`). Rejected — overkill for the small column count per type (Maturity has the
  most at 4 columns beyond the shared set, Buy/Sell have 3). The extension-table pattern works well
  for Investment **subtypes** (per ADR-0009) where each subtype's metadata is rich; for transaction
  types it would multiply table count and JOIN cost without a meaningful schema win.

- **Enforce the subtype→type matrix at the DB layer via a trigger or function-based CHECK.**
  Rejected — Postgres function-based CHECKs with sub-queries are not portable across DB engines (a
  non-goal today, but a soft constraint per ADR-0013's "hosting deferred" posture), trigger-based
  enforcement adds an opaque failure surface, and the repo-layer helper is the same pattern already
  established by `validateInvestmentSnapshotShape` for an identical class of cross-table constraint.

## Consequences

- One table, ~17 columns total, two indexes (`investment_id` partial; `(investment_id,
  transaction_date DESC)` partial). Index growth scales with row count, not type count.
- Adding a new transaction type means: extend the `transaction_type` CHECK enum in a migration, add
  a WHEN branch to the shape CHECK, extend `validateInvestmentTransactionType`'s matrix, extend
  `validateInvestmentTransactionShape`'s switch, and (frontend) decide whether the new type fits an
  existing dialog shape fork or needs a new one. The DB-level changes are mechanical; the frontend
  dialog decision is the real design surface.
- Adding a new Investment subtype means updating the matrix in `validateInvestmentTransactionType`
  to declare its allowed types (alongside the existing subtype CHECK + snapshot-shape mapping from
  ADR-0009 + ADR-0022).
- The Maturity transaction is uniquely terminal — recording one transitions the position to
  `status = 'matured'`. This **is** enforced in the repo: `CreateInvestmentTransaction` flips the
  status, sets `terminated_at` to the maturity date, and upserts a truthful 0-value close snapshot at
  the maturity month, atomically with the insert (issues #25/#17). The post-maturity freeze guard
  (`status != active → ErrPositionNotActive`) then rejects any further transaction. The schema does
  **not** enforce maturity uniqueness at the DB level (no partial unique index on `(investment_id)
  WHERE transaction_type = 'maturity' AND deleted_at IS NULL`); the freeze guard makes a second one
  unreachable through the live API, and the create-from-list seed (issue #90) rejects a second
  maturity row in its preview. The create-from-list import reuses this terminal behavior to restore a
  matured position faithfully (decision (b), #90): it seeds the ledger with the Maturity row last so
  the write-order matches the terminal-event model, producing the matured status + close snapshot.
  Note the seed inserts via the unguarded db query (not the guarded
  `repo.CreateInvestmentTransaction`), so intra-ledger order is not enforced by the freeze guard
  during seeding; the load-bearing ordering is snapshots-before-ledger, so the 0 close overwrites any
  seeded snapshot in the maturity month.
- Per ADR-0003, no transaction type auto-propagates to bank-account snapshots. The user reads cash
  off their bank statement at the next month-end.
- Per-shape frontend dialog forks (`Trade` / `CashIncome` / `Fee` / `Maturity`) mirror the snapshot
  dialog convention from ADR-0022 + HANDOFF. One shared `TransactionRow` switches Edit dialogs by
  `transaction_type` because the backend update endpoint is unified.
