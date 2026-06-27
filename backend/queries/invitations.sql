-- name: CreateInvitation :one
-- token_hash is the SHA-256 of the ≥256-bit random link token (ADR-0039/#281);
-- the plaintext lives only in the emailed link, never at rest.
INSERT INTO household_invitations (
    household_id, invited_email, token_hash, created_by, expires_at
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: GetInvitationByTokenHash :one
-- Read-only resolve by hashed token: callers hash the presented plaintext, then
-- look up by hash. Used by the Google `?invite=` hint (handlers) and the local
-- accept screen's GET preview — neither consumes the invite (validity is checked
-- by the caller); the atomic consume is ConsumeInvitationByTokenHash.
SELECT *
FROM household_invitations
WHERE token_hash = $1;

-- name: ConsumeInvitationByTokenHash :one
-- Single-use atomic consume for the local set-password accept (#281): marks the
-- invitation used iff it is still pending AND unexpired, returning the row. A
-- consumed/expired/forwarded-after-use link matches zero rows (pgx.ErrNoRows),
-- so it can never create a second account — the guard is the WHERE, not a prior
-- read. Mirrors MarkInvitationUsed but conditional and by hash.
UPDATE household_invitations
SET used_at = now()
WHERE token_hash = $1
  AND used_at IS NULL
  AND expires_at > now()
RETURNING *;

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
