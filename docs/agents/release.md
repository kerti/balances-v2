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
   `fix`→bug, `docs`→documentation, `build(deps)`→dependencies). **Test-only and CI/dev/build tooling
   PRs (`test`/`ci`/`build`/`chore`) go under `enhancement`** — there's no dedicated `chore`/`test`
   label (decided 2026-06-17).
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

   **Numbering — renumber at merge (not timestamps).** Goose reads the version from the
   `NNNNN_` filename prefix; `migrations.go` is a bare `//go:embed *.sql` glob with no registry,
   so a migration's number lives only in its filename. Author against the next free number at
   branch time; if it's taken by the time you merge (two `NNNNN_*` files share a prefix in the
   diff), bump the later one — `git mv 00002_foo.sql 00003_foo.sql`, nothing else to touch. Keeps
   apply-order == merge-order. The human squash-merge is the serialization point where this surfaces.

5. **CI is green on `main`.** `gh run list --branch main --limit 5`. The tag deploys whatever `main`
   points at — never tag a red `main`.

6. **Prune HANDOFF.md.** A tag is HANDOFF's pruning checkpoint: shipped bullets move out (their detail
   now lives in closed issues + release notes), leaving only in-progress / next-up state. Fold this
   into the same commit as any release-doc change (HANDOFF-atomic rule).

## Back up the database (migration-bearing cuts only)

**Trigger:** step 4 found one or more `NNNNN_*.sql` in the batch, **or** the batch carries a
schema/data change you couldn't reproduce by replaying migrations. A **no-migration cut** (e.g.
`v0.6.0-alpha.5`) needs no snapshot — skip this section.

This is the **operator's** DB snapshot of the deployed environment, taken *before* `deploy.yml` runs
`migrate up`. It is **not** the in-app **household** backup (ADR-0036) — that's a per-household JSON
export *inside* the product; this is a whole-database point-in-time copy the operator can roll back to.
Two layers, in order of convenience:

1. **Neon branch snapshot (instant, preferred).** Neon keeps point-in-time history and branches with
   no data copy (ADR-0030). Before pushing the tag, branch off the env's branch as a named restore
   point — from the Neon console, or the CLI if it's set up:
   ```sh
   neonctl branches create --name "preview-pre-<tag>" --parent preview   # e.g. preview-pre-v0.7.0-alpha.1
   ```
   If the migration goes wrong, restore by resetting `preview` to that branch (Neon console →
   *Restore*), or repoint the app's `DATABASE_URL` at the snapshot branch. Seconds, no dump/restore.

2. **`pg_dump` (portable, off-vendor) — belt-and-suspenders.** A copy that survives leaving Neon
   (ADR-0013's portability constraint; ADR-0030 leans on exactly this):
   ```sh
   pg_dump "$PREVIEW_DATABASE_URL" --no-owner --no-privileges -Fc -f "balances-preview-<tag>.dump"
   # roll back into a scratch / replacement DB:
   pg_restore --no-owner --no-privileges --clean --if-exists -d "$TARGET_DATABASE_URL" "balances-preview-<tag>.dump"
   ```
   `$PREVIEW_DATABASE_URL` is the env's Neon connection string — the value of the `DATABASE_URL` Fly
   secret (`fly secrets list -a <app>` shows the name, not the value; copy the string from the Neon
   console). Keep the `.dump` off the repo — it contains real data.

> Alpha/preview scope only. The full production backup cadence, retention, and DR policy are deferred
> to the `production` ADR that ADR-0030 promises ("backups, observability, … when it is stood up").

## Cut the release

1. **Tag and push** from `main` at the reviewed commit:
   ```sh
   git tag v0.6.0-alpha.2
   git push origin v0.6.0-alpha.2
   ```
   Pushing the tag is the trigger — `deploy.yml` fires on `push: tags: ['v*']`.

2. **Generate the auto-notes**, then **rewrite the body to the template below.** First capture the
   auto-generated PR list — you'll paste it into the fold:
   ```sh
   gh release create v0.6.0-alpha.2 --prerelease --generate-notes \
     --notes-start-tag v0.6.0-alpha.1
   gh release view v0.6.0-alpha.2 --json body --jq .body   # grab the `## What's Changed` block
   ```

   ### Release-notes template

   Every release from `v0.6.0-alpha.2` onward follows this shape (alpha.1 was a one-off product tour).
   Build the whole body, then publish it in one shot:
   ```sh
   gh release edit <tag> --notes "$(cat notes.md)"
   ```

   ```
   > **Alpha preview** on the `preview` environment. Schema is not guaranteed stable
   > between alphas; data may be reset. <MIGRATION CLAUSE>

   <one-line, plain-language summary of what this release is — do NOT lead with the version number>

   ## ✨ Added
   - **Bold lead-in.** What the reader can now do, in their words — not how it's built.

   ## 🐛 Fixed
   - The fix from the user's point of view.

   ## 🔧 Behind the scenes
   - Tooling / docs / CI / refactors with no user-visible behavior change.

   ---

   <details>
   <summary>Full technical changelog</summary>


   <the captured `## What's Changed` block + `**Full Changelog**` link, verbatim>

   </details>
   ```

   **Rules:**
   - **Banner blockquote is always first**, carries the environment + the stability disclaimer. Wording
     swaps by channel: `*-alpha.N` → "**Alpha preview** on the `preview` environment…"; `*-rc.N` /
     `*-beta.N` → "**Release candidate** on the `demo` environment."; `vX.Y.Z` → drop the instability
     line (prod is the stable contract, ADR-0033).
   - **`<MIGRATION CLAUSE>`** ends the banner — one of "No schema changes this release." /
     "Includes database migrations, applied automatically on deploy." (from step 4).
   - **Never lead the body with the version number** — it's already the release title.
   - **Sections are `## ✨ Added`, `## 🐛 Fixed`, `## 🔧 Behind the scenes`** (add `## 🔁 Changed` only
     for a real behavior change that's neither). Omit any that's empty. No other headings/emoji.
   - **Digest voice:** bold lead-in + plain explanation for the non-technical household audience
     (ADR-0029). No PR numbers in the digest — they live in the fold.
   - **Tail:** `---`, then the `<details><summary>Full technical changelog</summary>` block. **Keep the
     blank line after `<summary>` and before `</details>`** — GitHub needs it to render the markdown
     inside. Summary text is exactly "Full technical changelog".
   - The product is **Balances** (capitalised) in all copy.

   > ⚠️ `gh release edit … --notes "<body>"` **replaces the entire body — it does not append.** Assemble
   > the full body (digest + fold) and pass it once. Dropping the PR list or the collapsible silently is
   > the trap (hit both on alpha.5; alpha.2–alpha.5 were later retrofitted to this template).

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
- **Bump the self-host operator default tag.** If this cut is the new recommended self-hostable
  version, update `BALANCES_TAG` in `.env.example` and the three mirrors in `SELF-HOSTING.md`
  (image-tag example, quickstart snippet, config-reference table) to the new tag — it drifts
  silently otherwise (#353).
