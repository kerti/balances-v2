# Zone: ATTRIBUTION

> _Seeded next — the **per-owner routing layer** beneath FINANCE's
> `userBreakdowns`. FINANCE already owns the **reconciliation identity**
> (`INV-FINANCE-02`: per-user/joint attribution sums to the total) and the
> category/subtype bucketing (`INV-FINANCE-14`) — do **not** re-catalog either;
> this zone catalogues the *routing rule* that decides **which owner bucket** a
> position's net worth and income land in, the mechanism the reconciliation
> identity sits on top of. The whole rule is one small function, `ownerKey` in
> `monthly_reports_engine.go`: `ownershipType == "sole" && soleOwnerID != nil`
> routes to the owner's UUID-keyed bucket, **everything else** (joint, or a
> malformed sole row with a nil owner) falls through to the single `jointKey`
> ("joint") bucket. The defining risk is **mis-attribution that still
> reconciles**: a wrong owner key moves value between two members (or between a
> member and the joint bucket) while the grand total — and therefore
> `INV-FINANCE-02` — stays correct, so the household total looks right while an
> individual member's net worth is silently wrong. This is distinct from TENANCY
> (cross-*household* leak) and from FINANCE (the total): the failure is
> *intra-household* misallocation. Candidate invariants: (1) **sole routes to
> owner only** — a `sole` position/income with a `soleOwnerID` contributes to
> exactly that member's bucket and zero to any other member or the joint bucket,
> and a liability subtracts within the owner's bucket (the `bob = -200` case);
> (2) **joint is a whole bucket, never split** — a `joint` position lands
> entirely in `jointKey`, *not* divided across members (the non-technical-audience
> design, ADR-0004: household holdings shown jointly, not per-head); (3) **the
> bucket set is seeded from membership** — every household member gets a bucket
> (zero if they own nothing) plus the one joint bucket, so a member with no sole
> holdings still appears (and a new member doesn't retroactively change anyone
> else's number); (4) **malformed-sole degrades safe** — `sole` with a nil
> `soleOwnerID` falls to joint rather than panicking or dropping the value (the
> `ownerKey` guard), so a bad row can't vanish from the total. The flagship
> annotation target is the unannotated **`TestEngine_GroupsAndBreakdown`**
> (`monthly_reports_engine_test.go`) — it already exercises sole/joint/liability
> routing and the reconciliation sum; the income-side routing lives in the same
> engine pass (`ownerKey(inc.ownershipType, inc.soleOwnerID)`). Survey
> `monthly_reports_engine_test.go` + `monthly_reports_engine_categories_test.go`
> before writing; genuine new coverage is likely the malformed-sole degrade case
> and an income-attribution row. **Dedup discipline:** every row must pin the
> *routing*, never restate FINANCE-02's reconciliation or TENANCY's isolation.
> ADR-0004 (ownership model), ADR-0012 (per-user breakdown), ADR-0005 (the
> household boundary it rides within)._
