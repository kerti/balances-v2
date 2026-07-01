-- +goose Up
-- Emailed self-service password reset (#282, ADR-0039). Reuses the shared
-- set-password-token mechanism (#281): a single-use, short-TTL, ≥256-bit random
-- token whose plaintext lives only in the emailed link. Only its SHA-256 hash is
-- stored, so a database leak yields no usable reset link and the plaintext is
-- never persisted or logged. Like sessions and local_credentials this is
-- instance-local auth state, excluded from backup (ADR-0036) — a backup must
-- never carry a live reset link.
CREATE TABLE public.password_reset_tokens (
    token_hash text NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    -- expires_at is set explicitly at issue time (short TTL, distinct from the
    -- 72h invite) rather than defaulted, so the window is a code-level decision.
    expires_at timestamp with time zone NOT NULL,
    used_at timestamp with time zone,
    -- token_hash is the PK: it is unique by construction (a ≥256-bit random
    -- token) and every lookup keys on it, so it doubles as the identity.
    CONSTRAINT password_reset_tokens_pkey PRIMARY KEY (token_hash),
    CONSTRAINT password_reset_tokens_user_id_fkey FOREIGN KEY (user_id)
        REFERENCES public.users (id) ON DELETE CASCADE
);

-- The FK cascade and the post-reset "invalidate this user's other tokens" both
-- walk user_id.
CREATE INDEX password_reset_tokens_user_id_idx ON public.password_reset_tokens USING btree (user_id);

-- +goose Down
DROP TABLE public.password_reset_tokens;
