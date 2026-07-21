-- +goose Up
-- Investment-performance breakdown (ADR-0048 amendment): the PDF report reports
-- investment return as a *rate* (return over the capital it was earned on),
-- three ways — total, by risk profile, by instrument type — each paired with its
-- trailing-12-month figure. The rate is derived at render time (like the four
-- ratios, never materialized); these columns materialize the amounts + the
-- per-bucket invested-value bases the render divides by.
--
-- Two families, all additive nullable numeric (nil on the baseline month, like
-- the existing investment_return_* columns):
--
--   investment_return_{low,medium,high} — the month's investment return split by
--     the position's risk_profile. Parallels the existing per-subtype
--     investment_return_* columns; the two partitions both sum to
--     investment_return_total (INV-FINANCE-29).
--
--   investment_value_{stock,mutual_fund,bond,gold,time_deposit} and
--   investment_value_{low,medium,high} — the month's *closing* invested value per
--     bucket (FX-converted, carry-forward applied). The render reads the prior
--     month's column as the current month's opening rate base. The total opening
--     base is the existing nw_investments (no duplicate column). Both partitions
--     reconcile to nw_investments (INV-FINANCE-29).
--
-- A new report column is not a backup-format shape change — restore
-- rematerialises reports from inputs — so no format bump. The engine_version bump
-- (reportEngineVersion 2 -> 3) trips needsRegen so pre-existing rows recompute
-- these columns on next read; no manual backfill.
ALTER TABLE public.monthly_reports
    ADD COLUMN investment_return_low numeric(20,4),
    ADD COLUMN investment_return_medium numeric(20,4),
    ADD COLUMN investment_return_high numeric(20,4),
    ADD COLUMN investment_value_stock numeric(20,4),
    ADD COLUMN investment_value_mutual_fund numeric(20,4),
    ADD COLUMN investment_value_bond numeric(20,4),
    ADD COLUMN investment_value_gold numeric(20,4),
    ADD COLUMN investment_value_time_deposit numeric(20,4),
    ADD COLUMN investment_value_low numeric(20,4),
    ADD COLUMN investment_value_medium numeric(20,4),
    ADD COLUMN investment_value_high numeric(20,4);

-- +goose Down
ALTER TABLE public.monthly_reports
    DROP COLUMN investment_return_low,
    DROP COLUMN investment_return_medium,
    DROP COLUMN investment_return_high,
    DROP COLUMN investment_value_stock,
    DROP COLUMN investment_value_mutual_fund,
    DROP COLUMN investment_value_bond,
    DROP COLUMN investment_value_gold,
    DROP COLUMN investment_value_time_deposit,
    DROP COLUMN investment_value_low,
    DROP COLUMN investment_value_medium,
    DROP COLUMN investment_value_high;
