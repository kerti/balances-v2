---
name: release
description: Cut a batched alpha/rc/production release for balances-v2. Tag-driven SemVer pre-releases per ADR-0029/0030. Use when user says "cut a release", "ship alpha", "tag a release", or wants to publish a new version.
---

# Release

The procedure lives in `docs/agents/release.md` — the single source of truth. This
skill exists only to route release intent ("cut a release", "ship alpha", `/release`)
to that runbook; it does not restate it, so the two cannot drift.

Steps:

1. Read `docs/agents/release.md` in full.
2. Execute its sections in order: pick version → pre-flight → label the batch →
   check migrations → back up DB (migration-bearing cuts only) → cut the tag →
   generate + rewrite release notes → verify the deploy → post-release cleanup.
3. Confirm the version with the user before tagging, and never tag a red `main`.
