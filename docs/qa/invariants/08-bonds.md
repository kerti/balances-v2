# Zone: BONDS

The bond / time-deposit valuation rules that feed FINANCE but are neither pure
cost basis (COST-BASIS) nor a raw snapshot (SNAPSHOTS). A bond's defining
quantity — its **outstanding nominal** — is not stored; it is derived from the
transaction ledger, so multi-tranche top-ups and partial sells scale it
correctly without a drift-prone `face_value` column (#27, ADR-0003). The
time-deposit term is the other guard: a fixed forward window that snapshots and
the terminal maturity must stay inside (#62). A wrong face silently misstates a
bond's value; a snapshot outside its term silently misstates a deposit's history.

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-BONDS-01 | Outstanding nominal round-trips through the ledger: a govt-primary bond created from `face_value` F seeds a placement Buy at par (quantity = F ÷ 1,000,000, price_per_unit = 1,000,000) and its outstanding face derives back to F via `outstandingFaceFromLedger` = (Σ buy_qty − Σ sell_qty) × 1,000,000 — no stored scalar, multi-tranche and partial-sell aware by construction | ADR-0003 | High |
| INV-BONDS-02 | A bond/time-deposit snapshot uses the accrued-interest shape — `amount` (total value, incl. accrued) plus `accrued_interest`, and not the quantity/price shape — per ADR-0022's value-column XOR | ADR-0022 | High |
| INV-BONDS-03 | A time deposit's term is a non-empty forward window (maturity strictly after placement, else `ErrInvalidDepositTerm`/the migration CHECK), and it is enforced both ways: a snapshot's month must fall within placement..maturity (inclusive), the terminal Maturity transaction within placement..maturity to the day, and a term edit cannot be narrowed so it strands an existing snapshot or transaction outside the new window | ADR-0003 | High |
| INV-BONDS-04 | A bond's `coupon_disposition` is a per-bond enum (`pays_out` \| `accrues`, #66) that round-trips through create/get/update; an omitted or empty value is backfilled to the `pays_out` default (the historical behaviour) rather than rejected, while a non-empty value outside the enum is refused; the column survives backup export→restore, and a pre-column (`format_version` 1) backup restores as `pays_out` via the `transforms[1]` backfill | #66, ADR-0036 | Medium |
