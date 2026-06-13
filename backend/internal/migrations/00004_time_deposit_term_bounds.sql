-- +goose Up
-- A time deposit's term is the window [placement_date, maturity_date]: the
-- principal is locked at placement and released at maturity. A maturity that
-- is not strictly after placement is nonsense (issue #62). This is the single-
-- table half of the rule the DB can enforce; the cross-table confinement of a
-- deposit's snapshots and transactions into this same window lives in the repo
-- layer (a CHECK can't reach time_deposit_details from the snapshot/transaction
-- tables). Added NOT VALID so the ALTER does not scan pre-existing rows that may
-- predate this rule (alpha data); every INSERT/UPDATE from here on is enforced.
ALTER TABLE public.time_deposit_details
    ADD CONSTRAINT time_deposit_details_maturity_after_placement
    CHECK (maturity_date > placement_date) NOT VALID;

-- +goose Down
ALTER TABLE public.time_deposit_details DROP CONSTRAINT IF EXISTS time_deposit_details_maturity_after_placement;
