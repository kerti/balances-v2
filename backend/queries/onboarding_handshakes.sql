-- name: CreateOnboardingHandshake :one
INSERT INTO onboarding_handshakes (
    id, google_sub, email, display_name, picture_url, seed_locale, hint_invitation_id, expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: GetOnboardingHandshake :one
SELECT *
FROM onboarding_handshakes
WHERE id = $1 AND expires_at > now();

-- name: DeleteOnboardingHandshake :exec
DELETE FROM onboarding_handshakes WHERE id = $1;

-- name: DeleteExpiredOnboardingHandshakes :exec
DELETE FROM onboarding_handshakes WHERE expires_at <= now();
