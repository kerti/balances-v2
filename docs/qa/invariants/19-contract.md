# Zone: CONTRACT

> _Seeded next — the **HTTP error-envelope contract** (`internal/httperr`,
> ADR-0027). Every 4xx/5xx from `internal/*` handlers ships a typed
> `Envelope{Code, Args?}` the frontend looks up in the react-i18next
> `errors:code.<CODE>` catalog — the wire carries **no `message` field**, so a
> raw DB / internal error string never reaches the client. This zone owns the
> **wire failure contract**: the same risk shape as a leak (an unmapped error
> spilling internals) and a false-state (a conflict that reads as success or the
> wrong status). It's the error twin of the success-path zones; the OAuth
> callback redirect flow and the dev-only mock-OIDC subcommand are the
> documented exceptions that keep plain `http.Error` bodies (out of scope here).
>
> **Candidate rows** (all already test-backed in `internal/httperr/httperr_test.go`
> — this is annotation work, not new tests, like NOTIFICATIONS rows 1 & 3):
> - **Unknown error never leaks** — `WriteRepo` maps an *unrecognised* error to
>   `CodeInternal` / 500 and `slog.Error`s the raw `err` **server-side only**;
>   the wire envelope carries the code, never the raw string. The info-disclosure
>   guard — a regression that echoed `err.Error()` to the client would leak
>   schema/SQL/internal detail. Covered by `TestWriteRepo_unknownErrorMapsToInternal`
>   (+ `TestWriteRepo_unauthenticatedFallsThrough`: `ErrUnauthenticated` is
>   deliberately unmapped → 500, because every route is `RequireAuth`-gated so a
>   userless repo call is a server bug, not a client 401).
> - **Sentinel→code+status fidelity** — each repo sentinel maps to its stable
>   wire `Code` and the *correct* HTTP status (404 not-found, 409 conflict, 400
>   bad-shape), through `errors.Is` so a wrapped sentinel still maps. A
>   conflict that read as 200/404 is a false-state. Covered by
>   `TestWriteRepo_sentinelMapping` / `TestWriteRepo_wrappedSentinel`.
> - **Envelope shape is code-only** — `Write` emits `{"code": ...}` with `Args`
>   **omitted when nil** (never `"args":null`) and no `message` field, the shape
>   the i18n catalog contract depends on. Covered by `TestWrite_envelope` /
>   `TestWrite_argsOmittedWhenNil`.
> - **Validation reports the wire field, not the Go name** — `WriteValidation`
>   surfaces `args.field` as the JSON tag (`amount`, not `Amount`) + `args.rule`
>   as the validator tag that fired, so the frontend catalog keys on stable
>   snake_case names; a non-validator error falls through to INTERNAL. Covered by
>   `TestWriteValidation_envelopeFromFirstFieldError` / `_oneofTagSurfaces` /
>   `_nonValidatorErrorMapsToInternal` (+ `TestNewValidator_untaggedFieldFallsBackToGoName`).
>
> **No new tests expected** — survey `httperr_test.go`, add `// covers:` rows.
> Targets: `internal/httperr/httperr.go`, `codes.go`. Source: ADR-0027 (error
> envelope), ADR-0026 (the i18n catalog the codes key into)._
