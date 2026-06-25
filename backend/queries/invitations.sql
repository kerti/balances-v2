-- name: CreateInvitation :one
INSERT INTO household_invitations (
    household_id, invited_email, token, created_by, expires_at
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: GetInvitationByToken :one
SELECT *
FROM household_invitations
WHERE token = $1;

-- name: GetInvitationByID :one
SELECT *
FROM household_invitations
WHERE id = $1;

-- name: MarkInvitationUsed :exec
UPDATE household_invitations
SET used_at = now()
WHERE id = $1;

-- name: ListPendingInvitationsForEmail :many
-- The onboarding gate's join rows (ADR-0038): pending invitations addressed to
-- the verified email, one row per distinct Household (same-Household
-- double-invites deduped, keeping the most-recent inviter), ordered
-- most-recently-invited first. Keyed by the *verified* email, never by which
-- link was clicked — so a forwarded link can't surface someone else's invites.
SELECT invitation_id, household_id, household_name, inviter_name
FROM (
    SELECT DISTINCT ON (hi.household_id)
        hi.id                                 AS invitation_id,
        hi.household_id                       AS household_id,
        h.display_name                        AS household_name,
        COALESCE(u.nickname, u.display_name)  AS inviter_name,
        hi.created_at                         AS created_at
    FROM household_invitations hi
    JOIN households h ON h.id = hi.household_id
    JOIN users u ON u.id = hi.created_by
    WHERE hi.invited_email = $1
      AND hi.used_at IS NULL
      AND hi.expires_at > now()
    ORDER BY hi.household_id, hi.created_at DESC
) dedup
ORDER BY created_at DESC;

-- name: GetValidInvitationForEmail :one
-- TOCTOU re-validation at commit (ADR-0038): the chosen invitation must still
-- be pending AND addressed to the handshake's verified email. Re-checking the
-- email here (not just the id) is the forwarded-link guard — the client's claim
-- of which invitation is never trusted.
SELECT *
FROM household_invitations
WHERE id = $1
  AND invited_email = $2
  AND used_at IS NULL
  AND expires_at > now();
