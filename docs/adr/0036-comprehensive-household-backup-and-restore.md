# Comprehensive household backup and restore

A Household can **export its entire data as a single versioned `.json.gz` artifact** and **restore
it into a fresh or wiped-clean Household on any instance**. This is full-on disaster recovery and
SaaS↔self-host portability — *not* a merge tool. The artifact is a **logical** JSON export (decoupled
from the physical schema), carries its own standalone `format_version`, re-links members by Google's
stable `sub`, and restores via a **streaming, preview→commit, single-transaction wipe-then-load**.
Backup-format immutability begins at the first production release, by direct analogy to [[adr-0033]]'s
migration-immutability line. Builds on [[adr-0005]] (row-level tenancy), [[adr-0007]] (soft-delete),
[[adr-0011]] (decimals as strings on the wire), [[adr-0017]] (Google OAuth identity), and [[adr-0033]]
(self-hostable; the upgrade contract).

Status: accepted.

## Why now

[[adr-0033]] settled the distribution model as **self-hostable** and named the operator as the
consumer of the SemVer upgrade contract. The moment Balances runs on a stranger's mini-PC or
Raspberry Pi, "my house burned down, restore from the ground up" becomes a first-class requirement,
and the SaaS↔self-host cutover (either direction) needs an instance-portable artifact. The format
and identity decisions here are hard to reverse once a *released* build has produced a backup file in
a real user's hands, so — exactly as [[adr-0033]] argued for migrations — the rules must exist before
the first such file does.

## The decision

### Purpose: disaster recovery + portability, not merge

The backup answers three needs with one artifact:

- **Disaster recovery** for self-hosters — a device bricks or burns; restore the whole Household onto
  a fresh install.
- **Portability** both directions — SaaS→self-host (a hosted user takes their data home) and
  self-host→SaaS (a self-hoster offloads the ops burden).
- **"Download my data"** peace-of-mind / takeout — falls out for free from the export artifact.

**Merge is explicitly out of scope.** Restore targets a **fresh or wiped-clean** Household, never a
populated one it has to reconcile. Importing *some* data into a *live* Household is already served by
the per-position XLSX import (`importcreate`); that is a different feature with a different shape and
is not touched here.

### A logical JSON export, not a physical dump

The artifact is **JSON**, gzipped on disk as `household-backup-<date>.json.gz`:

- **Rejected `pg_dump`/SQL** — it couples the backup to the *physical* schema version, can't ride the
  format-version transform chain (you'd be rewriting SQL), isn't human-inspectable, needs `psql`, and
  leaks instance internals (sequences, derived marts). A logical export is the opposite of a physical
  dump by design.
- **Rejected Protobuf/Avro/SQLite/binary** — not human-inspectable (kills "download my data"), needs
  tooling on a self-host box, and transforms are far easier on a self-describing tree. Overkill for a
  KB–low-MB payload.
- **Rejected NDJSON/JSONL** — line-orientation makes the version header and tree-shaped transforms
  awkward; revisit only if exports ever became huge (they won't).
- **Rejected CSV/XLSX** — can't carry heterogeneous entities + relationships; already the merge tool's
  format.

Three riders, each from an existing decision:

1. **Decimals are strings, never JSON numbers** ([[adr-0011]]) — IEEE-754 would silently corrupt
   amounts/quantities/FX. `shopspring/decimal` already string-marshals.
2. **Gzip is the artifact** — for a downloaded file the artifact *is* the transport; `.json.gz` keeps
   it small while `gunzip` preserves inspectability. The importer transparently accepts `.json.gz`
   **and** plain `.json`.
3. **Integrity is gzip CRC32 + per-section declared counts** asserted during the streaming load
   (expected 108,000 snapshots, got 107,999 → hard fail). A global content hash fights single-pass
   streaming and is dropped; an optional `.sha256` sidecar can serve a paranoid operator. The threat
   model is accidental corruption/truncation, not adversarial tampering.

### Payload scope

**In, verbatim:** Household settings (`display_name`, `reporting_currency`, `multi_currency_enabled`,
locale/tz defaults); all Positions across the four groups + investment subtypes with every field
(`status`, `terminated_at`, `termination_note`, `ownership`, tag assignment); Tags; all Snapshots
(every per-group table); all Transactions; all Income; FX rates; Users (incl. `google_sub` — see
identity below). **Original UUIDs are preserved verbatim** — restore targets an empty DB, so there are
no collisions, and keeping PKs intact preserves every foreign key (snapshot→position, tag→position,
txn→position, ownership→user) for free and makes the round-trip exact.

**Out:** materialized monthly reports ([[adr-0006]]) — derived, **recomputed** on restore rather than
shipping stale marts; sessions and pending email invitations — ephemeral auth state.

**Soft-deleted rows ([[adr-0007]]) are an export-time choice:** *Full fidelity* carries them with
`deleted_at` intact (exact round-trip); *Compacted* ships live rows only (a clean snapshot of current
truth). The UI explains the consequence. This is safe because every uniqueness index is partial
(`WHERE deleted_at IS NULL`), so a soft-deleted row and its live successor already coexist in the
source DB; restoring both verbatim reproduces that state with zero new collision. Backup never
*resurrects* soft-deleted rows — that (and its attendant unique-constraint collisions) is the parked
in-app Recycle Bin, a separate feature. **Import is always full fidelity** — it faithfully reproduces
whatever the file contains; a compacted file simply has no deleted rows to carry.

### Identity re-link via Google's stable `sub`

Google's OIDC `sub` is globally stable per Google account across OAuth clients ([[adr-0017]]), so the
same person signing into a *different* instance (with a *different* OAuth client ID) presents the
**same `google_sub`**. The backup therefore carries Users verbatim including `google_sub`, and the
re-link is automatic:

- The member who restores is matched to their backup User row by `google_sub` (email is the fallback
  key for a future non-Google IdP).
- **Non-founder members are restored in place** as full members. They are **not** re-invited —
  invitations are ephemeral and excluded; the member already exists in the data. On their next
  sign-in to the new instance their `google_sub` matches and they land straight in the Household. If
  they never sign in, their data still lives there; they simply can't authenticate until they do.
- **Founder is lineage only**, never a privilege ([[adr-0017]] / CONTEXT) — so the restorer need not
  be the Founder; any carried member qualifies.

### Restore semantics: in-app, preview→commit, atomic wipe-then-load

Restore is an **in-app, authenticated** action (it serves both portability directions; a hosted user
has no shell). On a fresh instance the restorer first signs in — `createFounder` bootstraps a
throwaway empty Household — then uploads the backup, which **replaces** it.

- **Two phases, stateless re-upload.** *Preview* uploads the file, streams-validates it (transform to
  current `format_version`, CRC + counts, full-graph referential integrity, the membership guard),
  and returns a summary ("erase X, load Y") **writing nothing and keeping nothing** server-side.
  *Commit* re-uploads the same file plus a client-supplied confirm token (the typed Household name)
  and **re-runs the full validation from scratch** before writing. No server-side pending-restore
  state to store, expire, or leak; commit never trusts that preview passed; there is no TOCTOU window
  because commit wipes-then-loads atomically against whatever exists at commit time.
- **Commit is one transaction**: delete the current Household's rows **then** insert the backup
  verbatim. A half-restored Household is the one outcome never permitted. Delete-then-insert within
  the transaction dodges the `users_google_sub_idx` collision between the bootstrap row and the
  backup's matching row.
- **Domain invariants are re-asserted on load, not blindly trusted.** The DB CHECKs (snapshot
  `as_of_date`-within-month, time-deposit term bounds, partial-unique indexes) are the backstop, but
  the repo layer validates too and **fails loudly** on any violation — a forward-migrated file should
  have been brought into compliance by its transform, so a violation means the transform or the file
  is wrong, and we want a hard error rather than a silent partial load. The transform chain owns
  "make old data satisfy new invariants."

**Gating** is **type-to-confirm + membership** (the caller's `sub`/email must be in the backup),
**not Founder-role** — inventing a privilege tier the domain explicitly refuses ([[adr-0017]]). The
membership guard also stops anyone loading a stranger's backup and walking into it.

### Streaming, and a parents-before-children file layout

A worst-case Household (≈30 years × ≈300 positions ≈ 216k snapshot/txn rows ≈ 75 MB JSON / ≈9 MB
gzip) would cost ≈200–400 MB heap to load whole — enough to OOM a 256 MB Fly machine or a tight Pi,
the exact self-host target. So:

- **Export is gzip-streamed at the HTTP layer**, and the envelope's section order is the firm
  contract (below). The first implementation (#174) *assembles* the Household in memory and then
  encodes it — `O(Household)` memory, fine for realistic KB-scale data — rather than per-table cursor
  batching; **constant-memory batched export is a documented refinement** deferred until a Household is
  large enough to need it. The parents-before-children layout is honored in the output regardless, so
  this never requires a format break.
- **The file is laid out parents-before-children** (`household → users → tags → positions → snapshots
  → transactions → income → fx`). This is the load-bearing contract decision: it lets import **stream
  in a single pass** holding only the small parent-id sets (hundreds of UUIDs, <1 MB) plus the current
  insert batch, validating each child's FK against the resident parent sets and feeding rows straight
  into the open transaction. Peak RAM is `O(parents + batch)` ≈ a few MB even for the 75 MB file. The
  ordering is committed now even if a first importer implementation buffers, so streaming never
  requires a format break.
- A **documented soft size cap** is defense-in-depth (reject absurd uploads with a clear error).

### Format versioning: standalone, transform-chained, immutable at release

- **`format_version` is a standalone monotonic integer** in the envelope, **decoupled** from the
  goose migration number and the app SemVer. [[adr-0033]] versions the *operator upgrade contract*;
  the backup is a third surface and bumps only when the backup *shape* changes.
- **Backwards compatibility = a forward-migration chain on the payload.** Importing a `format_version:
  N` file into an app speaking `M > N` runs `N→N+1→…→M` transforms in memory before loading —
  goose-shaped, but for the JSON envelope. A **newer/unknown version is refused**, not guessed.
- **Immutability begins at the first production release**, by analogy to [[adr-0033]]: a transform
  becomes sacred only once a *non-resettable artifact* exists at that version — a backup saved from a
  *released* build. Pre-release, dev/preview backups are re-exportable (resettable), so the format
  chain is **squashable**, and the last in-repo squash collapses it to a clean `format_version: 1` at
  `v1.0.0`. **Exception, stricter than goose:** a **destructive** format change (one that can't
  represent something an older version could) **always bumps and ships an `N→N+1` transform — never
  squash-in-place, even pre-release** — because no-loss round-trip is the feature's entire reason to
  exist. Additive/cosmetic diffs squash freely pre-release.
- **The seam is proven by tests, not speculative product code.** Shipped product stays at
  `format_version: 1` until a real change forces `2`; the **machinery + a fixture-locked
  backwards-compat harness ship now**, and a **test-only synthetic v1→v2 transform** loads a frozen
  golden v1 fixture through a v2-configured importer (the genuine "v1 file into v2 system" proof) — in
  the test suite, where throwaway code belongs. The first *real* format-changing feature during the
  alpha ramp becomes the genuine `v2` and exercises the chain for real. Process commitment: every
  future format change ships its transform **and** a frozen golden fixture, enforced by a test that
  loads every historical fixture.

### Notifications: best-effort, localized

On a successful restore, best-effort emails fire (never blocking the restore — a fresh self-host may
have no SMTP configured; mirrors the welcome-email pattern), localized per recipient's carried
`locale`:

- **To the restorer** — a *restore complete* confirmation: the instance URL + summary counts, a
  sanity check that the load matched expectations.
- **To other live members** — a *relocation + security notice*: "your Household was restored to a new
  instance at `<url>` by `<member>` on `<date>`; sign in here; if unexpected, secure your account."
  This both tells members the URL changed and is a tamper tripwire. Soft-deleted users are not
  emailed.

## Presentation / UX

Per [[adr-0034]], the UI of a backend decision is documented here, not in a separate ADR — and for a
non-technical audience this presentation *is* the feature's guardrails, so it's a correctness concern.
This section is the basis for the BACKUP/RESTORE UI invariants in the QA matrix.

- **Home:** Settings → Data, one section each for Backup and Restore, each with a plain-language
  one-liner (what it is; that restore is destructive). No jargon, no raw error codes anywhere.
- **State-adaptive prominence.** When the current Household is **empty/fresh** (the burned-house /
  new-Raspberry-Pi first-run), Restore is surfaced **prominently** (*"New here? Restore a backup to
  bring your data in."*) and Export is **hidden behind an empty-state hint** (*"Nothing to back up yet
  — add some data first."*) until there's backup-worthy data. When the Household is **populated**, the
  Restore section **leads with its destructive framing** (*"Restoring replaces everything in this
  household"*) before the file picker, so the stakes are set before a file is chosen.
- **Export feedback** is **indeterminate** (size is unknown up front — no fake percentage): button
  disabled + *"Preparing your backup…"*, the `.json.gz` download begins when the stream completes, a
  success toast ([[adr-0032]]) confirms, and copy notes large households take a moment. The
  **fidelity toggle** (full vs compacted) carries consequence-explaining copy at the point of choice.
- **Restore is preview → summary → confirm.** Upload → *"Checking your backup…"* → a summary screen
  that names **both** Households' (derived) names, the counts erased and loaded, and states the action
  is irreversible.
- **The confirmation gate is stakes-scaled.** Target Household **empty** → a single checkbox (*"I
  understand this replaces all data in this household"*) + button — nothing real to lose. Target
  **populated** → **type `ERASE` to confirm** (a constant word, localized; the typed input is compared
  against the **localized** word) — friction matched to irreversible loss. `ERASE` is chosen over
  `RESTORE` (which sounds safe and defeats the guard) and over `REPLACE` (softer) because it echoes the
  warning sentence above it. The derived Household name is **displayed** for context but never the
  thing typed (it is auto-derived and never surfaced elsewhere — see parking lot).
- **Commit (the long, dangerous op):** controls disabled + *"Restoring — don't close this window."*,
  **double-submit blocked**, **navigation blocked** until done. Success → success screen + toast
  (emails fire best-effort). Failure → the transaction rolled back, surfaced as *"Restore failed;
  nothing was changed. Your current data is intact."* — making the atomic-rollback guarantee visible
  so a failure doesn't read as data loss.
- **Error states** ride the [[adr-0027]] `{code, args}` envelope → localized, actionable, never
  silent: `INVALID_BACKUP_FILE`, `CORRUPT_BACKUP`, `BACKUP_FORMAT_TOO_NEW` (the refuse-newer guard made
  visible — names the version to upgrade to when known), `NOT_A_MEMBER_OF_BACKUP` (the membership guard
  made visible), `BACKUP_VALIDATION_FAILED`. Preview failures touch nothing and commit failures roll
  back, so messages reassure *"nothing was changed"* where true.

## Considered alternatives

- **`pg_dump` / physical SQL dump** — rejected; couples to the physical schema, no transform chain,
  not portable across schema evolution. (Above.)
- **Binary formats (Protobuf/Avro/SQLite)** — rejected; not human-inspectable, tooling burden.
- **Merge-on-import into a populated Household** — rejected as out of scope; the per-position XLSX
  import already covers partial imports, and merge drags in ID-collision, dedup, and identity
  reconciliation the DR/portability use cases don't need.
- **Founder-only restore** — rejected; the domain grants every member equal access and treats Founder
  as lineage, not privilege. Gating is confirm + membership.
- **Ship a synthetic `v2` through the product binary to prove the chain, then squash it** — rejected
  as motion without information; the squash deletes the only worked example, so the seam's first real
  exercise would still be post-`1.0.0`. The proof lives in tests instead.
- **A global SHA-256 over the payload** — rejected; it fights single-pass streaming. Gzip CRC +
  declared counts + an optional sidecar cover the accidental-corruption threat model.
- **Operator CLI restore as part of core** — deferred to the parking lot; in-app restore already
  serves both portability directions.

## Consequences

- **No schema migration.** Backup reads existing tables; restore wipes + inserts into them. This is an
  **additive feature** → a **minor** bump under [[adr-0033]], drop-in for operators.
- **The parents-before-children layout is a frozen part of the `format_version: 1` contract** — child
  sections may never precede their parents without a format bump.
- **A new family of QA invariants** (a BACKUP/RESTORE zone) is seeded: exact round-trip, atomic
  all-or-nothing restore, membership-gated restore, no-resurrection of soft-deleted rows, refuse
  newer/unknown `format_version`, best-effort locale-correct notifications.
- **Self-host (#116) gains a concrete pillar** — the upgrade contract of [[adr-0033]] is only credible
  if an operator can also get their data *out* and *back*.
- **`docs/parking-lot.md` is introduced** as the home for the deferred CLI, member Recycle Bin,
  standalone account/household deletion, and async restore.
