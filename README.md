<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/brand/svg/wordmark-dark.svg">
  <img src="docs/brand/svg/wordmark-light.svg" alt="Balances" width="284" height="88">
</picture>

**Track your household's net worth without itemising a single transaction.**

Each month you enter your balances; Balances tracks your net worth over time.

[Live demo](https://balances-demo.fly.dev) · [Self-hosting](SELF-HOSTING.md) · [The domain model](CONTEXT.md)

[![CI](https://github.com/kerti/balances-v2/actions/workflows/ci.yml/badge.svg)](https://github.com/kerti/balances-v2/actions/workflows/ci.yml)
[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)

</div>

---

## What it is

Most personal-finance apps make you record every coffee. Balances doesn't. It's built on a single
ritual: **at the end of each month, you read the balances off your statements and type them in.** From
those snapshots it computes your household net worth over time — no bank sync, no transaction feed, no
categorising.

That one design choice is deliberate ([ADR-0001](docs/adr/0001-snapshot-based-net-worth-tracking.md)). Itemised
cash-flow tracking (Mint / YNAB style) is a *non-feature*. If you've bounced off budgeting apps because
the daily upkeep never stuck, this is the opposite bargain: ten minutes a month, and you still get the
one number that matters.

## Why it's different

- **Household-first, not person-first.** Every position, snapshot, and income event belongs to a
  *Household* — the people sharing economic life. Multiple members, one shared picture of net worth,
  with per-member and *Joint* breakdowns. No app in this niche is built household-first.
- **Snapshot-first, not sync-first.** You own the numbers. Nothing is scraped from your bank; nothing
  leaves your control. The month-end reading is the source of truth.
- **Real coverage for the instruments people actually hold** — bank accounts, property, and vehicles;
  stocks, mutual funds, and gold *by the gram*; bonds and time deposits with coupons and maturity. It
  handles Indonesian retail instruments most apps ignore outright — ORI/SBR/SR/ST government bonds and
  *deposito* — alongside everything else.
- **Multi-currency when you need it, invisible when you don't.** Hold a foreign account and the currency
  pickers and FX entry appear; otherwise the whole surface stays pinned to your reporting currency.
- **The residual-expense insight, for free.** Because net worth, earned income, and investment returns
  are all tracked, the app derives what you *spent* last month as a single residual number — without you
  logging a single expense.

## What you get

- Net worth over time, per-Household and broken down per member and by household-defined **Tags**
  (by goal, by bank, by risk — you decide).
- An income statement and a cash-spending proxy, both derived from the same snapshots.
- A transaction ledger *only where it earns its keep* — investment instruments — for cost basis and
  income reporting. Everything else is just the monthly balance.
- Spreadsheet import for backfilling history, and a full-fidelity **backup/restore** that moves an
  entire Household between instances (SaaS ↔ self-host, either direction).
- English and Indonesian throughout.

## Try it

- **Hosted demo** — [https://balances-demo.fly.dev](https://balances-demo.fly.dev). A shared, resets-nightly instance;
  poke at it without signing up for anything.
- **Self-host it** — one `docker-compose.yml`, a published image, Postgres, done. See
  [Self-hosting](#self-hosting) below.

## Self-hosting

The repo-root `docker-compose.yml` is the operator stack ([ADR-0037](docs/adr/0037-self-hostable-docker-compose-stack.md)):
it pulls the published image `ghcr.io/kerti/balances:<tag>`, runs Postgres, applies migrations once, and
serves the app on a single origin — no build step.

```sh
cp .env.example .env          # edit: pin BALANCES_TAG, set APP_URL + Google OAuth client
docker compose up             # migrations apply once, then login at http://localhost:8080
```

Upgrade by bumping `BALANCES_TAG` and running `docker compose pull && docker compose up -d`. The full
operator walkthrough — three TLS topologies, the Google OAuth client, the upgrade contract, and database
backups — is in [`SELF-HOSTING.md`](SELF-HOSTING.md).

Licensed under [AGPL-3.0](LICENSE) ([ADR-0042](docs/adr/0042-project-license-agpl-3-0.md)).

## Local development

Prerequisites: Docker (OrbStack recommended on macOS), Go 1.26.4+, Node 22+ (`.nvmrc` pins the version).

```sh
make setup                    # first clone only: git hooks + frontend deps + seed .env
make up                       # starts Postgres + Mailpit (docker-compose.dev.yml)
make backend-migrate-up       # applies pending migrations
make backend-run              # http://localhost:8080  (terminal 1)
make frontend-dev             # http://localhost:5173  (terminal 2)
```

Mailpit's web UI is at <http://localhost:8025> for inspecting dev emails. The Vite dev server proxies
`/healthz` and `/api/*` to the backend at `:8080`. `make help` lists every target.

**Going deeper?** [`docs/architecture.md`](docs/architecture.md) is how the pieces fit;
[`CONTRIBUTING.md`](CONTRIBUTING.md) is the workflow; [`CONTEXT.md`](CONTEXT.md) is the domain glossary;
the decisions behind the design live in [`docs/adr/`](docs/adr/); and `HANDOFF.md` tracks current
project state (written for AI agents, but the fastest read on where things stand).

> **Coverage note:** the frontend codecov number is scoped to `src/lib/**` and
> `src/components/positionList/**` (`frontend/vitest.config.ts`), not the whole frontend — most
> components and hooks aren't in the denominator yet.
