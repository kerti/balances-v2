-- +goose Up
-- Pension income category (ADR-0048, #412 stats epic). Pension is recurring,
-- clearly-passive income the app lacked; the financial-statistics panel needs it
-- both as its own line and as half of "passive cash income" (rental + pension).
-- Additive: widens the income category CHECK and adds the matching monthly_reports
-- earned-income breakdown column. No backup-format bump — a new allowed `text`
-- value is not a shape change, and restore validates categories against this CHECK.
ALTER TABLE public.income DROP CONSTRAINT income_category_check;
ALTER TABLE public.income
    ADD CONSTRAINT income_category_check
        CHECK ((category = ANY (ARRAY['salary'::text, 'business_income'::text, 'rental_income'::text, 'pension'::text, 'gift'::text, 'tax_refund'::text, 'insurance_payout'::text, 'other'::text])));

ALTER TABLE public.monthly_reports
    ADD COLUMN earned_income_pension numeric(20,4);

-- +goose Down
ALTER TABLE public.monthly_reports DROP COLUMN earned_income_pension;

ALTER TABLE public.income DROP CONSTRAINT income_category_check;
ALTER TABLE public.income
    ADD CONSTRAINT income_category_check
        CHECK ((category = ANY (ARRAY['salary'::text, 'business_income'::text, 'rental_income'::text, 'gift'::text, 'tax_refund'::text, 'insurance_payout'::text, 'other'::text])));
