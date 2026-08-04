## Agent skills

### SDLC

The spine that orders every other tool below — idea → triage → grill/ADR → slice → TDD → guard (incl. coverage matrix) → ship → release. How far up the spine a change starts scales with how much it commits the project to a shape. See `docs/agents/sdlc.md`.

### Issue tracker

Issues live in GitHub at `kerti/balances-v2`, accessed via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default canonical strings (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout — one `CONTEXT.md` and one `docs/adr/` at the repo root. Code is split into `frontend/` and `backend/` for organisation, but the domain language is shared. See `docs/agents/domain.md`.

### Releases

Batched, tag-driven SemVer pre-releases (ADR-0029/0030). Cutting a release — pick version, label the batch, check migrations, tag, publish auto-generated notes, verify the deploy. See `docs/agents/release.md`.

### Codebase conventions

Tactical, load-bearing rules that don't rise to an ADR — tenancy filters in SQL, subtype guards, the snapshot/transaction dialog forks, decimal shapes, the error envelope, E2E `data-testid` discipline, and the rest. Consult before touching the area they cover. See `docs/agents/conventions.md`.

### Local dev / lint / tests

Makefile-based run loop, the backend-restart-after-Go-edits gotcha, smoke-test recipe, lint, and test suites. See `docs/agents/dev.md` (`make help` lists every target).

### QA coverage matrix

The app's must-hold invariants are catalogued with stable IDs in the per-zone files under `docs/qa/invariants/` (indexed by `docs/qa/README.md`; mechanism in `docs/qa/how-it-works.md`); a test declares which it verifies via a `// covers: INV-...` annotation (same token in Go and TS). When you write a test for a catalogued invariant, add the annotation; when you establish a new invariant worth guarding, add a catalog row. `make qa-matrix` regenerates the per-zone coverage under `docs/qa/coverage/` and reports uncovered invariants (advisory). `make qa-strict` is the CI gate (wired into `ci.yml` + `make check`): it fails if any invariant lacks **per-PR** coverage — uncovered, or covered only by a nightly (non-smoke) Playwright spec.

## Things explicitly NOT to do

- **Don't autoflush commits.** When work seems ready, stage + show the diff + ask. Push only on
  explicit green light. After every push, watch CI to completion (`gh run list --branch <branch>` /
  `gh run watch <id>`); if a workflow fails, surface the failure with logs and ask whether to fix now
  or defer. Don't declare a commit done while runs are queued or in_progress.
- **Don't dive into UI alone.** User has near-zero frontend skill and relies heavily on you for UI —
  but expects to be consulted on UX choices (form density, navigation, button labels). Always surface
  tradeoffs.
- **Don't fear backtracking on prior decisions** if suboptimal — pre-alpha migrations are not sacred.
  User explicitly accepted this. Flag the issue, propose the better path, let user decide.
- **Don't create planning/analysis documents** unless asked. Live state goes in `HANDOFF.md` or in
  memory; design decisions go in ADRs; nothing else.
- **Don't bypass `--no-verify` or `--no-gpg-sign`** on git commits.
- **Don't add features beyond the task.** No speculative abstractions. Three similar lines beats
  premature abstraction.
- **Don't add comments that just restate the code.** Only when WHY is non-obvious.
- **Don't auto-start the next milestone** without explicit user instruction. User pauses between
  milestones to direct.
