-- name: GetUserByID :one
SELECT *
FROM users
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByGoogleSub :one
-- The ::text cast keeps the lookup parameter a plain (non-null) string even
-- though google_sub is nullable since ADR-0039 — callers always have a concrete
-- subject to look up, never NULL, and this avoids a *string param rippling
-- through every caller. A NULL column value simply never equals the arg.
SELECT *
FROM users
WHERE google_sub = $1::text AND deleted_at IS NULL;

-- name: GetUserByEmail :one
-- Email is the human-facing handle and (since ADR-0039) the lookup key for local
-- password login. Soft-delete-aware and unique (users_email_idx), so it returns
-- at most one live row. Callers must treat a miss as a generic auth failure —
-- never leak whether the email exists (no user enumeration on login).
SELECT *
FROM users
WHERE email = $1 AND deleted_at IS NULL;

-- name: CreateUser :one
-- The Google-identity create path: google_sub is the verified subject, always
-- present. The ::text cast keeps the param a plain (non-null) string so the many
-- existing callers are unaffected by google_sub going nullable (ADR-0039); the
-- local-only path uses CreateLocalUser, which leaves google_sub NULL.
INSERT INTO users (
    household_id, display_name, email, google_sub, locale, time_zone, picture_url, created_by, updated_by
) VALUES (
    sqlc.arg(household_id), sqlc.arg(display_name), sqlc.arg(email), sqlc.arg(google_sub)::text,
    sqlc.arg(locale), sqlc.arg(time_zone), sqlc.narg(picture_url), sqlc.narg(created_by), sqlc.narg(created_by)
)
RETURNING *;

-- name: CreateLocalUser :one
-- The local-password create path (ADR-0039): no google_sub (left NULL — a
-- local-only User is identified by email and proves it with a local_credentials
-- row). Mirrors CreateUser otherwise. The credential row is written separately
-- (UpsertLocalCredential) so the secret never rides the identity insert.
INSERT INTO users (
    household_id, display_name, email, locale, time_zone, picture_url, created_by, updated_by
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $7
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
-- (issue #105). The DB CHECK (migration 00002) enforces the allowed set; the
-- handler additionally validates before issuing this query so the client gets a
-- 400 rather than a 500 on a bad value.
UPDATE users
SET carryover_date_mode = $2,
    updated_by          = $1,
    updated_at          = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;
