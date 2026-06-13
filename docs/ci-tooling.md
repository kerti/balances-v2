# CI tooling — current state & pre-alpha reassessment

Living record of what runs in CI beyond the build/test/deploy basics, plus the
backlog of tooling we considered and deliberately deferred. **Revisit the
"Reassess before alpha" section before cutting the alpha release.**

Last reviewed: 2026-06-13 (pre-alpha hardening, #70).

## Wired now

| Tool | What it does | Where | Notes |
|------|--------------|-------|-------|
| golangci-lint | Go lint | `ci.yml` → `backend-lint` | pre-existing |
| go test -race + coverage | Backend tests | `ci.yml` → `backend-test` | → Codecov |
| eslint + vitest + build | Frontend checks | `ci.yml` → `frontend-checks` | → Codecov |
| Codecov | Coverage reporting (informational, not a gate) | `codecov.yml` | pre-existing |
| **CodeQL** | SAST for Go + TS/JS; Security tab + PR annotations | `codeql.yml` | added 2026-06-06; weekly cron + per-PR |
| **govulncheck** | Go dependency vuln scan (reachability-based) | `ci.yml` → `backend-vuln` | added 2026-06-06 |
| **Dependabot** | Weekly update PRs + security alerts | `dependabot.yml` | added 2026-06-06; gomod + npm + github-actions |
| **SHA-pinned actions** | Third-party Actions pinned to commit SHA (supply-chain) | all `.github/workflows/*` | added 2026-06-13 (#70); `# vN` comment lets Dependabot bump pins |
| **E2E (Playwright)** | Smoke gate per-PR + nightly full suite | `e2e.yml` → `e2e-run.yml` | added 2026-06-13 (#70); tiered via `{ tag: '@smoke' }`; offline harness (mock-oidc + `services: postgres`) |

## Why these three

Public financial app → the security surface (injection, auth flaws, vulnerable
deps) matters more than additional lint. CodeQL covers SAST, govulncheck covers
known-CVE-in-reachable-code, Dependabot keeps deps current and feeds Actions
version bumps. All GitHub-native, zero infra, free for public repos.

## Considered and rejected (for now)

- **SonarQube / SonarCloud** — declined 2026-06-06. Heavy overlap with
  golangci-lint + eslint for smell detection; the real adds (dashboard, dup
  detection, quality-gate) didn't justify a second coverage gate competing with
  Codecov or another required check alongside the existing CI jobs. Self-host adds a
  server to maintain. Revisit only if we want the trend dashboard.

## Reassess before alpha

Deferred items worth a second look once the app faces real users:

1. **gitleaks** — secret scanning in CI/history. Marginal if GitHub
   push-protection is enabled; cheap insurance for a money app.
2. **Concurrency cancellation** — `cancel-in-progress` to stop paying for stale
   runs on rapid pushes. Pure cost hygiene. `e2e.yml` already does this; `ci.yml`
   and `codeql.yml` still open.
3. **Container/Trivy scanning** — deferred with deployment; reassess when the
   deploy story lands.

## Setup notes / one-time actions

- CodeQL needs no secrets for a public repo. If the repo ever goes private,
  CodeQL requires GitHub Advanced Security.
- govulncheck pins to `@latest` so it tracks the vuln DB without a version bump;
  acceptable because it's a scanner, not a build dependency.
