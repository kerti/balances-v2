-- +goose Up
-- Backfill for #575: before the delete path cascaded, SoftDelete<Group> stamped
-- the parent row only, leaving live snapshots/transactions hanging off a
-- tombstoned position. Every named read path re-derives child visibility by
-- joining the parent, so these orphans were invisible — but five child queries
-- take an ID array with no parent join at all, and the next consumer that
-- sourced IDs from anywhere but a household-scoped live list would have
-- resurrected them (INV-SOFT-DELETE-03's leaked tombstone). The code fix
-- removes the class going forward; this cleans the rows already written.
--
-- Each child inherits its parent's deleted_at verbatim rather than now(), so
-- the tombstone reads as what it always should have been — the moment the
-- position was deleted — instead of the moment this migration ran.
--
-- updated_at is deliberately left alone. It is a monthly-report staleness input
-- (MaxReportInputUpdatedAt, whose snapshot subqueries do not filter deleted_at),
-- so touching it would mark every report in every household stale and force a
-- mass regen for rows that are already excluded from the gather and change no
-- number on screen.
--
-- Additive + idempotent: it only fills deleted_at where it is NULL under an
-- already-deleted parent, so re-running is a no-op.

UPDATE public.asset_snapshots s
SET deleted_at = a.deleted_at,
    updated_by = a.updated_by
FROM public.assets a
WHERE s.asset_id = a.id
  AND a.deleted_at IS NOT NULL
  AND s.deleted_at IS NULL;

UPDATE public.liability_snapshots s
SET deleted_at = l.deleted_at,
    updated_by = l.updated_by
FROM public.liabilities l
WHERE s.liability_id = l.id
  AND l.deleted_at IS NOT NULL
  AND s.deleted_at IS NULL;

UPDATE public.receivable_snapshots s
SET deleted_at = r.deleted_at,
    updated_by = r.updated_by
FROM public.receivables r
WHERE s.receivable_id = r.id
  AND r.deleted_at IS NOT NULL
  AND s.deleted_at IS NULL;

UPDATE public.investment_snapshots s
SET deleted_at = i.deleted_at,
    updated_by = i.updated_by
FROM public.investments i
WHERE s.investment_id = i.id
  AND i.deleted_at IS NOT NULL
  AND s.deleted_at IS NULL;

UPDATE public.investment_transactions t
SET deleted_at = i.deleted_at,
    updated_by = i.updated_by
FROM public.investments i
WHERE t.investment_id = i.id
  AND i.deleted_at IS NOT NULL
  AND t.deleted_at IS NULL;

-- +goose Down
-- Irreversible by design: once a child carries its parent's deleted_at there is
-- nothing left to distinguish a row this backfill tombstoned from one the user
-- deleted by hand in the same second, and guessing wrong would resurrect a
-- deliberately deleted snapshot. The rows are invisible to every read path
-- either way, so the down migration is a no-op rather than a reconstruction.
SELECT 1;
