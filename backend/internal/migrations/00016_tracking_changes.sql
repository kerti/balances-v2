-- +goose Up
-- The Tracking Change term, in both its directions (ADR-0053, issue #594).
--
-- tracking_changes is the new signed term in the comprehensive-income identity
-- (ADR-0008, amended by ADR-0052 and now again here):
--
--   living_expenses = earned_income + investment_return + asset_value_change
--                     + write_offs + tracking_changes − ΔNW
--
-- It carries the value a Position brought into, or took out of, the Household's
-- books when what changed was the books' *coverage* rather than earning,
-- spending or investing — a spouse's accounts joining at marriage, a dormant
-- passbook finally entered years into tracking, a departing member's Positions
-- leaving. Before it existed that whole amount landed in the derived Living
-- Expenses residual, reading as a huge negative month's spending on the way in
-- and as spending on the way out (issue #591). Sign follows the effect on net
-- worth, so a departing member's Liability contributes positively. Nullable
-- numeric, nil on the baseline month like the other derived lines (ADR-0006).
--
-- tracking_change_positions is the constituent list behind the figure — the same
-- shape as write_off_positions (id/name/group/subtype + the signed amount). One
-- signed term covering both directions means a month holding an arrival and a
-- departure can net toward zero on the line (ADR-0053 §1), so without the
-- constituents such a month reads as "nothing happened". NOT NULL DEFAULT '[]'
-- so a reader never has to distinguish "none" from "not computed", matching
-- stale_positions' and write_off_positions' contract.
ALTER TABLE public.monthly_reports
    ADD COLUMN tracking_changes numeric(20,4),
    ADD COLUMN tracking_change_positions jsonb NOT NULL DEFAULT '[]'::jsonb;

-- entry_type declares what a Position's *birth* was (ADR-0053 §3). It cannot be
-- inferred: an acquisition funded from tracked wealth and a Position that was
-- already owned present to the engine as the same thing — a first Snapshot with
-- no prior value, literally the same `!okPrev` branch — so a blanket birth-month
-- term would fix the second and break every instance of the first by its full
-- value (ADR-0053, "What the engine actually cannot see").
--
-- DEFAULT 'acquired' reproduces today's behaviour exactly for every existing
-- row, so no month changes without a deliberate act, and the mass-onboarding
-- case is already covered by the engine's first-month baseline suppression. The
-- data correction — flipping already-onboarded Positions to 'newly_tracked' — is
-- applied by hand to the two live Households, not backfilled here (ADR-0053 §7,
-- following ADR-0052 §8).
ALTER TABLE public.assets
    ADD COLUMN entry_type text NOT NULL DEFAULT 'acquired',
    ADD CONSTRAINT assets_entry_type_check
        CHECK (entry_type = ANY (ARRAY['acquired'::text, 'newly_tracked'::text]));

ALTER TABLE public.liabilities
    ADD COLUMN entry_type text NOT NULL DEFAULT 'acquired',
    ADD CONSTRAINT liabilities_entry_type_check
        CHECK (entry_type = ANY (ARRAY['acquired'::text, 'newly_tracked'::text]));

ALTER TABLE public.receivables
    ADD COLUMN entry_type text NOT NULL DEFAULT 'acquired',
    ADD CONSTRAINT receivables_entry_type_check
        CHECK (entry_type = ANY (ARRAY['acquired'::text, 'newly_tracked'::text]));

ALTER TABLE public.investments
    ADD COLUMN entry_type text NOT NULL DEFAULT 'acquired',
    ADD CONSTRAINT investments_entry_type_check
        CHECK (entry_type = ANY (ARRAY['acquired'::text, 'newly_tracked'::text]));

-- 'untracked' is the exit-side declaration: the Position left the Household's
-- books without being sold, paid off, collected or lost (ADR-0053 §3). It is the
-- one terminal status available to *every* group — including Investment, which
-- ADR-0052 §5 deliberately gave no write-off status, because a departing
-- member's portfolio did not lose its value and booking it as a large negative
-- Investment Return is the same class of falsehood #576 complained about
-- (ADR-0053 §5 amends ADR-0052 §5 in that one place).
--
-- The status/terminated_at biconditional (migration 00012) applies unchanged:
-- 'untracked' is terminal, so it carries a termination date.
ALTER TABLE public.assets
    DROP CONSTRAINT assets_status_check,
    ADD CONSTRAINT assets_status_check
        CHECK (status = ANY (ARRAY['active'::text, 'closed'::text, 'sold'::text, 'disposed'::text, 'untracked'::text]));

ALTER TABLE public.liabilities
    DROP CONSTRAINT liabilities_status_check,
    ADD CONSTRAINT liabilities_status_check
        CHECK (status = ANY (ARRAY['active'::text, 'paid_off'::text, 'forgiven'::text, 'written_off'::text, 'untracked'::text]));

ALTER TABLE public.receivables
    DROP CONSTRAINT receivables_status_check,
    ADD CONSTRAINT receivables_status_check
        CHECK (status = ANY (ARRAY['active'::text, 'collected'::text, 'written_off'::text, 'untracked'::text]));

ALTER TABLE public.investments
    DROP CONSTRAINT investments_status_check,
    ADD CONSTRAINT investments_status_check
        CHECK (status = ANY (ARRAY['active'::text, 'sold'::text, 'matured'::text, 'untracked'::text]));

-- +goose Down
ALTER TABLE public.investments
    DROP CONSTRAINT investments_status_check,
    ADD CONSTRAINT investments_status_check
        CHECK (status = ANY (ARRAY['active'::text, 'sold'::text, 'matured'::text]));

ALTER TABLE public.receivables
    DROP CONSTRAINT receivables_status_check,
    ADD CONSTRAINT receivables_status_check
        CHECK (status = ANY (ARRAY['active'::text, 'collected'::text, 'written_off'::text]));

ALTER TABLE public.liabilities
    DROP CONSTRAINT liabilities_status_check,
    ADD CONSTRAINT liabilities_status_check
        CHECK (status = ANY (ARRAY['active'::text, 'paid_off'::text, 'forgiven'::text, 'written_off'::text]));

ALTER TABLE public.assets
    DROP CONSTRAINT assets_status_check,
    ADD CONSTRAINT assets_status_check
        CHECK (status = ANY (ARRAY['active'::text, 'closed'::text, 'sold'::text, 'disposed'::text]));

ALTER TABLE public.investments  DROP COLUMN entry_type;
ALTER TABLE public.receivables  DROP COLUMN entry_type;
ALTER TABLE public.liabilities  DROP COLUMN entry_type;
ALTER TABLE public.assets       DROP COLUMN entry_type;

ALTER TABLE public.monthly_reports
    DROP COLUMN tracking_changes,
    DROP COLUMN tracking_change_positions;
