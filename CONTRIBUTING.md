# Contributing to Balances

Thanks for your interest. This is a small, opinionated project; a little orientation saves a lot of
back-and-forth.

## Start here

- **What it is / how the pieces fit:** [`docs/architecture.md`](docs/architecture.md) (one page).
- **The domain language:** [`CONTEXT.md`](CONTEXT.md) — use these exact words in code and copy.
- **Why things are the way they are:** [`docs/adr/`](docs/adr/) — decisions, each numbered.
- **Where the project is right now:** [`HANDOFF.md`](HANDOFF.md) and [`docs/ROADMAP.md`](docs/ROADMAP.md).

## Local setup

Prerequisites: Docker (OrbStack on macOS), Go 1.26.4+, Node 22+ (`.nvmrc` pins it).

```sh
make setup                 # first clone: git hooks + frontend deps + seed .env
make up                    # Postgres + Mailpit
make backend-migrate-up
make backend-run           # :8080   (terminal 1)
make frontend-dev          # :5173   (terminal 2)
```

`make help` lists every target. After editing Go, `make backend-restart` — the dev server runs a
built binary, so there is no hot reload.

## How a change flows

The full idea→release spine is [`docs/agents/sdlc.md`](docs/agents/sdlc.md). The short version:

1. **An issue first.** Work is tracked as GitHub issues on `kerti/balances-v2`. For anything beyond a
   typo, open or claim one so the shape can be discussed before code.
2. **Decisions get an ADR.** If a change makes an architectural or hard-to-reverse call (a data-model
   shape, a new contract), record it in `docs/adr/` before coding. Follow existing decisions without
   a new ADR.
3. **Branch → PR → squash-merge** (GitHub Flow, ADR-0029). `main` is protected; the human
   squash-merge is the sign-off. Migrations stay in the same PR as their feature (label
   `needs-migration` / `migration:additive|destructive`).
4. **Label the PR** with a type (`enhancement` / `bug` / `documentation` / `dependencies`) at merge
   time — unlabeled PRs fall through the auto-generated release notes.

## Before you open a PR

- **`make check`** — the local mirror of CI: golangci-lint · eslint · prettier · tsc · go test ·
  vitest · qa-strict · generated-artifact freshness. Green here ≈ green in CI.
- **Tests.** Backend changes → Go tests; frontend → vitest (+ Playwright for e2e-affecting work).
  Write the test with the change, not after (ADR-0021; the `tdd` flow in the SDLC doc).
- **Coverage matrix.** If your change touches a catalogued invariant, add the `// covers: INV-...`
  annotation to the verifying test; if it establishes a new one worth guarding, add a catalog row
  under `docs/qa/invariants/`. `make qa-matrix` regenerates coverage; `make qa-strict` is the gate.
- **Regenerate what's generated.** Changed a route's `Mount`? `make api-routes`. Changed a
  wire-facing Go struct? `make backend-gen-ts-types`. Both are CI-gated, so commit the result.
- **Scrub the diff.** No real names, real financial figures, or laptop-specific absolute paths — this
  is a public repo. Use neutral fixtures and toy numbers.

## Reporting bugs / requesting features

Open an issue on `kerti/balances-v2`. New issues start as `needs-triage`; see
[`docs/agents/triage-labels.md`](docs/agents/triage-labels.md) for the label vocabulary. Security
issues follow [`SECURITY.md`](SECURITY.md) — do not open a public issue for a vulnerability.

## Licence

By contributing you agree your contributions are licensed under [AGPL-3.0](LICENSE) (ADR-0042).
