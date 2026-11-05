# Agent operating model: one orchestrator, pinned role agents

balances-v2 is built by one maintainer working through AI agents, and until now `CLAUDE.md` said
what an agent may *do* without saying what an agent *is*. One session did everything: it read
half the repo to answer a small question, and it brought the same depth to a mechanical
sweep across dozens of files as to the finance engine. That spends the scarce thing — context in the main
window — on cheap work, and leaves no clear place for depth where it pays.

Model and effort are configuration, not prose. They are set by `.claude/settings.json` and by
`.claude/agents/*.md` frontmatter (`model`, `effort`); a sentence in `CLAUDE.md` asking for "maximum
effort" changes nothing. So this decision lands as committed config, adopted from kerti/uruni's
ADR-023, which has run this model since 2026-08.

## Decisions

1. **The main session is the orchestrator: Opus at `medium` effort**, pinned by `model` (the
   `opus` alias) and `effortLevel` in a committed `.claude/settings.json`. It plans, makes and
   surfaces decisions, dispatches, verifies diffs, writes PR bodies and asks for commits. A turn
   that needs more takes `ultrathink` or `/effort`; the pin is a default, not a ceiling.
2. **Six role agents in `.claude/agents/`**, each pinning its own tier and effort: `researcher`
   (sonnet · medium, read-only), `planner` (sonnet · xhigh), `grill` (sonnet · max), `builder`
   (sonnet · medium), `builder-deep` (sonnet · high), `reviewer` (sonnet · high, read-only). Their
   briefs carry this repo's rules (decimal strings, tenancy in SQL, the error envelope, both
   locale catalogs, invariant annotations, PII scrubbing). `builder-deep` owns the trust core here:
   the finance and report engine, tenancy, migrations and the backup format, CSV import, auth, and
   how money is parsed, rounded and formatted.
3. **`xhigh`, not `max`, is the planning default;** `max` is reserved for `grill`, where
   overthinking is the assignment.
4. **Delegation is approval-gated.** `.claude/hooks/agent-gate.sh` answers `ask` on every
   `PreToolUse(Agent)`, built-in agents included. The built-in `fork` type is switched off
   (`CLAUDE_CODE_FORK_SUBAGENT=0`) because it forces every subagent into the background.
5. **Delegation is a context optimization, not a token one.** A subagent re-derives the repo cold
   and misses the prompt cache, so total spend rises; what it buys is a main window that keeps
   its judgment. Below roughly three files or two tool calls, inline wins.
6. **The sign-off surface is never delegated:** commits, pushes, PRs and labels, issue writes,
   tags and releases, accepting an ADR, UX choices, scope calls.
7. **Blocking yes, polling no.** Agents run `make check`, `go test`, `vitest` and wait. Only the
   orchestrator runs the Playwright suite, in the background, when a slice needs it or the
   maintainer asks, and always before a release tag. This extends the existing never-watch-CI rule.

## Considered options

- **No pinning, rules in `CLAUDE.md` only.** Cheaper to set up, but effort cannot be set from
  prose, so the main lever would stay unpulled.
- **Ungated delegation.** Fewer prompts, but an `Agent` call is the one action here that can
  spend a lot without touching a file; cost control the maintainer can see beats a rule the
  orchestrator is asked to remember.
- **A seventh `verifier` agent** for browser passes and e2e triage. Not now: the six cover it, and
  a role can be added when a real slice shows the need.

## Consequences

- `CLAUDE.md` loads into every subagent on every launch, so its length is now a running cost; the
  new section is compressed, and reference material lives in `.claude/README.md`, which agents
  don't load.
- The pins apply to every clone and are overridable: `/effort`, `--effort`, and silently
  `CLAUDE_CODE_EFFORT_LEVEL`. `opus` and `sonnet` are aliases, so the model behind a tier moves
  when a new one ships, with no commit here.
- Role agents inherit the session's permissions but not its conversation, which makes the
  brief-writing rules load-bearing.
- Every delegation adds a permission prompt, accepted deliberately.
