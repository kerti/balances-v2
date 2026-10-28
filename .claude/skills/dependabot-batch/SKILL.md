---
name: dependabot-batch
description: Consolidate open Dependabot PRs into one PR for balances-v2 - apply every update at its exact version on one branch, regenerate the lockfile and THIRD-PARTY-NOTICES, move go.mod's toolchain line with a golang image bump, run make check, and open one PR without Closes lines so Dependabot retires its own PRs. Use when the user asks to batch, consolidate or "do the Dependabot PRs", or when Dependabot PRs are red on the notices, vuln-scan or image-scan checks.
---

# Dependabot batch

The procedure lives in `docs/agents/dependabot-batch.md` — the single source of truth. This
skill exists only to route batching intent to that runbook; it does not restate it, so the two
cannot drift.

Steps:

1. Read `docs/agents/dependabot-batch.md` in full.
2. Execute its sections in order: inventory → branch → apply per ecosystem → regenerate and
   gate → commit and PR → post-merge check.
3. Show the inventory before touching anything, ask before committing, never merge, never
   watch CI, never close a Dependabot PR.
