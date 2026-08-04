-- name: CreateReceivable :one
INSERT INTO receivables (
    household_id, display_name, description,
    ownership_type, sole_owner_user_id, native_currency,
    counterparty_name, due_date,
    created_by, updated_by, entry_type
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $9, $10
)
RETURNING *;

-- name: GetReceivableByID :one
SELECT *
FROM receivables
WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL;

-- name: ListReceivablesByHousehold :many
SELECT *
FROM receivables
WHERE household_id = $1
  AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: UpdateReceivable :one
UPDATE receivables
SET display_name       = $3,
    description        = $4,
    ownership_type     = $5,
    sole_owner_user_id = $6,
    counterparty_name  = $7,
    due_date           = $8,
    updated_by         = $9,
    -- Editable after the fact — the only remedy for a mis-declared birth
    -- (ADR-0053 §3); see the note on UpdateAsset for why COALESCE.
    entry_type         = COALESCE(sqlc.narg('entry_type')::text, entry_type),
    updated_at         = now()
WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateReceivableLifecycle :one
UPDATE receivables
SET status           = $3,
    terminated_at    = $4,
    termination_note = $5,
    updated_by       = $6,
    updated_at       = now()
WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteReceivable :execrows
UPDATE receivables
SET deleted_at = now(),
    updated_by = $3,
    updated_at = now()
WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL;
