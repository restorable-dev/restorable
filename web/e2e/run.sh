#!/usr/bin/env bash
# End-to-end acceptance test for Phase 3 (SPEC.md): control plane MVP.
#
# Proves:
#   1. fresh account → registration token → `restorable register` → agent key
#   2. a local `restorable test` run appears in the account's data (the same
#      RLS-guarded rows the dashboard renders) within 10 seconds
#   3. registration tokens are single-use
#   4. RLS: user B cannot read user A's rows via direct PostgREST queries
#   5. /api/v1 rejects unauthenticated submissions
#
# Requirements: a running local Supabase stack (supabase start, from web/),
# the web app serving (PORT, default 3100), go, restic, python3.
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:3100}"
SUPABASE_URL="${SUPABASE_URL:-http://127.0.0.1:54321}"
ANON_KEY="${SUPABASE_ANON_KEY:?SUPABASE_ANON_KEY must be set (see supabase status)}"

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

log()  { printf '\n==> %s\n' "$*"; }
fail() { printf 'E2E FAIL: %s\n' "$*" >&2; exit 1; }

# jqpy FILTER FILE — tiny jq replacement using python3.
jqpy() {
  python3 -c "
import json, sys
doc = json.load(open(sys.argv[2]))
for part in sys.argv[1].split('.'):
    if part:
        doc = doc[int(part)] if part.lstrip('-').isdigit() else doc[part]
print(doc if not isinstance(doc, (dict, list)) else json.dumps(doc))
" "$1" "$2"
}

# signup EMAIL → prints "user_id access_token"
signup() {
  local out="$WORK/signup.json"
  curl -sf -X POST "$SUPABASE_URL/auth/v1/signup" \
    -H "apikey: $ANON_KEY" -H "Content-Type: application/json" \
    -d "{\"email\":\"$1\",\"password\":\"e2e-password-123\"}" > "$out" \
    || fail "signup for $1"
  echo "$(jqpy user.id "$out") $(jqpy access_token "$out")"
}

log "creating users A and B"
STAMP="$(date +%s)"
read -r USER_A JWT_A <<< "$(signup "a-$STAMP@e2e.local")"
read -r USER_B JWT_B <<< "$(signup "b-$STAMP@e2e.local")"
[ -n "$USER_A" ] && [ -n "$JWT_B" ] || fail "user creation"

log "user A mints a registration token (via PostgREST, RLS-scoped)"
TOKEN="rrt_$(python3 -c 'import secrets; print(secrets.token_urlsafe(32))')"
TOKEN_HASH="$(python3 -c "import hashlib,sys; print(hashlib.sha256(sys.argv[1].encode()).hexdigest())" "$TOKEN")"
curl -sf -X POST "$SUPABASE_URL/rest/v1/registration_tokens" \
  -H "apikey: $ANON_KEY" -H "Authorization: Bearer $JWT_A" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"$USER_A\",\"token_hash\":\"$TOKEN_HASH\"}" \
  || fail "minting registration token"

log "building agent and fixture repo"
BIN="$WORK/restorable"
(cd "$ROOT/agent" && go build -o "$BIN" .)
SRC="$WORK/src"
mkdir -p "$SRC"
echo "important data" > "$SRC/data.txt"
export RESTIC_PASSWORD="e2e-password"
restic -r "$WORK/repo" init -q
restic -r "$WORK/repo" backup -q "$SRC"
SANDBOX="$WORK/sandboxes"; mkdir -p "$SANDBOX"
printf 'name: e2e\nchecks:\n  - type: files\n    require:\n      - path: data.txt\n' > "$WORK/recipe.yaml"
printf "repo: $WORK/repo\nrecipes: [recipe.yaml]\nsandbox:\n  dir: $SANDBOX\n" > "$WORK/agent.yaml"

log "registering agent with one-time token"
CREDS="$WORK/creds.json"
"$BIN" register --url "$BASE_URL" --token "$TOKEN" --name e2e-agent --credentials "$CREDS" \
  || fail "agent registration"
[ -f "$CREDS" ] || fail "credentials file not written"

log "reusing the token must fail (single use)"
if "$BIN" register --url "$BASE_URL" --token "$TOKEN" --name sneaky --credentials "$WORK/creds2.json" 2>/dev/null; then
  fail "token reuse was accepted"
fi

log "running a local test with cloud reporting"
"$BIN" test --config "$WORK/agent.yaml" --credentials "$CREDS" > /dev/null || fail "restorable test"
FINISHED_AT_MS="$(python3 -c 'import time; print(int(time.time()*1000))')"

log "run must appear in user A's data within 10 seconds"
DEADLINE=$((SECONDS + 10))
RUNS="$WORK/runs.json"
while :; do
  curl -sf "$SUPABASE_URL/rest/v1/test_runs?select=id,status,results" \
    -H "apikey: $ANON_KEY" -H "Authorization: Bearer $JWT_A" > "$RUNS"
  COUNT="$(python3 -c "import json,sys; print(len(json.load(open(sys.argv[1]))))" "$RUNS")"
  [ "$COUNT" -ge 1 ] && break
  [ "$SECONDS" -lt "$DEADLINE" ] || fail "run did not appear within 10s"
  sleep 1
done
ELAPSED_MS=$(( $(python3 -c 'import time; print(int(time.time()*1000))') - FINISHED_AT_MS ))
echo "    run visible ${ELAPSED_MS}ms after completion"
[ "$(jqpy 0.status "$RUNS")" = "pass" ] || fail "run status is not pass: $(cat "$RUNS")"

curl -sf "$SUPABASE_URL/rest/v1/repos?select=label,fingerprint" \
  -H "apikey: $ANON_KEY" -H "Authorization: Bearer $JWT_A" > "$WORK/repos.json"
[ "$(python3 -c "import json,sys; print(len(json.load(open(sys.argv[1]))))" "$WORK/repos.json")" -ge 1 ] \
  || fail "repo row missing for user A"

log "RLS: user B must see nothing of user A's"
for table in test_runs repos agents registration_tokens; do
  curl -sf "$SUPABASE_URL/rest/v1/$table?select=id" \
    -H "apikey: $ANON_KEY" -H "Authorization: Bearer $JWT_B" > "$WORK/b.json"
  GOT="$(cat "$WORK/b.json")"
  [ "$GOT" = "[]" ] || fail "user B can read $table: $GOT"
done
# Anonymous (no JWT) must also see nothing.
for table in test_runs repos agents; do
  GOT="$(curl -s "$SUPABASE_URL/rest/v1/$table?select=id" -H "apikey: $ANON_KEY")"
  [ "$GOT" = "[]" ] || echo "$GOT" | grep -q '"code"' || fail "anonymous can read $table: $GOT"
done

log "/api/v1 must reject unauthenticated and garbage requests"
CODE="$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE_URL/api/v1/runs" \
  -H "Content-Type: application/json" -d '{}')"
[ "$CODE" = "401" ] || fail "unauthenticated /runs returned $CODE, want 401"
API_KEY="$(jqpy api_key "$CREDS")"
CODE="$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE_URL/api/v1/runs" \
  -H "Authorization: Bearer $API_KEY" -H "Content-Type: application/json" -d '{"nope":1}')"
[ "$CODE" = "400" ] || fail "invalid payload returned $CODE, want 400"

log "dashboard must be auth-gated"
CODE="$(curl -s -o /dev/null -w '%{http_code}' "$BASE_URL/dashboard")"
{ [ "$CODE" = "307" ] || [ "$CODE" = "302" ]; } || fail "/dashboard without session returned $CODE, want redirect"

log "all control-plane e2e cases passed"
