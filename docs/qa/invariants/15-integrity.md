# Zone: INTEGRITY

The **write-side shape & ownership constraints** that make a malformed row
unrepresentable in the first place — the upstream guarantee every read-side zone
(FINANCE, ATTRIBUTION, LIFECYCLE) silently assumes. The pattern is **two-layer**:
a DB `CHECK` enforces the hard shape, and (where one exists) a repo-layer
validator rejects the same bad input early with a typed error so the handler
returns a 4xx rather than surfacing a raw constraint-violation 500. This zone
catalogues the *guards*, not the calculations they protect. It deliberately does
**not** re-catalogue neighbours that own their own constraint: LIFECYCLE owns the
status↔`terminated_at` biconditional (`<table>_lifecycle_chk`), SNAPSHOTS owns the
`as_of_in_month` pin (migration 00003), BONDS owns the TD term bounds /
maturity-after-placement (migration 00004). INV-ATTRIBUTION-04 is the *read-side
degrade* for a malformed sole row that somehow exists; INV-INTEGRITY-01 is the
*write-side CHECK that stops it existing* — two ends of one risk. Source: baseline
migration `00001_baseline.sql` (the CHECKs), the repo validators in
`investments.go` / `investment_transactions.go`, ADR-0004 (ownership model).

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-INTEGRITY-01 | Ownership biconditional — every owned-entity table (`assets`, `investments`, `liabilities`, `receivables`, `income`) carries `CHECK ((ownership_type='sole') = (sole_owner_user_id IS NOT NULL))`, so the DB rejects **both** malformed halves: a `sole` row with a nil owner *and* a `joint` row that names one. This is the write-side guarantee that makes ATTRIBUTION-04's "malformed sole" an impossible row in normal operation; the enum CHECK additionally bounds `ownership_type` to `{sole, joint}`. DB-enforced only (the repo passes ownership straight through), so a bad write fails the transaction rather than persisting | ADR-0004 / migration 00001 | High |
| INV-INTEGRITY-02 | Snapshot shape is exactly-one — an investment snapshot is either unit-priced (`quantity` + `price_per_unit`, no `accrued_interest`) **or** interest-accruing (`accrued_interest`, no `quantity`/`price_per_unit`), never both and never neither, keyed by the position's subtype. Enforced two-layer: the DB `investment_snapshot_shape` CHECK and the repo `ErrInvalidSnapshotShape` validator that rejects the mismatch before write (and rolls back a create-with-snapshots fan-out so a bad shape leaves nothing behind) | migration 00001 | Critical |
| INV-INTEGRITY-03 | Transaction type→shape — each `transaction_type` (`buy/sell/coupon/dividend/distribution/fee/maturity`) admits only its own value-column combination, and a type must be legal for the position's subtype. Enforced two-layer: the DB `investment_transaction_shape` CHECK and the repo `ValidateSeedTransaction` validator (`ErrInvalidTransactionType` for an off-subtype type, `ErrInvalidTransactionShape` for a wrong column combo), so the engine's per-type cash-flow branches never see an impossible row | migration 00001 | High |
