# Zone: EXPORT

> _Seeded next — the per-position export workbook (the `Export<Position>` repo
> path that feeds the downloadable Detail sheet). Each of the six position types
> has one: `ExportBankAccount` (`bank_accounts.go`), `ExportProperty`
> (`properties.go`), `ExportVehicle` (`vehicles.go`), `ExportLiability`
> (`liabilities.go`), `ExportReceivable` (`receivables.go`), `ExportInvestment`
> (`investment_export.go`). Each gathers the aggregate, **resolves the two
> id-typed fields to human-facing values** (`OwnerEmail`, `TagName`), and lists
> the full snapshot history. The defining risk is a **privacy leak in the
> resolution step**, which is why this is its own zone rather than a corner of
> TENANCY: a joint position must export with an empty `OwnerEmail` (no member
> identity attached to shared holdings), an untagged one an empty `TagName`, and
> the owner/tag/snapshot lookups must stay household-scoped (the export reuses
> the `Get<Position>` ownership check, so a wrong-subtype or cross-household id is
> 404/`ErrNotFound`, never a silent resolve against another household's user or
> tag). Candidate invariants: (1) export is household-scoped + subtype-checked —
> it funnels through the same `Get` path and inherits its 404; (2) `OwnerEmail`
> resolves the `sole_owner_user_id`'s email for a personal position and is `""`
> for a joint one — no owner identity on shared holdings; (3) `TagName` resolves
> the assigned tag's name (household-scoped `GetTagByID`) and is `""` when
> untagged; (4) the snapshot history is the position's full list, household-
> scoped. Annotation targets already exist: `export_test.go` (property, vehicle,
> liability, receivable — each asserts the resolved name and the joint-untagged
> "both empty" case), `bank_account_export_test.go`, `investment_export_test.go`.
> Survey those before writing new tests; most invariants here are likely already
> verified and just need the `// covers:` annotation. Fill this table when
> seeding the zone._
