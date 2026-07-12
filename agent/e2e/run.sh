#!/usr/bin/env bash
# End-to-end acceptance test for Phase 1 (SPEC.md).
#
# Generates restic fixtures from scratch (nothing is checked in), then proves:
#   1. `restorable test` PASSES against a healthy repo          (exit 0)
#   2. `restorable test` FAILS when a recipe assertion breaks    (exit 1)
#   3. `restorable test` ERRORS against a corrupted repo         (exit 2)
#   4. the sandbox directory is empty after every run, including failures
#
# Requirements: go (to build the agent) and restic. No Docker, no network.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

log()  { printf '\n==> %s\n' "$*"; }
fail() { printf 'E2E FAIL: %s\n' "$*" >&2; exit 1; }

# expect_exit WANT CMD... — run CMD, assert its exit code.
expect_exit() {
  local want=$1; shift
  local got=0
  "$@" || got=$?
  [ "$got" -eq "$want" ] || fail "expected exit $want, got $got: $*"
}

assert_sandbox_empty() {
  [ -z "$(ls -A "$SANDBOX")" ] || fail "sandbox base dir not empty after run: $(ls -A "$SANDBOX")"
}

log "building agent"
BIN="$WORK/restorable"
(cd "$ROOT" && go build -o "$BIN" .)

log "creating known source tree"
SRC="$WORK/src"
mkdir -p "$SRC/data/sub"
echo "<?php \$config = true;" > "$SRC/config.php"
for i in $(seq 1 25); do echo "content of file $i" > "$SRC/data/f$i.txt"; done
echo "nested" > "$SRC/data/sub/deep.txt"

export RESTIC_PASSWORD="e2e-password"

log "creating healthy repo fixture"
GOOD="$WORK/good-repo"
restic -r "$GOOD" init -q
restic -r "$GOOD" backup -q "$SRC"

SANDBOX="$WORK/sandboxes"
mkdir -p "$SANDBOX"

cat > "$WORK/recipe.yaml" <<EOF
name: e2e
checks:
  - type: files
    require:
      - path: config.php
      - path: data/
        min_files: 25
      - path: data/sub/deep.txt
    checksum_sample: 5
EOF

cat > "$WORK/agent-good.yaml" <<EOF
repo: $GOOD
recipes: [recipe.yaml]
sandbox:
  dir: $SANDBOX
EOF

log "case 1: healthy repo must pass (exit 0)"
expect_exit 0 "$BIN" test --config "$WORK/agent-good.yaml"
assert_sandbox_empty

log "case 1b: --json output must be valid JSON with status pass"
JSON_OUT="$WORK/result.json"
expect_exit 0 "$BIN" test --config "$WORK/agent-good.yaml" --json
"$BIN" test --config "$WORK/agent-good.yaml" --json > "$JSON_OUT"
python3 - "$JSON_OUT" <<'PY'
import json, sys
with open(sys.argv[1]) as f:
    doc = json.load(f)
assert doc["status"] == "pass", doc
assert doc["snapshot_id"], doc
assert len(doc["checks"]) == 1, doc
PY
assert_sandbox_empty

log "case 1c: 'restorable init' generates a working config + recipe and passes"
INIT_DIR="$WORK/init-out"
mkdir -p "$INIT_DIR"
( cd "$INIT_DIR" && RESTIC_REPOSITORY="$GOOD" "$BIN" init --yes )
[ -f "$INIT_DIR/agent.yaml" ] || fail "init did not write agent.yaml"
ls "$INIT_DIR"/*.yaml | grep -qv agent.yaml || fail "init did not write a recipe"
# The generated config must itself pass a real test.
expect_exit 0 "$BIN" test --config "$INIT_DIR/agent.yaml"

log "case 2: broken recipe assertion must fail (exit 1)"
cat > "$WORK/recipe-broken.yaml" <<EOF
name: e2e-broken
checks:
  - type: files
    require:
      - path: does/not/exist.txt
EOF
cat > "$WORK/agent-broken-recipe.yaml" <<EOF
repo: $GOOD
recipes: [recipe-broken.yaml]
sandbox:
  dir: $SANDBOX
EOF
expect_exit 1 "$BIN" test --config "$WORK/agent-broken-recipe.yaml"
assert_sandbox_empty

log "case 3: corrupted repo must error (exit 2)"
BAD="$WORK/bad-repo"
cp -R "$GOOD" "$BAD"
# Garble every data pack so the restore cannot succeed. restic writes packs
# read-only, so make them writable first.
find "$BAD/data" -type f | while read -r pack; do
  chmod u+w "$pack"
  head -c 512 /dev/urandom > "$pack"
done
cat > "$WORK/agent-bad.yaml" <<EOF
repo: $BAD
recipes: [recipe.yaml]
sandbox:
  dir: $SANDBOX
EOF
expect_exit 2 "$BIN" test --config "$WORK/agent-bad.yaml"
assert_sandbox_empty

log "case 4: missing password env must error (exit 2)"
env -u RESTIC_PASSWORD "$BIN" test --config "$WORK/agent-good.yaml" && fail "expected failure without password" || {
  got=$?
  [ "$got" -eq 2 ] || fail "expected exit 2 without password, got $got"
}
assert_sandbox_empty

log "all e2e cases passed"
