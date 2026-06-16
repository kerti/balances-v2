# Zone: BACKUP

Whole-Household backup — the versioned `.json.gz` export (ADR-0036, issue #174).
Distinct from EXPORT (the per-position workbook): this is the *entire* Household
as one portable artifact for disaster recovery and SaaS↔self-host portability.
The defining risks are a **cross-tenant leak** (the export reads ~22 tables, each
of which must stay household-scoped), **silent precision corruption** (decimals
must ride the wire as strings, ADR-0011), and a **format-contract break** (the
parents-before-children section order is frozen so a future importer can stream
the file in one pass). Restore (preview→commit, wipe-then-load) and the
format-version transform chain arrive in later slices (#175/#177) and will extend
this zone. Code: `internal/backup/{format,export}.go`, `queries/backup.sql`; the
frontend export lives in `components/BackupCard.tsx` + `lib/backup.ts`.

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-BACKUP-01 | Export is household-scoped: every section read is keyed by the caller's `HouseholdID` (top-level tables directly; detail/snapshot/ledger tables via a join to their parent on `household_id`), so a backup contains exactly one Household's rows and never another's — there is no export-only path around ADR-0005 row-level tenancy | ADR-0005 | Critical |
| INV-BACKUP-02 | Fidelity is honored exactly: a **full** backup carries soft-deleted rows verbatim with `deleted_at` intact (an exact round-trip); a **compacted** backup carries live rows only, and detail/snapshot/ledger liveness follows the parent so no orphaned history of a deleted position leaks into a compacted file. Backup never resurrects soft-deleted rows | ADR-0007, ADR-0036 | High |
| INV-BACKUP-03 | Monetary, quantity, and FX values serialize as JSON **strings**, never bare numbers, so decimal precision survives the round-trip (no IEEE-754 corruption) | ADR-0011 | Critical |
| INV-BACKUP-04 | The envelope is **parents-before-children**: household → users → tags → positions → their detail → snapshots → transactions → income → fx. Every child section follows its parent. This order is a frozen part of the `format_version: 1` contract — it is what lets a future importer stream the file in one pass — and may not change without a format bump | ADR-0036 | High |
| INV-BACKUP-05 | The artifact is a gzip stream delivered as `household-backup-<date>.json.gz`; the client derives the save name from `Content-Disposition`, falling back to a date-stamped default when the header is absent or unusual | ADR-0036 | Medium |
