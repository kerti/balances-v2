# Internationalization with react-i18next and a persisted user locale

The app launches bilingual: **English (`en`) and Indonesian (`id`)**, with room to add more
languages without code changes. UI strings move from hard-coded JSX literals to **react-i18next**
catalogs, locale-aware number/date/currency formatting consolidates behind a single helper module,
and the active locale is **persisted on the user row** so it survives device changes. Backend HTTP
responses stay English in this milestone; a follow-up ADR will introduce a typed error-code envelope
([[adr-0027]]) so future locales don't touch Go code.

## Why now

The app's audience is non-technical household members; the
primary user reports the household co-owner reads Indonesian comfortably and English haltingly, so
shipping EN-only is a usability ceiling not a translation luxury. Three structural reasons make now
the cheapest moment:

- **The string surface is finite and tractable** — ~165 frontend files, ~570 JSX literal-text sites,
  no untranslated content stores. Every additional screen makes the extraction sweep larger.
- **Domain values are already language-neutral in the DB.** Income categories, transaction types,
  status enums, regularity, risk profile, and ownership are stored as English tokens
  (`salary`, `routine`, `low`, `personal`) and rendered through FE-side label maps
  (`CATEGORY_LABEL`, `TYPE_LABELS`, `ownershipLabel`, etc.). Those maps are natural translation
  seams — i18n drops in where labels already concentrate, no schema change required.
- **Locale-aware formatting is already inconsistent.** `lib/format.ts` hardcodes `'id-ID'` for
  currency, `'en-US'` for year-month, `'en-GB'` for dates; `DashboardScreen` and `SnapshotChartImpl`
  reach for `Intl` directly with hardcoded locales. Centralising on the active locale is overdue
  regardless of i18n.

## The decision

### react-i18next, not LinguiJS or FormatJS

react-i18next is the largest-ecosystem React i18n library (namespaces, plurals, interpolation,
`<Trans/>` for inline JSX, well-documented Vite recipes, hot-reload of catalogs during dev). It is
chosen over:

- **LinguiJS** — smaller runtime and compile-time message IDs, but a macro-based pattern that adds a
  Babel/SWC step we don't have, and a smaller community. The bundle savings don't matter at our scale.
- **FormatJS / react-intl** — first-class ICU MessageFormat (best plural/select expressiveness), but
  heavier API surface and verbose call sites for the 90% case (`<FormattedMessage id=...
  defaultMessage=.../>`). react-i18next's interpolation is enough for our copy.

The trade-off accepted: react-i18next has runtime IDs (no compile-time check that a key exists). We
mitigate with a strict catalog-shape TypeScript type and an ESLint rule against bare JSX text.

### EN and ID at launch; locale is a string, not an enum, in code

Two languages ship as fully-translated catalogs. A third language is **adding a JSON file** —
nothing in the code switches on `'en'` vs `'id'`. The catalogs are namespaced by feature
(`common`, `nav`, `dashboard`, `assets`, `liabilities`, `receivables`, `investments`, `income`,
`settings`, `errors`) so each extraction issue ships an independently translatable unit.

### Locale persists on the user, not the browser

`users.locale` (existing column from migration 00002, repurposed here) is the source of truth.
Migration 00020 pins the allowed set via CHECK to the BCP47 forms the app supports — initially
`'en-GB'` and `'id-ID'` — and leaves the existing default (`'id-ID'`, the household-targeted
language) and stored values untouched. **The DB stays BCP47**, not 2-letter; Intl APIs want BCP47
anyway, and storing the region keeps the door open for variants (`en-US`, `id-ID`) without a
schema change. `en-GB` (not `en-US`) is the canonical English because the existing copy
already uses day-first dates (`"15 May 2024"`) — `en-US` would produce `"May 15, 2024"`.

Catalog source directories under `src/locales/` stay 2-letter (`en/`, `id/`) for filesystem
cleanliness. The runtime imports each JSON statically and re-keys the resource bundles by full
BCP47 (`'en-GB'`, `'id-ID'`) so the lookup matches `supportedLngs` directly — no region-strip
step is needed. Adding a regional variant doesn't require splitting catalogs unless the
translations actually diverge.

**Catalogs are bundled statically, not fetched at runtime.** Earlier drafts of this ADR called
for `i18next-http-backend` so future languages would be a JSON-file drop. Switched to static
imports at the chrome-extraction slice (issue #5). Reasons:

- **Simplicity at our scale.** Bundled catalogs are ~30 KB for 10 namespaces × 2 locales —
  immaterial in a single-household app. The lazy-load wins of HttpBackend (smaller initial JS,
  catalog-per-language fetches) don't pay off until 5+ languages or much heavier catalogs.
- **No first-paint race.** Bundled resources are present synchronously on the first render, so
  `t()` returns real copy immediately. The HttpBackend path required either a Suspense boundary
  or a deferred `createRoot` mount to avoid a "raw key" flash.
- **Build-time validation.** TypeScript sees every catalog import, so a typo'd path or missing
  file fails the build instead of silently 404'ing in the browser.

Trade-off accepted: adding a new language is no longer "drop a JSON file." It also requires
extending the static import block + the `resources` map in `i18n/index.ts` (~12 lines of edit),
plus the existing CHECK + `SUPPORTED_LOCALES` extension. The list below ("Future languages are
JSON-only") is softened accordingly.

A bug found and fixed along the way, worth recording so it doesn't get re-introduced: with
`supportedLngs: ['en-GB', 'id-ID']`, `load: 'languageOnly'` strips the detected `id-ID` to `id`
*before* the supportedLngs check, then rejects it because `nonExplicitSupportedLngs` defaults
to false — i18next resolves no language, the backend never fires, and t() returns the key.
Resource bundles (and the supportedLngs list) must be keyed by the *full* BCP47 form to match.

Backend exposes locale via the existing user-self endpoints (`GET /api/users/me`,
`PATCH /api/users/me`). The PATCH handler decodes via `map[string]json.RawMessage` so it can
distinguish field-absent (skip) from field-present-null (clear, for nickname; 400 for locale —
locale has no "unset" state). Settings gains a "Language" dropdown that calls the PATCH and
mirrors the choice into localStorage. First-login fallback in `AppShell` reads
`navigator.languages`, maps to a supported BCP47, and writes the result back to the user row, so
the next device picks it up automatically. **(Superseded by [[adr-0035]]: once a pre-auth language
picker exists, navigator detection is demoted to a display-only pre-fill and no longer PATCHes the
user row; locale is seeded server-side at account birth instead.)**

Browser-only / cookie-only alternatives rejected: device-switching is a real flow for this
household app (phone + laptop), and the user row already holds the cousin field `nickname` — the
shape is familiar.

### Number / date / currency: one locale-aware helper module

`lib/format.ts` is rewritten to read the active locale from a thin `useLocale()` hook (or accept it
as a parameter for non-React call sites). Every hardcoded `'id-ID'` / `'en-US'` / `'en-GB'` becomes
the active locale. Currency formatting keeps the `NO_DECIMAL_CURRENCIES` rule unchanged — that's a
currency property, not a locale property. The signed-percent helper stays locale-agnostic.

### Backend stays English in this milestone

Error bodies remain plain English text via `http.Error(...)`. The frontend maps known statuses (per
endpoint where it matters) to translated friendly toasts; an unmapped error falls back to a generic
translated "Something went wrong" with the raw English body shown only in dev mode. **Backend
error-code envelope is the deferred follow-up** — a future ADR introduces a typed
`{code, args}` JSON shape for known sentinels so future locales don't touch Go. Shipping that now
would double the milestone's scope; deferring is cheap because the FE error mapping table is small
and can be replaced wholesale when the envelope lands.

### E2E pins to `en` rather than testid-sweeping

Playwright specs that use `getByText` on English copy stay correct by **pinning the E2E user's
locale to `en`** in the seeded session — one line in `e2e/global-setup.ts`. A separate testid sweep
for the bleed cases is unnecessary; the project convention already prefers `data-testid`
and existing specs are mostly compliant.

### A glossary doc precedes ID translation

[`docs/glossary-id.md`](../glossary-id.md) lists the ~30 financial-vocab decisions (Receivable →
Piutang, Liability → Liabilitas, Snapshot → Snapshot, etc.) and is written first. Subsequent
extraction issues translate against the fixed dictionary; the consistency cost of
inline-translation-then-sweep is avoided.

## Considered alternatives

- **LinguiJS / react-intl.** Covered above.
- **Browser-only locale (no DB column).** Smaller change, but cross-device drift is a real bug for a
  shared household app. Rejected.
- **Full backend error-code envelope in the same milestone.** Cleanest end state, but doubles the
  scope and touches every HTTP handler. Deferred to its own ADR. The frontend mapping table is a
  cheap stopgap.
- **No glossary, translate inline during extraction.** Faster start, but consistency drift across
  screens (`Liabilitas` vs `Kewajiban`) needs a sweep later anyway. Rejected.
- **Don't ship ID at launch; scaffold EN catalogs only.** Defeats the point — the use case is the
  Indonesian-reading co-owner. Rejected.

## Consequences

- **Dependencies.** `react-i18next`, `i18next`, `i18next-browser-languagedetector` added; runtime
  bundle grows modestly (~30 KB gzipped). No Babel/SWC additions.
- **`lib/format.ts` becomes locale-aware.** Every existing `formatCurrency`/`formatDate`/
  `formatYearMonth` call site is unchanged at the call surface; the helper internally consumes the
  active locale.
- **ESLint rule against bare JSX text.** `react/jsx-no-literals` (or the equivalent) catches
  regressions. Allowlist for code tokens (`px-2`, `IDR`, etc.) in tests/fixtures.
- **[`docs/glossary-id.md`](../glossary-id.md) is the canonical ID dictionary.** Translation PRs
  reference it; a new term expands it.
- **Migration `00020` pins `users.locale` to the BCP47 allowed set** via a CHECK constraint
  (`locale IN ('en-GB','id-ID')`). The column existed since 00002; this only constrains it. The
  PATCH handler additionally validates so clients get a 400 rather than a 500 on a CHECK
  violation. Adding a language: extend the CHECK + the FE `SUPPORTED_LOCALES` + the handler's
  `supportedLocales` map.
- **HANDOFF gains a "Don't reintroduce bare JSX text" convention** under the existing FE-lint
  bullet, and an i18n entry in the M6-shipped list when the work completes.
- **[[adr-0027]] introduces the backend error-code envelope** that closes the Shape-C transition;
  tracking issue #13 links it.
- **Future languages: JSON files + a small `i18n/index.ts` edit.** Add
  `src/locales/<lang>/<ns>.json` files, extend the static-import block and the `resources` map
  in `i18n/index.ts` (keyed by full BCP47 — `'fr-FR'`, not `'fr'`), extend `SUPPORTED_LOCALES`,
  and add the matching CHECK in backend migration 00020 plus the Settings dropdown entry. No
  switching-on-locale anywhere in app code beyond the imports/map.
