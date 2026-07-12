-- +goose Up
-- Monthly inflation rate + assumed-inflation setting (ADR-0048, #412 stats epic).
-- Structurally an FX rate minus the currency dimension: household-scoped, one
-- annualized (YoY) percentage figure per month, soft-deleted, carried forward.
-- Feeds only the Fund Resilience projection. Deflation is allowed, so `rate` has
-- no positivity constraint (unlike fx_rates.rate). assumed_annual_inflation is the
-- fallback the projection uses before any monthly figure exists (default 3.5% —
-- slightly conservative vs recent Indonesian CPI, so the runway estimate errs
-- short/safe).
CREATE TABLE public.inflation_rates (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    household_id uuid NOT NULL,
    year_month date NOT NULL,
    rate numeric(9,4) NOT NULL,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_by uuid,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone,
    CONSTRAINT inflation_rates_pkey PRIMARY KEY (id),
    CONSTRAINT inflation_rates_household_id_fkey FOREIGN KEY (household_id) REFERENCES public.households(id),
    CONSTRAINT inflation_rates_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id),
    CONSTRAINT inflation_rates_updated_by_fkey FOREIGN KEY (updated_by) REFERENCES public.users(id)
);

CREATE INDEX inflation_rates_household_id_idx
    ON public.inflation_rates USING btree (household_id) WHERE (deleted_at IS NULL);
CREATE UNIQUE INDEX inflation_rates_household_year_month_idx
    ON public.inflation_rates USING btree (household_id, year_month) WHERE (deleted_at IS NULL);

ALTER TABLE public.households
    ADD COLUMN assumed_annual_inflation numeric(9,4) NOT NULL DEFAULT 3.5;

-- +goose Down
ALTER TABLE public.households DROP COLUMN assumed_annual_inflation;
DROP TABLE public.inflation_rates;
