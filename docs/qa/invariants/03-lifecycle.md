# Zone: LIFECYCLE

The write-side twin of FINANCE: the position state machine of ADR-0009, whose
guarantees the report engine **assumes** on read. INV-FINANCE-11/-12/-13 only
hold if the repo actually writes them on mutation — a terminated position must
carry a truthful 0-value close snapshot, a maturity must flip status, a rollover
must link its successor. A break here corrupts the derived return silently, the
same failure mode as the FINANCE zone but introduced at the mutation rather than
the calculation. Write-side code lives in `internal/repo/lifecycle.go` and the
maturity path of `internal/repo/investment_transactions.go`.

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-LIFECYCLE-01 | Lifecycle status is validated before the DB: group-defined enum + the status/terminated_at biconditional (active ⟺ no date; any terminal status ⟺ a date); violations are 400 | ADR-0009 | Critical |
| INV-LIFECYCLE-02 | A Maturity transaction flips the position to `matured` and sets terminated_at automatically | ADR-0009 | Critical |
| INV-LIFECYCLE-03 | An investment terminal flip writes a truthful 0-value close snapshot at the termination month (the INV-FINANCE-11/-13 read-side assumption) | ADR-0009, ADR-0008 | Critical |
| INV-LIFECYCLE-04 | Reactivating a terminated investment (back to active) drops that close snapshot, so it carries its last real value, not 0 | ADR-0009 | Critical |
| INV-LIFECYCLE-05 | Editing a Maturity transaction's date re-syncs terminated_at and relocates the close snapshot, leaving exactly one | ADR-0009 | High |
| INV-LIFECYCLE-06 | No further transaction is accepted on a terminal (matured) position — rejected with 409 | ADR-0009 | Critical |
| INV-LIFECYCLE-07 | Rollover successor linkage: linking sets `rolled_from_investment_id` / the source resolves `rolled_to`; self-link and unknown source are rejected (the INV-FINANCE-12 read-side assumption) | ADR-0009 | High |
