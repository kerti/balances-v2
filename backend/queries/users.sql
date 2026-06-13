-- name: GetUserByID :one
SELECT *
FROM users
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByGoogleSub :one
SELECT *
FROM users
WHERE google_sub = $1 AND deleted_at IS NULL;

-- name: CreateUser :one
INSERT INTO users (
    household_id, display_name, email, google_sub, locale, time_zone, picture_url, created_by, updated_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $8
)
RETURNING *;

-- name: ListUsersByHousehold :many
SELECT *
FROM users
WHERE household_id = $1
  AND deleted_at IS NULL
ORDER BY display_name ASC;

-- name: SetUserPicture :one
-- Refresh the Google-sourced profile picture on login. This is a system sync
-- from the OIDC claims, not a user edit, so updated_by is deliberately left
-- alone (it stays the last human editor). The caller only invokes this when the
-- incoming picture differs from the stored one, to avoid touching the row on
-- every login. NULL clears it (Google omitted one).
UPDATE users
SET picture_url = $2,
    updated_at  = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateUserNickname :one
-- Self-attributed: a user updates only their own nickname (id = updated_by).
-- nickname NULL clears it; the length CHECK guards 1..32 chars when set.
UPDATE users
SET nickname   = $2,
    updated_by = $1,
    updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateUserLocale :one
-- Self-attributed UI-language change. The DB CHECK (migration 00020) enforces
-- the allowed BCP47 set; the handler additionally validates before issuing
-- this query so the client gets a 400 rather than a 500 on a bad value.
UPDATE users
SET locale     = $2,
    updated_by = $1,
    updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateUserTheme :one
-- Self-attributed UI-theme change (light/dark). The DB CHECK (migration 00024)
-- enforces the allowed set; the handler additionally validates before issuing
-- this query so the client gets a 400 rather than a 500 on a bad value.
UPDATE users
SET theme      = $2,
    updated_by = $1,
    updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateUserCarryoverDateMode :one
-- Self-attributed: the user sets how the carryover dialog seeds its as-of date
-- (issue #105). The DB CHECK (migration 00026) enforces the allowed set; the
-- handler additionally validates before issuing this query so the client gets a
-- 400 rather than a 500 on a bad value.
UPDATE users
SET carryover_date_mode = $2,
    updated_by          = $1,
    updated_at          = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;
