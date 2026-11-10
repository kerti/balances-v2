# Handoff — pick this up cold

You are an agent resuming work on **balances-v2**. This document is a **pointer to live state**:
where the cursor is right now, and the standing decisions that have no other home. Deliberately short.

**What does _not_ live here** (ADR-0029, amended 2026-08-04):

| Looking for | Go to |
|---|---|
| What shipped in a release | the **GitHub Release** notes for that tag |
| Why a change was made / its detail | the **closed issue + PR** |
| A design decision with alternatives | `docs/adr/` (index: `docs/adr/README.md`) |
| Domain language | `CONTEXT.md` |
| Codebase conventions ("touch X, also do Y") | `docs/agents/conventions.md` |
| Milestone goals + close criteria | `docs/ROADMAP.md` |
| How to run / lint / test | `docs/agents/dev.md` |
| Backlog | GitHub labels [`backlog`](https://github.com/kerti/balances-v2/labels/backlog) / [`security`](https://github.com/kerti/balances-v2/labels/security) |

The pre-alpha journal is frozen at `docs/history/CHANGELOG-pre-alpha.md`.

Read these first, in order:
1. `CLAUDE.md` (project instructions; indexes `docs/agents/*`)
2. `docs/ROADMAP.md` (eight milestones)
3. `CONTEXT.md` (domain language)
4. This document
5. `docs/adr/README.md` (one-line ADR index — open the ones touching your task)
6. `docs/agents/conventions.md` (before editing code in an area it covers)
7. `git log --oneline -20` + the closed issues it references (most recent direction)

## Where we are now

**M1–M7 closed; M8 (next domain features) is the active milestone**, opened 2026-07-06. CI is green.

- **Latest release:** `v0.9.0-alpha.9` — on `preview`; promote to `demo` by hand when wanted
  (ADR-0049; promotion is a deliberate act, not a follow-on).
- **Unreleased on `main`:** `git log v0.9.0-alpha.9..main` — the merged PRs and their closed issues
  are the record. Highlights carry into the next tag's release notes, not into this file.
- **Deployment shape:** single-origin — one Fly app per environment (region `sin`) serves the SPA +
  `/api`; Neon Postgres (per-env branch), Resend mail, Google + optional local OAuth. Custom domain
  on Cloudflare DNS-only with Fly-managed TLS. Pre-release tags auto-deploy to `preview`; `demo` is
  promoted manually by `workflow_dispatch` (build-once/promote-many). The bare `vX.Y.Z` → production
  route stays dormant. (ADR-0029/0030/0031, routing revised by ADR-0049; runbook in
  `docs/agents/release.md`.)

Per-milestone history — what each tag contained — is in the GitHub Releases, one entry per tag.

## What's next

**No named next slice.** The mobile line (epics #428/#503, ADR-0050/0051) closed at
`v0.9.0-alpha.3`; the lifecycle/integrity line (#576, ADR-0052) and its sibling the Tracking Changes
epic (#591, ADR-0053) shipped in `v0.9.0-alpha.4`. Ask the user for direction rather than picking the
next item off the backlog — they pause between items to direct.

**Active line: building the 2026-11-04 UI/UX audit of the demo.** Decided in themed grill rounds; each round
files its own `ready-for-agent` issues. Quick wins #779–#784; round (a), money entry and the monthly
flow (ADR-0056, ADR-0046 amendment), is #785–#792. Round (c), feedback and safety (ADR-0058: replaces
archive, single-item Undo toasts, confirm tiers, one toast per write), is #820–#825; it unblocks
#788 (after #820 + #822), #791 (after #820) and #805 (after #822); #790 still waits on #788. Round
(b), close/import/trades (no ADR), is #827–#830. Round (d), wording (no ADR; vocabulary in
`CONTEXT.md`), is #832 copy sweep + #833 counts/months; it renamed ADR-0058's "Record month" button
to **Record snapshot**. Round (e), colour (ADR-0054 amendment), is #835 tokens + #836 palettes and
badges. Round (f), layout and shell (ADR-0050 amendment), is #837–#842. Round (g), scope calls (ADR-0022
amendment), is #843–#847, with member removal deferred to an ADR issue (#848). Phone width is
#849 fixes + #850 real-device check (`ready-for-human`, after #849 and #841). The final pass mapped
every finding ID to an outcome (#851–#853 closed the gaps; table in a comment on #834). **Decisions
are complete; #779–#853 is the build backlog.** Build in dependency order: #820 and #822 first (they
unblock #788, #791, #805; #788 then unblocks #790 and #843). `social-card.png` (ADR-0054, released in alpha.5) still needs a manual upload
under repo **Settings → Social preview**.

Standing carry-forwards:

- **Production Resend domain** — the one M7 bullet that didn't literally close. Moves with prod's
  eventual standup; tracked via #218 and #299's remaining GDPR scope, not its own milestone.
- Smaller open items ride a convenient batch, not their own cut. Hardening follow-ups:
  `actions/checkout` Node-20 bump, HSTS header, `cloudflared` dev-tunnel.

## Standing decisions

Live calls that aren't an ADR and aren't captured by an open issue. Everything else has moved to the
issue, PR, or ADR that owns it.

- **Production is deferred indefinitely** (2026-07-02). Demo is the active public-facing line. First
  prod is **not** pinned to `v1.0.0` (ADR-0033 amended) — it lands on whatever `0.x` minor is current
  when prod unparks; migration immutability and major-vs-minor discipline switch on at *first
  production deploy*, not at a number. Milestone-close still rolls to the next minor's alpha unless it
  happens to coincide with dropping the suffix for a real production cut.
- **Production SaaS data protection** (2026-07-02): ordinary GDPR compliance is sufficient — #222
  (zero-knowledge encryption) closed as disproportionate; it conflicts with core server-side
  aggregation. Rescoped into #299 (privacy policy, open) and #300 (household erasure, shipped,
  ADR-0040). Access/portability is already satisfied by the backup/export epic (#52). Self-host (#116)
  is the zero-exposure option for anyone unwilling to accept hosted SaaS.
- **Subdomain scheme** (#215, decided): nested product subtree — `app.balances.<domain>` for prod
  (unmarked), `balances.<domain>` for the landing, `preview.`/`demo.` as siblings. **DNS-only, never
  proxied.** Preview and demo are both migrated.
- **Release labels:** every PR carries exactly one type label at merge —
  `enhancement` / `bug` / `documentation` / `dependencies`. Test-only and CI/dev/build tooling PRs go
  under **`enhancement`** (decided 2026-06-17 — no dedicated `chore`/`test` label).

## Updating this document

Keep it a **pointer**, not a journal. It should stay roughly this length; if it's growing, the growth
almost certainly belongs in an issue, a PR description, an ADR, or the release notes.

- **Don't** add a bullet for something you shipped — the closed issue and the release notes hold it.
  A release is no longer a pruning checkpoint for this file, because there is nothing to prune.
- **Do** update it when the *cursor* moves: a milestone opens or closes, a release is cut, the active
  line changes, or a standing decision is made or reversed.
- Fold the update into the same commit as the change that caused it. Hard-wrap prose at ~100 columns.
