-- +goose Up
-- A snapshot's as_of_date (the statement date) must fall within the same
-- calendar month as its year_month. year_month is always stored first-of-month
-- (handlers + the importer force it), so the check compares the as_of_date's
-- truncated month against it directly. NULL as_of_date is allowed — the date is
-- optional. Added NOT VALID so the ALTER does not scan pre-existing rows that
-- may predate this rule (alpha data); every INSERT/UPDATE from here on is
-- enforced regardless. See ADR-0001 (snapshot = one monthly reading).
ALTER TABLE public.asset_snapshots
    ADD CONSTRAINT asset_snapshots_as_of_in_month
    CHECK (as_of_date IS NULL OR date_trunc('month', as_of_date)::date = year_month) NOT VALID;

ALTER TABLE public.liability_snapshots
    ADD CONSTRAINT liability_snapshots_as_of_in_month
    CHECK (as_of_date IS NULL OR date_trunc('month', as_of_date)::date = year_month) NOT VALID;

ALTER TABLE public.receivable_snapshots
    ADD CONSTRAINT receivable_snapshots_as_of_in_month
    CHECK (as_of_date IS NULL OR date_trunc('month', as_of_date)::date = year_month) NOT VALID;

ALTER TABLE public.investment_snapshots
    ADD CONSTRAINT investment_snapshots_as_of_in_month
    CHECK (as_of_date IS NULL OR date_trunc('month', as_of_date)::date = year_month) NOT VALID;

-- +goose Down
ALTER TABLE public.asset_snapshots DROP CONSTRAINT IF EXISTS asset_snapshots_as_of_in_month;
ALTER TABLE public.liability_snapshots DROP CONSTRAINT IF EXISTS liability_snapshots_as_of_in_month;
ALTER TABLE public.receivable_snapshots DROP CONSTRAINT IF EXISTS receivable_snapshots_as_of_in_month;
ALTER TABLE public.investment_snapshots DROP CONSTRAINT IF EXISTS investment_snapshots_as_of_in_month;
