# Zone: PRESENTATION

The first **frontend-native** zone: the **client presentation + input-guardrail
layer** the non-technical audience actually touches (ADR-0021). It catalogues the
pure functions in `frontend/src/lib` that render the backend's truth without
lying to a household member, and the form-side guards that stop bad input before
it round-trips into an opaque API error. The zone **mirrors, never re-owns, the
backend truth** — FINANCE owns the net-worth number, SNAPSHOTS owns the
server-side future-date CHECK, EXPORT owns the joint-privacy rule; the rows here
pin the *client twin* of each, cross-linked, never a clone. A client-side render
or guard failing does not corrupt stored data, but it *misinforms or mis-guards*
the one reader who can't tell a display bug from a real number — which is the bar
for a row here (silent corruption or a leaked/false number, not mere mechanics).
The later rows (04+) extend the zone from pure `lib` helpers to the **component
tier** — the descriptor-driven list core (ADR-0043), verified by RTL+MSW+jsdom
suites that activate ADR-0021's deferred component tests (#69). Every row here is
verified by a vitest suite that runs in the same per-PR gate it's credited in
(`how-it-works.md`), so there is no `-strict` hazard. Source:
ADR-0021 (non-technical audience), ADR-0026 (i18n/locale). A later **E2E-native**
extension (flows a unit test can't reach, e.g. the OAuth round-trip, ADR-0024) is
seeded separately once these vitest rows land, since Playwright is subject to the
`@smoke`/nightly gate wrinkle.

| ID | Invariant | Source | Severity |
|----|-----------|--------|----------|
| INV-PRESENTATION-01 | Money never renders as a lie — `formatCurrency` (`format.ts`) shows an amount in the household locale (`en-GB` / `id-ID`), drops the fractional part for no-decimal currencies (IDR, JPY, KRW, VND) and keeps two decimals for ordinary ones, and **returns the raw input rather than `NaN`/`undefined`** when the amount isn't a number — a non-technical reader never sees a broken cell. The client twin of FINANCE's value: FINANCE-01/02 own the number, this owns its faithful display; the same NaN-safe-fallback contract holds for the sibling display helpers (`formatNumber`, `formatSignedPercent`, `roundToCurrency`). Verified in `format.test.ts` | ADR-0021 / ADR-0026 | Medium |
| INV-PRESENTATION-02 | Client date guardrail mirrors the server — `dateLimits.ts` (`thisYearMonth` / `todayDate` / `monthEndDateCapped`, and the `carryoverSeed*` clamp) caps snapshot & as-of inputs to not-future in **local** time, the form-side twin of INV-SNAPSHOTS-05 (the server's future-date 400). The guard exists so the household member is stopped in the form, not bounced by an API error they can't parse; SNAPSHOTS-05 remains the authoritative rejection — this never relaxes it, only front-runs it. Verified in `dateLimits.test.ts` | ADR-0021 / INV-SNAPSHOTS-05 | Medium |
| INV-PRESENTATION-03 | Ownership label is privacy-safe — `ownershipLabel` (`ownership.ts`) renders **"Joint" with no member identity** for joint holdings (the UI face of INV-EXPORT-02's joint-is-blank-owner rule), and the owner's nickname-or-`display_name` + a "(you)" suffix for sole, degrading to a generic "Sole" when the member list is loading or the owner can't be resolved (e.g. soft-deleted user) — never leaking a name onto a holding the household owns jointly. The display twin of EXPORT-02; EXPORT-02 owns the export-side blank owner, this owns the on-screen label. Verified in `ownership.test.ts` | ADR-0021 / INV-EXPORT-02 | High |
| INV-PRESENTATION-04 | The descriptor-driven Position list core (`PositionListScreen`, ADR-0043) always renders the **Position shared surface** for every row — display name + secondary line, lifecycle status, latest-snapshot value, and the ⋮ edit/delete actions — for any position type; a concrete type is reduced to a descriptor and the core never drops a shared column or absorbs a group-specific field into it. The core also owns, type-independently, the sort toggle, the show-inactive filter, and the loading/empty/error states — so one generic replaces ten per-type screens + eleven `*ListRow` files without re-testing list behaviour per type. Verified in `PositionListScreen.test.tsx` (synthetic descriptor — asserts the abstraction, not one type's incidentals) + `descriptors/bankAccount.test.tsx` (per-type conformance over MSW) | ADR-0043 / #69 | High |
| INV-PRESENTATION-05 | The list core keeps **group extras off the compact mobile card** — the two renderers (`PositionListTable` web, `PositionListCards` mobile) consume one presentation-neutral column list; the web table shows every column, the mobile card shows the shared surface plus only the extras that opted in with `mobile: "secondary"`. A group-specific field never leaks onto the card by default, and the shared surface always survives the renderer switch (`useIsMobile`, 768px). Guards the planned dense-web / compact-mobile divergence against a hand-written second component per type. Verified in `PositionListScreen.test.tsx` | ADR-0043 | Medium |
