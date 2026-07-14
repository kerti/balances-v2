-- +goose Up
-- Financial-statistics panel inputs (ADR-0048, #412 stats epic): the Pension
-- income category and the monthly inflation model, batched into one migration
-- (both are unreleased inputs to the same panel).
--
-- Pension and Interest are recurring, clearly-passive income the app lacked; the
-- panel needs each both as its own line and as part of "passive cash income"
-- (rental + pension + interest). Interest is bank/deposit interest that lands as
-- cash — an external stream, not investment-pool return — so it never overlaps
-- with InvestmentReturn (no double-count in either passive scope). Additive:
-- widens the income category CHECK and adds the matching monthly_reports
-- earned-income breakdown columns. A new allowed `text` value is not a shape
-- change — restore validates categories against this CHECK — so this half needs
-- no backup-format bump.
ALTER TABLE public.income DROP CONSTRAINT income_category_check;
ALTER TABLE public.income
    ADD CONSTRAINT income_category_check
        CHECK ((category = ANY (ARRAY['salary'::text, 'business_income'::text, 'rental_income'::text, 'pension'::text, 'interest'::text, 'gift'::text, 'tax_refund'::text, 'insurance_payout'::text, 'other'::text])));

ALTER TABLE public.monthly_reports
    ADD COLUMN earned_income_pension numeric(20,4);
ALTER TABLE public.monthly_reports
    ADD COLUMN earned_income_interest numeric(20,4);

-- Monthly inflation rate + assumed-inflation setting. Structurally an FX rate
-- minus the currency dimension: household-scoped, one annualized (YoY) percentage
-- figure per month, soft-deleted, carried forward. Feeds only the Fund Resilience
-- projection. Deflation is allowed, so `rate` has no positivity constraint (unlike
-- fx_rates.rate). assumed_annual_inflation is the fallback the projection uses
-- before any monthly figure exists (default 3.5% — slightly conservative vs recent
-- Indonesian CPI, so the runway estimate errs short/safe).
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

ALTER TABLE public.monthly_reports DROP COLUMN earned_income_interest;
ALTER TABLE public.monthly_reports DROP COLUMN earned_income_pension;

ALTER TABLE public.income DROP CONSTRAINT income_category_check;
ALTER TABLE public.income
    ADD CONSTRAINT income_category_check
        CHECK ((category = ANY (ARRAY['salary'::text, 'business_income'::text, 'rental_income'::text, 'gift'::text, 'tax_refund'::text, 'insurance_payout'::text, 'other'::text])));
