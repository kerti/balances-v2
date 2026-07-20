# Zone: FINANCE

The calculation correctness that *is* the product (ADR-0021's second heavy
zone). The materialized monthly report (ADR-0006) derives net worth, the
comprehensive-income statement (ADR-0008), and the multi-currency conversion
(ADR-0002) from snapshots + transactions. A wrong number here silently misstates
a household's wealth — the failure is invisible until someone reconciles by hand.
The compute core is the pure, DB-free engine `internal/repo/monthly_reports_engine.go`;
its rules are unit-tested without a container.

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-FINANCE-01 | Net worth = Assets + Receivables + Investments − Liabilities (liabilities subtract) | ADR-0001 | Critical |
| INV-FINANCE-02 | Per-user/Joint net-worth attribution reconciles with the total | ADR-0004, ADR-0012 | High |
| INV-FINANCE-03 | A month with no fresh snapshot carries the latest snapshot ≤ M and is flagged stale | ADR-0006 | High |
| INV-FINANCE-04 | A position contributes nothing before its first snapshot; the series starts at the first month with data | ADR-0006 | High |
| INV-FINANCE-05 | A terminated position contributes through its termination month, then drops out | ADR-0009 | Critical |
| INV-FINANCE-06 | Comprehensive-income identity closes: ΔNW = EarnedIncome + InvestmentReturn + AssetValueChange − LivingExpenses | ADR-0008 | Critical |
| INV-FINANCE-07 | The first reportable month suppresses the derived income-statement lines (return, asset-value-change, living-expenses NULL) | ADR-0006, ADR-0008 | High |
| INV-FINANCE-08 | Investment return per instrument per month = ΔSnapshot + cash_out − cash_in | ADR-0008 | Critical |
| INV-FINANCE-09 | Transaction→cash-flow mapping: buy=in; sell/coupon/dividend/distribution=out; cash fee=in; unit-deducting fee=none; maturity=full terminal value out | ADR-0008 | Critical |
| INV-FINANCE-10 | Only property + vehicle revaluation lands in asset-value-change; bank cash and investment marks stay out of it | ADR-0008 | High |
| INV-FINANCE-11 | A liquidation (maturity/sale) books gain only — terminal-value cash_out offsets the truthful 0-value close, leaving no net-worth bubble | ADR-0008, ADR-0009 | Critical |
| INV-FINANCE-12 | A rolled time deposit's terminal-value cash_out is offset by the successor's cash_in; combined return is interest only, no phantom loss/gain even when the close snapshot under-accrues | ADR-0008, ADR-0009 | Critical |
| INV-FINANCE-13 | Deployed capital nets to zero in the placement month (TD synthetic placement cash_in; bond Buy, incl. multi-tranche) | ADR-0008, ADR-0009 | Critical |
| INV-FINANCE-14 | Every earned-income category and investment-return subtype accumulates to its own bucket and sums to the total | ADR-0012 | High |
| INV-FINANCE-15 | A foreign amount is converted at the latest rate ≤ M (carry-forward) and the rate is recorded in fx_rates_used | ADR-0002 | Critical |
| INV-FINANCE-16 | A foreign currency with no rate ≤ M is excluded from net worth and flagged in missing_fx — never summed 1:1 | ADR-0002 | Critical |
| INV-FINANCE-17 | With multi-currency off, amounts sum at face value — no conversion, missing_fx, or fx_rates_used | ADR-0002 | High |
| INV-FINANCE-18 | Itemized per-position detail (`GET /api/reports/{yearMonth}/positions`) sums, per group, to the same `nw_assets`/`nw_liabilities`/`nw_investments`/`nw_receivables` the aggregate report shows for that month — same carry-forward/FX/termination rules, extracted not reimplemented | ADR-0006, ADR-0045 | Critical |
| INV-FINANCE-19 | The four statistics-panel ratios compute from trailing-12 flow averages + reported-month stocks: Cash-Flow = (Income−LivingExpenses)/Income, Passive-Income = TotalPassive/LivingExpenses, Instant-Liquidity = bank cash/investments; every income term is **routine** income only (regularity-filtered, per INV-FINANCE-24); LivingExpenses stays all-income; all render-time derived from materialized routine subtotals | ADR-0048 | High |
| INV-FINANCE-20 | A statistics ratio with unavailable inputs is undefined (rendered "—"): no flow month (baseline/<1 month), Income ≤ 0, LivingExpenses ≤ 0, or investments = 0; Instant-Liquidity is a stock and stays defined on the baseline when investments > 0 | ADR-0048 | High |
| INV-FINANCE-21 | Fund Resilience projects the investment pool to depletion (or "indefinite" past the ~100-yr horizon); its draw-offset is **routine** passive *cash* income (Rental + Pension + Interest, regularity=routine) only — Investment Return stays out of the offset (it is the pool's growth g), guarding the double-count | ADR-0048 | Critical |
| INV-FINANCE-22 | Interest (bank/deposit) is passive *cash* income: it folds into the passive-cash scope alongside Rental + Pension (draw-offset + Passive-Income numerator) and, being external cash never present in Investment Return, carries no double-count | ADR-0048 | High |
| INV-FINANCE-23 | A position's first snapshot is an acquisition, not a revaluation/return: its birth month contributes nothing to asset-value-change (property/vehicle) or investment-return — there is no prior value to diff against — so a fixed asset financed by cash + debt books zero living-expenses; only net worth reflects the acquisition | ADR-0008 | Critical |
| INV-FINANCE-24 | Income tagged `incidental` is excluded from every statistics income term (Cash-Flow income, Passive-Income + Fund Resilience passive cash) — the panel judges against income the household relies on; incidental income still counts in full toward net worth, the income statement, and LivingExpenses. Backed by materialized routine subtotals, not a re-read of raw income (native-currency vs FX-converted, per INV-FINANCE-18) | ADR-0048 | High |
| INV-FINANCE-25 | A paid-out bond coupon (`coupon_disposition='pays_out'`) is passive *cash*: the engine materializes it as `passive_coupon_cash`, and buildStats adds it to the passive-cash scope (Passive-Income numerator + Fund Resilience draw-offset) **and** subtracts it from own-return `g` — the two-scope split that guards the double-count. The coupon yield stays inside `investment_return_total` (the domain keeps it there), so the Passive-Income numerator is unchanged by the split; only the resilience `g`/offset partition moves. Accruing coupons (no Coupon Transaction) never enter `passive_coupon_cash` and stay mark-to-market in `g` | ADR-0048 | Critical |
