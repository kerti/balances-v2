-- +goose Up
-- DESTRUCTIVE: drops users.carryover_date_mode, the per-user "Carry-over date"
-- preference (migration 00002, issue #105). It seeded the statement date that
-- Copy carryover pre-stamped on a snapshot; #788 removed Copy carryover and #790
-- (ADR-0046 amendment) stops pre-filling the statement date anywhere, so nothing
-- reads the column. The stored value cannot be recovered after the drop.
ALTER TABLE public.users
    DROP CONSTRAINT IF EXISTS users_carryover_date_mode_check,
    DROP COLUMN IF EXISTS carryover_date_mode;

-- +goose Down
-- Restores the column exactly as migration 00002 defined it; every row comes
-- back with the 'today' default (the original per-user choices are not kept).
ALTER TABLE public.users
    ADD COLUMN carryover_date_mode text DEFAULT 'today' NOT NULL,
    ADD CONSTRAINT users_carryover_date_mode_check
        CHECK (carryover_date_mode = ANY (ARRAY[
            'today'::text,
            'end_of_last_month'::text,
            'end_of_month_after_last_snapshot'::text
        ]));
