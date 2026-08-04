-- name: CreateAsset :one
INSERT INTO assets (
    household_id, display_name, description, subtype,
    ownership_type, sole_owner_user_id, native_currency,
    created_by, updated_by, entry_type
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $8, $9
)
RETURNING *;

-- name: GetAssetByID :one
SELECT *
FROM assets
WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL;

-- name: ListAssetsByHousehold :many
SELECT *
FROM assets
WHERE household_id = $1
  AND (sqlc.narg('subtype')::text IS NULL OR subtype = sqlc.narg('subtype')::text)
  AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: UpdateAsset :one
UPDATE assets
SET display_name       = $3,
    description        = $4,
    ownership_type     = $5,
    sole_owner_user_id = $6,
    updated_by         = $7,
    -- entry_type is editable after the fact on purpose (ADR-0053 §3): with no
    -- engine-side advisory for a mis-declared birth, flipping it here is the
    -- only remedy — including for one inherited from a restore or an import.
    -- COALESCE rather than a plain assignment because omitting it must mean
    -- "leave the declaration alone", never "reset it to acquired" — silently
    -- undoing a newly_tracked declaration is the exact residual ADR-0053 warns
    -- restore and import against.
    entry_type         = COALESCE(sqlc.narg('entry_type')::text, entry_type),
    updated_at         = now()
WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateAssetLifecycle :one
UPDATE assets
SET status           = $3,
    terminated_at    = $4,
    termination_note = $5,
    updated_by       = $6,
    updated_at       = now()
WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteAsset :execrows
UPDATE assets
SET deleted_at = now(),
    updated_by = $3,
    updated_at = now()
WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL;
