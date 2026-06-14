# Zone: TAGS

> _Seeded next — the user-defined position tag primitive (ADR-0028, issue #28).
> A tag is a household-scoped label assignable to any position (asset, liability,
> receivable, investment); its financial framing (RDN/custodian/LPS lineage) was
> deliberately kept out of the ADR — it's a neutral grouping primitive. Code:
> `internal/repo/tags.go` (`CreateTag`, `UpdateTag`, `DeleteTag`, `AssignTag`,
> `TagBreakdown`). Tenancy is already INV-TENANCY-12 (`tags_tenancy_test.go`);
> this zone is the **lifecycle + referential** behaviour on top. Candidate
> invariants: a tag name is unique within a household on both create and rename
> (`ErrTagNameExists`); **DeleteTag detaches, never orphans** — it soft-deletes
> the tag AND clears `tag_id` across all four position groups in one transaction,
> so no position is left pointing at a dead tag; AssignTag sets/reassigns/clears
> (nil) a position's tag and validates household ownership of BOTH the tag and the
> position (a cross-household tag or position is `ErrNotFound`, never a silent
> cross-tenant link); and `TagBreakdown` net worth grouped by tag reconciles with
> the total (untagged bucket included), the INV-FINANCE-01 cut by tag. Annotation
> targets: `tags_tenancy_test.go` (extend beyond the tenancy case it already
> carries) plus any handler-level tag tests; a frontend tag-picker vitest if one
> exists. Survey those before writing new tests; fill this table when seeding the
> zone._
