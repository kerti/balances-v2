---
name: release
description: Cut a batched alpha/rc/production release for balances-v2. Tag-driven SemVer pre-releases per ADR-0029/0030. Use when user says "cut a release", "ship alpha", "tag a release", or wants to publish a new version.
---

# Release Runbook

Source of truth: `docs/agents/release.md`. This skill walks the steps in order.

## Tag → environment routing

| Tag shape | Environment | Approval |
|---|---|---|
| `*-alpha.N` | `preview` | auto |
| `*-rc.N` / `*-beta.N` | `demo` | auto |
| `vX.Y.Z` (no suffix) | `production` | GitHub gate |

## Step 1 — Pick the version

- Within milestone: advance `alpha.N` counter
- Milestone close: drop suffix or roll to next minor's alpha
- Confirm with user before proceeding

## Step 2 — Pre-flight (run from clean main)

```sh
PREV=$(git describe --tags --abbrev=0)
git log "$PREV"..main --oneline
gh pr list --state merged --base main \
  --search "merged:>$(git log -1 --format=%cI $PREV)" \
  --json number,title,labels,mergedAt
```

Check CI on main: `gh run list --branch main --limit 5` — never tag a red main.

## Step 3 — Label every PR in the batch

Unlabeled PRs fall to "Other Changes". Each PR needs exactly one type label:

| Prefix | Label |
|---|---|
| `feat` | `enhancement` |
| `fix` | `bug` |
| `docs` | `documentation` |
| `build(deps)` | `dependencies` |
| `test`/`ci`/`build`/`chore` | `enhancement` |

```sh
for n in <pr numbers>; do
  printf "#%s: " "$n"; gh pr view $n --json labels --jq '[.labels[].name]|join(",")'
done
gh pr edit <n> --add-label enhancement
```

## Step 4 — Check migrations

```sh
git diff --stat "$PREV"..main -- backend/internal/migrations/
```

- New `NNNNN_*.sql` files → migration-bearing cut → **back up DB first** (step 5)
- None → safe, skip step 5

Breaking signals: column drop/rename, `NOT NULL` on existing tables, type narrowing. Confirm `migrate up` + `migrate down` both apply cleanly on a scratch DB before tagging.

## Step 5 — Back up DB (migration-bearing cuts only)

```sh
# Neon branch snapshot (preferred, instant)
neonctl branches create --name "preview-pre-<tag>" --parent preview

# pg_dump (belt-and-suspenders)
pg_dump "$PREVIEW_DATABASE_URL" --no-owner --no-privileges -Fc -f "balances-preview-<tag>.dump"
```

Keep `.dump` off the repo — real data.

## Step 6 — Prune HANDOFF.md

Shipped bullets move out; only in-progress / next-up state remains. Do this in its own commit before tagging, or fold into step 7's release-doc commit.

## Step 7 — Tag and push

```sh
git tag v0.6.0-alpha.2
git push origin v0.6.0-alpha.2
```

Pushing the tag triggers `deploy.yml`.

## Step 8 — Generate and rewrite release notes

```sh
gh release create <tag> --prerelease --generate-notes --notes-start-tag <prev-tag>
gh release view <tag> --json body --jq .body   # capture the What's Changed block
```

Build the full body (digest + fold) and publish in **one shot** — `gh release edit` replaces the entire body:

```sh
gh release edit <tag> --notes "$(cat notes.md)"
```

### Notes template

```
> **Alpha preview** on the `preview` environment. Schema is not guaranteed stable
> between alphas; data may be reset. <No schema changes this release. | Includes database migrations, applied automatically on deploy.>

<one-line plain-language summary — do NOT lead with the version number>

## ✨ Added
- **Bold lead-in.** What the reader can now do.

## 🐛 Fixed
- The fix from the user's point of view.

## 🔧 Behind the scenes
- Tooling / docs / CI / refactors with no user-visible change.

---

<details>
<summary>Full technical changelog</summary>

<the captured ## What's Changed block + Full Changelog link, verbatim>

</details>
```

Rules:
- Banner blockquote always first; wording varies by channel (`alpha` → "Alpha preview", `rc`/`beta` → "Release candidate", `vX.Y.Z` → drop instability line)
- No PR numbers in the digest — they live in the fold
- Omit empty sections
- Product name: **Balances** (capital B)
- Keep blank line after `<summary>` and before `</details>` — GitHub needs it

## Step 9 — Verify the deploy

```sh
gh run watch   # or gh run list --workflow deploy.yml
```

Confirm the app footer shows the new tag + env. Smoke-test headline flows for anything in the batch.

## Step 10 — Post-release

- Close any issues the release finishes that weren't auto-closed
- Confirm HANDOFF reflects post-tag state (pruned, next-up only)
