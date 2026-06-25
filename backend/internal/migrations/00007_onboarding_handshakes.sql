-- +goose Up
-- Post-auth onboarding handshake (#158/#267, ADR-0038): the transient,
-- pre-account verified identity that lets the founder-vs-join decision move
-- *after* Google verifies the email. Modelled on `sessions` — an opaque token
-- in an httpOnly cookie → DB lookup → identify — not a signed cookie, so it
-- needs no new signing-key secret (ADR-0037). It is auth state, NOT household
-- data: it has no household_id, is never part of a backup (ADR-0036), and is
-- swept by the same expiry path as sessions. Additive: a brand-new table, no
-- existing row touched.
CREATE TABLE public.onboarding_handshakes (
    id text NOT NULL,
    google_sub text NOT NULL,
    email text NOT NULL,
    display_name text NOT NULL,
    picture_url text,
    seed_locale text NOT NULL,
    hint_invitation_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    CONSTRAINT onboarding_handshakes_pkey PRIMARY KEY (id)
);

-- The gate's pending-invite lookup keys off the verified email; the sweep keys
-- off expires_at. Both stay small (handshakes live ≈15 min) but the email index
-- mirrors the access pattern the next slice (#268) leans on.
CREATE INDEX onboarding_handshakes_email_idx ON public.onboarding_handshakes (email);
CREATE INDEX onboarding_handshakes_expires_at_idx ON public.onboarding_handshakes (expires_at);

-- +goose Down
DROP TABLE public.onboarding_handshakes;
