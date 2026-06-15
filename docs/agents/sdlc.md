# The balances-v2 SDLC

How a change travels from idea to released code here. The individual tools
(skills, ADRs, the coverage matrix, the release runbook) already exist and are
authoritative on their own mechanics; this file is the **spine** that orders
them, so a change doesn't skip the step that would have caught its problem.

Not every change runs the whole spine — a typo fix is a branch + PR. The rule of
thumb: **the more a change commits the project to a shape (data model, domain
language, an externally-visible contract), the earlier in the spine it must
start.** A new feature starts at Phase 1; a bug fix usually starts at Phase 4; a
docs tweak is just Phase 6.

## The spine

1. **Capture** — an idea becomes a tracked issue. Use the `triage` skill / issue
   tracker (`docs/agents/issue-tracker.md`, `docs/agents/triage-labels.md`). A
   half-formed idea is fine here; triage is where it gets sharpened or parked.

2. **Sharpen against the domain** — for anything non-trivial, run `grill-with-docs`:
   stress the idea against `CONTEXT.md` and the ADRs until the terminology and the
   decision tree are settled. This is where you discover the feature is really a
   rename of an existing concept, or that it fights a prior decision. Cheap to
   change your mind here; expensive later.

3. **Decide (ADR)** — if the change makes an architectural or hard-to-reverse
   decision (a data-model shape, a new contract, a philosophy call like
   "observed vs projected"), record it as an ADR in `docs/adr/` *before* coding.
   UI/UX decisions follow the ADR-0034 convention. Pre-alpha ADRs are not sacred
   — backtracking on a prior one is allowed, but do it explicitly in a new ADR.
   No ADR for changes that only follow existing decisions.

4. **Slice** — break the work into independently-shippable tracer-bullet slices
   with `to-prd` / `to-issues`. Each slice should be a thin vertical (DB →
   repo → handler → UI) that leaves main releasable.

5. **Build (TDD)** — red-green-refactor with the `tdd` skill. Backend in Go,
   frontend in React/TS. Mind the run loop in `docs/agents/dev.md` (esp.
   `make backend-restart` after Go edits — the dev server runs a built binary).

6. **Guard** — before the PR, the gates that keep a regression from shipping:
   - **Pre-commit:** scrub PII (real names / figures / laptop paths) from the
     staged diff, and run `-coverprofile` on changed packages to cover new
     branches — don't wait for CI.
   - **`make check`** — the local mirror of CI (golangci-lint · eslint · tsc ·
     go test · vitest).
   - **Coverage matrix** — if the change touches a catalogued invariant, add the
     `// covers: INV-...` annotation to the verifying test; if it establishes a
     *new* invariant worth guarding, add a catalog row under
     `docs/qa/invariants/`. Regenerate with `make qa-matrix`. Mechanism:
     `docs/qa/how-it-works.md`.

7. **Ship** — GitHub Flow (ADR-0029): branch → PR → squash-merge. main is
   protected; the human squash-merge is the sign-off (the agent never merges).
   Label the PR with its type at merge time. Migrations stay atomic with their
   feature PR (label `needs-migration` / `migration:additive|destructive`).
   After merge, close the issue.

8. **Release** — batched, tag-driven SemVer pre-releases. Follow
   `docs/agents/release.md`: pick the version, label the batch, check migrations,
   tag, publish the auto-generated notes, verify the deploy.

## Worked example

The QA coverage matrix zones (TENANCY … BONDS) are a partial run of this spine —
each zone was a catalog row + annotations shipped as a labelled PR (Phases 6–7
over an existing codebase). A *greenfield* feature exercising every phase
(Capture → ADR → slice → TDD → matrix) is still pending; the deferred
fixed-rate coupon-projection idea is the earmarked candidate for the first full
pass.
