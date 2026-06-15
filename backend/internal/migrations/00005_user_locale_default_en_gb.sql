-- +goose Up
-- Flip the users.locale column default from id-ID to en-GB (ADR-0035). en-GB is
-- the lingua-franca fallback for an unknown visitor — broader-audience
-- discoverability — while an Indonesian browser is still routed to id-ID by the
-- pre-auth picker's navigator pre-fill. Additive: the CHECK allowed set
-- (en-GB, id-ID) and all stored values are untouched; only the default a row is
-- born with when no value is supplied changes. Both account-birth paths now seed
-- locale explicitly, so this default is the belt-and-braces fallback.
ALTER TABLE public.users ALTER COLUMN locale SET DEFAULT 'en-GB';

-- +goose Down
ALTER TABLE public.users ALTER COLUMN locale SET DEFAULT 'id-ID';
