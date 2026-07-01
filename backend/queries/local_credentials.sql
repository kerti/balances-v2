-- name: GetLocalCredentialByUserID :one
SELECT *
FROM local_credentials
WHERE user_id = $1;

-- name: ListDormantMembersByHousehold :many
-- Founder-assisted reactivation (ADR-0039, #283): the members a founder may
-- reactivate — the DORMANT ones. A member is dormant when they are unreachable:
-- no google_sub (can't sign in with Google) AND no local_credentials row (no
-- password). Post-restore local members land exactly here (ADR-0036/0039). The
-- LEFT JOIN + `lc.user_id IS NULL` is the has-no-credential test; `google_sub IS
-- NULL` excludes Google members (reachable, not dormant); soft-deleted rows are
-- skipped. The founder themselves is always reachable, so never appears here.
SELECT u.*
FROM users u
LEFT JOIN local_credentials lc ON lc.user_id = u.id
WHERE u.household_id = $1
  AND u.deleted_at IS NULL
  AND u.google_sub IS NULL
  AND lc.user_id IS NULL
ORDER BY u.display_name ASC;

-- name: UpsertLocalCredential :one
-- Create or replace a User's local password credential. The founder register
-- creates the first one; reset/reactivation (later slices) replace it. Keyed on
-- the user_id PK so a member never accumulates more than one local credential.
INSERT INTO local_credentials (user_id, password_hash)
VALUES ($1, $2)
ON CONFLICT (user_id) DO UPDATE
SET password_hash = EXCLUDED.password_hash,
    updated_at    = now()
RETURNING *;
