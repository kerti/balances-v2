-- name: GetHouseholdByID :one
SELECT *
FROM households
WHERE id = $1 AND deleted_at IS NULL;

-- name: CreateHousehold :one
INSERT INTO households (
    display_name, reporting_currency
) VALUES (
    $1, $2
)
RETURNING *;

-- name: UpdateHouseholdSettings :one
UPDATE households
SET display_name           = $2,
    reporting_currency     = $3,
    multi_currency_enabled = $4,
    updated_by             = $5,
    updated_at             = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- Count of positions denominated in a currency other than $2 across the four
-- groups — guards turning multi-currency off while foreign positions exist.
-- name: CountForeignCurrencyPositions :one
SELECT
    (SELECT COUNT(*) FROM assets a       WHERE a.household_id = $1  AND a.deleted_at IS NULL  AND a.native_currency <> $2)
  + (SELECT COUNT(*) FROM liabilities l  WHERE l.household_id = $1  AND l.deleted_at IS NULL  AND l.native_currency <> $2)
  + (SELECT COUNT(*) FROM receivables rc WHERE rc.household_id = $1 AND rc.deleted_at IS NULL AND rc.native_currency <> $2)
  + (SELECT COUNT(*) FROM investments i  WHERE i.household_id = $1  AND i.deleted_at IS NULL  AND i.native_currency <> $2)
  AS foreign_count;
