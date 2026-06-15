# Balances

[![CI](https://github.com/kerti/balances-v2/actions/workflows/ci.yml/badge.svg)](https://github.com/kerti/balances-v2/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/kerti/balances-v2/graph/badge.svg)](https://codecov.io/gh/kerti/balances-v2)

A snapshot-based personal-finance app for tracking household net worth without itemising every
transaction.

The domain glossary lives in `CONTEXT.md`. The decisions behind the design and tech stack live in
`docs/adr/`. The implementation outline lives in `docs/ROADMAP.md`.

## Local development

Prerequisites: Docker (OrbStack recommended on macOS), Go 1.26.4+, Node 22+.

```sh
cp .env.example .env
make up                       # starts Postgres + Mailpit
make backend-migrate-up       # applies pending migrations
make backend-run              # http://localhost:8080  (terminal 1)
make frontend-install         # first time only
make frontend-dev             # http://localhost:5173  (terminal 2)
```

Mailpit's web UI is at <http://localhost:8025> for inspecting dev emails. The Vite dev server
proxies `/healthz` and `/api/*` to the backend at `:8080`.
