# ADR index

One line per decision so you can pick which to open without reading all 40. Read the full ADR before
touching the area it governs.

| ADR | Decision | Touches |
|----|----|----|
| [0001](0001-snapshot-based-net-worth-tracking.md) | Net worth via month-end balance snapshots; no per-transaction cash tracking | core model |
| [0002](0002-multi-currency-native-and-reporting.md) | Store native currency on every amount; convert to reporting currency via monthly FX | currency, FX |
| [0003](0003-income-events-decoupled-from-bank-balances.md) | Income / investment cash events do **not** auto-update bank snapshots | income, transactions |
| [0004](0004-household-scope-and-sole-joint-ownership.md) | Data is Household-scoped; each Position is Sole or Joint owned | tenancy, ownership |
| [0005](0005-single-db-row-level-multi-tenancy.md) | One DB, row-level tenancy filtered by `household_id` in SQL | tenancy, queries |
| [0006](0006-materialized-monthly-net-worth-reports.md) | Monthly reports are materialized, regenerated lazily on write | reports, dashboard |
| [0007](0007-soft-delete-for-domain-mutations.md) | Soft-delete everything incl. snapshots; hard-delete is not a UI feature | all mutations |
| [0008](0008-earned-income-tracking-with-derived-returns-and-residual-expenses.md) | Earned income explicit; investment return derived; expenses are the residual | income, comprehensive income |
| [0009](0009-position-storage-lifecycle-and-maturity-disposition.md) | Position storage, status/lifecycle, Maturity disposition (roll vs cash-out) | positions, maturity |
| [0010](0010-user-and-household-entity-shape.md) | User + Household entity shapes and fields | identity |
| [0011](0011-decimal-precision-for-amounts-quantities-and-fx-rates.md) | DECIMAL(20,4) money, (20,8) qty/rates/FX; decimals are strings on wire | data types |
| [0012](0012-monthly-report-row-hybrid-column-layout.md) | Monthly report row: hybrid columns + JSON layout | reports |
| [0013](0013-postgresql-with-docker-portability-hosting-deferred.md) | PostgreSQL via Docker; hosting deferred (see 0030) | infra |
| [0014](0014-chi-as-the-go-http-router.md) | Chi as the Go HTTP router | backend routing |
| [0015](0015-react-with-vite-typescript-tanstack-query-and-shadcn-ui.md) | React + Vite + TS + TanStack Query + shadcn/ui | frontend stack |
| [0016](0016-mobile-as-pwa-with-capacitor-upgrade-path.md) | Ship mobile as a PWA; Capacitor is the upgrade path | mobile |
| [0017](0017-google-oauth-server-side-sessions-and-email-token-invitations.md) | Google OAuth, server-side sessions, email-token invites | auth |
| [0018](0018-pgx-and-sqlc-for-typed-postgres-access.md) | pgx + sqlc for typed Postgres access | backend data |
| [0019](0019-goose-for-migrations-embedded-in-the-app-binary.md) | goose migrations embedded in the app binary | migrations |
| [0020](0020-backend-miscellany-validator-env-slog-resend.md) | Backend libs: validator, env-config, slog, Resend mail | backend infra |
| [0021](0021-testing-strategy.md) | Testing strategy across the stack | tests |
| [0022](0022-snapshot-table-strategy-per-group.md) | One snapshot table per position group — no polymorphic table | snapshots, schema |
| [0023](0023-investment-transaction-table-strategy.md) | Single polymorphic table for investment transactions | transactions, schema |
| [0024](0024-e2e-tests-with-playwright-and-session-injection.md) | E2E via Playwright with session injection | e2e |
| [0025](0025-client-side-routing-with-react-router-and-a-sidebar-shell.md) | React Router + sidebar shell; routes from `routes.ts` constants | frontend routing, nav |
| [0026](0026-internationalization-with-react-i18next-and-a-persisted-user-locale.md) | i18n via react-i18next + persisted per-user locale | i18n |
| [0027](0027-backend-error-code-envelope.md) | 4xx/5xx ship `{code, args}` envelope via `internal/httperr` | errors, API |
| [0028](0028-user-defined-position-tags.md) | User-defined Tags: at most one per Position, no built-in financial meaning | tags |
| [0029](0029-branching-and-release-strategy-for-alpha.md) | GitHub Flow; tag-driven batched SemVer pre-releases | release, branching |
| [0030](0030-hosting-and-deployment-for-alpha.md) | Single-origin Fly app + Neon + Cloudflare DNS; tag→deploy pipeline | hosting, deploy |
| [0031](0031-baseline-migration-squash-at-alpha.md) | Squash pre-alpha migrations into one baseline at alpha | migrations |
| [0032](0032-toast-feedback-for-buttonless-autosaves.md) | Toast feedback for buttonless autosaves | frontend UX |
| [0033](0033-versioning-the-upgrade-contract-and-migration-immutability.md) | SemVer = machine upgrade contract (not the "Balances" brand); `1.0.0` first prod; migration immutability + squash rules; majors invisible to pipeline | versioning, release, migrations |
| [0034](0034-where-ui-ux-decisions-are-documented.md) | UI/UX documented as a Presentation section on the relevant behavior ADR; UI-native concerns get their own ADR; both are the basis for UI invariants in the QA matrix | docs, UX, qa |
| [0035](0035-pre-auth-language-picker-locale-precedence-and-backend-email-i18n.md) | Pre-auth language picker; locale precedence; backend email i18n; default `en-GB` | i18n, auth |
| [0036](0036-comprehensive-household-backup-and-restore.md) | Whole-Household backup as versioned `.json.gz`; restore = streaming preview→commit wipe-then-load into a fresh/wiped Household; identity re-link by Google `sub`; format immutability at release | backup, restore, portability |
| [0037](0037-self-hostable-docker-compose-stack.md) | Operator self-host artifact: pull-based GHCR image + Postgres compose; one-shot migrate service; `APP_URL` origin collapse; 3 TLS topologies (localhost / BYO-proxy / bundled Caddy); `EMAIL_ENABLED` flag makes mail optional; Google-only auth | self-host, deploy, infra |
| [0038](0038-post-auth-onboarding-gate.md) | First sign-in branches founder-vs-join on the **verified email** (not the invite link), after auth, via an explicit gate; pending identity held in a transient DB **onboarding handshake** (not a signed cookie); founding is deliberate; dangling invites left to expire | auth, onboarding |
| [0039](0039-optional-local-password-auth-for-self-hosting.md) | Optional email+password (Argon2id) auth alongside Google, selected by boot flags (`AUTH_GOOGLE_ENABLED`/`AUTH_LOCAL_ENABLED`); `google_sub` nullable + `password_hash` + at-least-one-credential CHECK; local-only self-host needs no Google project / no OIDC call; reuses sessions + onboarding gate; invite link = email proof; password creds in a backup-excluded `local_credentials` table (no secret in the file), restored local members dormant→founder-assisted in-app reactivation (per-member one-time secret, no shared default) or CLI, email-scoped membership guard (amends 0036) | auth, self-host, backup |
| [0040](0040-household-erasure.md) | Household erasure: founder-only, single-endpoint hard delete reusing `wipeHousehold` (restore's wipe with no load); server-enforced confirm-by-name; UI-only export nudge (no server-side gate); best-effort peer notification captured pre-wipe; no re-login, dedicated post-erasure screen | backup, auth, GDPR |
| [0041](0041-demo-environment-shared-account-and-nightly-reset.md) | Demo: one shared pre-founded Household (`FOUNDING_DISABLED`) not per-visitor sandboxes; `DEMO_MODE` umbrella flag blocks Erasure, exposes login-prefill hint, gates the nightly-reset endpoint; GitHub Actions cron reuses `wipeHousehold` + reseed, bearer-authed by `DEMO_RESET_TOKEN` | demo, auth, backup |
