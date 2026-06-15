# Zone: PRESENTATION

> _Seeded next — the first **frontend-native** zone, and the place the matrix's
> language-agnostic `covers:` token finally earns its keep on the frontend. This
> zone catalogues the **client presentation + input-guardrail layer** that the
> non-technical audience actually touches (ADR-0021): the pure functions in
> `frontend/src/lib` that render the backend's numbers without lying to a
> household member, and the form-level guards that stop bad input before it
> round-trips into an opaque API error. **It mirrors, never re-owns, the backend
> truth** — FINANCE owns the net-worth number, INTEGRITY/SNAPSHOTS own the
> server-side CHECKs, EXPORT-02 owns the joint-privacy rule; the rows here pin the
> *client twin* of each, cross-linked, never a clone. Annotate vitest specs
> first: per `how-it-works.md` they run in the same per-PR gate they'd be credited
> in, so there is **no `-strict` hazard** (unlike non-`@smoke` Playwright, which
> only runs nightly). Three already-written, already-green vitest suites are the
> obvious starting annotations:
>
> (1) **Money never renders as a lie** — `formatCurrency` (`format.ts`) shows an
> amount in the household locale (`en-GB` / `id-ID`), drops the fractional part
> for no-decimal currencies (IDR, JPY, KRW, VND) and keeps two decimals for
> ordinary ones, and **returns the raw input rather than `NaN`/`undefined`** when
> the amount isn't a number — a non-technical reader never sees a broken cell.
> Target: `format.test.ts`. The client twin of FINANCE's number; FINANCE owns the
> value, this owns its faithful display.
>
> (2) **Client date guardrail mirrors the server** — `dateLimits.ts`
> (`thisYearMonth` / `todayDate` / `monthEndDateCapped`) caps snapshot & as-of
> inputs to not-future in **local** time, the form-side twin of INV-SNAPSHOTS-05
> (the server's future-date 400). The guard exists so the household member is
> stopped in the form, not bounced by an API error they can't parse. Target:
> `dateLimits.test.ts`. Cross-link SNAPSHOTS-05; do not restate it.
>
> (3) **Ownership label is privacy-safe** — `ownershipLabel` (`ownership.ts`)
> renders **"Joint" with no member identity** for joint holdings (the UI face of
> EXPORT-02 / ATTRIBUTION's joint-is-whole stance), and the owner's
> nickname-or-`display_name` + a "(you)" suffix for sole, degrading cleanly when
> there is no current user. Target: `ownership.test.ts`. Cross-link EXPORT-02.
>
> Survey `frontend/src/lib/*.test.ts` before writing — many of these libs
> (`fx.ts`, `months.ts`, `reconciliation.ts`, `totals.ts`) are candidate rows too,
> but only pin the ones whose failure would *misinform or mis-guard* the
> non-technical user (the catalog bar: silent corruption or a leaked/false number,
> not mere mechanics). The likely genuinely-new work is small: most guards are
> already tested and need only the annotation. A later extension is the
> **E2E-native** half — flows a unit test can't reach, e.g. the OAuth
> button→redirect→callback→session round-trip (ADR-0024) — but those are
> Playwright specs subject to the `@smoke`/nightly gate wrinkle, so seed them
> separately once the vitest rows land. Source: ADR-0021 (non-technical audience),
> ADR-0024 (OAuth flow), CONTEXT.md for the domain language the labels render._
