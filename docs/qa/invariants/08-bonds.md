# Zone: BONDS

> _Seeded next — the bond / time-deposit valuation rules that feed FINANCE but
> are neither pure cost basis (COST-BASIS) nor a raw snapshot (SNAPSHOTS). The
> defining quantity is **outstanding nominal derived from the ledger**, not a
> stored scalar: `outstandingFaceFromLedger` in `internal/repo/cost_basis.go`
> computes (Σ buy_qty − Σ sell_qty) × `bondFaceUnit` (IDR 1,000,000 per unit,
> #27, ADR-0003), so multi-tranche top-ups and partial sells scale the face
> correctly without a drift-prone `face_value` column. Candidate invariants: face
> = ledger-derived nominal (multi-tranche aware); the coupon helper scales off
> that same outstanding face; the accrued-interest snapshot shape (amount = total
> incl. accrued, plus `accrued_interest`) per ADR-0022's value-column XOR; the
> time-deposit term bounds (a maturity date within the deposit's term, #62); and
> the govt-primary 1,000,000-unit convention. Severity High — a wrong face
> silently misstates a bond's value and its coupon. Code: `cost_basis.go`
> (`outstandingFaceFromLedger`, `bondFaceUnit`), `internal/repo/bonds.go`,
> `internal/repo/time_deposits.go`, `time_deposit_term_bounds.go`. Annotation
> targets: `internal/investments/bonds_test.go`, `time_deposit_term_bounds_test.go`,
> and the bond/TD branches of `monthly_reports_engine_test.go`. Survey those
> before writing new tests; fill this table when seeding the zone._
