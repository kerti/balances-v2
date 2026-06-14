# Zone: COST-BASIS

> _Seeded next — the investment transaction ledger that INV-FINANCE-08's return
> math sits on top of. Where SNAPSHOTS guards the dated **value** record,
> COST-BASIS guards the **ledger replay**: `costBasisFromLedger` in
> `internal/repo/cost_basis.go` walks an investment's transactions in
> `transaction_date` order to the "money still in" basis (ADR-0023), and it must
> stay byte-for-byte aligned with the frontend `frontend/src/lib/costBasis.ts` so
> the list screen and detail screen never disagree. Candidate invariants: the
> per-type ledger rules (buy adds cost+qty; sell reduces cost proportionally by
> avg-cost with sellQty clamped to qty held; cash fee capitalises into cost;
> coupon/dividend/distribution are income, not a cost adjustment; maturity is
> terminal and ignored); replay is order-sensitive and the batch query already
> sorts ascending; the transaction shape CHECK (migration 00010) rejects a txn
> carrying the wrong null-shape fields for its type so the defensive skip never
> has to fire on live data; and the backend↔frontend parity itself. Severity is
> Critical — a wrong basis silently misstates realised/unrealised gain, the same
> invisible-until-reconciled failure mode as FINANCE. Code: `cost_basis.go`,
> `investment_transactions.go`, ADR-0023's shape CHECK; annotation targets:
> `cost_basis_test.go` and the investment-transaction tests in `internal/repo/`,
> plus any `costBasis.ts` vitest on the frontend (a real cross-language `covers:`
> opportunity per how-it-works.md). Survey those before writing new tests; fill
> this table when seeding the zone._
</content>
