#!/usr/bin/env bash
# End-to-end acceptance test for Phase 2 (SPEC.md): postgres recipe + cleanup.
#
# Proves, with fixtures generated from scratch:
#   1. a pg_dump in the backup restores and loads into a throwaway postgres
#      container, and row assertions pass                          (exit 0)
#   2. a deliberately truncated dump fails with a clear message    (exit 1)
#   3. every container and volume is removed after every run,
#      including the failure path (asserted via docker ps -a / volume ls)
#
# Requirements: go, restic, docker.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

log()  { printf '\n==> %s\n' "$*"; }
fail() { printf 'E2E FAIL: %s\n' "$*" >&2; exit 1; }

expect_exit() {
  local want=$1; shift
  local got=0
  "$@" || got=$?
  [ "$got" -eq "$want" ] || fail "expected exit $want, got $got: $*"
}

# Docker state must be identical before and after every agent run.
docker_state() {
  { docker ps -aq | sort; echo "--"; docker volume ls -q | sort; }
}

assert_docker_clean() {
  local after
  after="$(docker_state)"
  [ "$after" = "$DOCKER_BEFORE" ] || fail "docker state changed after $1:
--- before ---
$DOCKER_BEFORE
--- after ---
$after"
}

assert_sandbox_empty() {
  [ -z "$(ls -A "$SANDBOX")" ] || fail "sandbox base dir not empty: $(ls -A "$SANDBOX")"
}

log "building agent"
BIN="$WORK/restorable"
(cd "$ROOT" && go build -o "$BIN" .)

log "creating fixture with a pg_dump file"
SRC="$WORK/src"
mkdir -p "$SRC/db"
echo "app data" > "$SRC/app.txt"
{
  printf 'CREATE TABLE users (id integer primary key, name text not null);\n'
  printf 'COPY users (id, name) FROM stdin;\n'
  printf '1\talice\n2\tbob\n3\tcarol\n'
  printf '\\.\n'
} > "$SRC/db/dump.sql"

export RESTIC_PASSWORD="e2e-password"
GOOD="$WORK/good-repo"
restic -r "$GOOD" init -q
restic -r "$GOOD" backup -q "$SRC"

SANDBOX="$WORK/sandboxes"
mkdir -p "$SANDBOX"

cat > "$WORK/recipe.yaml" <<EOF
name: e2e-postgres
checks:
  - type: postgres
    dump: db/dump.sql
    tables:
      - name: users
        min_rows: 3
EOF

cat > "$WORK/agent.yaml" <<EOF
repo: $GOOD
recipes: [recipe.yaml]
sandbox:
  dir: $SANDBOX
EOF

DOCKER_BEFORE="$(docker_state)"

log "case 1: pg_dump loads into throwaway postgres, row assertions pass (exit 0)"
"$BIN" test --config "$WORK/agent.yaml" --json > "$WORK/result.json"
python3 - "$WORK/result.json" <<'PY'
import json, sys
doc = json.load(open(sys.argv[1]))
assert doc["status"] == "pass", doc
[check] = doc["checks"]
assert check["type"] == "postgres", check
assert "table assertion" in check["message"], check
PY
assert_docker_clean "passing run"
assert_sandbox_empty

log "case 2: truncated dump fails with a clear message (exit 1)"
BROKEN_SRC="$WORK/src-broken"
mkdir -p "$BROKEN_SRC/db"
head -c 70 "$SRC/db/dump.sql" > "$BROKEN_SRC/db/dump.sql"  # cut mid-COPY
BAD="$WORK/bad-repo"
restic -r "$BAD" init -q
restic -r "$BAD" backup -q "$BROKEN_SRC"

cat > "$WORK/recipe-broken.yaml" <<EOF
name: e2e-postgres-broken
checks:
  - type: postgres
    dump: db/dump.sql
    tables:
      - name: users
        min_rows: 3
EOF
cat > "$WORK/agent-broken.yaml" <<EOF
repo: $BAD
recipes: [recipe-broken.yaml]
sandbox:
  dir: $SANDBOX
EOF

got=0
"$BIN" test --config "$WORK/agent-broken.yaml" --json > "$WORK/broken.json" || got=$?
[ "$got" -eq 1 ] || fail "expected exit 1 for truncated dump, got $got"
python3 - "$WORK/broken.json" <<'PY'
import json, sys
doc = json.load(open(sys.argv[1]))
assert doc["status"] == "fail", doc
[check] = doc["checks"]
assert check["status"] == "fail", check
assert "failed to load" in check["message"], f"message not clear: {check['message']!r}"
PY
assert_docker_clean "failing run"
assert_sandbox_empty

log "all docker e2e cases passed"
