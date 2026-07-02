# Versioning, the upgrade contract, and migration immutability

SemVer is a contract for **machines**, not a human brand. The version string versions one surface —
the **operator's upgrade contract** — because Balances is **self-hostable** (see issue #116), so the
machine that consumes the version is a stranger's `docker compose pull && up -d`. The product's
human identity is just **"Balances"** and is decoupled from the number. First production release is
**`v1.0.0`**; the `0.x` ramp is unstable by SemVer convention. Migration immutability begins at
`1.0.0`, and squashing is permitted for migrations that only ever ran in **resettable** environments.
This builds on [[adr-0029]] (release strategy), [[adr-0030]] (hosting), and [[adr-0031]] (baseline
squash), and resolves the versioning questions those left open for production.

## Amended (2026-07-02)

Production may now launch on a `0.x` minor, not necessarily `v1.0.0`. Prod was deferred indefinitely
(2026-07-02, see HANDOFF) and self-hosting (#116, the one blocker this ADR named below) has since
shipped and closed — nothing structural is left pinning first-production to a specific number. Every
rule below that reads "at `1.0.0`" now reads **"at the first tag that deploys to `production`,
whatever that version turns out to be"**:

- **Migration immutability** begins at the first production deploy, not literally at `1.0.0`.
- **Major-vs-minor discipline** switches on at the same point.
- **The last in-repo squash** happens right before that tag, not specifically before `1.0.0`.
- `1.0.0` is not reserved for anything — it may land on first production, or it may end up meaning
  nothing at all if prod ships at, say, `0.8.0`. No separate "1.0" branding milestone is implied or
  promised.

Everything else below — the operator-upgrade-contract framing, the squash rules once the boundary is
crossed, the repo-boundary logic — is unchanged; only the trigger's *name* moves from a fixed number
to an event.

## Why now

[[adr-0029]] started the line at `v0.6.0-alpha.1` and called `0.x` "unstable schema/API, honestly,"
but deliberately deferred the production rules — what a major *means*, where migrations become
sacred, how the human brand relates to the number. Two facts now force the question. The
distribution model is settled as **self-hostable** (#116), which finally names the consumer of the
version. And the alpha chain is about to keep accruing migrations toward `1.0.0`, so the
immutability line and the squash rules must exist *before* the first production database does — once
real operator data lands, the rules can no longer be chosen freely.

## The decision

### SemVer is for machines; "Balances" is for humans

The version string and the product name answer different questions and must not share a digit.

- **The number is the machine contract.** It tells an operator what an upgrade will cost (below).
- **The human name is "Balances", unnumbered.** Balances v1 (the unfinished
  `github.com/kerti/balances`) **never shipped**, so no user ever held a predecessor — a "2" on the
  logo would signal a continuity that exists for nobody and only invites "where's 1?". The brand
  stays plain "Balances"; no logo numeral.
- **`balances-v2` is repo lineage, not branding.** The repo name is an internal marker; it does not
  leak onto the product face and does not pin the SemVer major.

A human generation number only earns a place on the brand the day a *shipped* predecessor exists in
users' hands — i.e. at the next repo (below), not now.

### The versioned surface is the operator upgrade contract

Because Balances is self-hostable, the consuming machine is the operator's upgrade process. The
version tiers map to **what the operator must do**, on a single axis — how much the upgrade and the
data suffer:

| Release | Meaning | Operator action |
|---|---|---|
| **patch** | bug/hotfix, no migration | drop-in `pull && up -d` |
| **minor** | additive feature, additive migration | drop-in `pull && up -d`; migration runs on boot |
| **major** | breaking but **data survives** — destructive/irreversible migration, a required config change, or a dropped upgrade path | **read the release notes**; expect manual steps |
| **new repo** | prod data **cannot** forward-migrate | fresh install of the next generation |

- **`0.x` makes no compatibility promise.** Through the alpha ramp, breaking changes ride **minor**
  bumps — `v0.6.0-alpha.N` → `v0.7.0-alpha.1` at milestone 7, and so on. Major-vs-minor discipline
  switches on **at `v1.0.0`**, the first production release.
- The number is **not** locked at any major (an earlier idea of pinning to `2.x` to mirror "Balances
  v2" is rejected below). Majors are spent freely whenever the upgrade is breaking-but-survivable.

### Migration immutability begins at `v1.0.0`; squash is scoped to resettable environments

A migration becomes sacred the moment an environment that **cannot be reset** has applied it. Before
that, files are not history anyone depends on.

- **Resettable today:** `dev` (the laptop), `preview`/alpha and `demo`/RC (disposable Neon branches,
  schema "not guaranteed stable" — [[adr-0030]]), and **feature branches**. Only `production` (which
  does not yet exist, arriving at `1.0.0`) is non-resettable.
- **Squash rule (generalises [[adr-0031]]):** *any* migrations that have only ever run in resettable
  environments may be squashed. [[adr-0031]] was the first application (25 pre-alpha files → one
  baseline); it is now a reusable procedure, not a one-off.
- **The last in-repo squash is right before `v1.0.0`:** collapse the alpha chain into a fresh
  `00001_baseline.sql` for the production database to start clean. After `1.0.0` the chain is
  **append-only and forward-only — never squashed — including across major bumps.** A major does not
  buy a squash.
- **Branch-scoped squash for long-lived features.** A feature needing many iterations and migrations
  may develop on a long-lived branch with throwaway migrations, then **squash them into one before
  merging to `main`**. The intermediates only ever ran on the branch's resettable DBs, so this is the
  same safe operation, scoped smaller. It preserves "**one feature → one migration on `main`**" from
  [[adr-0029]]. Discipline carries from [[adr-0031]]: generate the squash from `pg_dump --schema-only`
  of the **net** applied schema (never a hand-merge of the `CREATE`s — transforms make that diverge),
  hand-write a coherent `-- +goose Down`, and **renumber + re-run testcontainers** at merge time if
  `main` advanced.
- **The next repo is the only post-`1.0.0` squash**, by virtue of being a fresh baseline.

### Majors are invisible to the deploy pipeline

Tag→env routing keys off the **prerelease channel suffix**, never the numeric tuple
([[adr-0030]]: `*-alpha.N`→`preview`, `*-rc.N`→`demo`, stable→`production`). A `v2.0.0` is "stable →
production," routed identically to a minor or patch. Major-ness surfaces in exactly three places:

1. the **version string**,
2. the **release notes** — flagging the break and the manual upgrade steps,
3. a **heavier pre-promotion checklist** — a longer RC soak on `demo` and a migration dry-run against
   a clone of production data before promoting.

The checklist is manual release *discipline*, not pipeline logic — keeping `deploy.yml`'s routing a
simple regex on the suffix, with no special-case branch that could wedge a production deploy.

### The repo boundary is the next human generation

A new repo (the human-facing "Balances 3", starting again at `v0.0.0`) is triggered on **data
continuity, not narrative**: *can production data forward-migrate at all?* If yes — however drastic
the schema change — it is a **major bump in this repo**. If no (a true rewrite, or abandoning the old
data model), it is a **new repo**. The narrative usually correlates with the data break, but the
yes/no data question is what makes the boundary decidable. This mirrors how v1
(`github.com/kerti/balances`) → v2 (this repo) already worked.

## Considered alternatives

- **Lock the major at `2.x` to mirror "Balances v2."** Rejected — it conflates a human brand with a
  machine contract and throws away the only channel for signalling breaking upgrades to operators.
  Firefly-III is the precedent: SemVer numbers serve the self-hoster's upgrade process, not marketing.
- **Number the human brand "Balances 2" now (new logo).** Rejected — v1 never shipped, so the numeral
  signals a predecessor no user knew. Brand stays "Balances."
- **Keep squashing migrations in place after `1.0.0`.** Rejected — it rewrites history that ran on a
  non-resettable production DB, exactly the operation [[adr-0031]]'s drift gate exists to forbid once
  a DB can't be rebuilt.
- **Route majors to a dedicated env / run versions in parallel.** Rejected for now — one production,
  the operator cuts over. Revisit only if Balances becomes zero-downtime multi-tenant SaaS needing
  `1.x` and `2.x` side by side during cutover.
- **Per-major branch ceremony (GitFlow `release/*`).** Rejected — [[adr-0029]] already chose GitHub
  Flow; majors need no parallel release train.

## Consequences

- **Self-hosting becomes a `1.0.0`-blocking requirement** (#116): the upgrade contract above is only
  real if there is a maintained `docker compose` stack for operators to upgrade.
- **Majors cost no pipeline change** — only a heavier manual pre-promotion checklist. `deploy.yml`
  stays version-agnostic.
- **[[adr-0031]]'s squash recipe is now a named, reusable procedure** — applied branch-scoped during
  development and once more right before `v1.0.0`, then retired within this repo.
- **Operators get a predictable upgrade contract:** patch/minor are drop-in; a major means "read the
  notes, expect manual steps"; a new repo means a fresh install.
- **[[adr-0029]]'s "milestone = minor convention" continues** through the `0.x` ramp; this ADR adds
  the production-tier meaning that switches on at `1.0.0`.
- HANDOFF/memory notes about staying at `2.x` or treating the human "v2" as the SemVer major are
  retired by this ADR.
