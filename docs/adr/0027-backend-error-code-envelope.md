# Backend error-code envelope

The backend's HTTP error responses move from **plain English `http.Error(...)` bodies** to a typed
JSON envelope `{ "code": "<CODE>", "args": { … } }`. The frontend maps the code through its existing
react-i18next catalogs and interpolates `args`, so adding a third language never touches Go. This
closes the deferred "Shape C" follow-up from [[adr-0026]] and supersedes the temporary
status-and-raw-body pattern the dialogs currently use.

## Why now

[[adr-0026]] shipped the i18n stream end-to-end (issues #1–#12: chrome, dashboard, every position
group, transactions, snapshots, E2E pin) but deliberately left error copy alone. Today every dialog
that surfaces a mutation error carries a local `formatError(err, unknownMsg)` clone (~50 copies)
that does one of three things:

1. If `ApiError` with a string body, render the raw body verbatim — i.e. English, regardless of
   the user's locale.
2. Else render `${err.status} ${err.statusText}`.
3. Else render `err.message`.

For the Indonesian-reading co-owner, `"snapshot date cannot
be in the future"` rendering inside an otherwise-Indonesian Create-Snapshot dialog is the most
visible remaining English bleed. The mapping work is also the only piece of the i18n stream that
scales linearly with future locales — every English sentence in `internal/*/*.go` is one more
translation per language unless the contract is a stable code.

Three structural reasons make now the right time, the same reasons [[adr-0026]] cited for the
chrome sweep:

- **The sentinel surface is small and frozen.** `internal/repo/errors.go` declares ten sentinel
  errors, every HTTP-reachable one is already mapped in seven near-identical `writeRepoError`
  funcs. The inline `http.Error(...)` sites (~270) reduce to ~10 distinct English phrases
  (invalid id, invalid json body, invalid request, cannot be in the future, …) after de-duping.
  The total code-namespace is ~20 entries, not 270.
- **No backend test asserts on body content.** A grep across `internal/**/*_test.go` finds two
  body-string assertions (`auth/handlers_test.go` checks the response does *not* leak
  `google_sub`; `auth/callback_test.go` checks an invitation-failure redirect mentions
  `"invitation"`). Both keep working unchanged because the redirect / leak paths are out of
  scope for this envelope (see "Out of scope" below). The other 31 body reads are formatting-only
  (`Fatalf("status %d, body %s", …)`).
- **The frontend already centralises through `ApiError`.** `src/api/client.ts` parses the response
  body as JSON when possible and otherwise falls back to text; the envelope is just a
  better-typed `body`. Replacing the 50 local `formatError` clones with one
  `errorMessage(err, t, fallback)` is a mechanical sweep, not a redesign.

## The decision

### Envelope shape: `{code, args}` — no `message` field

```jsonc
{
  "code": "SNAPSHOT_FUTURE_DATE",
  "args": { "date": "2027-01-15" }
}
```

- **`code`** is a stable `SCREAMING_SNAKE_CASE` string. It is the wire contract. The frontend
  looks it up as `errors:code.<CODE>` in the react-i18next catalogs.
- **`args`** is an optional object of interpolation values. Keys match the placeholders in the
  catalog string (e.g. `{{date}}`, `{{field}}`, `{{rule}}`). Values are JSON primitives only —
  no nested objects — so the frontend can pass them straight into `t(key, args)`.
- **No `message` field.** The frontend catalog is the single source of truth for human copy;
  shipping the English string alongside the code would invite copy drift. An unknown code on
  the frontend falls back to the generic `errors:code.UNKNOWN` line ("Something went wrong"),
  with the raw envelope logged to the console in dev (`import.meta.env.DEV`). This is a
  deliberate trade-off — see "Considered alternatives".

The envelope ships as `application/json` with `Content-Type` set explicitly. `http.Error` is
replaced because it writes `text/plain; charset=utf-8` and appends a trailing `\n`, both wrong
for JSON.

### A shared `internal/httperr` package

The seven existing `writeRepoError` funcs (in `assets`, `liabilities`, `receivables`,
`investments`, `income`, `fxrates`, `reports`) collapse into one helper package:

```go
// internal/httperr/httperr.go (sketch)
package httperr

type Envelope struct {
    Code string         `json:"code"`
    Args map[string]any `json:"args,omitempty"`
}

func Write(w http.ResponseWriter, status int, code string, args map[string]any)
func WriteRepo(w http.ResponseWriter, op string, err error) // sentinel -> code mapping
func WriteValidation(w http.ResponseWriter, err error)      // validator.ValidationErrors -> VALIDATION
```

`WriteRepo` owns the sentinel-to-code mapping (`ErrNotFound` → `NOT_FOUND` + 404,
`ErrInvalidLifecycle` → `INVALID_LIFECYCLE` + 400, …) so the seven near-duplicate switch
statements consolidate. Per-package nuance (e.g. `fxrates` mapping `ErrFxRateExists` to 409,
`investments` mapping `ErrInvalidSnapshotShape` to 400) lives in the same switch — there is no
package-specific subclass; the sentinel itself carries the dispatch.

### Codes — sentinel-derived and inline

Sentinel codes (10) are 1:1 with `internal/repo/errors.go`:

| Sentinel                       | Code                          | Status |
|--------------------------------|-------------------------------|--------|
| `ErrNotFound`                  | `NOT_FOUND`                   | 404    |
| `ErrInvalidSnapshotShape`      | `INVALID_SNAPSHOT_SHAPE`      | 400    |
| `ErrInvalidTransactionType`    | `INVALID_TRANSACTION_TYPE`    | 400    |
| `ErrInvalidTransactionShape`   | `INVALID_TRANSACTION_SHAPE`   | 400    |
| `ErrInvalidLifecycle`          | `INVALID_LIFECYCLE`           | 400    |
| `ErrFxRateExists`              | `FX_RATE_EXISTS`              | 409    |
| `ErrForeignPositionsExist`     | `FOREIGN_POSITIONS_EXIST`     | 409    |
| `ErrPositionNotActive`         | `POSITION_NOT_ACTIVE`         | 409    |
| `ErrUnauthenticated`           | *(unreachable — fall to 500)* | —      |
| *(default)*                    | `INTERNAL`                    | 500    |

`ErrUnauthenticated` keeps the existing treatment from the HANDOFF convention: not mapped,
falls through to 500 because `RequireAuth` gates every route — a misconfigured route is a
server bug, not a client error.

Inline codes (the call sites that don't go through a sentinel) cover the validation/parsing
layer above the repo. Roughly:

| Phrase today                                                      | Code                       | Status |
|-------------------------------------------------------------------|----------------------------|--------|
| `"invalid id"` / `"invalid snapshot id"`                          | `INVALID_ID`               | 400    |
| `"invalid json body"`                                             | `INVALID_JSON_BODY`        | 400    |
| `"invalid request: <validator err>"`                              | `VALIDATION` (see below)   | 400    |
| `"as_of_date cannot be in the future"`                            | `SNAPSHOT_FUTURE_DATE`     | 400    |
| `"transaction_date cannot be in the future"`                      | `TRANSACTION_FUTURE_DATE`  | 400    |
| `"year_month cannot be in the future"`                            | `FUTURE_YEAR_MONTH`        | 400    |
| `"invalid year_month: expected YYYY-MM or YYYY-MM-DD"`            | `INVALID_YEAR_MONTH`       | 400    |
| `"invalid request: rate must be > 0"`                             | `INVALID_RATE`             | 400    |
| `"invalid mode: expected preview or commit"`                      | `INVALID_IMPORT_MODE`      | 400    |
| `"missing or oversized file upload (field \"file\")"`             | `INVALID_FILE_UPLOAD`      | 400    |
| `"could not read spreadsheet: <reason>"`                          | `INVALID_SPREADSHEET`      | 400    |
| `"cannot invite yourself"`                                        | `CANNOT_INVITE_SELF`       | 400    |
| `"internal error"` (generic 500)                                  | `INTERNAL`                 | 500    |

The exact list lives in `internal/httperr/codes.go` as exported constants. Adding a new code
means: add the constant, emit it from the handler, add the catalog entry in `errors.json` for
both locales.

### Validator errors: one generic `VALIDATION` code

`go-playground/validator` produces field-level errors like `"Field amount: required"`. Today
these are pre-concatenated into `"invalid request: <full output>"` and rendered raw. The
envelope replaces this with:

```jsonc
{
  "code": "VALIDATION",
  "args": { "field": "amount", "rule": "required" }
}
```

The frontend catalog has one `errors:code.VALIDATION` template that interpolates `field` and
`rule`. Field names are the JSON field name (`amount`, not `Amount`), rule names are
validator's tag identifier (`required`, `oneof`, `min`, `gt`, …). When a request has multiple
field errors, only the first is surfaced — this matches today's behaviour
(`err.Error()` returns all but the dialog rendered them as one wall of text anyway) and keeps
the envelope flat.

Rule translation lives under `errors:code.VALIDATION.rule.<rule>`; the field name passes
through as-is (`amount`, `as_of_date`, …) because translating field names every place they
appear in copy is out of scope here — those will land naturally as the existing per-form
field labels already do.

### Out of scope for this envelope

- **OAuth callback redirects** (`internal/auth/callback.go`). On failure they redirect with
  `?error=…` query params, not JSON. Translating the post-redirect error screen is a smaller
  separate slice and doesn't need an envelope.
- **Mock OIDC** (`cmd/balances/mockoidc.go`). Dev/test only; the existing English `http.Error`
  bodies stay.
- **Per-row snapshot-importer errors.** The xlsx importer returns `422` with
  `{ to_insert, to_update, errors: [{row, message}] }` — a structured body the FE already
  renders row-by-row. The per-row `message` stays English in this slice; converting each
  spreadsheet-validation outcome to a code is high churn for marginal gain because the
  surrounding context is already a list of file-row diagnostics, not a single user-facing
  toast. May revisit if user feedback warrants.
- **The repo's English error strings themselves.** `ErrNotFound = errors.New("repo: not
  found")` stays English — it's Go-internal context for logs, not the HTTP surface.

### Frontend: one `errorMessage` helper, ~50 deletions

```ts
// src/lib/errorMessage.ts (sketch)
import type { TFunction } from 'i18next'
import { ApiError, type ErrorEnvelope } from '@/api/client'

export function errorMessage(err: unknown, t: TFunction, fallback?: string): string {
  if (err instanceof ApiError && isEnvelope(err.body)) {
    const env = err.body
    const key = `errors:code.${env.code}`
    return t(key, { ...env.args, defaultValue: t('errors:code.UNKNOWN') })
  }
  if (err instanceof Error) return err.message
  return fallback ?? t('common:unknownError')
}
```

`ApiError` gains a small narrowing helper (`isEnvelope(body): body is ErrorEnvelope`) so
callers don't repeat the shape guard. The 50-odd local `formatError(err, unknownMsg)` clones
across `Create*Dialog`/`Edit*Dialog`/`Import*Dialog`/snapshot dialogs delete in favour of
`errorMessage(err, t)`. Existing `unknownMsg` fallbacks become the optional third arg only
where the call site needs a context-specific string.

The `errors` namespace grows:

```jsonc
// frontend/src/locales/en/errors.json
{
  "failedToLoad": "Failed to load: {{message}}",
  "unknownError": "unknown error",
  "code": {
    "UNKNOWN": "Something went wrong.",
    "NOT_FOUND": "Not found.",
    "INVALID_ID": "Invalid id.",
    "INVALID_JSON_BODY": "Malformed request body.",
    "VALIDATION": "{{field}} {{rule}}",
    "SNAPSHOT_FUTURE_DATE": "Snapshot date cannot be in the future.",
    "TRANSACTION_FUTURE_DATE": "Transaction date cannot be in the future.",
    "FUTURE_YEAR_MONTH": "Month cannot be in the future.",
    "INVALID_YEAR_MONTH": "Invalid month: expected YYYY-MM.",
    "INVALID_SNAPSHOT_SHAPE": "Snapshot values don't match this investment type.",
    "INVALID_TRANSACTION_TYPE": "Transaction type isn't supported for this investment.",
    "INVALID_TRANSACTION_SHAPE": "Transaction values don't match the chosen type.",
    "INVALID_LIFECYCLE": "Invalid position status.",
    "FX_RATE_EXISTS": "An FX rate already exists for that month and currency.",
    "FOREIGN_POSITIONS_EXIST": "Foreign-currency positions exist; remove them before turning off multi-currency.",
    "POSITION_NOT_ACTIVE": "This position is no longer active.",
    "INVALID_RATE": "Rate must be greater than zero.",
    "INVALID_IMPORT_MODE": "Import mode must be preview or commit.",
    "INVALID_FILE_UPLOAD": "Missing or oversized file upload.",
    "INVALID_SPREADSHEET": "Could not read the spreadsheet.",
    "CANNOT_INVITE_SELF": "You can't invite yourself.",
    "INTERNAL": "Server error. Please try again."
  }
}
```

The matching `id` catalog ships in the same slice, terms drawn from
[`docs/glossary-id.md`](../glossary-id.md). The "Validation rule" sub-keys
(`code.VALIDATION.rule.required` etc.) populate as the inline-code list calls them out.

### Slicing

Per agreed-on cadence:

1. **This ADR alone**, no code. Land it, get explicit sign-off on the shape (this commit).
2. `internal/httperr` package + ADR-0026 link update + `repoErrorStatus`/`writeRepoError`
   consolidation in one BE package as a template (receivables — smallest existing surface).
3. Remaining 6 BE packages converted (assets / liabilities / investments / income / fxrates /
   reports / auth-non-callback).
4. FE swap: `ApiError` envelope parsing + `errorMessage` helper + delete 50 local
   `formatError` clones + `errors.json` populated in both locales.

Each slice ships independently, CI-green.

## Considered alternatives

- **Keep a `message` field in the envelope.** Cheaper dev fallback when a code is missing from
  the FE catalog, but invites copy drift — once two places hold the human string, they
  disagree. The `UNKNOWN` fallback covers the missing-code case and the raw envelope in the
  dev console covers the dev-mode visibility need. Rejected.
- **Per-rule validator codes** (`VALIDATION_REQUIRED`, `VALIDATION_MIN`, `VALIDATION_GT`, …).
  Closer to one-key-one-string but doesn't scale: validator has 30+ tags and combining tags
  (`required,oneof=a b c`) would explode the matrix. The single `VALIDATION` code with
  `{field, rule}` args is one catalog entry and grows by one sub-key per new rule we actually
  emit. Rejected.
- **Map at the frontend on HTTP status alone, no envelope** (the original "Shape B" from
  [[adr-0026]]). Already accepted as a stopgap and shipped that way; this ADR is the planned
  Shape C transition. Status alone collapses `INVALID_LIFECYCLE`, `INVALID_SNAPSHOT_SHAPE`,
  `VALIDATION`, and the future-date checks all into one "400 Bad Request" — the user toast
  can't distinguish them.
- **Locale on the backend** (Accept-Language → translated bodies). Reintroduces the
  rejected-in-[[adr-0026]] design: Go code grows a translation table that drifts from the FE
  catalog, and tests would have to assert against locale-specific strings. Rejected.
- **Wrap `http.Error` rather than replace it.** Keep the call sites, change the writer.
  Mechanically simpler but the wire format (`text/plain` body, trailing `\n`) is wrong for
  JSON, and call sites couldn't pass `args` without a second helper anyway. Rejected.

## Consequences

- **`internal/httperr` is the new shared error-response package.** All 4xx/5xx JSON responses
  from `internal/**` go through it. New handlers reach for `httperr.Write(w, status, code,
  args)`, not `http.Error(...)`. The OAuth callback (which redirects) and the mock OIDC (dev
  only) are the explicit exceptions.
- **`internal/repo/errors.go` is the canonical sentinel list.** Adding a new sentinel = add
  the `Err…` var, add the case to `httperr.WriteRepo`, add the code constant, add the
  catalog entry in both locales.
- **The seven per-package `writeRepoError` funcs are deleted.** `repoErrorStatus` (where it
  exists, in investments) is folded into the shared `WriteRepo` switch.
- **`ApiError.body` is typed as `ErrorEnvelope | string | undefined`** at the FE; a small
  `isEnvelope()` narrowing guard handles the discriminated union. Existing dialogs that
  consumed `err.body` as a string keep working through `errorMessage` until they're swept
  (one slice).
- **The 50-odd local `formatError(err, unknownMsg)` clones in `Create*` / `Edit*` /
  `Import*` / snapshot dialogs delete.** Each call site uses `errorMessage(err, t)` directly.
- **`errors.json` carries a `code.<CODE>` block per locale.** Catalog extension follows
  [`docs/glossary-id.md`](../glossary-id.md) the same way every other namespace does.
- **HANDOFF gains an "error envelope" convention** under the existing FE-lint / BE-lint
  bullets: `code` is the wire contract; FE catalogs are the source of human copy; no `message`
  field; OAuth-callback redirects are exempt.
- **Pre-alpha changelog (M6 section)** records the envelope landing alongside the i18n stream's #1–#12.
- **Future locales: error catalogs are JSON-only**, like the other namespaces. Adding French
  is `src/locales/fr/errors.json` + the same backend zero edits.

## How to extend

1. **Add a sentinel.** Declare the `Err…` var in `internal/repo/errors.go`. Add the
   `errors.Is` case to `httperr.WriteRepo`. Add the code constant to `internal/httperr/codes.go`.
   Add the catalog entry in `errors.json` for both locales.
2. **Add an inline (non-sentinel) error.** Add the code constant; emit
   `httperr.Write(w, http.StatusBadRequest, codes.XYZ, map[string]any{...})` from the handler.
   Add the catalog entry in both locales.
3. **Add a validator rule.** The handler emits `VALIDATION` automatically. Add
   `errors:code.VALIDATION.rule.<tag>` in both locales when a new validator tag becomes
   user-visible.
