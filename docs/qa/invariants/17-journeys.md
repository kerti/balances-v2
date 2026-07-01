# Zone: JOURNEYS

The **E2E-native** companion to PRESENTATION. Where PRESENTATION pins pure-fn
vitest twins of the backend truth, this zone pins the invariants that only exist
in the **whole-browser round-trip** — the ones that cross a redirect, a
session-cookie hand-off, or an export→re-import file boundary that no handler
unit test and no pure-fn test can reach end-to-end. The catalog bar is unchanged
(silent corruption or a leaked/false state, not mere mechanics), applied to user
journeys. Each row **mirrors, never re-owns** the backend truth it exercises:
AUTH owns the handler-level OAuth halves, IMPORT owns the preview/commit parity,
PRESENTATION-02 owns the pure-fn date cap — the rows here pin the *browser loop*
that stitches them together, cross-linked.

**Gate tier matters here** (`how-it-works.md`): Playwright is tiered — only
`@smoke`-tagged specs gate per-PR; the full suite runs nightly (`e2e.yml`, #70).
The **Tier** column below records, per row, whether the covering spec runs in the
per-PR gate (`@smoke`) or is **nightly-verified by design**. A future per-PR
`-strict` gate must treat a nightly-only row as verified-nightly, not credit it
per-PR. Source: ADR-0024 (OAuth flow), ADR-0021 (the non-technical audience these
journeys serve), #70 (tiered E2E in CI).

| ID | Invariant | Source | Severity | Tier |
|----|-----------|--------|----------|------|
| INV-JOURNEYS-01 | OAuth sign-in completes the full browser round-trip — clicking "Sign in with Google" from an unauthenticated context navigates `→ /auth/google/start → mock-OIDC authorize → callback → minted session cookie → back to the authenticated app shell` as the seeded user, the one path session-injection can't cover. The redirect chain and the cookie hand-off are exactly what the handler tests stub, so this is the only place the loop is verified intact. Mirrors, never re-owns, AUTH's handler halves (INV-AUTH-02 state/CSRF, INV-AUTH-03 session cookie, INV-AUTH-05 founder bootstrap); a break here strands every user at the door. Verified in `login.spec.ts` (runs the local mock-oidc provider, ADR-0024 option B — never contacts accounts.google.com) | ADR-0024 / INV-AUTH-02·03·05 | High | `@smoke` |
| INV-JOURNEYS-02 | Carryover seeds a submittable form — opening the snapshot carryover dialog pre-fills the prior amount **and a statement date that is already valid** (defaults to today, within the date input's min/max), so the non-technical user can save without editing the month down. The browser/journey face of INV-PRESENTATION-02 (the pure-fn local-time date cap) and #60/#119; PRESENTATION-02 owns the cap, this owns the dialog wiring that surfaces a ready-to-submit value. Verified in `snapshot.spec.ts` (carryover prefill assertion) | ADR-0021 / INV-PRESENTATION-02 / #119 | Medium | `@smoke` |
| INV-JOURNEYS-04 | A first-time Google identity onboards through the gate in the browser — a sign-in whose `google_sub` matches no user navigates `→ /auth/google/start → mock-OIDC authorize → callback` and, finding nothing to sign into, records a handshake and lands on the `/onboarding` gate (not the app shell); the founder choice then bootstraps the Household + real session and the authed shell renders as the brand-new founder. The browser sibling of INV-JOURNEYS-01 (which lands as the *existing* seeded user) for the new-identity branch. Mirrors, never re-owns, AUTH's gate halves (INV-AUTH-05 handshake/redirect-instead-of-bootstrap, INV-AUTH-13 founder commit); the mock issues a per-run-unique unseeded `sub` via the `mock_oidc_sub` cookie (mockoidc.go, #274). Verified in `onboarding.spec.ts` (local mock-oidc, ADR-0024 option B) | ADR-0024 / ADR-0038 / INV-AUTH-05·13 | High | `@smoke` |
| INV-JOURNEYS-03 | Import preview→commit→list parity in the browser — feeding an exported position workbook through the list-screen Import dialog: the dry-run check reports **no errors and a would-create** (writing nothing), and only the explicit commit creates the position, which then appears in the list. The end-to-end UI face of INV-IMPORT-01 (preview/commit parity, dry-run no-op); IMPORT-01 owns the server contract, this owns the dialog's check→commit→reflected-in-list loop including the export→re-import file round-trip. Verified in `import-create-roundtrip.spec.ts` (bank-account workbook; `investment-import-create-roundtrip.spec.ts` is its investment twin) | ADR-0022 / INV-IMPORT-01 | High | nightly |

> _Known candidate, deliberately not yet catalogued (no covering spec exists):
> **session expiry → re-auth** — a stale/expired session should land the user
> back at the sign-in screen, not a broken authed shell or a raw 401. `login.spec.ts`
> proves the unauthenticated→sign-in-screen direction incidentally (it starts
> with an empty `storageState`), but nothing exercises an **expired** session
> mid-app. Mint INV-JOURNEYS-04 when a spec drives it; until then this is a noted
> gap, not an uncovered catalog row (kept out of the matrix denominator on
> purpose, per the seed convention — candidates live in prose until a test lands)._
