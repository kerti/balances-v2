# Batching Dependabot PRs

Fold every open Dependabot PR into one PR. Fixing each on its own branch doesn't scale: a Dependabot
PR someone pushed to stops rebasing itself, and every PR costs its own update-branch + CI + merge
cycle. One consolidated PR costs one. Ported from uruni's `dependabot-batch` skill (uruni#594).

CLAUDE.md's authority rules bind every step: ask before committing (a "PR it" is the grant for
commit + push + open), never merge, never watch CI, never close, merge or comment on a Dependabot PR.

## 1. Inventory

```sh
gh pr list --author app/dependabot --state open \
  --json number,title,headRefName,labels,files,statusCheckRollup
```

For each PR note the ecosystem (gomod, npm, github-actions, docker), every package with its
from → to version (a grouped PR lists several in its body), and whether it is a **security** update.

Read the failing checks before blaming the bump. When every PR fails the same check (Backend vuln
scan, Container image scan), the cause is usually upstream of all of them — a new Go stdlib advisory
fixed by a toolchain bump, not by any of the PRs. Pull it out with
`gh run view <id> --log-failed | grep -A8 '^Vulnerability #'` (govulncheck) or `grep -E 'CVE-|Total:'`
(Trivy). `Third-party notices freshness` red on an npm PR is expected — step 4 fixes it — and that
check is not required.

Show the maintainer the list before touching anything. A green PR that touches neither the notices
nor the Go toolchain can simply be merged by the maintainer — say so and leave it out unless asked.
A lone security update can go through this as a batch of one.

## 2. Branch

```sh
make start-task && git switch -c deps/batch-<yyyy-mm-dd>
```

## 3. Apply, one ecosystem at a time

Run each ecosystem's checks before moving to the next, so a failure names one ecosystem. Use the
exact versions Dependabot proposed — never "latest".

- **gomod (`backend/`):** `go get <module>@<version>` per module, then `go mod tidy`. Check:
  `go build ./... && go test ./...`. Read the changelog of anything in the money path
  (`shopspring/decimal`) for rounding or serialisation changes before including it.
- **npm (`frontend/`):** `npm install <pkg>@<version>` (`-D` for a devDependency), keeping the range
  style `package.json` already uses for that package. **npm regenerates `package-lock.json`; never
  hand-merge or cherry-pick a lockfile.** Check: `npx tsc -b` (not `--noEmit` — a no-op here),
  `npm run lint`, `npm run format:check`, `npm run test`, `npm run build`.
- **github-actions:** edit the `uses:` lines to the new full commit SHA with its `# vX.Y.Z` comment,
  exactly as `gh pr diff <n>` has them. Keep every action SHA-pinned. Run `actionlint` if installed.
- **docker:** copy the new `tag@sha256:...` from the PR diff. For a **golang** image bump, set
  `backend/go.mod`'s `toolchain` line to the same version — CI's `setup-go` reads it via
  `go-version-file`, so this is what makes CI test and govulncheck scan the compiler the image
  ships. Leave the `go` line alone: it is the module's minimum language version and moves only by
  deliberate review.

A PR whose diff touches something else (a workflow input, a config file) — apply it from
`gh pr diff <n>` verbatim and say what it was.

If an ecosystem's checks fail and the cause isn't obvious within that ecosystem, stop and report:
which update, the failing output, and the options (drop that update from the batch, or fix it).
Don't widen the batch into a refactor.

## 4. Regenerate and gate

```sh
(cd frontend && npm ci)   # make licenses reads LOCAL node_modules; a stale tree downgrades entries
make licenses             # THIRD-PARTY-NOTICES
git diff THIRD-PARTY-NOTICES
make check
```

Read the notices diff: every changed entry should move **forward** to a version in this batch. An
entry moving backward means `node_modules` was stale — re-run `npm ci` and regenerate. Report
whether the notices changed and why (package added/dropped from the bundle, licence text changed).

## 5. Commit and PR

Ask before committing unless the maintainer said "PR it". Commit message:

```
build(deps): batch dependabot updates (<yyyy-mm-dd>)
```

Body: one line per update — `package from → to (#n)` — then a line for anything regenerated
(lockfile, `THIRD-PARTY-NOTICES`, the `toolchain` line). End with the session's attribution lines.

Open one PR labelled `dependencies`, with an `Area | Change` table of the updates and a
"Verified locally" list, and **no `Closes #n` lines**. Dependabot sees that `main` already has each
version and closes its own PRs; a `Closes` line would close them as "completed" under our name and
hide a version Dependabot still thinks is pending. Report the PR link and stop — no CI watching.

## 6. After the maintainer merges

One call, not a loop:

```sh
gh pr list --author app/dependabot --state open --json number,title
```

Dependabot usually closes the superseded PRs within minutes of the merge, but only when it next
re-checks. Report any still open with their numbers; the maintainer decides (wait,
`@dependabot recreate`, or close). Never close them yourself.
