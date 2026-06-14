## Agent skills

### Issue tracker

Issues live in GitHub at `kerti/balances-v2`, accessed via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

Default canonical strings (`needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`). See `docs/agents/triage-labels.md`.

### Domain docs

Single-context layout — one `CONTEXT.md` and one `docs/adr/` at the repo root. Code is split into `frontend/` and `backend/` for organisation, but the domain language is shared. See `docs/agents/domain.md`.

### Releases

Batched, tag-driven SemVer pre-releases (ADR-0029/0030). Cutting a release — pick version, label the batch, check migrations, tag, publish auto-generated notes, verify the deploy. See `docs/agents/release.md`.

### Local dev / lint / tests

Makefile-based run loop, the backend-restart-after-Go-edits gotcha, smoke-test recipe, lint, and test suites. See `docs/agents/dev.md` (`make help` lists every target).

### QA coverage matrix

The app's must-hold invariants are catalogued with stable IDs in the per-zone files under `docs/qa/invariants/` (indexed by `docs/qa/README.md`; mechanism in `docs/qa/how-it-works.md`); a test declares which it verifies via a `// covers: INV-...` annotation (same token in Go and TS). When you write a test for a catalogued invariant, add the annotation; when you establish a new invariant worth guarding, add a catalog row. `make qa-matrix` regenerates the per-zone coverage under `docs/qa/coverage/` and reports uncovered invariants (advisory; `-strict` is the future CI gate).
