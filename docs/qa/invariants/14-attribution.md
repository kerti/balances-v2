# Zone: ATTRIBUTION

The **per-owner routing layer** beneath FINANCE's `userBreakdowns`. FINANCE owns
the **reconciliation identity** (`INV-FINANCE-02`: the per-user/joint buckets sum
to the total) and the category/subtype bucketing (`INV-FINANCE-14`) — neither is
re-catalogued here. This zone catalogues the *routing rule* that decides **which
owner bucket** a position's net worth, earned income, and investment return land
in: the whole rule is one function, `ownerKey` in `monthly_reports_engine.go` —
`ownershipType == "sole" && soleOwnerID != nil` routes to the owner's UUID-keyed
bucket, **everything else** (joint, or a malformed sole row with a nil owner)
falls through to the single `jointKey` ("joint") bucket. The defining risk is
**mis-attribution that still reconciles**: a wrong owner key moves value between
two members (or between a member and the joint bucket) while the grand total —
and `INV-FINANCE-02` — stays correct, so the household total looks right while an
individual member's number is silently wrong. Distinct from TENANCY (cross-
*household* leak) and FINANCE (the total): the failure here is *intra-household*
misallocation. ADR-0004 (ownership model), ADR-0012 (per-user breakdown),
ADR-0005 (the household boundary it rides within).

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-ATTRIBUTION-01 | Sole routes to owner only — a `sole` row with a non-nil `soleOwnerID` contributes to exactly that member's bucket and zero to any other member or the joint bucket, across **every** channel that calls `ownerKey`: net worth (`bob`'s liability subtracts *within his own bucket*, `bob = -200`), earned income, and investment return. A sole row never leaks value into another member's column | ADR-0004/0012 | High |
| INV-ATTRIBUTION-02 | Joint is a whole bucket, never split — a `joint` row lands entirely in `jointKey`, *not* divided across members (the non-technical-audience design, ADR-0004: household holdings shown jointly, not per-head). The joint bucket is the catch-all the routing rule defaults to, so it must hold the full joint amount, never a per-capita share | ADR-0004 | High |
| INV-ATTRIBUTION-03 | The bucket set is seeded from membership — every household member gets a bucket (zero if they own nothing) plus the one `jointKey` bucket, before any routing runs. A member with no sole holdings still appears with a zero number rather than being absent, and the bucket set is membership-driven, not derived from which owners happen to appear in the data | ADR-0012 | Medium |
| INV-ATTRIBUTION-04 | Malformed-sole degrades safe — a `sole` row with a **nil** `soleOwnerID` falls through to `jointKey` rather than panicking, dropping the value, or fabricating an owner bucket. The value stays in the household total (reconciliation holds); it lands in joint, not in limbo | ADR-0004 | Medium |
