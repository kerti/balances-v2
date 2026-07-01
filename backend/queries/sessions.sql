-- name: GetSessionByID :one
SELECT *
FROM sessions
WHERE id = $1 AND expires_at > now();

-- name: CreateSession :one
INSERT INTO sessions (
    id, user_id, expires_at, user_agent
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: TouchSession :exec
UPDATE sessions
SET last_seen_at = now(), expires_at = $1
WHERE id = $2;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id = $1;

-- name: DeleteSessionsForUser :exec
-- Revoke every session a user holds. Called on a successful password reset so
-- the reset boots any other (possibly attacker) session before the fresh one is
-- minted — the "reset because compromised" guarantee (ADR-0039, #282).
DELETE FROM sessions WHERE user_id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at <= now();
