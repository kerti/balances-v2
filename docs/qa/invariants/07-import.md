# Zone: IMPORT

Bulk ingestion — the widest single write surface in the app, and the one most
able to corrupt many rows at once. The flow has a shared spine in
`internal/importcreate/importcreate.go` (`Run` / `RunWithLedger`): parse the
workbook, resolve the Detail-sheet conventions, validate every row, and gate a
write behind an explicit `mode=commit`. Parsing lives in the DB-free
`internal/snapshotimport/importer.go`; the atomic writes are the per-group
`CreateXWithSnapshots` / `…AndLedger` repo methods. The defining risk is a
**partial or misclassified** import: a preview that promises one thing and a
commit that does another, or a batch that writes half its rows before failing.
The per-month snapshot upsert is already INV-SNAPSHOTS-04, and cross-household
rejection is INV-TENANCY/INV-SNAPSHOTS-04 — this zone guards the
preview/commit/atomicity machinery on top of them.

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-IMPORT-01 | Preview/commit parity, dry-run is a no-op: `mode=preview` (default) validates and reports the same insert/update (and ledger) counts a commit would apply, but never calls the commit func — it writes nothing | ADR-0022 | Critical |
| INV-IMPORT-02 | All-or-nothing: a workbook carrying any field or row error is refused at commit (422) with the commit func never called, and a mid-batch repo failure (e.g. a snapshot row whose shape doesn't match the subtype) rolls the whole transaction back — no position, tag, or snapshot left behind | ADR-0022 | Critical |
| INV-IMPORT-03 | Create-with-snapshots fan-out is atomic and complete: one transaction writes the position, its resolved tag (or untagged when none), and every seeded snapshot together — across all five position groups and the investment ledger variant | ADR-0022 | Critical |
| INV-IMPORT-04 | Detail-sheet identity resolution: a sole-owner email resolves to a household user id and a tag name to a tag id; an unresolvable email or tag is a `FieldError` (blocks `would_create`, fails commit 422) — never a silent write and never a 500 | ADR-0022 | High |
| INV-IMPORT-05 | Per-row parsing is strict: a malformed month/number/value-shape is a `RowError` (not a silent skip or a coerced value), genuinely blank rows are skipped, a duplicate month is flagged, and an exported workbook round-trips back through Parse unchanged | ADR-0022 | High |
| INV-IMPORT-06 | The client drop zone accepts only a spreadsheet: a file is `.xlsx` by extension (case-insensitive) or by the spreadsheet MIME, a multi-file drop takes the first, a non-xlsx drop is rejected `invalid` and an empty drop is `empty` — a frontend-native guardrail before any upload (the backend's "not a spreadsheet is 400" is the server-side twin) | ADR-0022 | Medium |
| INV-IMPORT-07 | Import entry points are ownership-gated: the template-download/meta and import paths resolve a position only within the caller's household; an unknown or cross-household id is `ErrNotFound` (404) — never a template leak or a write into another household's position (the import-side face of INV-TENANCY) | ADR-0022, ADR-0005 | High |
