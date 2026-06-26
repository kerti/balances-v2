-- name: GetLocalCredentialByUserID :one
SELECT *
FROM local_credentials
WHERE user_id = $1;

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
