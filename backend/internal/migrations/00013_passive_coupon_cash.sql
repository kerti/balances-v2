-- +goose Up
-- Coupon-payout passive-cash (ADR-0048 amendment, #476, split out of the #412
-- stats epic): paid-out bond coupons (coupon_disposition='pays_out') are
-- dependable external cash, like Rental/Pension/Interest, so they belong in the
-- statistics passive-cash scope (Passive-Income numerator + Fund Resilience
-- draw-offset) and must NOT also compound in the pool's own-return g.
--
-- The domain keeps coupon yield inside investment_return (CONTEXT: investment
-- return covers yield from Coupons/Dividends/Distributions), so this column does
-- not move the coupon out of investment_return_total. It materialises the
-- paid-out coupon slice on its own so buildStats can, at render time, add it to
-- passive cash and subtract it from investment_return_total when forming g — the
-- two-scope split that guards the double-count (INV-FINANCE-25). Accruing coupons
-- record no Coupon Transaction (their yield lands as snapshot growth), so they
-- never enter this column and stay in g, unchanged.
--
-- Additive nullable numeric column (nil on the baseline month, like the other
-- income-statement figures). A new report column is not a shape change — restore
-- rematerialises reports from inputs — so no backup-format bump. The
-- engine_version bump (reportEngineVersion → 2) trips needsRegen so pre-existing
-- rows recompute the new column on next read.
ALTER TABLE public.monthly_reports
    ADD COLUMN passive_coupon_cash numeric(20,4);

-- +goose Down
ALTER TABLE public.monthly_reports DROP COLUMN passive_coupon_cash;
