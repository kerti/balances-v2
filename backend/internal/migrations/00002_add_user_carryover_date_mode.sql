-- +goose Up
-- Per-user preference for the date the carryover dialog pre-fills (issue #105,
-- follow-up from #60). The carryover helper previously always defaulted the
-- as-of date to today; this lets a user pick how that date is seeded. Additive,
-- backfills every existing row with 'today' (the prior hardcoded behaviour), so
-- the migration is non-destructive. The CHECK mirrors the frontend's
-- SUPPORTED_CARRYOVER_DATE_MODES and the handler's supportedCarryoverDateModes
-- map; extend all three to add a mode.
ALTER TABLE public.users
    ADD COLUMN carryover_date_mode text DEFAULT 'today' NOT NULL,
    ADD CONSTRAINT users_carryover_date_mode_check
        CHECK (carryover_date_mode = ANY (ARRAY[
            'today'::text,
            'end_of_last_month'::text,
            'end_of_month_after_last_snapshot'::text
        ]));

-- +goose Down
ALTER TABLE public.users
    DROP CONSTRAINT IF EXISTS users_carryover_date_mode_check,
    DROP COLUMN IF EXISTS carryover_date_mode;
