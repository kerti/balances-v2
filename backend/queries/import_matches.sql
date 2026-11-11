-- Create-from-file import (issue #828): the positions a workbook's Detail sheet
-- would duplicate. A match is the same subtype and the same display name
-- (case-insensitive, trimmed) in the caller's household; a bank account must
-- also carry the same account number. Terminated positions are included (a
-- closed account is still the same account); soft-deleted rows are not.

-- name: FindAssetImportMatches :many
SELECT a.id, a.display_name, a.status, a.subtype, a.created_at
FROM assets a
LEFT JOIN bank_account_details b ON b.asset_id = a.id
WHERE a.household_id = sqlc.arg('household_id')
  AND a.subtype = sqlc.arg('subtype')::text
  AND a.deleted_at IS NULL
  AND lower(btrim(a.display_name)) = lower(btrim(sqlc.arg('display_name')::text))
  -- NULL for subtypes with no identifier; for a bank account the (possibly
  -- blank) number must equal the stored one.
  AND (sqlc.narg('account_number')::text IS NULL
       OR btrim(b.account_number) = btrim(sqlc.narg('account_number')::text))
ORDER BY a.created_at, a.id;

-- name: FindInvestmentImportMatches :many
SELECT id, display_name, status, subtype, created_at
FROM investments
WHERE household_id = sqlc.arg('household_id')
  AND subtype = sqlc.arg('subtype')::text
  AND deleted_at IS NULL
  AND lower(btrim(display_name)) = lower(btrim(sqlc.arg('display_name')::text))
ORDER BY created_at, id;

-- name: FindLiabilityImportMatches :many
SELECT id, display_name, status, subtype, created_at
FROM liabilities
WHERE household_id = sqlc.arg('household_id')
  AND subtype = sqlc.arg('subtype')::text
  AND deleted_at IS NULL
  AND lower(btrim(display_name)) = lower(btrim(sqlc.arg('display_name')::text))
ORDER BY created_at, id;

-- name: FindReceivableImportMatches :many
SELECT id, display_name, status, created_at
FROM receivables
WHERE household_id = sqlc.arg('household_id')
  AND deleted_at IS NULL
  AND lower(btrim(display_name)) = lower(btrim(sqlc.arg('display_name')::text))
ORDER BY created_at, id;
