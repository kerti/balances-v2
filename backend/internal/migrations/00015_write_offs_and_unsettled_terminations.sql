-- +goose Up
-- The Write-Off term + the unsettled-termination advisory (ADR-0052, issue #586).
--
-- write_offs is the new signed term in the comprehensive-income identity
-- (ADR-0008, amended by ADR-0052):
--
--   living_expenses = earned_income + investment_return + asset_value_change
--                     + write_offs − ΔNW
--
-- It carries the value a Position took into its termination month when *no cash
-- settled it* — a `disposed` Asset, a `forgiven`/`written_off` Liability, a
-- `written_off` Receivable. Before it existed, that value had nowhere to go but
-- the derived Living Expenses residual, which is exactly the "our net worth
-- moved and nothing explains it" complaint in #576. The sign follows the effect
-- on net worth, so a forgiven Liability contributes positively. Nullable numeric,
-- nil on the baseline month like the other derived lines (ADR-0006).
--
-- Investment takes no write-off status: a position that genuinely lost its value
-- is a truthful negative Investment Return (ADR-0052 §5), so it never enters this
-- term.
--
-- write_off_positions is the constituent list behind the figure — the same shape
-- as stale_positions plus the signed amount. One signed term nets a forgiven debt
-- against a written-off receivable, so without the constituents a month with both
-- reads as "nothing happened" (ADR-0052 §4); the PDF and the dashboard render the
-- Positions beneath the line.
--
-- unsettled_terminations is a report-side advisory (ADR-0052 §7), NOT part of any
-- figure: Investments terminated with no proceeds recorded in their termination
-- month. Once the terminate dialog captures proceeds (#587) this is only reachable
-- by a path that bypasses it — restore-from-backup (ADR-0036 writes rows
-- directly), import, or the raw API — which is a real way for a household to
-- inherit bad data with no way to notice. Its own column rather than an extension
-- of stale_positions, whose "stale" means one precise thing (no recent snapshot)
-- and is worth keeping precise.
--
-- Both jsonb lists are NOT NULL DEFAULT '[]' so a reader never has to distinguish
-- "none" from "not computed" — matching stale_positions' contract.
--
-- New report columns are not a backup-format shape change (restore rematerialises
-- reports from inputs), so no format bump. reportEngineVersion 4 -> 5 trips
-- needsRegen, so pre-existing rows recompute on next read; no data backfill here
-- (ADR-0052 §8 — the 0-value close snapshots for already-terminated A/L/R
-- Positions are applied by hand to the two live households).
ALTER TABLE public.monthly_reports
    ADD COLUMN write_offs numeric(20,4),
    ADD COLUMN write_off_positions jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN unsettled_terminations jsonb NOT NULL DEFAULT '[]'::jsonb;

-- +goose Down
ALTER TABLE public.monthly_reports
    DROP COLUMN write_offs,
    DROP COLUMN write_off_positions,
    DROP COLUMN unsettled_terminations;
