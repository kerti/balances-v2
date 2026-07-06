.PHONY: help up down logs ps backend-run backend-build backend-test backend-migrate-up backend-migrate-down backend-migrate-status backend-tidy backend-sqlc backend-gen-ts-types backend-gen-ts-types-check frontend-install frontend-dev frontend-build backend-stop backend-restart frontend-stop frontend-restart restart servers-status e2e-db-create e2e-seed e2e-backend e2e-mock-oidc e2e start-task check qa-matrix qa-strict qa-gaps session-token hooks-install setup

# `make` with no target prints help.
.DEFAULT_GOAL := help

-include .env
export

# Dev scaffolding (postgres + mailpit) lives in docker-compose.dev.yml; the
# repo-root docker-compose.yml is the operator self-host stack (ADR-0037).
DEV_COMPOSE := docker-compose.dev.yml

# Pin the compose project name so container names (and thus PG_CONTAINER below)
# are deterministic regardless of the clone directory name — without this,
# Compose derives the project name from the containing folder, so cloning
# under anything other than `balances-v2` silently breaks `docker exec
# $(PG_CONTAINER)` (e2e-db-create, session-token) with a confusing "no such
# container" error (issue #369).
COMPOSE_PROJECT_NAME := balances-v2

# Background dev-server logs. tail -f to follow.
BACKEND_LOG  := /tmp/balances-backend.log
FRONTEND_LOG := /tmp/balances-frontend.log

# Port the backend serves on (matches config default); used by the readiness
# poll in `backend-restart`. Override by setting PORT in .env.
BACKEND_PORT := $(or $(PORT),8080)

# E2E (ADR-0024): a dedicated database in the same Postgres container, plus the
# backend pointed at it. PG_CONTAINER / PG_USER match docker-compose defaults
# (name fixed by COMPOSE_PROJECT_NAME above, not the clone directory).
# E2E_DATABASE_URL is DATABASE_URL with the db name swapped to balances_e2e, so
# it inherits host/port/credentials from .env without duplicating them.
PG_CONTAINER := balances-v2-postgres-1
PG_USER      := balances
PG_DB        := balances
E2E_DB       := balances_e2e
E2E_DATABASE_URL := $(shell echo "$(DATABASE_URL)" | sed 's|/balances?|/$(E2E_DB)?|')

help:
	@echo "balances-v2 — make targets (run 'make <target>')"
	@echo ""
	@echo "Docker (compose stack):"
	@echo "  up                      start the stack (postgres etc.) in the background"
	@echo "  down                    stop the stack"
	@echo "  logs                    follow compose logs"
	@echo "  ps                      show compose service status"
	@echo ""
	@echo "Backend (Go):"
	@echo "  backend-run             run the backend in the foreground"
	@echo "  backend-build           build the backend binary to backend/bin/balances"
	@echo "  backend-test            run all Go tests"
	@echo "  backend-migrate-up      apply pending DB migrations"
	@echo "  backend-migrate-down    roll back the last DB migration"
	@echo "  backend-migrate-status  show migration status"
	@echo "  backend-tidy            go mod tidy"
	@echo "  backend-sqlc            regenerate sqlc code"
	@echo "  backend-gen-ts-types    regenerate frontend/src/api/generated.types.ts"
	@echo "  backend-gen-ts-types-check  CI gate: fail if generated.types.ts is stale"
	@echo ""
	@echo "Frontend (Vite/React):"
	@echo "  frontend-install        npm install"
	@echo "  frontend-dev            run the vite dev server in the foreground"
	@echo "  frontend-build          production build"
	@echo ""
	@echo "Background dev servers (see issue #30):"
	@echo "  backend-restart         restart the background backend (logs: $(BACKEND_LOG))"
	@echo "  backend-stop            stop the background backend"
	@echo "  frontend-restart        restart the background frontend (logs: $(FRONTEND_LOG))"
	@echo "  frontend-stop           stop the background frontend"
	@echo "  restart                 restart both background servers"
	@echo "  servers-status          show which dev servers are running"
	@echo ""
	@echo "E2E (Playwright; ADR-0024):"
	@echo "  e2e                     full run — create+seed db, start mock OIDC, run suite"
	@echo "  e2e-db-create           create the balances_e2e database if missing"
	@echo "  e2e-seed                migrate + reset balances_e2e to the fixture"
	@echo "  e2e-backend             run the backend against balances_e2e (foreground)"
	@echo "  e2e-mock-oidc           run the fake OIDC provider (foreground, :8090)"
	@echo ""
	@echo "Workflow helpers (terse output; see docs/agents/dev.md):"
	@echo "  start-task              pre-flight: clean tree? GitHub access? then sync main"
	@echo "  check                   pre-push gate: lint + tests, pass/fail only (logs in /tmp)"
	@echo "  qa-matrix               regenerate docs/qa/coverage/ from invariant annotations"
	@echo "  qa-strict               CI gate: fail if any invariant lacks per-PR coverage"
	@echo "  qa-gaps                 list within-zone test files that carry no covers: annotation"
	@echo "  session-token           print a live session token for curl smoke tests"
	@echo "  hooks-install           enable the pre-commit pii-guard (run once per clone)"
	@echo "  setup                   first-clone entry point: hooks-install + frontend-install + seed .env"

up:
	docker compose -f $(DEV_COMPOSE) up -d

down:
	docker compose -f $(DEV_COMPOSE) down

logs:
	docker compose -f $(DEV_COMPOSE) logs -f

ps:
	docker compose -f $(DEV_COMPOSE) ps

backend-run:
	( cd backend && go run ./cmd/balances serve )

backend-build:
	( cd backend && go build -o bin/balances ./cmd/balances )

backend-test:
	( cd backend && go test ./... )

backend-migrate-up:
	( cd backend && go run ./cmd/balances migrate up )

backend-migrate-down:
	( cd backend && go run ./cmd/balances migrate down )

backend-migrate-status:
	( cd backend && go run ./cmd/balances migrate status )

backend-tidy:
	( cd backend && go mod tidy )

backend-sqlc:
	( cd backend && sqlc generate )

# Regenerate frontend/src/api/generated.types.ts, the structural (field
# names + nullability) mirror of the sqlc/repo wire-facing Go structs. Run
# after a migration/sqlc regen changes one of those structs — see
# backend/tools/gen-ts-types and frontend/src/api/types.ts's header (issue
# #365).
backend-gen-ts-types:
	( cd backend && go run ./tools/gen-ts-types )

# The CI gate: fails if generated.types.ts is stale relative to the Go
# source, without rewriting it.
backend-gen-ts-types-check:
	( cd backend && go run ./tools/gen-ts-types -check )

# Regenerate docs/api-routes.md, the flat inventory of every backend HTTP
# route. Walked from the live chi router (not the source), so it can't drift.
# Run after adding/removing/renaming a route in any handler's Mount. See
# backend/tools/gen-routes and issue #370.
api-routes:
	( cd backend && go run ./tools/gen-routes )

# The CI gate: fails if docs/api-routes.md is stale relative to the router,
# without rewriting it.
api-routes-check:
	( cd backend && go run ./tools/gen-routes -check )

frontend-install:
	( cd frontend && npm install )

frontend-dev:
	( cd frontend && npm run dev )

frontend-build:
	( cd frontend && npm run build )

# ----- background dev servers ---------------------------------------------
# `make restart` kills any running backend + frontend dev processes and
# starts fresh ones in the background, redirecting output to log files.
# Useful after schema/migration changes (backend re-runs goose on serve)
# or when iterating on env vars. Per-side variants exist for partial
# restarts; `servers-status` shows what's running.
#
# Stops wait for the process to actually exit (SIGTERM → graceful shutdown,
# bounded by SHUTDOWN_TIMEOUT, then SIGKILL escalation) and starts poll for
# real readiness (backend /healthz; vite's "Local:" line) instead of a blind
# `sleep` — both servers come up in well under a second, so fixed sleeps were
# pure dead time. See issue #30.
#
# Starts fail loud (issue #181): the poll loops exit non-zero if the process
# dies (compile error, panic, port already bound) or never signals readiness
# within the timeout, printing the tail of the log to stderr. A failed
# `backend-restart` short-circuits the `restart` chain, so frontend isn't
# restarted on a backend that never came up.

backend-stop:
	@pkill -f 'go run ./cmd/balances' 2>/dev/null || true
	@pkill -x balances 2>/dev/null || true
	@for i in $$(seq 1 50); do \
	  pgrep -x balances >/dev/null 2>&1 || pgrep -f 'go run ./cmd/balances' >/dev/null 2>&1 || break; \
	  sleep 0.1; \
	done
	@pkill -9 -x balances 2>/dev/null || true
	@echo "backend: stopped"

backend-restart: backend-stop
	@( cd backend && exec nohup go run ./cmd/balances serve ) > $(BACKEND_LOG) 2>&1 < /dev/null &
	@seen=0; for i in $$(seq 1 100); do \
	  curl -fsS http://localhost:$(BACKEND_PORT)/healthz >/dev/null 2>&1 && { echo "backend: started (log: $(BACKEND_LOG))"; exit 0; }; \
	  if pgrep -f 'go run ./cmd/balances serve' >/dev/null 2>&1 || pgrep -x balances >/dev/null 2>&1; then seen=1; elif [ $$seen = 1 ]; then break; fi; \
	  sleep 0.1; \
	done; \
	echo "✗ backend failed to start (died or timed out) — tail of $(BACKEND_LOG):" >&2; \
	tail -n 20 $(BACKEND_LOG) >&2; \
	exit 1

frontend-stop:
	@pkill -f 'frontend/node_modules/.bin/vite' 2>/dev/null || true
	@pkill -f 'npm run dev' 2>/dev/null || true
	@for i in $$(seq 1 50); do \
	  pgrep -f 'frontend/node_modules/.bin/vite' >/dev/null 2>&1 || break; \
	  sleep 0.1; \
	done
	@echo "frontend: stopped"

frontend-restart: frontend-stop
	@: > $(FRONTEND_LOG)
	@( cd frontend && exec nohup npm run dev ) > $(FRONTEND_LOG) 2>&1 < /dev/null &
	@seen=0; for i in $$(seq 1 150); do \
	  grep -q 'Local:' $(FRONTEND_LOG) 2>/dev/null && { echo "frontend: started (log: $(FRONTEND_LOG))"; exit 0; }; \
	  if pgrep -f 'frontend/node_modules/.bin/vite' >/dev/null 2>&1 || pgrep -f 'npm run dev' >/dev/null 2>&1; then seen=1; elif [ $$seen = 1 ]; then break; fi; \
	  sleep 0.1; \
	done; \
	echo "✗ frontend failed to start (died or timed out) — tail of $(FRONTEND_LOG):" >&2; \
	tail -n 20 $(FRONTEND_LOG) >&2; \
	exit 1

# `restart` chains both; make's own error propagation stops the chain (and skips
# the final echo) the moment either side exits non-zero.
restart: backend-restart frontend-restart
	@echo "both servers restarted"

# ----- e2e (Playwright; ADR-0024) -----------------------------------------
# e2e            : full run — create DB, seed it, start the mock OIDC provider,
#                  then Playwright launches its own backend (:8099) + vite
#                  (:5273) and runs the suite
# e2e-db-create  : create balances_e2e in the running container if missing
# e2e-seed       : migrate + reset balances_e2e to the Playwright fixture, print SESSION_ID
# e2e-backend    : run the backend against balances_e2e (foreground)
# e2e-mock-oidc  : run the fake OIDC provider in the foreground (:8090) for debugging
#
# The seed runs synchronously before Playwright so balances_e2e is fully
# migrated by the time Playwright's backend boots (auto-migrate becomes a
# no-op, no race). The mock OIDC provider (ADR-0024 option B) must be up BEFORE
# the backend boots, because auth.New does OIDC discovery at startup — so `e2e`
# starts it, waits for its discovery endpoint, then hands off to Playwright and
# kills it on exit. Playwright owns the e2e backend/vite lifecycle on dedicated
# ports, so the 8080/5173 dev servers are never touched.

e2e: e2e-db-create e2e-seed
	@( cd backend && go build -o /tmp/balances-e2e ./cmd/balances )
	@/tmp/balances-e2e mock-oidc & \
	  MOCK_PID=$$!; \
	  trap "kill $$MOCK_PID 2>/dev/null" EXIT; \
	  for i in $$(seq 1 50); do \
	    curl -sf http://localhost:8090/.well-known/openid-configuration >/dev/null && break; \
	    sleep 0.2; \
	  done; \
	  ( cd frontend && E2E_DATABASE_URL="$(E2E_DATABASE_URL)" npm run test:e2e -- $(E2E_ARGS) )

# CI variant of `e2e` (issue #70). Differs only in the DB-create step: CI runs a
# GitHub `services: postgres` reachable on localhost — there is no docker
# container to `docker exec` into — and the balances_e2e DB is created by the
# service's POSTGRES_DB, so e2e-db-create is skipped. E2E_ARGS forwards Playwright
# flags, e.g. `make e2e-ci E2E_ARGS='--grep @smoke'` for the per-PR smoke gate.
e2e-ci: e2e-seed-ci
	@( cd backend && go build -o /tmp/balances-e2e ./cmd/balances )
	@/tmp/balances-e2e mock-oidc & \
	  MOCK_PID=$$!; \
	  trap "kill $$MOCK_PID 2>/dev/null" EXIT; \
	  for i in $$(seq 1 50); do \
	    curl -sf http://localhost:8090/.well-known/openid-configuration >/dev/null && break; \
	    sleep 0.2; \
	  done; \
	  ( cd frontend && E2E_DATABASE_URL="$(E2E_DATABASE_URL)" npm run test:e2e -- $(E2E_ARGS) )

e2e-seed-ci:
	@( cd backend && DATABASE_URL="$(E2E_DATABASE_URL)" go run ./cmd/balances seed-e2e )

e2e-mock-oidc:
	@( cd backend && go run ./cmd/balances mock-oidc )

e2e-db-create:
	@docker exec $(PG_CONTAINER) psql -U $(PG_USER) -d postgres -tAc \
	  "SELECT 1 FROM pg_database WHERE datname='$(E2E_DB)'" | grep -q 1 \
	  || docker exec $(PG_CONTAINER) createdb -U $(PG_USER) $(E2E_DB)
	@echo "e2e db: $(E2E_DB) ready"

e2e-seed: e2e-db-create
	@( cd backend && DATABASE_URL="$(E2E_DATABASE_URL)" go run ./cmd/balances seed-e2e )

e2e-backend: e2e-db-create
	@( cd backend && DATABASE_URL="$(E2E_DATABASE_URL)" go run ./cmd/balances serve )

servers-status:
	@if pgrep -f 'cmd/balances serve' >/dev/null; then \
	  echo "backend:  running (pid $$(pgrep -f 'cmd/balances serve' | head -1))"; \
	else \
	  echo "backend:  stopped"; \
	fi
	@if pgrep -f 'frontend/node_modules/.bin/vite' >/dev/null; then \
	  echo "frontend: running (pid $$(pgrep -f 'frontend/node_modules/.bin/vite' | head -1))"; \
	else \
	  echo "frontend: stopped"; \
	fi

# ---- Workflow helpers ----------------------------------------------------
# Each emits a single status line per step so they're cheap to read in an
# agent's context; verbose output goes to a /tmp log read only on failure.

# Pre-task pre-flight: refuse on a dirty tree, verify GitHub access, then
# fast-forward main. Run before starting any new piece of work so you never
# branch off a stale local main.
start-task:
	@test -z "$$(git status --porcelain)" || { echo "✗ working tree dirty — commit or stash first, then re-run"; exit 1; }
	@git ls-remote origin HEAD >/dev/null 2>&1 || { echo "✗ no GitHub access — unlock the SSH key (ssh-add) or run 'gh auth login'"; exit 1; }
	@git checkout main >/dev/null 2>&1 || { echo "✗ could not switch to main"; exit 1; }
	@git pull --ff-only >/dev/null 2>&1 || { echo "✗ pull failed (diverged or no upstream) — resolve manually"; exit 1; }
	@echo "✓ on main, up to date @ $$(git rev-parse --short HEAD)"

# Pre-push gate: backend + frontend lint and tests. Each step's full output is
# redirected to /tmp/balances-check-*.log; stdout gets only a pass/fail line.
# e2e is excluded — run `make e2e` separately (it's slow and verbose).
check:
	@fail=0; \
	printf '%-14s' 'golangci-lint'; (cd backend && golangci-lint run) >/tmp/balances-check-be-lint.log 2>&1 && echo '✓' || { echo '✗ → /tmp/balances-check-be-lint.log'; fail=1; }; \
	printf '%-14s' 'eslint';        (cd frontend && npm run -s lint)   >/tmp/balances-check-fe-lint.log 2>&1 && echo '✓' || { echo '✗ → /tmp/balances-check-fe-lint.log'; fail=1; }; \
	printf '%-14s' 'prettier';      (cd frontend && npm run -s format:check) >/tmp/balances-check-fe-fmt.log 2>&1 && echo '✓' || { echo '✗ → /tmp/balances-check-fe-fmt.log'; fail=1; }; \
	printf '%-14s' 'tsc';           (cd frontend && npx tsc -b)        >/tmp/balances-check-fe-tsc.log  2>&1 && echo '✓' || { echo '✗ → /tmp/balances-check-fe-tsc.log';  fail=1; }; \
	printf '%-14s' 'go test';       (cd backend && go test ./...)      >/tmp/balances-check-be-test.log 2>&1 && echo '✓' || { echo '✗ → /tmp/balances-check-be-test.log'; fail=1; }; \
	printf '%-14s' 'vitest';        (cd frontend && npm run -s test)   >/tmp/balances-check-fe-test.log 2>&1 && echo '✓' || { echo '✗ → /tmp/balances-check-fe-test.log'; fail=1; }; \
	printf '%-14s' 'qa-matrix';     (cd backend && go run ./tools/qa-matrix -report -strict) >/tmp/balances-check-qa.log 2>&1; qa=$$?; \
	  sed -n '1s/^qa-matrix: /  /p' /tmp/balances-check-qa.log; \
	  [ $$qa -eq 0 ] || { echo '              ✗ → /tmp/balances-check-qa.log'; fail=1; }; \
	printf '%-14s' 'gen-ts-types'; (cd backend && go run ./tools/gen-ts-types -check) >/tmp/balances-check-ts-types.log 2>&1 && echo '✓' || { echo '✗ → /tmp/balances-check-ts-types.log'; fail=1; }; \
	printf '%-14s' 'api-routes';   (cd backend && go run ./tools/gen-routes -check)   >/tmp/balances-check-api-routes.log 2>&1 && echo '✓' || { echo '✗ → /tmp/balances-check-api-routes.log'; fail=1; }; \
	if [ $$fail -eq 0 ]; then echo 'all green'; else echo 'FAILED — read the ✗ log(s) above'; exit 1; fi

# Regenerate docs/qa/coverage/ from the `// covers: INV-...` annotations in the
# test suite, joined against the docs/qa/invariants/ catalog. Advisory: prints
# uncovered / nightly-only invariants but does not fail. See docs/qa/README.md.
qa-matrix:
	@( cd backend && go run ./tools/qa-matrix )

# The CI gate (also run inside `make check`). Fails if any catalogued invariant
# lacks per-PR coverage — uncovered, or covered only by a nightly (non-smoke)
# Playwright spec. -report so it doesn't rewrite the coverage files. See
# docs/qa/how-it-works.md.
qa-strict:
	@( cd backend && go run ./tools/qa-matrix -report -strict )

# Advisory gap-finder: test files with no covers: annotation that sit in a
# directory where another test does carry one — the likeliest within-zone
# stragglers. Excludes wholly-unannotated dirs (uncatalogued zones, expected
# blank). Does not rewrite the coverage files. See docs/qa/README.md.
qa-gaps:
	@( cd backend && go run ./tools/qa-matrix -gaps )

# Print one live session token (newest, unexpired) for curl smoke tests against
# authenticated endpoints. Empty result → non-zero exit with a hint on stderr.
session-token:
	@tok=$$(docker exec $(PG_CONTAINER) psql -U $(PG_USER) -d $(PG_DB) -tAc \
	  "SELECT s.id FROM sessions s WHERE s.expires_at > now() ORDER BY s.expires_at DESC LIMIT 1" 2>/dev/null); \
	if [ -z "$$tok" ]; then echo "✗ no live session — log in via the dev UI first" >&2; exit 1; fi; \
	echo "$$tok"

# Install the repo git hooks (core.hooksPath=.githooks) and seed the local,
# gitignored .pii-patterns denylist from the template + your git identity, so
# the pre-commit pii-guard protects commits out of the box. Idempotent.
hooks-install:
	@git config core.hooksPath .githooks
	@chmod +x .githooks/pre-commit
	@if [ ! -f .pii-patterns ]; then \
	  cp .pii-patterns.example .pii-patterns; \
	  { git config user.name; git config user.email; } \
	    | sed 's/[][\\.^$$*+?(){}|]/\\&/g' >> .pii-patterns; \
	  echo "hooks-install: seeded .pii-patterns (gitignored) from template + git identity"; \
	fi
	@echo "✓ git hooks installed (core.hooksPath=.githooks); pre-commit pii-guard active"

# First-clone entry point (issue #369): bundles the two separate, missable
# install steps (git hooks + frontend deps) and seeds .env, so a fresh clone
# is one command away from `make up`. Idempotent — safe to re-run.
setup: hooks-install frontend-install
	@if [ ! -f .env ]; then \
	  cp .env.dev.example .env; \
	  echo "setup: created .env from .env.dev.example"; \
	fi
	@echo "✓ setup complete — next: make up && make backend-migrate-up && make backend-run"
