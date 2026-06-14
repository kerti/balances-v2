# Testing strategy

The testing approach is shaped by two non-negotiable constraints: ADR-0005's tenancy threat model
requires multi-Household leak tests against a real database from day one, and the project's value is
concentrated in a handful of financial computations that must be correct. Everything else is
calibrated to maximise signal per minute of test maintenance — modest coverage on plumbing, heavy
coverage on calculations and tenancy.

## Backend

### Test database: `testcontainers-go`

Integration tests run against a real Postgres container spun up per `go test` invocation.

- Programmatic Docker management; same setup works on local OrbStack and on any CI runner with
  Docker available.
- The ~2–5s container startup is paid once per test binary, not per test function.
- Tests within a run are isolated by wrapping each test in a transaction (`BEGIN` in setup,
  `ROLLBACK` in `t.Cleanup`), which avoids cross-test pollution without paying a container-per-test
  cost.
- Migrations are applied to the fresh container at startup using the same goose runner (ADR-0019)
  the app uses.

**Considered alternatives:**
- A long-running docker-compose Postgres shared by all test runs. Faster local loop, but tests
  become responsible for their own cleanup, and a teardown bug silently pollutes later tests.
  Rejected for the flake risk.
- Schema-per-test in one DB. Mid-speed and clean, but more plumbing than testcontainers'
  programmatic API. Rejected as more work for the same result.

### Assertions: stdlib `testing` + `google/go-cmp`

Scalar checks use plain `if`-and-`t.Errorf`. Structural checks use `cmp.Diff(want, got)` — semantic
diffs are excellent for spotting wrong fields in financial calculation outputs.

**Considered alternatives:**
- `testify/assert` + `testify/require`. Most popular Go assertion library; ergonomic. Rejected —
  community has been migrating away since ~2020; the DSL hides line-of-failure details, and
  structural diffs are weaker than `cmp.Diff`'s. Defensible if a future maintainer strongly prefers,
  but starting plain is cleaner.

### What gets tested, in priority order

1. **Heavy — financial calculations.** Net worth aggregation, comprehensive-income identity
   (ADR-0008), investment-return formula, carry-forward semantics, FX conversions,
   materialized-report generation and staleness regeneration. These are the *value* of the app;
   they're pure functions that test cheaply. Target ~80%+ coverage of the calculation code —
   which lives in `internal/repo/monthly_reports_engine.go`, `internal/reports/`, and
   `internal/income/` (an `internal/finance/` package was anticipated here but never created).
2. **Heavy — tenancy isolation.** For every endpoint that touches per-Household data, a test
   verifies that requests authenticated as a different Household see zero rows. ADR-0005's threat
   model is a one-line failure away if a repository ever forgets its `household_id` filter.
3. **Medium — handlers.** Happy path plus a representative validation-failure case per endpoint.
   Don't aim for every error branch.
4. **Light — repositories.** Wrapping logic gets light coverage; covered transitively by handler
   integration tests.
5. **Skip — trivial getters, library wrappers, generated sqlc code** (sqlc is tested upstream).

## Frontend

### Unit / component tests: Vitest + React Testing Library + MSW

- **Vitest** is the Vite-native test runner; reuses our existing Vite config; near-instant feedback
  (ADR-0015).
- **React Testing Library** for component and hook tests — assert against rendered DOM, not
  implementation details.
- **MSW (Mock Service Worker)** for mocking API responses in tests. Pairs naturally with TanStack
  Query so tests exercise the actual data-fetching layer rather than a mocked client.

### What gets tested

- **Heavy:** custom forms (date pickers, currency inputs, ownership selectors), custom hooks
  (especially around TanStack Query usage), derived calculations on the frontend (e.g., side-by-side
  currency display, Q15c).
- **Skip:** shadcn/ui components — they're library code, tested upstream.
- **Skip:** trivial display components.

### E2E tests (Playwright): adopted — see [[adr-0024]]

Real browser end-to-end tests are valuable but high-maintenance. They were originally deferred for
v1 of a solo-dev personal app — they pay off mainly once a UI flow has broken in a way E2E would
have caught, multiple developers need a regression net, or the UI is stable enough that tests don't
churn.

They are now being adopted as the app's UI surface has grown. **[[adr-0024]]** records the approach:
Playwright authenticating by injecting a pre-seeded server-side session cookie (no real Google
login), against a dedicated `balances_e2e` database. Tenancy and financial-calculation coverage stay
in the Go suites described above — E2E does not take them over.

## Consequences

- `internal/testutil/` exposes helpers for spinning up a test Postgres, applying migrations,
  creating fixture Households / Users / Positions. Tests don't repeat infrastructure code.
- A "leak test" pattern is established early — realized as the `*_tenancy_test.go` files in
  `internal/repo/`, each asserting one resource's cross-Household isolation (catalogued as the
  `INV-TENANCY-*` invariants in `docs/qa/invariants/`).
- `go-cmp` is added as a dev dependency.
- Frontend tests run via `vitest run` and `vitest --watch`; CI runs the non-watch command.
- The dev container (OrbStack on the user's machine) has Docker available, satisfying
  testcontainers' requirement; CI runners must too.
- Playwright is now adopted; see [[adr-0024]] for the session-injection approach and the dedicated
  `balances_e2e` database.
