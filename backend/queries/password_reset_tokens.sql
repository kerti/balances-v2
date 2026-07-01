-- name: CreatePasswordResetToken :one
-- token_hash is the SHA-256 of a ≥256-bit random token (#281/#282, ADR-0039);
-- the plaintext lives only in the emailed link, never at rest.
INSERT INTO password_reset_tokens (
    token_hash, user_id, expires_at
) VALUES (
    $1, $2, $3
)
RETURNING *;

-- name: GetPasswordResetToken :one
-- Read-only resolve by hashed token for the reset-set screen's preview, which
-- validates the link without consuming it (so a reload doesn't burn the
-- single-use token). The atomic consume is ConsumePasswordResetToken.
SELECT *
FROM password_reset_tokens
WHERE token_hash = $1;

-- name: ConsumePasswordResetToken :one
-- Single-use atomic consume for the emailed reset (#282): marks the token used
-- iff it is still unused AND unexpired, returning the row. A consumed / expired
-- token matches zero rows (pgx.ErrNoRows), so a reset link works exactly once —
-- the guard is the WHERE, not a prior read. Mirrors ConsumeInvitationByTokenHash.
UPDATE password_reset_tokens
SET used_at = now()
WHERE token_hash = $1
  AND used_at IS NULL
  AND expires_at > now()
RETURNING *;

-- name: DeletePasswordResetTokensForUser :exec
-- Invalidate a user's outstanding reset tokens. Called after a successful reset
-- so a second still-valid link (e.g. a double request) can't be replayed once
-- one has set the password.
DELETE FROM password_reset_tokens WHERE user_id = $1;

-- name: DeleteExpiredPasswordResetTokens :exec
-- Housekeeping sweep of spent windows, mirroring DeleteExpiredSessions /
-- DeleteExpiredOnboardingHandshakes.
DELETE FROM password_reset_tokens WHERE expires_at <= now();
