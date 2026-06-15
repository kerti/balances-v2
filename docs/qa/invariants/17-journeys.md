# Zone: JOURNEYS

> _Seeded next — the **E2E-native** companion to PRESENTATION. Where
> PRESENTATION pins pure-fn vitest twins of the backend truth, this zone pins the
> invariants that only exist in the **whole-browser round-trip**: the ones that
> cross a redirect, a session-cookie hand-off, or a navigation boundary that no
> handler unit test and no pure-fn test can reach end-to-end. The catalog bar is
> the same as everywhere else (silent corruption or a leaked/false state, not
> mere mechanics) — applied to user journeys._
>
> **The charter row is the OAuth sign-in flow (ADR-0024):** button → provider
> redirect → callback → session cookie set → landed authenticated. Only the
> callback half is unit-tested today (the handler — see AUTH); the *full loop* is
> exercised in the browser against the mock-OIDC server. That round-trip is the
> archetypal can't-reach-it-any-other-way invariant: the redirect and cookie
> boundaries are exactly what a handler test stubs out.
>
> **Other candidates** (pin only those whose failure misinforms or strands the
> non-technical user, per the catalog bar):
> - **Session expiry → re-auth** — a stale/expired session lands the user back at
>   sign-in, not at a broken authed shell or a raw 401.
> - **Import preview → commit → list** — the browser face of IMPORT's
>   preview/commit parity (INV-IMPORT-*): what the preview showed is what the list
>   holds after commit, and a dry-run leaves the list untouched.
> - **Carryover dialog → submittable form** — the *journey* face of
>   INV-PRESENTATION-02 / #119: opening the dialog seeds a `{ yearMonth, asOfDate }`
>   pair that submits without the user editing the month down. Cross-link
>   PRESENTATION-02; do not restate it.
>
> **Gate wrinkle — read before annotating** (`how-it-works.md`): Playwright is
> tiered. Only `@smoke`-tagged specs gate per-PR; the full suite runs nightly
> (`e2e.yml`, #70). So an invariant covered *only* by a non-smoke spec would be
> credited by a future per-PR `-strict` gate without having run in that PR — it
> ran nightly instead. Before an annotation here counts toward a per-PR strict
> gate, either **tag the covering spec `@smoke`** so it runs in the gate, or
> record which rows are **nightly-verified by design**. This is the one structural
> difference from PRESENTATION's safe-to-annotate-first vitest rows, and it is why
> JOURNEYS is seeded as its own zone rather than folded in.
>
> Target specs: `frontend/e2e/*.spec.ts` (pick/assert via `data-testid`). Survey
> the existing specs before writing — several flows may already be covered and
> need only the annotation (plus a `@smoke` tag where a row must gate per-PR).
> Source: ADR-0024 (OAuth flow), #70 (tiered E2E in CI), ADR-0021 (the audience
> these journeys serve)._
