# Release runbook: cutting a batched alpha

Releases are **tag-driven SemVer pre-releases** (ADR-0029). Several merged PRs accumulate on `main`,
then one `vX.Y.Z-alpha.N` tag cuts a release: pushing the tag triggers `deploy.yml`, which builds the
single-origin image, runs `goose up` inside Fly, and rolls out to the routed environment (ADR-0030).
The first alpha (`v0.6.0-alpha.1`) was hand-written; every cut **from `v0.6.0-alpha.2` onward** follows
this runbook and auto-generates notes from merged PRs.

Tag → environment routing (`deploy.yml`):

| Tag shape          | Environment  | Approval        |
|--------------------|--------------|-----------------|
| `*-alpha.N`        | `preview`    | auto            |
| `*-rc.N` / `*-beta.N` | `demo`    | auto            |
| `vX.Y.Z` (no suffix) | `production` | GitHub Environment gate |

## Pick the version

- Within a milestone, advance the `alpha.N` counter (`v0.6.0-alpha.1` → `v0.6.0-alpha.2`).
- Milestone close → drop the suffix (`v0.6.0`) or roll to the next minor's alpha (`v0.7.0-alpha.1`),
  decided at the time. "Milestone = minor" is a convention; **the version is the public contract.**

## Pre-flight checklist

Run from a clean, up-to-date `main`.

1. **Enumerate the batch.** List what landed since the last tag:
   ```sh
   PREV=$(git describe --tags --abbrev=0)
   git log "$PREV"..main --oneline
   gh pr list --state merged --base main --search "merged:>$(git log -1 --format=%cI $PREV)" \
     --json number,title,labels,mergedAt
   ```
   Squash-merge means one commit ≈ one PR. Note dependabot PRs that merged *before* the prev tag —
   they belong to the earlier batch, not this one.

2. **Label every PR in the batch** (THE recurring trap — unlabeled PRs land under "Other Changes").
   Auto-notes group by the label map in `.github/release.yml`:

   | Label         | Section            |
   |---------------|--------------------|
   | `enhancement` | ✨ Added            |
   | `bug`         | 🐛 Fixed            |
   | `documentation` | 📝 Documentation  |
   | `dependencies`| ⬆️ Dependencies     |

   Each PR carries **one** type label (rides the conventional-commit prefix: `feat`→enhancement,
   `fix`→bug, `docs`→documentation, `build(deps)`→dependencies).
   ```sh
   for n in <pr numbers>; do
     printf "#%s: " "$n"; gh pr view $n --json labels --jq '[.labels[].name]|join(",")'
   done
   gh pr edit <n> --add-label enhancement   # backfill any blanks
   ```

3. **Review the diff for surprises.**
   ```sh
   git diff --stat "$PREV"..main
   ```
   Read anything touching backend repo/query/handler code, auth, or wire types.

4. **Check for DB migrations — and whether they're breaking.**
   ```sh
   git diff --stat "$PREV"..main -- backend/internal/migrations/
   ```
   - New `NNNNN_*.sql` files run via `release_command = "migrate up"` on deploy (goose, inside Fly).
   - **Breaking?** Flag any column drop/rename, `NOT NULL` on existing tables, type narrowing, or
     destructive backfill. Preview's Neon branch is disposable (schema "not guaranteed stable" per
     the alpha notes), but a breaking migration still needs a deliberate call before tagging —
     it is irreversible against any data you care about. Confirm forward + `migrate down` both apply
     cleanly against a scratch DB before tagging.
   - No migration files changed → safe; `migrate up` is a no-op on deploy.

5. **CI is green on `main`.** `gh run list --branch main --limit 5`. The tag deploys whatever `main`
   points at — never tag a red `main`.

6. **Prune HANDOFF.md.** A tag is HANDOFF's pruning checkpoint: shipped bullets move out (their detail
   now lives in closed issues + release notes), leaving only in-progress / next-up state. Fold this
   into the same commit as any release-doc change (HANDOFF-atomic rule).

## Cut the release

1. **Tag and push** from `main` at the reviewed commit:
   ```sh
   git tag v0.6.0-alpha.2
   git push origin v0.6.0-alpha.2
   ```
   Pushing the tag is the trigger — `deploy.yml` fires on `push: tags: ['v*']`.

2. **Generate notes + publish the GitHub Release.** Notes auto-group from PR labels (step 2 above):
   ```sh
   gh release create v0.6.0-alpha.2 --prerelease --generate-notes \
     --notes-start-tag v0.6.0-alpha.1
   ```
   Then **edit the generated notes into a terse, user-facing digest** — grouped Added / Fixed /
   Changed, written for the non-technical audience, not a commit dump (ADR-0029). Issues + PRs stay
   the system of record; the Release is the rollup.

   > Note: the hand-written `docs/releases/v0.6.0-alpha.1.md` was a one-off for the first alpha.
   > From alpha.2 the GitHub Release is the artifact — no per-tag file under `docs/releases/` unless
   > we deliberately revive that.

## Verify the deploy

1. **Watch the pipeline:** `gh run watch` / `gh run list --workflow deploy.yml`. The `route` job picks
   the env; `deploy` runs `flyctl deploy` (build → `goose up` release_command → rollout).
2. **Confirm the footer** on `https://preview.<personal-domain>` shows the new tag + `preview` env —
   `APP_VERSION`/`DEPLOY_ENV` are baked into the SPA bundle at build (issue #75, `appInfo.ts`).
3. **Smoke-test** the headline flows for anything in the batch.

## Post-release

- **Close any issues** the release finishes that weren't auto-closed by their PR `closes #n`.
- Confirm HANDOFF reflects post-tag state (pruned, next-up only).
