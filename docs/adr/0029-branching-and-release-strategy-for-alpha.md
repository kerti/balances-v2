# Branching and release strategy for alpha

Entering alpha, the project adopts **GitHub Flow**: `main` is always deployable, every change
lands through a short-lived branch and a pull request, and merges are **squash-merged** so one
issue maps to one commit on `main`. Releases are cut as **batched, tag-driven** SemVer
pre-releases starting at `v0.6.0-alpha.1`. This supersedes the pre-alpha working rule of committing
straight to `main`.

## Amended (2026-08-04)

The "HANDOFF at release time" rule below is **retired**. This ADR made issues + PRs + Release notes
the system of record for what changed, then kept `HANDOFF.md` as a parallel narrative of the same
material with a per-tag pruning ritual to stop it diverging. It diverged anyway: by
`v0.9.0-alpha.4` the file had reached 488 lines, of which ~200 restated merged PR bodies and the
per-tag Release notes, while its own rule asked for "one-line bullet per shipped item."

The fix splits the file by *what kind of thing each part is*, rather than pruning harder:

- **Codebase conventions** (155 lines) → `docs/agents/conventions.md`, indexed from `CLAUDE.md`.
  These duplicated nothing, but they are project **instruction**, not working state — they were
  filed in the one document `CLAUDE.md` does not index, while `docs/agents/{domain,dev,release}.md`
  (the same category of thing) are indexed there.
- **Behavioural rules** ("Things explicitly NOT to do") → `CLAUDE.md` directly. Always-on rules have
  to be loaded, not one hop away.
- **The per-tag ledger and the "unreleased on `main`" narrative** → **deleted**. That is exactly
  what this ADR already designated the Release notes and closed issues to hold.
- **What survives in `HANDOFF.md`** is the pointer this ADR intended: cold-start orientation, where
  the cursor is, and the standing decisions that no issue or ADR owns.

Consequences: the release runbook loses its "prune HANDOFF" step, and a feature PR no longer owes a
HANDOFF narrative bullet (the HANDOFF-atomic rule now fires only when the *cursor* moves — a
milestone opening or closing, a release cut, the active line changing, a standing decision made or
reversed). Nothing about branching, tagging, or release-notes generation changes.

## Why now

Until now the rule was "commit straight to `main`, no branches or protection until alpha" — correct
for a solo pre-alpha repo with no users and no deploy target. Alpha changes the contract: there
will be a deployed environment (see [[adr-0030]]) and outside eyes on a public repo. That makes two
things worth their small cost — a **CI gate before code reaches a deployable `main`**, and a
**reviewable diff trail** for each change. GitHub Flow buys both without the ceremony of heavier
models.

## The decision

### One short-lived branch per issue, PR per change

- Branch naming follows the existing commit convention: `feat/<n>-<slug>`, `fix/<n>-<slug>`
  (e.g. `feat/64-income-reorg`). The branch exists only for the life of one issue.
- Every change opens a PR whose description carries `closes #<n>`, so merging auto-closes the issue.
- **Squash-merge only.** History stays at one commit per issue on `main`, matching the shape the
  repo already has. The PR retains the granular history for anyone who wants it.

### Branch protection on `main`

- Require a PR (no direct pushes) and require CI green before merge.
- Linear history (squash enforces this).
- **Admin bypass stays on** — single maintainer must never be locked out of their own `main`.

### Reviews are advisory; the human merge is the gate

- `/code-review` runs on the PR and posts findings as comments. It is **non-blocking** — advice,
  not a required CI check. This keeps the maintainer unblocked when review flags noise, appropriate
  for alpha.
- **The merge is the signoff.** The maintainer reads the diff and the review, then squash-merges.
  Claude never merges; AI advises, the human decides.
- A wired CI-blocking review is rejected for alpha (can wedge on false positives) but remains a
  future option once the project tolerates the friction.

### Versioning: SemVer pre-release, batched, milestone-aligned by convention

- Start at **`v0.6.0-alpha.1`**. The `0.x` major signals an unstable schema/API honestly — fitting
  for pre-alpha→alpha migrations that are not treated as sacred.
- **Cadence is batched**: several merged PRs accumulate on `main`, then a single
  `v0.6.0-alpha.N` tag cuts a release. Tagging drives deployment (see [[adr-0030]]).
- **"Milestone = minor" is a convention, not a law.** Today milestone M6 lines up with `0.6` by
  luck; the mapping is a mnemonic, not a coupling. The minor bumps when a milestone closes; within a
  milestone the `alpha.N` counter advances per batch. If milestone numbering ever forks from release
  numbering, releases win — the version is the public contract, the milestone is internal tracking.
- Milestone close → either drop the pre-release suffix (`v0.6.0`) or roll straight into the next
  minor's alpha (`v0.7.0-alpha.1`), decided at the time.

### Release notes live in GitHub Releases; the CHANGELOG file is retired

- Each tag cuts a **GitHub Release**. The notes are a terse, **user-facing** digest grouped
  Added / Fixed / Changed, written for the non-technical audience — not a commit dump.
- From the first batched alpha onward, notes are **auto-generated from merged PRs**
  (`gh release create --generate-notes`), grouped by PR label via `.github/release.yml`. This makes
  **issues + PRs the system of record** for what changed; the hand-maintained `CHANGELOG.md` is
  retired and frozen at `docs/history/CHANGELOG-pre-alpha.md` (git history retains it regardless).
- The **first** alpha (`v0.6.0-alpha.1`) is **hand-written** — pre-alpha work landed direct-to-`main`
  with no PRs and no prior tag, so there is nothing to auto-diff. The draft lives at
  `docs/releases/v0.6.0-alpha.1.md`.
- For clean auto-grouping, each PR carries one type label (`enhancement` / `bug` / `documentation`),
  riding on the conventional-commit discipline already in place.

### HANDOFF at release time

> **Retired by the 2026-08-04 amendment above.** Kept for the record. A tag is no longer a pruning
> checkpoint, because shipped detail no longer enters `HANDOFF.md` in the first place.

`HANDOFF.md` stays the live working-state pointer; a release does not replace it. But a tag is its
**pruning checkpoint** — shipped items are now captured in the release notes and closed issues, so
their bullets are pruned from HANDOFF, leaving only in-progress / next-up state. HANDOFF's
detail-pointer retargets from the retired CHANGELOG to issues + release notes.

## Considered alternatives

- **Keep committing straight to `main`.** Rejected — no CI gate before a now-deployable `main`, and
  no review trail on a public repo gaining alpha users. The thing that made it correct (no users, no
  deploy) no longer holds.
- **Full GitFlow** (`develop` / `release/*` / `hotfix/*`). Rejected — multi-branch ceremony is pure
  overhead for a single maintainer. Nothing about alpha needs parallel release trains.
- **Trunk-based with feature flags.** Rejected — flag infrastructure is premature; short-lived
  branches give the same "small, frequent, always-green `main`" outcome without a flag system to
  build and maintain.
- **Per-issue tagging instead of batched.** Rejected — would couple every merge to a deploy; the
  batched cadence lets the maintainer accumulate merges and cut a release deliberately.

## Consequences

- Each change costs ~30 seconds of branch + PR overhead. The repeated cycle (`gh pr create`,
  `gh pr merge --squash`, tag push) is scriptable — a `make pr` / `make ship` pair is the planned
  follow-up if it proves repetitive.
- Issues auto-close via PR `closes #<n>`, replacing the manual `gh issue close` wrap step.
- A `production` GitHub Environment can later add a **required reviewer** as a second hard gate on
  top of the merge signoff (see [[adr-0030]]).
- CHANGELOG records the version scheme and the "milestone = convention not law" rule so the intent
  survives the maintainer's memory.
- HANDOFF/memory note "commit straight to `main` until alpha" is now retired by this ADR.
