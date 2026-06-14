# Zone: TENANCY

ADR-0005's threat model: every per-household repository must filter on
`household_id`; a request authenticated as one household must see **zero** rows
of another. One forgotten `WHERE` is a cross-tenant finance leak. Each row below
is the isolation guarantee for one resource. Severity: **Critical**.

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-TENANCY-01 | Bank-account/asset reads & mutations never cross households | ADR-0005 | Critical |
| INV-TENANCY-02 | Property reads & mutations never cross households | ADR-0005 | Critical |
| INV-TENANCY-03 | Vehicle reads & mutations never cross households | ADR-0005 | Critical |
| INV-TENANCY-04 | Liability reads & mutations never cross households | ADR-0005 | Critical |
| INV-TENANCY-05 | Receivable reads & mutations never cross households | ADR-0005 | Critical |
| INV-TENANCY-06 | Investment reads & mutations never cross households | ADR-0005 | Critical |
| INV-TENANCY-07 | Investment-transaction reads & mutations never cross households | ADR-0005 | Critical |
| INV-TENANCY-08 | Position-lifecycle mutations never cross households | ADR-0005, ADR-0009 | Critical |
| INV-TENANCY-09 | Monthly-report reads never expose another household's positions | ADR-0005 | Critical |
| INV-TENANCY-10 | Income reads & mutations never cross households | ADR-0005 | Critical |
| INV-TENANCY-11 | FX-rate reads & mutations never cross households | ADR-0005 | Critical |
| INV-TENANCY-12 | Tag reads, assignment & breakdown never cross households | ADR-0005, ADR-0028 | Critical |
