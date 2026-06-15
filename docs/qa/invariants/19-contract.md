# Zone: CONTRACT

The **HTTP error-envelope contract** (`internal/httperr`, ADR-0027). Every 4xx/5xx
from an `internal/*` handler ships a typed `Envelope{Code, Args?}` the frontend
looks up in the react-i18next `errors:code.<CODE>` catalog (ADR-0026) — the wire
carries **no `message` field**, so a raw DB / internal error string never reaches
the client. This zone owns the **wire failure contract**: the same risk shape as a
leak (an unmapped error spilling schema/SQL internals) and a false-state (a
conflict that reads as success or the wrong status). It is the error twin of the
success-path zones. The OAuth callback redirect flow and the dev-only mock-OIDC
subcommand are the documented exceptions that keep plain `http.Error` bodies and
are out of scope here. Every row below is already test-backed in
`internal/httperr/httperr_test.go` — this was annotation work, not new tests.
Source: ADR-0027 (error envelope), ADR-0026 (the i18n catalog the codes key into).

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-CONTRACT-01 | Unknown error never leaks — `WriteRepo` maps an *unrecognised* error to `CodeInternal` / 500 and `slog.Error`s the raw `err` **server-side only**; the wire envelope carries the code with `Args` nil, never the raw string. The info-disclosure guard: a regression that echoed `err.Error()` to the client would leak schema / SQL / internal detail. `errs.ErrUnauthenticated` is deliberately *unmapped* → 500 (not 401), because every route is `RequireAuth`-gated so a userless repo call is a server bug, not a client 401. Covered by `TestWriteRepo_unknownErrorMapsToInternal` / `TestWriteRepo_unauthenticatedFallsThrough` | ADR-0027 | High |
| INV-CONTRACT-02 | Sentinel→code+status fidelity — each repo sentinel maps to its stable wire `Code` and the *correct* HTTP status (404 not-found, 409 conflict, 400 bad-shape), resolved through `errors.Is` so a wrapped sentinel (`fmt.Errorf("...: %w", errs.ErrNotFound)`) still maps. A conflict that read as 200/404 is a false-state. Covered by `TestWriteRepo_sentinelMapping` / `TestWriteRepo_wrappedSentinel` | ADR-0027 | High |
| INV-CONTRACT-03 | Envelope shape is code-only — `Write` emits `{"code": ...}` with `Args` **omitted when nil** (never `"args":null`) and no `message` field, the exact shape the i18n catalog lookup depends on; `Content-Type: application/json`. Covered by `TestWrite_envelope` / `TestWrite_argsOmittedWhenNil` | ADR-0027 / ADR-0026 | Medium |
| INV-CONTRACT-04 | Validation reports the wire field, not the Go name — `WriteValidation` surfaces `args.field` as the JSON tag (`amount`, not `Amount`) + `args.rule` as the validator tag that fired (`required`, `oneof`), so the frontend catalog keys on stable snake_case names; a non-validator error falls through to `CodeInternal` / 500. An untagged field falls back to the Go name (safety-net, locked so a contributor doesn't silently leak it). Covered by `TestWriteValidation_envelopeFromFirstFieldError` / `_oneofTagSurfaces` / `_nonValidatorErrorMapsToInternal` / `TestNewValidator_untaggedFieldFallsBackToGoName` | ADR-0027 | Medium |

> _Zone complete at 4/4. Grows when a new failure surface lands that doesn't fit
> the envelope (e.g. a streaming endpoint, a non-JSON download error path); until
> then every `internal/*` handler funnels through `httperr.Write*` and is covered
> by the rows above._

---

## After CONTRACT — the floor (read before seeding a zone 20)

CONTRACT is the **intended last zone of this push**. The Critical/High
correctness-and-leak surface is fully mapped by zones 01–18; everything past
CONTRACT is the long tail where severity drops to Medium and overlap with
existing zones rises. Two slices still genuinely clear the catalog bar (silent
corruption / leaked-or-false state) and are **deliberately deferred, not
forgotten** — seed them only if the matrix is being driven to provable
completeness:

- **AUTOSAVE / FEEDBACK** (ADR-0032, frontend-native) — buttonless controls
  (Tag dropdown, Language/Appearance selects) fire-and-forget a mutation; a
  *silently* failed autosave leaves the non-technical user believing a choice
  stuck when it didn't (false-state / silent data loss for the exact audience
  ADR-0021 targets). A `sonner` toast surface + per-control `onError` exist
  (`SettingsScreen.tsx`, `DetailTagControl.tsx`, the `use*` hooks) → annotatable
  via component tests or an `@smoke` spec. Medium.
- **PRECISION** (ADR-0011) — `DECIMAL(20,4)` money / `(20,8)` qty+fx, storage
  **never rounds** (half-up only at the display boundary). Distinct slice = the
  Go↔`pgtype.Numeric` round-trip doesn't drift/truncate on write. Real but
  **thin** and overlaps FINANCE/FX/PRESENTATION — carve as a small zone with
  cross-refs, never clones. Medium.

**Confirmed non-zones** (checked — do not carve): CONFIG/BOOT (only
`DATABASE_URL` is `required`, already tested; sessions are opaque server-side
rows so there's no signing secret to fail-fast on); CONCURRENCY (no
optimistic-locking/version column exists — deliberate for a low-concurrency
household app, so nothing to catalogue); MIGRATIONS (ADR-0033 immutability is a
release/process guarantee via goose checksums, not a runtime invariant a repo
test asserts); pure infra (chi/pgx/sqlc/vite/PWA/hosting — ADR-0013/14/15/16/18/30);
meta/process (ADR-0021/29/31/34); i18n *completeness* (a missing catalog key is
cosmetic, below the bar — the privacy-safe rendering half is already
PRESENTATION).
