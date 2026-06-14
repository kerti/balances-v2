# Zone: IMPORT

> _Seeded next — bulk ingestion, the widest single write surface in the app and
> the one most able to corrupt many rows at once. Two flavours: **snapshot
> import** (already partly guarded — INV-SNAPSHOTS-04 covers the per-month upsert)
> and **create-with-snapshots fan-out** (#88/#89), where one transaction creates
> a position, resolves+assigns its tag, and seeds its whole snapshot history.
> Code: `internal/snapshotimport/importer.go` + `internal/importcreate/importcreate.go`
> (parsing/validation), `internal/repo/import_snapshots.go` and the per-group
> `*_import_create.go` / fan-out helpers (the writes). Candidate invariants the
> SNAPSHOTS zone did NOT already claim: **dry-run/commit parity** — the preview's
> insert-vs-update classification matches what commit actually does, and dry-run
> writes nothing; **all-or-nothing** — a malformed row aborts the whole batch in
> one transaction, no partial write; **per-row shape validation surfaces in the
> dry-run preview** (the `ValidateSeedTransaction` path, so a bad investment-txn
> row never reaches commit); **cross-household import is `ErrNotFound`**, never a
> silent write into someone else's position (overlaps TENANCY but the import entry
> point is its own guard); **the fan-out is atomic** — position + tag + every
> snapshot commit together or not at all. Severity Critical — a partial or
> misclassified import is silent data corruption. Annotation targets:
> `import_create_test.go`, `import_create_fanout_test.go`, `import_meta_test.go`,
> `investment_import_create_test.go`, `importcreate_test.go`, `importer_test.go`;
> a frontend import-preview vitest if one exists. Survey those before writing new
> tests; fill this table when seeding the zone._
