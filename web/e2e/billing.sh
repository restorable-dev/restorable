#!/usr/bin/env bash
# End-to-end acceptance test for Phase 5 (SPEC.md): billing + plan gating.
#
# Proves, with Stripe webhook payloads signed by the SDK's own test helper
# (the handler's signature verification runs its real code path):
#   1. free-plan limits are enforced server-side: 2nd repo rejected (403),
#      2nd alert channel rejected at the DATABASE (RLS policy)
#   2. an upgrade webhook unlocks limits immediately — no redeploy: the same
#      running server then accepts the 2nd repo and 2nd channel, and the
#      agent config endpoint reports pro entitlements
#   3. webhook replay is idempotent (acknowledged, no reprocessing)
#   4. cancellation downgrades gracefully: limits re-applied, all data
#      retained, existing repos keep accepting runs
#
# Requirements: local supabase, web running (WITH .env.local stripe config),
# go, restic, node, python3.
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:3100}"
SUPABASE_URL="${SUPABASE_URL:-http://127.0.0.1:54321}"
ANON_KEY="${SUPABASE_ANON_KEY:?SUPABASE_ANON_KEY must be set}"
WEBHOOK_SECRET="${STRIPE_WEBHOOK_SECRET:-whsec_local_dev_secret}"

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WEB="$ROOT/web"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

log()  { printf '\n==> %s\n' "$*"; }
fail() { printf 'E2E FAIL: %s\n' "$*" >&2; exit 1; }

# deliver_event NAME — signs $WORK/NAME.json and POSTs it to the webhook.
# Prints the HTTP status code.
deliver_event() {
  local payload="$WORK/$1.json"
  local sig
  sig="$(node "$WEB/e2e/sign-stripe-event.mjs" "$WEBHOOK_SECRET" < "$payload")"
  curl -s -o "$WORK/wh-resp.json" -w '%{http_code}' -X POST "$BASE_URL/api/webhooks/stripe" \
    -H "stripe-signature: $sig" -H "Content-Type: application/json" \
    --data-binary "@$payload"
}

log "creating user + agent"
curl -sf -X POST "$SUPABASE_URL/auth/v1/signup" \
  -H "apikey: $ANON_KEY" -H "Content-Type: application/json" \
  -d "{\"email\":\"billing-$(date +%s)@e2e.local\",\"password\":\"e2e-password-123\"}" > "$WORK/u.json"
JWT="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["access_token"])' "$WORK/u.json")"
USER_ID="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["user"]["id"])' "$WORK/u.json")"

RT="rrt_$(python3 -c 'import secrets; print(secrets.token_urlsafe(32))')"
RTH="$(python3 -c 'import hashlib,sys; print(hashlib.sha256(sys.argv[1].encode()).hexdigest())' "$RT")"
curl -sf -X POST "$SUPABASE_URL/rest/v1/registration_tokens" \
  -H "apikey: $ANON_KEY" -H "Authorization: Bearer $JWT" -H "Content-Type: application/json" \
  -d "{\"user_id\":\"$USER_ID\",\"token_hash\":\"$RTH\"}"

BIN="$WORK/restorable"
(cd "$ROOT/agent" && go build -o "$BIN" .)
CREDS="$WORK/creds.json"
"$BIN" register --url "$BASE_URL" --token "$RT" --name billing-e2e --credentials "$CREDS" > /dev/null

export RESTIC_PASSWORD="e2e-password"
SANDBOX="$WORK/sandboxes"; mkdir -p "$SANDBOX"
make_repo() { # make_repo N → config file path for repo N
  local dir="$WORK/src$1"; mkdir -p "$dir"
  echo "data $1" > "$dir/data.txt"
  restic -r "$WORK/repo$1" init -q
  restic -r "$WORK/repo$1" backup -q "$dir"
  printf 'name: r%s\nchecks:\n  - type: files\n    require:\n      - path: data.txt\n' "$1" > "$WORK/recipe$1.yaml"
  printf "repo: $WORK/repo$1\nrecipes: [recipe$1.yaml]\nsandbox: {dir: $SANDBOX}\n" > "$WORK/agent$1.yaml"
}
make_repo 1
make_repo 2
make_repo 3

log "case 1a: free plan accepts the first repo"
"$BIN" test --config "$WORK/agent1.yaml" --credentials "$CREDS" > /dev/null

log "case 1b: free plan rejects a second repo (server-side 403)"
OUT="$("$BIN" test --config "$WORK/agent2.yaml" --credentials "$CREDS" 2>&1 >/dev/null)" || true
echo "$OUT" | grep -q "plan limit" || fail "expected plan-limit rejection, got: $OUT"
COUNT="$(curl -sf "$SUPABASE_URL/rest/v1/repos?select=id" -H "apikey: $ANON_KEY" -H "Authorization: Bearer $JWT" | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')"
[ "$COUNT" = "1" ] || fail "repo count = $COUNT, want 1"

log "case 1c: free plan caps alert channels at 1 (database-level)"
curl -sf -X POST "$SUPABASE_URL/rest/v1/alert_channels" \
  -H "apikey: $ANON_KEY" -H "Authorization: Bearer $JWT" -H "Content-Type: application/json" \
  -d '{"user_id":"'$USER_ID'","type":"ntfy","config":{"topic":"e2e-one"}}' || fail "first channel should be allowed"
CODE="$(curl -s -o /dev/null -w '%{http_code}' -X POST "$SUPABASE_URL/rest/v1/alert_channels" \
  -H "apikey: $ANON_KEY" -H "Authorization: Bearer $JWT" -H "Content-Type: application/json" \
  -d '{"user_id":"'$USER_ID'","type":"ntfy","config":{"topic":"e2e-two"}}')"
[ "$CODE" = "403" ] || fail "second free channel returned $CODE, want 403 (RLS)"

log "case 2: upgrade via signed stripe webhooks unlocks limits (no redeploy)"
CUS="cus_e2e_$(date +%s)"
SUB="sub_e2e_$(date +%s)"
PERIOD_END=$(python3 -c 'import time; print(int(time.time()) + 30*24*3600)')
cat > "$WORK/checkout.json" <<EOF
{"id":"evt_e2e_checkout_$USER_ID","object":"event","type":"checkout.session.completed","data":{"object":{"id":"cs_e2e_1","object":"checkout.session","customer":"$CUS","subscription":"$SUB","client_reference_id":"$USER_ID"}}}
EOF
cat > "$WORK/subupdated.json" <<EOF
{"id":"evt_e2e_subup_$USER_ID","object":"event","type":"customer.subscription.updated","data":{"object":{"id":"$SUB","object":"subscription","customer":"$CUS","status":"active","current_period_end":$PERIOD_END}}}
EOF
[ "$(deliver_event checkout)" = "200" ] || fail "checkout webhook rejected: $(cat "$WORK/wh-resp.json")"
[ "$(deliver_event subupdated)" = "200" ] || fail "subscription webhook rejected: $(cat "$WORK/wh-resp.json")"

PLAN="$(curl -sf "$SUPABASE_URL/rest/v1/subscriptions?select=plan,status" -H "apikey: $ANON_KEY" -H "Authorization: Bearer $JWT" | python3 -c 'import json,sys; d=json.load(sys.stdin)[0]; print(d["plan"], d["status"])')"
[ "$PLAN" = "pro active" ] || fail "subscription row = $PLAN, want 'pro active'"

# The same running server must now accept what it just rejected.
"$BIN" test --config "$WORK/agent2.yaml" --credentials "$CREDS" > /dev/null || fail "second repo still rejected after upgrade"
curl -sf -X POST "$SUPABASE_URL/rest/v1/alert_channels" \
  -H "apikey: $ANON_KEY" -H "Authorization: Bearer $JWT" -H "Content-Type: application/json" \
  -d '{"user_id":"'$USER_ID'","type":"ntfy","config":{"topic":"e2e-two"}}' || fail "second channel still rejected after upgrade"
API_KEY="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["api_key"])' "$CREDS")"
curl -sf "$BASE_URL/api/v1/agents/config" -H "Authorization: Bearer $API_KEY" > "$WORK/config.json"
grep -q '"plan":"pro"' "$WORK/config.json" || fail "config endpoint does not report pro: $(cat "$WORK/config.json")"

log "case 3: webhook replay is idempotent"
CODE="$(deliver_event checkout)"
[ "$CODE" = "200" ] || fail "replay returned $CODE, want 200"
grep -q '"replay":true' "$WORK/wh-resp.json" || fail "replay was not detected: $(cat "$WORK/wh-resp.json")"
PLAN="$(curl -sf "$SUPABASE_URL/rest/v1/subscriptions?select=plan,status" -H "apikey: $ANON_KEY" -H "Authorization: Bearer $JWT" | python3 -c 'import json,sys; d=json.load(sys.stdin)[0]; print(d["plan"], d["status"])')"
[ "$PLAN" = "pro active" ] || fail "replay mutated the subscription: $PLAN"

log "case 4: bad signature is rejected"
CODE="$(curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE_URL/api/webhooks/stripe" \
  -H "stripe-signature: t=1,v1=deadbeef" -H "Content-Type: application/json" \
  --data-binary "@$WORK/checkout.json")"
[ "$CODE" = "400" ] || fail "bad signature returned $CODE, want 400"

log "case 5: cancellation downgrades gracefully"
cat > "$WORK/subdeleted.json" <<EOF
{"id":"evt_e2e_subdel_$USER_ID","object":"event","type":"customer.subscription.deleted","data":{"object":{"id":"$SUB","object":"subscription","customer":"$CUS","status":"canceled","current_period_end":$PERIOD_END}}}
EOF
[ "$(deliver_event subdeleted)" = "200" ] || fail "deletion webhook rejected"
PLAN="$(curl -sf "$SUPABASE_URL/rest/v1/subscriptions?select=plan,status" -H "apikey: $ANON_KEY" -H "Authorization: Bearer $JWT" | python3 -c 'import json,sys; d=json.load(sys.stdin)[0]; print(d["plan"], d["status"])')"
[ "$PLAN" = "free canceled" ] || fail "after cancel: $PLAN, want 'free canceled'"

# Data retained: both repos and their runs are still there.
COUNT="$(curl -sf "$SUPABASE_URL/rest/v1/repos?select=id" -H "apikey: $ANON_KEY" -H "Authorization: Bearer $JWT" | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')"
[ "$COUNT" = "2" ] || fail "repos lost on downgrade: $COUNT"
# Limits re-applied: a third repo is rejected again…
OUT="$("$BIN" test --config "$WORK/agent3.yaml" --credentials "$CREDS" 2>&1 >/dev/null)" || true
echo "$OUT" | grep -q "plan limit" || fail "third repo accepted after downgrade: $OUT"
# …but existing repos keep working (grace: nothing already reporting breaks).
"$BIN" test --config "$WORK/agent2.yaml" --credentials "$CREDS" > /dev/null || fail "existing repo broken by downgrade"
# Channel cap re-applied (2 channels exist ≥ limit 1 → deny new).
CODE="$(curl -s -o /dev/null -w '%{http_code}' -X POST "$SUPABASE_URL/rest/v1/alert_channels" \
  -H "apikey: $ANON_KEY" -H "Authorization: Bearer $JWT" -H "Content-Type: application/json" \
  -d '{"user_id":"'$USER_ID'","type":"ntfy","config":{"topic":"e2e-three"}}')"
[ "$CODE" = "403" ] || fail "third channel after downgrade returned $CODE, want 403"

log "all billing e2e cases passed"
