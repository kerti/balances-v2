# Zone: TAGS

The user-defined position Tag primitive (ADR-0028, #28): a household-scoped
label assignable to any Position (asset, liability, receivable, investment) via a
nullable `tag_id` on each group's shared parent table. Its financial framing
(RDN/custodian/LPS lineage) was deliberately kept out of the ADR — a Tag is a
neutral grouping primitive, orthogonal to a Position's identity. TENANCY already
owns the cross-household isolation case (INV-TENANCY-12, `tags_tenancy_test.go`);
this zone is the **lifecycle + referential** behaviour layered on top. The
defining risk is a *dangling reference*: a Position pointing at a Tag that no
longer exists, or a name collision that splits one logical group into two. Code:
`internal/repo/tags.go` (+ `queries/tags.sql`), `frontend/src/lib/tagBreakdown.ts`.

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-TAGS-01 | A Tag name is unique within a household, **case-insensitively**, on both create and rename — the `tags_household_name_live` partial unique index over `(household_id, lower(name))` raises 23505, surfaced as `ErrTagNameExists` from `CreateTag` and `UpdateTag` alike. The index is `WHERE deleted_at IS NULL`, so a soft-deleted Tag's name is freed for reuse and never collides with a live one | ADR-0028 | High |
| INV-TAGS-02 | `DeleteTag` detaches, never orphans: in **one transaction** it soft-deletes the Tag (`SoftDeleteTag`) and clears `tag_id` across all four position groups (`ClearAssetTag`/`ClearLiabilityTag`/`ClearReceivableTag`/`ClearInvestmentTag`), so no Position is left pointing at a dead Tag — each falls back to the Untagged bucket. A zero-row soft-delete (missing or cross-tenant id) returns `ErrNotFound` before any clear runs; any step's failure rolls the whole transaction back | ADR-0028 | High |
| INV-TAGS-03 | `AssignTag` sets, reassigns, or clears (nil `tagID`) a Position's Tag and validates household ownership of **both** sides: a non-nil Tag is `GetTag`-checked first (a cross-household or missing Tag is `ErrNotFound`, with no write), and the per-group UPDATE filters the Position by `household_id` too (zero rows affected — cross-household or unknown Position — is `ErrNotFound`). An invalid `TagGroup` is `ErrNotFound`. No path produces a silent cross-tenant link | ADR-0028 | High |
| INV-TAGS-04 | `TagBreakdown` is INV-FINANCE-01 cut by Tag: per `(tag_id, grp, currency)` it sums each contributing Position's most-recent snapshot (`year_month <= current month`, active or terminated in/after the current month — the net-worth carry-forward rule), with `tag_id` NULL as the Untagged bucket. The union of every Tag's cells plus the Untagged bucket reconciles to the household's net-worth components; the result is household-scoped (another household sees none of these rows) | ADR-0028, ADR-0006 | High |
| INV-TAGS-05 | The client fold `aggregateTagBreakdown` (the report's presentation layer over the flat breakdown rows) sums holdings as asset + receivable + investment, keeps liabilities as a separate positive magnitude (`net = holdings − liabilities`), routes null `tag_id` into an Untagged cell with the muted colour, sorts cells holdings-desc with Untagged always last, splits currencies into their own breakdowns ordered by code, and defensively drops non-finite totals | ADR-0028 | Medium |
