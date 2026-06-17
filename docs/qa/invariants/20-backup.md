# Zone: BACKUP

Whole-Household backup — the versioned `.json.gz` export (ADR-0036, issue #174).
Distinct from EXPORT (the per-position workbook): this is the *entire* Household
as one portable artifact for disaster recovery and SaaS↔self-host portability.
The defining risks are a **cross-tenant leak** (the export reads ~22 tables, each
of which must stay household-scoped), **silent precision corruption** (decimals
must ride the wire as strings, ADR-0011), and a **format-contract break** (the
parents-before-children section order is frozen so a future importer can stream
the file in one pass). Restore (preview→commit, wipe-then-load) and the
format-version transform chain arrive across #175/#177 and extend this zone; the
destructive wipe-then-load commit lands in `restore_commit.go` (its HTTP
preview→commit wrapper and the restore UI follow). Code:
`internal/backup/{format,export,restore,restore_commit}.go`, `queries/backup.sql`;
the frontend export lives in `components/BackupCard.tsx` + `lib/backup.ts`.

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-BACKUP-01 | Export is household-scoped: every section read is keyed by the caller's `HouseholdID` (top-level tables directly; detail/snapshot/ledger tables via a join to their parent on `household_id`), so a backup contains exactly one Household's rows and never another's — there is no export-only path around ADR-0005 row-level tenancy | ADR-0005 | Critical |
| INV-BACKUP-02 | Fidelity is honored exactly: a **full** backup carries soft-deleted rows verbatim with `deleted_at` intact (an exact round-trip); a **compacted** backup carries live rows only, and detail/snapshot/ledger liveness follows the parent so no orphaned history of a deleted position leaks into a compacted file. Backup never resurrects soft-deleted rows | ADR-0007, ADR-0036 | High |
| INV-BACKUP-03 | Monetary, quantity, and FX values serialize as JSON **strings**, never bare numbers, so decimal precision survives the round-trip (no IEEE-754 corruption) | ADR-0011 | Critical |
| INV-BACKUP-04 | The envelope is **parents-before-children**: household → users → tags → positions → their detail → snapshots → transactions → income → fx. Every child section follows its parent. This order is a frozen part of the `format_version: 1` contract — it is what lets a future importer stream the file in one pass — and may not change without a format bump | ADR-0036 | High |
| INV-BACKUP-05 | The artifact is a gzip stream delivered as `household-backup-<date>.json.gz`; the client derives the save name from `Content-Disposition`, falling back to a date-stamped default when the header is absent or unusual | ADR-0036 | Medium |
| INV-BACKUP-06 | Restore refuses a backup whose `format_version` is **newer** than this build speaks (`ErrFormatTooNew`) rather than guessing, and rejects a sub-1 version as invalid; an older version is migrated forward through the registered transform chain (identity at v1) | ADR-0036 | High |
| INV-BACKUP-07 | Restore verifies integrity before any load: a truncated/corrupt gzip stream (CRC) is rejected (`ErrCorruptBackup`), and every declared per-section count must match the payload or the file is rejected | ADR-0036 | High |
| INV-BACKUP-08 | Restore validates the whole object graph before commit — every position is in the backup's household and every owner/tag/parent reference resolves within the payload (no dangling FK), and the **caller must be a member** of the backup's household (matched by `google_sub` only — email is mutable/reassignable and must not gate a destructive restore) or it is refused (`ErrNotMemberOfBackup`) | ADR-0005, ADR-0017, ADR-0036 | Critical |
| INV-BACKUP-09 | Restore commit is **all-or-nothing**: the wipe (caller's current Household, children→parents, incl. sessions/invitations/derived reports not in the backup) and the verbatim load run in one transaction, so any failure rolls back and the caller's data is left exactly as it was ("nothing was changed") | ADR-0036 | Critical |
| INV-BACKUP-10 | Restore loads the backup **verbatim, adopting the backup's Household UUID** — a full-fidelity export→restore→re-export is an exact round-trip (every section count and soft-deleted row preserved). The load never touches another Household's rows (cross-tenant isolation holds through the destructive path) | ADR-0005, ADR-0036 | Critical |
