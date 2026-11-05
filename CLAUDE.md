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

### Dependabot batches

Fold every open Dependabot PR into one PR — exact versions, npm-regenerated lockfile, `make licenses`, `go.mod` `toolchain` moved with a golang image bump, no `Closes` lines. See `docs/agents/dependabot-batch.md` (skill: `dependabot-batch`).

### Codebase conventions

Tactical, load-bearing rules that don't rise to an ADR — tenancy filters in SQL, subtype guards, the snapshot/transaction dialog forks, decimal shapes, the error envelope, E2E `data-testid` discipline, and the rest. Consult before touching the area they cover. See `docs/agents/conventions.md`.

### Local dev / lint / tests

Makefile-based run loop, the backend-restart-after-Go-edits gotcha, smoke-test recipe, lint, and test suites. See `docs/agents/dev.md` (`make help` lists every target).

### QA coverage matrix

The app's must-hold invariants are catalogued with stable IDs in the per-zone files under `docs/qa/invariants/` (indexed by `docs/qa/README.md`; mechanism in `docs/qa/how-it-works.md`); a test declares which it verifies via a `// covers: INV-...` annotation (same token in Go and TS). When you write a test for a catalogued invariant, add the annotation; when you establish a new invariant worth guarding, add a catalog row. `make qa-matrix` regenerates the per-zone coverage under `docs/qa/coverage/` and reports uncovered invariants (advisory). `make qa-strict` is the CI gate (wired into `ci.yml` + `make check`): it fails if any invariant lacks **per-PR** coverage — uncovered, or covered only by a nightly (non-smoke) Playwright spec.

## How agents work here (ADR-0057)

The main session is the **orchestrator**: Opus at `medium` effort, pinned in `.claude/settings.json`. It holds the plan, makes and surfaces the decisions, reads the diffs, writes the PR body, and asks for the commit. Bulk reading, writing and reasoning go to the role agents in `.claude/agents/`, each pinned to a model tier and effort in its frontmatter. **The frontmatter is the mechanism; this table is the map.**

| Agent | Tier · effort | Give it |
|---|---|---|
| `researcher` | sonnet · medium | Where something lives, how it already works, what exists before we add another. Read-only. |
| `planner` | sonnet · xhigh | Docs, ADRs, issue breakdowns, slice sequencing (`to-issues`, `triage`). Drafts issues; never files them. |
| `grill` | sonnet · max | Stress-testing a plan or draft ADR before code exists (`grill-with-docs`). |
| `builder` | sonnet · medium | Code whose shape is decided: wiring, UI assembly, copy/i18n, fixtures, mechanical sweeps. |
| `builder-deep` | sonnet · high | Finance/report engine, tenancy, migrations and backup format, CSV import, auth, money handling, unexplained failures. |
| `reviewer` | sonnet · high | A finished slice, before the PR. Read-only. |

- **Delegation needs a yes, every time.** `.claude/hooks/agent-gate.sh` makes every `Agent` call a permission prompt. Propose the batch in one message (agent, task, why it beats inline); one yes covers that batch only.
- **Delegate for context, not tokens.** A subagent starts cold and misses the prompt cache, so spend goes *up*; what it buys is a clean main window. Under ~3 files or 2 tool calls, do it inline. Anything that would dump listings, logs, screenshots or search results into the main window, delegate.
- **Every brief carries its own context:** issue, branch, exact paths, ADRs in play, definition of done, "do not commit, push, or open a PR". Dispatch in the foreground.
- **Reports come back bounded** (paths changed, decisions made, what's open, what contradicted the docs) — and **are never trusted**: confirm a builder with `git diff` and `make check` yourself.
- **Never delegated:** commits, pushes, PRs and labels, issue writes, tags/releases, ADR acceptance, UX choices, scope calls. That is the sign-off surface.
- **One writer per path.** Parallel code work uses `isolation: worktree`; migrations and generated files are single-writer.
- **Agent context is public-repo context.** No agent reads `.env`, server logs or the dev DB (a real household's data) without a reason in the brief.
- **Blocking yes, polling no.** Agents run `make check` / `go test` / `vitest` and wait. Only the orchestrator runs `make e2e` (in the background, when a slice needs the full suite or the maintainer asks, and always before a release tag); agents never run it, a foreground server, a `--watch` mode, or a CI query.

## Things explicitly NOT to do

- **Don't autoflush commits.** When work seems ready, stage + show the diff + ask. Push only on
  explicit green light.
- **Don't watch PR CI.** After pushing, open the PR and stop — no `gh run list` / `gh run watch`
  polling (that policy was retired 2026-06-10; idling on a re-triggerable pipeline burns tokens). The
  user watches it and cold-starts an agent if a check fails; then `gh run view <id> --log-failed`,
  summarise the cause, and ask "fix now or defer?" before touching anything. Watching the **deploy**
  after a release tag is separate and still wanted — see `docs/agents/release.md`.
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
