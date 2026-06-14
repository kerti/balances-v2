# Zone: COST-BASIS

The investment transaction ledger that INV-FINANCE-08's return math sits on top
of. Where SNAPSHOTS guards the dated **value** record, COST-BASIS guards the
**ledger replay**: the avg-cost convention of ADR-0023, walked over an
investment's transactions in `transaction_date` order to the "money still in"
basis. The defining risk of this zone is **dual-implementation drift** — the
same convention is computed twice, by `costBasisFromLedger` / `costSeriesAtMonths`
in `backend/internal/repo/cost_basis.go` (list + time-series endpoints) and by
`computeCostBasis` / `costBasisSeries` in `frontend/src/lib/costBasis.ts`
(the screens) — so the list and detail figures must agree to the cent. The
matrix encodes that parity directly: INV-COST-BASIS-01..03 are each annotated in
**both** the Go and the vitest suite, so a one-sided change shows up as a
half-covered invariant. A wrong basis silently misstates realised/unrealised
gain — the invisible-until-reconciled failure mode of FINANCE.

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-COST-BASIS-01 | Avg-cost replay rules: buy adds cost+qty; sell reduces cost proportionally (`cost·sellQty/qty`) with `sellQty` clamped to qty held (no negative basis); a cash fee capitalises into cost; coupon/dividend/distribution/maturity never adjust cost; a buy missing amount or quantity is skipped defensively | ADR-0023 | Critical |
| INV-COST-BASIS-02 | Replay is driven by transaction date, not input order: the basis is independent of inter-buy ordering, and a sell always applies after the buys that precede it by date (the Go path relies on the ascending query; the TS path sorts by date) | ADR-0023 | High |
| INV-COST-BASIS-03 | The per-month cost series samples cumulative cost at each snapshot month — carry-forward between transactions, a transaction after the last sampled month is excluded, multiple transactions in one month all apply, and a flat (principal-only) position emits a constant series | ADR-0023 | High |
| INV-COST-BASIS-04 | A transaction row is well-shaped before it can land: the value-column combo must match `transaction_type` (buy/sell ⇒ amount+quantity+price, no maturity cols; income ⇒ amount only; fee ⇒ amount, quantity/price paired; maturity ⇒ principal+interest+both dispositions, no trade cols) and the subtype→type matrix must allow it (e.g. time deposit only Maturity, coupon only on bond); violations are 400. Keeps the ledger rows clean so the replay's defensive skips never fire on live data | ADR-0023 | High |
