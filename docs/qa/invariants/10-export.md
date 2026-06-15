# Zone: EXPORT

The per-position export workbook: the `Export<Position>` repo path that feeds the
downloadable Detail sheet. Each of the six position types has one —
`ExportBankAccount` / `ExportProperty` / `ExportVehicle` (`AssetRepo`),
`ExportLiability` (`LiabilityRepo`), `ExportReceivable` (`ReceivableRepo`), and
the five investment subtypes off `InvestmentRepo`'s shared `exportCommon`
(`investment_export.go`). Each gathers the aggregate, **resolves the two
id-typed fields to human-facing values** (`OwnerEmail`, `TagName`), and lists the
full snapshot history (plus the transaction ledger, for investments). The
defining risk is a **privacy leak in the resolution step**, which is why this is
its own zone rather than a corner of TENANCY: a joint position must export with
an empty `OwnerEmail` (no member identity attached to a shared holding), and the
owner/tag/snapshot lookups must stay household-scoped. Every export funnels
through the same `Get<Position>` ownership check it shares with the read path, so
a wrong-subtype or cross-household id is 404/`ErrNotFound` long before any
resolve runs — never a silent join against another household's user or tag. Code:
`internal/repo/{bank_accounts,properties,vehicles,liabilities,receivables}.go`,
`internal/repo/investment_export.go`; the HTTP layer lives in each group's
`export.go` handler.

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-EXPORT-01 | Export is household-scoped and subtype-checked: every `Export<Position>` funnels through the same `Get<Position>` ownership gate as the read path, so an unknown id, a cross-household id, or a wrong-subtype id is `ErrNotFound` (404 at the handler) before any owner/tag/snapshot resolution runs — there is no export-only path into another household's data | ADR-0005 | Critical |
| INV-EXPORT-02 | `OwnerEmail` carries no identity for shared holdings: it resolves the `sole_owner_user_id`'s email for a personal position and is **`""` for a joint one** (the resolve is gated on a non-nil `SoleOwnerUserID`). A joint position exports with a blank owner — no household member is named on a holding the household owns jointly | ADR-0005, ADR-0017 | Critical |
| INV-EXPORT-03 | `TagName` resolves the assigned tag's name via the household-scoped `GetTagByID` (`{ID, HouseholdID}`) and is **`""` when the position is untagged** (the resolve is gated on a non-nil `TagID`). The tag lookup is household-keyed, so it can never surface another household's tag name | ADR-0028 | High |
| INV-EXPORT-04 | The exported history is the position's own full snapshot list (and, for investments, its full transaction ledger), each gathered household-scoped via the `…ForInvestment`/`…ForAsset`/`…ForLiability`/`…ForReceivable` query keyed by `HouseholdID`. An untagged joint position with no history exports zero snapshots; the list round-trips back through the importer unchanged | ADR-0006, ADR-0023 | High |
