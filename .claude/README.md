# `.claude/`: operator notes

For the maintainer, not the agents. Claude Code loads `settings.json`, `agents/*.md`, `hooks/*.sh`
and `skills/`; it does not load this file. That is why the reference material lives here rather
than in `CLAUDE.md`, which is read into **every** subagent on **every** launch, so a line added
there is paid on each delegation.

The rules agents follow are in [`CLAUDE.md`](../CLAUDE.md); the reasoning is
[ADR-0057](../docs/adr/0057-agent-operating-model.md).

## What's here

| Path | Loaded when | Purpose |
|---|---|---|
| `settings.json` | Session start | Pins the orchestrator (`opus` · `medium`), turns off fork subagents, wires the agent gate |
| `settings.local.json` | Session start | Yours, gitignored. Personal permission approvals |
| `agents/*.md` | On delegation | One role per file; `model` and `effort` in the frontmatter are what actually binds those choices |
| `hooks/agent-gate.sh` | Before every Agent call | Turns every subagent spawn into a permission prompt (needs `jq`; without it the hook stays silent) |
| `skills/dependabot-batch/`, `skills/release/` | When asked | Repo workflows (see `docs/agents/`) |

`settings.json` is committed, so a fresh clone asks once to trust the workspace before the hook
runs. If the hook stops prompting after a pull, check that it is still executable
(`chmod +x .claude/hooks/agent-gate.sh`).

## Overriding the pinned defaults

| You want | Do this |
|---|---|
| Deeper reasoning, **one turn** | Put `ultrathink` anywhere in the prompt. Nothing to undo afterwards. Reach for this first. |
| Deeper reasoning, **this session** | `/effort max`; `max` is always session-only |
| A different level **going forward** | `/effort high` (or `low`/`medium`/`xhigh`) |
| Back to the project default | `/effort auto` |
| A different model | `/model` |
| **No delegation**, do it in the main session | Answer *no* at the agent-gate prompt, or say "do this inline" |
| Set it at launch | `claude --effort xhigh` |

Two ways this bites:

- **`/effort low|medium|high|xhigh` persists across sessions.** Set it once and it outlives the
  task. `/effort auto` is the undo; `max` and `ultrathink` never persist.
- **`CLAUDE_CODE_EFFORT_LEVEL` beats everything**, silently. If effort isn't behaving,
  `env | grep CLAUDE_CODE_EFFORT` first.

## The cost model, in one paragraph

Effort is the biggest lever: thinking tokens are billed at the output rate on every turn, trivial
ones included, which is why the orchestrator runs at `medium`. Delegation raises total tokens (a
cold start, no prompt cache) but keeps the exploration out of the main window, where it would
otherwise be re-read, cached, on every later turn. It inverts below a few files: re-reading
something the orchestrator already has cached is far cheaper than a subagent reading it cold.
Keep briefs small and self-contained; the context dump is the expensive part.
