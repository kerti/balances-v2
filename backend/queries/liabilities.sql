-- name: CreateLiability :one
INSERT INTO liabilities (
    household_id, display_name, description, subtype,
    ownership_type, sole_owner_user_id, native_currency,
    counterparty_name, principal, interest_rate,
    term_months, start_date, maturity_date,
    created_by, updated_by, entry_type
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $14, $15
)
RETURNING *;

-- name: GetLiabilityByID :one
SELECT *
FROM liabilities
WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL;

-- name: ListLiabilitiesByHousehold :many
SELECT *
FROM liabilities
WHERE household_id = $1
  AND (sqlc.narg('subtype')::text IS NULL OR subtype = sqlc.narg('subtype')::text)
  AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: UpdateLiability :one
UPDATE liabilities
SET display_name       = $3,
    description        = $4,
    ownership_type     = $5,
    sole_owner_user_id = $6,
    counterparty_name  = $7,
    principal          = $8,
    interest_rate      = $9,
    term_months        = $10,
    start_date         = $11,
    maturity_date      = $12,
    updated_by         = $13,
    -- Editable after the fact — the only remedy for a mis-declared birth
    -- (ADR-0053 §3); see the note on UpdateAsset for why COALESCE.
    entry_type         = COALESCE(sqlc.narg('entry_type')::text, entry_type),
    updated_at         = now()
WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateLiabilityLifecycle :one
UPDATE liabilities
SET status           = $3,
    terminated_at    = $4,
    termination_note = $5,
    updated_by       = $6,
    updated_at       = now()
WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteLiability :execrows
UPDATE liabilities
SET deleted_at = now(),
    updated_by = $3,
    updated_at = now()
WHERE id = $1 AND household_id = $2 AND deleted_at IS NULL;
