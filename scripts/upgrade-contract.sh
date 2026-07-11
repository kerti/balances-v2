#!/usr/bin/env bash
#
# upgrade-contract.sh — automates the manual upgrade rehearsal #229 did once
# (bundle #368 / CF-27): prove the CURRENT tree's migrations apply cleanly on
# top of the PREVIOUS release's schema, and that the app boots on the result.
#
# The sequence, against one throwaway database:
#   1. build the previous release tag's binary, `migrate up`  -> N-1 schema
#   2. build the current tree's binary,          `migrate up`  -> applies the
#      new-migration delta on top of the real N-1 schema (the regression this
#      catches: a migration that is invalid against what actually shipped)
#   3. boot the current binary's `serve` and assert /healthz  -> N boots on the
#      upgraded schema (DB reachable, migrations settled)
#
# Booting the previous binary is unnecessary to establish N-1: `migrate up` on
# its embedded migration set is the schema N-1 deployed. This is the migration
# upgrade contract, not a data-migration test.
#
# Contract with the caller (Makefile target / CI workflow):
#   - DATABASE_URL must point at a FRESH, EMPTY database this script may write.
#     The caller owns that DB's create/drop lifecycle; the script never touches
#     the developer's dev DB.
#   - PREV_REF (optional) overrides the baseline ref; defaults to the most
#     recent tag reachable from HEAD.
#   - HEALTHZ_PORT (optional) is the port the boot check listens on (default
#     8098, off the 8080 dev port).
set -euo pipefail

: "${DATABASE_URL:?set DATABASE_URL to a fresh, empty database}"
PREV_REF="${PREV_REF:-$(git describe --tags --abbrev=0)}"
HEALTHZ_PORT="${HEALTHZ_PORT:-8098}"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORKTREE="$(mktemp -d)/prev"
PREV_BIN="$(mktemp -d)/balances-prev"
HEAD_BIN="$(mktemp -d)/balances-head"
SERVER_PID=""

log() { printf '\n\033[1m▶ %s\033[0m\n' "$*"; }

cleanup() {
  local code=$?
  [ -n "$SERVER_PID" ] && kill "$SERVER_PID" 2>/dev/null || true
  git -C "$REPO_ROOT" worktree remove --force "$WORKTREE" 2>/dev/null || true
  return $code
}
trap cleanup EXIT

log "baseline ref: $PREV_REF   (HEAD $(git -C "$REPO_ROOT" rev-parse --short HEAD))"

log "build previous ($PREV_REF) binary"
git -C "$REPO_ROOT" worktree add --detach --force "$WORKTREE" "$PREV_REF" >/dev/null
( cd "$WORKTREE/backend" && go build -o "$PREV_BIN" ./cmd/balances )

log "build current (HEAD) binary"
( cd "$REPO_ROOT/backend" && go build -o "$HEAD_BIN" ./cmd/balances )

# migrate needs a valid config too (config.Load): DATABASE_URL + one auth
# provider. Local auth avoids any outbound OIDC discovery; mail stays off.
export AUTH_LOCAL_ENABLED=true
export AUTH_GOOGLE_ENABLED=false
export EMAIL_ENABLED=false

goose_version() { "$1" migrate version 2>&1 | grep -oE 'version:? [0-9]+' | grep -oE '[0-9]+' | tail -1; }

log "step 1 — previous binary migrate up  (establish N-1 schema)"
"$PREV_BIN" migrate up
echo "  N-1 goose version: $(goose_version "$PREV_BIN")"

log "step 2 — current binary migrate up   (apply new-migration delta)"
"$HEAD_BIN" migrate up
echo "  N   goose version: $(goose_version "$HEAD_BIN")"
"$HEAD_BIN" migrate status | tail -20

log "step 3 — current binary serve boots on the upgraded schema"
PORT="$HEALTHZ_PORT" "$HEAD_BIN" serve &
SERVER_PID=$!
ok=""
for _ in $(seq 1 50); do
  if curl -sf "http://localhost:${HEALTHZ_PORT}/healthz" >/dev/null; then ok=1; break; fi
  sleep 0.2
done
if [ -z "$ok" ]; then
  echo "✗ /healthz never came up on port ${HEALTHZ_PORT} after upgrade" >&2
  exit 1
fi
body="$(curl -s "http://localhost:${HEALTHZ_PORT}/healthz")"
echo "  /healthz: $body"

log "✓ upgrade contract holds: $PREV_REF → HEAD migrates cleanly and boots"
