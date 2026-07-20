# Decouple deployment target from release maturity

This ADR revises the **tag-driven deployment routing** decided in [[adr-0030]] (and restated in the
release runbook). Routing is changed from *"the tag's pre-release suffix picks the environment"* to
*"pre-release tags all land on `preview`; `demo` is promoted manually from any tag."* The rest of
[[adr-0030]] — single-origin Fly app, Neon per-env branch, Resend, CF DNS-only, `release_command`
migrations, build-once/promote-many via GHCR ([[adr-0033]]) — stands unchanged.

## Why now

[[adr-0030]] welded two orthogonal things onto one axis, the tag's pre-release suffix:

- **maturity** — a property of the *build*: `alpha` = features still moving, `rc` = feature-frozen
  candidate.
- **deployment target** — *where* the build should run: `preview`, `demo`, `production`.

They correlate (you want the public instance to run stable builds) but they are not the same, and the
`v0.8.0-rc.1` cut exposed the seam: two feature epics ([[adr-0046]] bulk monthly entry, [[adr-0048]]
financial statistics) landed on `main` after the `rc`. To get them in front of the `demo` audience the
old routing offered only two bad moves — mint an `rc.2` that **lies** (a "candidate" carrying two new
epics, violating the feature-freeze the `rc` label promises), or leave `demo` a whole minor behind.
The routing was forcing a dishonest version label. That is the signal the two concepts had diverged.

The deeper question the seam surfaced — **what is `demo` for?** — is answered here: `demo` is the
**public, shared, disposable showcase**; it must run a *deliberately chosen stable build*, never
whatever maturity label happened to route there. `preview` is the **maintainer's private instance**:
signups closed, not public, runs whatever is currently being tested regardless of maturity.

## The decision

### Two environments, named for audience — not for maturity

| Env | Audience | Signups | Deploy trigger |
|---|---|---|---|
| `preview` | maintainer only (private) | closed | **auto** on any pre-release tag |
| `demo` | public, shared, disposable | closed (shared account, [[adr-0041]]) | **manual** — promote any tag |
| `production` | real users | — | **deferred indefinitely** — routing dormant |

A hostname names a *purpose that outlives a build*, so it is named for its audience (`preview`,
`demo`), never for a transient build-quality label (`alpha`, `rc`). Maturity lives in the **tag and
its release notes**, not in DNS.

A standing **third private environment split by maturity** (a separate `rc` instance) was considered
and **rejected**: maturity is a property of the build, not of a standing environment, and an `rc`
typically promotes an `alpha` commit **verbatim** (as `v0.7.0-rc.1`/`rc.2` did), so a second private
env would re-run a byte-identical artifact — standing surface earning its keep only during the brief
windows an `rc` is in flight. One private `preview` running the current test build is sufficient. A
`rc` is still cut when a build is genuinely a frozen candidate; it deploys to `preview` like any other
pre-release, and the `rc`-ness lives in the tag, not a hostname.

### Routing

```
v X.Y.Z -alpha.N  ┐
v X.Y.Z -rc.N     ├─→ preview      (auto, on tag push)
v X.Y.Z -beta.N   ┘
demo              ──→ (manual) promote any existing tag via workflow_dispatch
v X.Y.Z           ──→ production    (dormant; deferred indefinitely — a future ADR when prod is real)
```

- **`preview` — auto.** Any tag with a pre-release suffix (`-alpha.N` / `-rc.N` / `-beta.N`) deploys
  to `preview` on push. Maturity no longer changes the target; it only shapes the release notes.
- **`demo` — manual promote.** A `workflow_dispatch` with a `tag` input deploys **that tag** to
  `demo`. This reuses the build-once artifact already in GHCR ([[adr-0033]]): the image
  `ghcr.io/kerti/balances:<tag>` exists for every tag ever cut, so promotion is a
  `flyctl deploy --app balances-demo --image ghcr.io/kerti/balances:<tag>` — **no rebuild**, the
  byte-identical artifact `preview` already ran. `release_command` still runs `goose up` against
  `demo`'s Neon branch on promote.
- **`production` — dormant.** Bare `vX.Y.Z` tags are not cut while production is deferred
  (production is deferred indefinitely; no real users, only `preview` + `demo` live). The production
  route stays in place but unexercised; the environment, its approval gate, and any rehearsal/staging
  tier are the subject of a future ADR the day production is stood up.

### What is unchanged

Single-origin Fly app per env, Neon branch per env, Resend, Cloudflare DNS-only, host-only session
cookie, `release_command` migrations, GitHub Environments for per-env secrets, and the GHCR
build-once/promote-many pipeline ([[adr-0033]]) all carry over verbatim. Only the `route` job of
`deploy.yml` and the addition of a `workflow_dispatch` promote path change.

## Considered alternatives

- **Keep the suffix→env coupling (status quo).** Rejected — it forces a dishonest version label
  whenever a post-`rc` feature must reach `demo`, and pins `demo`'s freshness to the maturity ladder
  rather than to a deliberate choice.
- **A third standing `rc` environment** (`alpha`/`rc`/`demo`). Rejected — re-introduces the same
  maturity↔environment conflation one hop over; maturity is a build property, `rc` usually promotes
  an `alpha` build verbatim, and the migration-rehearsal job an `rc` env might justify is already
  covered by the nightly upgrade-contract CI (#368). Revisit as a `staging` tier only when production
  is real and its promotion needs live rehearsal.
- **Rebuild-from-tag on demo promote.** Rejected — the artifact already exists in GHCR; rebuilding
  risks a non-reproducible drift from what `preview` validated. Promote the existing digest.
- **Auto-promote the latest `rc` to demo.** Rejected — that is the coupling under a new name; the
  point is that `demo`'s contents are a *deliberate* choice, not an automatic consequence of a label.

## Consequences

- `deploy.yml`'s `route` job changes to map every pre-release suffix to `preview`; a new
  `workflow_dispatch`-triggered path deploys a chosen tag's GHCR image to `demo`. Implementation is a
  grabbable slice, tracked in #489.
- The release runbook (`docs/agents/release.md`) routing table and HANDOFF's deploy description are
  updated to match. `rc` reverts to meaning only "frozen candidate," decoupled from any environment.
- `demo` can now run any minor, ahead of or behind `preview`, entirely at the maintainer's choice —
  which is the intended behaviour for a public, disposable showcase.
- No infra is added: still two live environments (`preview`, `demo`) on the existing wiring. A
  `staging`/rehearsal tier and the production route are a future ADR gated on production becoming real.
