-- +goose Up
-- Shared set-password-token mechanism (#281, ADR-0039): the emailed invite token
-- is generalised into a hashed, single-use, short-TTL set-password token reused
-- by the later reset/reactivation slices. Storage moves from plaintext to a
-- SHA-256 hash of a ≥256-bit random token: a DB leak no longer yields a usable
-- link, and the plaintext exists only in the email/URL, never at rest. The token
-- is high-entropy, so a fast hash (not Argon2id) is the right primitive — there
-- is nothing to brute-force.
--
-- Rename the column to name what it now holds. The UNIQUE constraint rides along
-- on rename; we rename it too so the schema reads honestly. Any pre-existing
-- pending invitation carried a plaintext value here and is silently invalidated
-- by the switch to hash-compare — acceptable pre-alpha (no production data; a
-- fresh invite is one click), and there is no in-place backfill because the
-- plaintext is unrecoverable from a hash by design.
ALTER TABLE public.household_invitations RENAME COLUMN token TO token_hash;
ALTER TABLE public.household_invitations
    RENAME CONSTRAINT household_invitations_token_key TO household_invitations_token_hash_key;

-- +goose Down
ALTER TABLE public.household_invitations
    RENAME CONSTRAINT household_invitations_token_hash_key TO household_invitations_token_key;
ALTER TABLE public.household_invitations RENAME COLUMN token_hash TO token;
