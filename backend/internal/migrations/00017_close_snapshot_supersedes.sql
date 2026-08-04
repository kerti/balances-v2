-- +goose Up
-- Make the close-snapshot displacement link explicit (ADR-0052 §2, issue #602).
--
-- A terminal flip writes a truthful 0-value close snapshot at the termination
-- month and *displaces* whatever the user had recorded there: the recorded row
-- is soft-deleted and the close row takes its place, so un-terminate can hand
-- the recorded value back (INV-LIFECYCLE-04). Until now the two rows were
-- paired by coincidence — the archive UPDATE and the close INSERT share one
-- transaction, so the displaced row's deleted_at equals the close row's
-- created_at exactly — plus an `amount <> 0` filter to skip a close row left
-- archived by an earlier terminate/un-terminate cycle.
--
-- That pairing is inferred, and it made the displaced row indistinguishable
-- from a row the user threw away. A *compacted* backup (ADR-0036 — live rows
-- only) therefore dropped it while keeping the live close row, and a household
-- restored from such a file read a carried-forward value from an earlier month
-- on the next undo, silently. The same misreading would have handed the row to
-- the parked Recycle Bin's "empty trash" and to any future purge.
--
-- supersedes declares the link instead of inferring it. It lives on the *close*
-- row, pointing at the row that close row displaced, because that is the
-- direction the write order allows: the partial unique index
-- (position_id, year_month) WHERE deleted_at IS NULL means the displaced row
-- must be archived *before* the close row is inserted, so only the second write
-- can carry the other's id. It also collapses the un-terminate lookup — the
-- close row is already read to tell it apart from a value the user re-recorded
-- while the Position was terminated, and now that same read yields the row to
-- restore, so the four GetArchived*SnapshotAtMonth queries and their
-- amount-based heuristic go away.
--
-- Nullable, so it is not a backup-format change: a pre-#602 file has no such
-- key and restores as NULL, which is exactly right for a snapshot that
-- displaced nothing. (A pre-#602 *compacted* file cannot be repaired — the rows
-- it needed were never written to it.) The self-reference is within one table,
-- which the restore's single bulk INSERT ... json_populate_recordset handles
-- the same way it already handles investments.rolled_from_investment_id:
-- Postgres checks referential integrity at statement end.
ALTER TABLE public.asset_snapshots
    ADD COLUMN supersedes uuid REFERENCES public.asset_snapshots(id);
ALTER TABLE public.liability_snapshots
    ADD COLUMN supersedes uuid REFERENCES public.liability_snapshots(id);
ALTER TABLE public.receivable_snapshots
    ADD COLUMN supersedes uuid REFERENCES public.receivable_snapshots(id);
ALTER TABLE public.investment_snapshots
    ADD COLUMN supersedes uuid REFERENCES public.investment_snapshots(id);

-- Partial: only close rows carry the link, and they are a rounding error next
-- to the snapshot history. The index serves the compacted-export lookup ("is
-- this archived row superseded by a live close row?") and the FK's own
-- referential checks.
CREATE INDEX asset_snapshots_supersedes_idx
    ON public.asset_snapshots (supersedes) WHERE supersedes IS NOT NULL;
CREATE INDEX liability_snapshots_supersedes_idx
    ON public.liability_snapshots (supersedes) WHERE supersedes IS NOT NULL;
CREATE INDEX receivable_snapshots_supersedes_idx
    ON public.receivable_snapshots (supersedes) WHERE supersedes IS NOT NULL;
CREATE INDEX investment_snapshots_supersedes_idx
    ON public.investment_snapshots (supersedes) WHERE supersedes IS NOT NULL;

-- Backfill from the rule the link replaces, so displacements that predate this
-- migration keep their undo. At most one row can match: `deleted_at` is a
-- transaction timestamp and only one live snapshot can exist per position-month,
-- so no two archived rows at the same month can share it.
UPDATE public.asset_snapshots c
SET supersedes = d.id
FROM public.asset_snapshots d
WHERE c.deleted_at IS NULL
  AND c.amount = 0
  AND d.asset_id = c.asset_id
  AND d.year_month = c.year_month
  AND d.deleted_at = c.created_at
  AND d.amount <> 0;

UPDATE public.liability_snapshots c
SET supersedes = d.id
FROM public.liability_snapshots d
WHERE c.deleted_at IS NULL
  AND c.amount = 0
  AND d.liability_id = c.liability_id
  AND d.year_month = c.year_month
  AND d.deleted_at = c.created_at
  AND d.amount <> 0;

UPDATE public.receivable_snapshots c
SET supersedes = d.id
FROM public.receivable_snapshots d
WHERE c.deleted_at IS NULL
  AND c.amount = 0
  AND d.receivable_id = c.receivable_id
  AND d.year_month = c.year_month
  AND d.deleted_at = c.created_at
  AND d.amount <> 0;

UPDATE public.investment_snapshots c
SET supersedes = d.id
FROM public.investment_snapshots d
WHERE c.deleted_at IS NULL
  AND c.amount = 0
  AND d.investment_id = c.investment_id
  AND d.year_month = c.year_month
  AND d.deleted_at = c.created_at
  AND d.amount <> 0;

-- +goose Down
ALTER TABLE public.investment_snapshots DROP COLUMN supersedes;
ALTER TABLE public.receivable_snapshots DROP COLUMN supersedes;
ALTER TABLE public.liability_snapshots  DROP COLUMN supersedes;
ALTER TABLE public.asset_snapshots      DROP COLUMN supersedes;
