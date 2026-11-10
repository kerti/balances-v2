-- name: CreateTag :one
INSERT INTO tags (
    household_id, name, color, created_by, updated_by
) VALUES (
    $1, $2, $3, $4, $4
)
RETURNING *;

-- name: GetTagByID :one
SELECT *
FROM tags
WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL;

-- name: ListTagsByHousehold :many
SELECT *
FROM tags
WHERE household_id = $1 AND deleted_at IS NULL
ORDER BY lower(name);

-- name: UpdateTag :one
UPDATE tags
SET name       = $3,
    color      = $4,
    updated_by = $5,
    updated_at = now()
WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteTag :execrows
UPDATE tags
SET deleted_at = now(),
    updated_by = $3,
    updated_at = now()
WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL;

-- ----- assignment (one per shared Position table) -------------------------
-- tag_id ($1) is nullable: a NULL unassigns. The caller validates the Tag's
-- household ownership before these run; the household_id filter here is the
-- belt-and-suspenders Position-side guard (ADR-0028 tenancy).

-- name: AssignAssetTag :execrows
UPDATE assets
SET tag_id = $1, updated_by = $4, updated_at = now()
WHERE id = $2 AND household_id = $3 AND deleted_at IS NULL;

-- name: AssignLiabilityTag :execrows
UPDATE liabilities
SET tag_id = $1, updated_by = $4, updated_at = now()
WHERE id = $2 AND household_id = $3 AND deleted_at IS NULL;

-- name: AssignReceivableTag :execrows
UPDATE receivables
SET tag_id = $1, updated_by = $4, updated_at = now()
WHERE id = $2 AND household_id = $3 AND deleted_at IS NULL;

-- name: AssignInvestmentTag :execrows
UPDATE investments
SET tag_id = $1, updated_by = $4, updated_at = now()
WHERE id = $2 AND household_id = $3 AND deleted_at IS NULL;

-- ----- batch assignment (Tags page, #853) ----------------------------------
-- Same guards as the single-item queries above, over a set of ids. The repo
-- runs all four in one transaction and compares each group's row count with
-- the ids it sent, so a foreign or deleted id rolls the whole batch back.

-- name: AssignAssetsTag :execrows
UPDATE assets
SET tag_id = $1, updated_by = $4, updated_at = now()
WHERE id = ANY($2::uuid[]) AND household_id = $3 AND deleted_at IS NULL;

-- name: AssignLiabilitiesTag :execrows
UPDATE liabilities
SET tag_id = $1, updated_by = $4, updated_at = now()
WHERE id = ANY($2::uuid[]) AND household_id = $3 AND deleted_at IS NULL;

-- name: AssignReceivablesTag :execrows
UPDATE receivables
SET tag_id = $1, updated_by = $4, updated_at = now()
WHERE id = ANY($2::uuid[]) AND household_id = $3 AND deleted_at IS NULL;

-- name: AssignInvestmentsTag :execrows
UPDATE investments
SET tag_id = $1, updated_by = $4, updated_at = now()
WHERE id = ANY($2::uuid[]) AND household_id = $3 AND deleted_at IS NULL;

-- ----- taggable items (Tags page expand + picker) ---------------------------
-- Every Position the breakdown could count (same active-or-terminated-this-
-- month rule), tagged or not, with its latest snapshot value when it has one.
-- A Position with no snapshot yet still lists (has_value false; the repo turns
-- currency/amount into nulls, since sqlc cannot see a LEFT JOIN null through a
-- UNION) so it can
-- be tagged before its first month is recorded.

-- name: ListTaggableItemsByHousehold :many
SELECT grp, id, subtype, display_name, tag_id, currency, amount, has_value, terminated_at FROM (
    SELECT 'asset'::text AS grp, a.id AS id, a.subtype AS subtype, a.display_name AS display_name,
           a.tag_id AS tag_id, COALESCE(ls.currency, '')::text AS currency, COALESCE(ls.amount, 0)::numeric AS amount, (ls.currency IS NOT NULL)::boolean AS has_value,
           a.terminated_at AS terminated_at
    FROM assets a
    LEFT JOIN LATERAL (
        SELECT amount, currency FROM asset_snapshots
        WHERE asset_id = a.id AND deleted_at IS NULL
          AND year_month <= date_trunc('month', CURRENT_DATE)::date
        ORDER BY year_month DESC LIMIT 1
    ) ls ON true
    WHERE a.household_id = $1 AND a.deleted_at IS NULL
      AND (a.terminated_at IS NULL OR a.terminated_at >= date_trunc('month', CURRENT_DATE)::date)

    UNION ALL

    SELECT 'liability'::text, l.id, l.subtype, l.display_name,
           l.tag_id, COALESCE(ls.currency, '')::text, COALESCE(ls.amount, 0)::numeric, (ls.currency IS NOT NULL)::boolean, l.terminated_at
    FROM liabilities l
    LEFT JOIN LATERAL (
        SELECT amount, currency FROM liability_snapshots
        WHERE liability_id = l.id AND deleted_at IS NULL
          AND year_month <= date_trunc('month', CURRENT_DATE)::date
        ORDER BY year_month DESC LIMIT 1
    ) ls ON true
    WHERE l.household_id = $1 AND l.deleted_at IS NULL
      AND (l.terminated_at IS NULL OR l.terminated_at >= date_trunc('month', CURRENT_DATE)::date)

    UNION ALL

    SELECT 'receivable'::text, r.id, 'receivable'::text, r.display_name,
           r.tag_id, COALESCE(ls.currency, '')::text, COALESCE(ls.amount, 0)::numeric, (ls.currency IS NOT NULL)::boolean, r.terminated_at
    FROM receivables r
    LEFT JOIN LATERAL (
        SELECT amount, currency FROM receivable_snapshots
        WHERE receivable_id = r.id AND deleted_at IS NULL
          AND year_month <= date_trunc('month', CURRENT_DATE)::date
        ORDER BY year_month DESC LIMIT 1
    ) ls ON true
    WHERE r.household_id = $1 AND r.deleted_at IS NULL
      AND (r.terminated_at IS NULL OR r.terminated_at >= date_trunc('month', CURRENT_DATE)::date)

    UNION ALL

    SELECT 'investment'::text, i.id, i.subtype, i.display_name,
           i.tag_id, COALESCE(ls.currency, '')::text, COALESCE(ls.amount, 0)::numeric, (ls.currency IS NOT NULL)::boolean, i.terminated_at
    FROM investments i
    LEFT JOIN LATERAL (
        SELECT amount, currency FROM investment_snapshots
        WHERE investment_id = i.id AND deleted_at IS NULL
          AND year_month <= date_trunc('month', CURRENT_DATE)::date
        ORDER BY year_month DESC LIMIT 1
    ) ls ON true
    WHERE i.household_id = $1 AND i.deleted_at IS NULL
      AND (i.terminated_at IS NULL OR i.terminated_at >= date_trunc('month', CURRENT_DATE)::date)
) items
ORDER BY lower(display_name), id;

-- ----- unassign-on-delete (clear references so the Position reads Untagged) -
-- Soft-deleting a Tag clears it everywhere in the same transaction, so no
-- Position is left pointing at a dead Tag (ADR-0028).

-- name: ClearAssetTag :exec
UPDATE assets SET tag_id = NULL WHERE tag_id = $1 AND household_id = $2;

-- name: ClearLiabilityTag :exec
UPDATE liabilities SET tag_id = NULL WHERE tag_id = $1 AND household_id = $2;

-- name: ClearReceivableTag :exec
UPDATE receivables SET tag_id = NULL WHERE tag_id = $1 AND household_id = $2;

-- name: ClearInvestmentTag :exec
UPDATE investments SET tag_id = NULL WHERE tag_id = $1 AND household_id = $2;

-- ----- breakdown report ----------------------------------------------------
-- Per (tag_id, group, currency): the summed most-recent-snapshot value of
-- every contributing Position. tag_id NULL = the Untagged bucket. A Position
-- contributes its latest snapshot with year_month <= the current month, and
-- only while active or terminated in/after the current month (the net-worth
-- carry-forward + termination rule, CONTEXT "Net Worth"). Liabilities ride
-- back under grp='liability' so the report layer can show them as their own
-- (negative) slice rather than netting them into a tag's assets.

-- name: TagBreakdownByHousehold :many
SELECT tag_id, grp, currency, total FROM (
    SELECT a.tag_id AS tag_id, 'asset'::text AS grp, ls.currency AS currency,
           COALESCE(SUM(ls.amount), 0)::numeric AS total
    FROM assets a
    JOIN LATERAL (
        SELECT amount, currency FROM asset_snapshots
        WHERE asset_id = a.id AND deleted_at IS NULL
          AND year_month <= date_trunc('month', CURRENT_DATE)::date
        ORDER BY year_month DESC LIMIT 1
    ) ls ON true
    WHERE a.household_id = $1 AND a.deleted_at IS NULL
      AND (a.terminated_at IS NULL OR a.terminated_at >= date_trunc('month', CURRENT_DATE)::date)
    GROUP BY a.tag_id, ls.currency

    UNION ALL

    SELECT l.tag_id, 'liability'::text, ls.currency,
           COALESCE(SUM(ls.amount), 0)::numeric
    FROM liabilities l
    JOIN LATERAL (
        SELECT amount, currency FROM liability_snapshots
        WHERE liability_id = l.id AND deleted_at IS NULL
          AND year_month <= date_trunc('month', CURRENT_DATE)::date
        ORDER BY year_month DESC LIMIT 1
    ) ls ON true
    WHERE l.household_id = $1 AND l.deleted_at IS NULL
      AND (l.terminated_at IS NULL OR l.terminated_at >= date_trunc('month', CURRENT_DATE)::date)
    GROUP BY l.tag_id, ls.currency

    UNION ALL

    SELECT r.tag_id, 'receivable'::text, ls.currency,
           COALESCE(SUM(ls.amount), 0)::numeric
    FROM receivables r
    JOIN LATERAL (
        SELECT amount, currency FROM receivable_snapshots
        WHERE receivable_id = r.id AND deleted_at IS NULL
          AND year_month <= date_trunc('month', CURRENT_DATE)::date
        ORDER BY year_month DESC LIMIT 1
    ) ls ON true
    WHERE r.household_id = $1 AND r.deleted_at IS NULL
      AND (r.terminated_at IS NULL OR r.terminated_at >= date_trunc('month', CURRENT_DATE)::date)
    GROUP BY r.tag_id, ls.currency

    UNION ALL

    SELECT i.tag_id, 'investment'::text, ls.currency,
           COALESCE(SUM(ls.amount), 0)::numeric
    FROM investments i
    JOIN LATERAL (
        SELECT amount, currency FROM investment_snapshots
        WHERE investment_id = i.id AND deleted_at IS NULL
          AND year_month <= date_trunc('month', CURRENT_DATE)::date
        ORDER BY year_month DESC LIMIT 1
    ) ls ON true
    WHERE i.household_id = $1 AND i.deleted_at IS NULL
      AND (i.terminated_at IS NULL OR i.terminated_at >= date_trunc('month', CURRENT_DATE)::date)
    GROUP BY i.tag_id, ls.currency
) breakdown
ORDER BY grp, currency;
