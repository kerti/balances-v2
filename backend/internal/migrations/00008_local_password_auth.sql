-- +goose Up
-- Optional local password auth (#280, ADR-0039): add email+password as a second,
-- deployment-selectable identity provider beside Google so a self-hoster can run
-- with no third-party OAuth (#116). Additive + one widening (the NOT NULL drop):
-- backward-compatible, low-regret. No row is rewritten.

-- google_sub becomes nullable: it is an *identifier, not a secret*, and a
-- local-only User has none (ADR-0039). The soft-delete-aware unique index is
-- recreated to also exclude nulls, so multiple local Users (all NULL google_sub)
-- coexist. Postgres already treats NULLs as distinct in a unique index, so this
-- is belt-and-braces over the implicit behaviour — it makes "nulls don't
-- collide" explicit in the schema rather than relying on the reader knowing it.
ALTER TABLE public.users ALTER COLUMN google_sub DROP NOT NULL;

DROP INDEX public.users_google_sub_idx;
CREATE UNIQUE INDEX users_google_sub_idx ON public.users USING btree (google_sub)
    WHERE (google_sub IS NOT NULL AND deleted_at IS NULL);

-- The existing users_email_idx already enforces a soft-delete-aware unique on
-- email across *all* Users (Google and local alike), so local accounts inherit
-- the email-uniqueness the ADR calls for with no new index needed.

-- local_credentials is instance-local auth state — grouped with sessions and,
-- like them, excluded from backup (ADR-0039/ADR-0036). Keeping the hash in its
-- own table (not a users column) makes "never serialize the secret" structural:
-- the whole table is out of the export, so there is no per-column omit footgun.
-- A User is *reachable* with a google_sub OR a local_credentials row; a User with
-- neither is legitimately dormant (owns data, cannot yet authenticate) — an
-- app-layer invariant, deliberately not a single-row CHECK.
CREATE TABLE public.local_credentials (
    user_id uuid NOT NULL,
    -- Argon2id PHC string ($argon2id$v=19$m=...,t=...,p=...$salt$hash): salt and
    -- cost params travel inside it, so they can be retuned with no schema change.
    password_hash text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT local_credentials_pkey PRIMARY KEY (user_id),
    CONSTRAINT local_credentials_user_id_fkey FOREIGN KEY (user_id)
        REFERENCES public.users (id) ON DELETE CASCADE
);

-- The local founder register routes through the same onboarding gate as Google
-- (ADR-0038/0039): no users/households row until the person commits the founder
-- choice. The handshake therefore carries the would-be credential across the
-- gate — google_sub goes nullable (a local handshake has none) and a nullable
-- password_hash stashes the already-hashed password until commit. Like the rest
-- of the handshake row this is transient (≈15 min), swept by expiry, and never
-- part of a backup.
ALTER TABLE public.onboarding_handshakes ALTER COLUMN google_sub DROP NOT NULL;
ALTER TABLE public.onboarding_handshakes ADD COLUMN password_hash text;

-- +goose Down
ALTER TABLE public.onboarding_handshakes DROP COLUMN password_hash;
ALTER TABLE public.onboarding_handshakes ALTER COLUMN google_sub SET NOT NULL;

DROP TABLE public.local_credentials;

DROP INDEX public.users_google_sub_idx;
CREATE UNIQUE INDEX users_google_sub_idx ON public.users USING btree (google_sub)
    WHERE (deleted_at IS NULL);
ALTER TABLE public.users ALTER COLUMN google_sub SET NOT NULL;
