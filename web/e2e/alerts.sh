#!/usr/bin/env bash
# End-to-end acceptance test for Phase 4 (SPEC.md): alerting + stale detection.
#
# Proves, against a mock Telegram API:
#   1. a forced-failure fixture fires a Telegram message (well under 5 min)
#   2. repeat failures are suppressed (one alert per incident)
#   3. the next pass sends a recovery notice
#   4. a stale simulation (clock-shifted rows) fires via the cron endpoint
#   5. cron re-runs do not duplicate the stale alert
#   6. a clock-shifted agent fires agent-silent; a daemon heartbeat resolves it
#
# Requirements: local Supabase stack; the web app running with
#   TELEGRAM_BOT_TOKEN=e2e-token TELEGRAM_API_BASE=http://127.0.0.1:8181
#   CRON_SECRET (default local-dev-cron-secret)
# plus go, restic, python3. The mock Telegram server is started by this script.
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:3100}"
SUPABASE_URL="${SUPABASE_URL:-http://127.0.0.1:54321}"
ANON_KEY="${SUPABASE_ANON_KEY:?SUPABASE_ANON_KEY must be set}"
SERVICE_KEY="${SUPABASE_SERVICE_ROLE_KEY:?SUPABASE_SERVICE_ROLE_KEY must be set}"
CRON_SECRET="${CRON_SECRET:-local-dev-cron-secret}"
MOCK_PORT=8181

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORK="$(mktemp -d)"
MOCK_LOG="$WORK/telegram.ndjson"
touch "$MOCK_LOG"

log()  { printf '\n==> %s\n' "$*"; }
fail() { printf 'E2E FAIL: %s\n' "$*" >&2; exit 1; }

log "starting mock Telegram API on :$MOCK_PORT"
python3 - "$MOCK_PORT" "$MOCK_LOG" <<'PY' &
from http.server import BaseHTTPRequestHandler, HTTPServer
import json, sys

class H(BaseHTTPRequestHandler):
    def _ok(self, body=b'{"ok":true}'):
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(body)
    def do_POST(self):
        n = int(self.headers.get("content-length", 0))
        body = self.rfile.read(n).decode()
        with open(sys.argv[2], "a") as f:
            f.write(json.dumps({"path": self.path, "body": body}) + "\n")
        self._ok()
    def do_GET(self):
        self._ok(b'{"ok":true,"result":[]}')
    def log_message(self, *args):
        pass

HTTPServer(("127.0.0.1", int(sys.argv[1])), H).serve_forever()
PY
MOCK_PID=$!
trap 'kill "$MOCK_PID" 2>/dev/null; rm -rf "$WORK"' EXIT
sleep 1

# message_count / assert helpers over the mock's capture file
message_count() { wc -l < "$MOCK_LOG" | tr -d ' '; }
last_message()  { tail -1 "$MOCK_LOG" | python3 -c 'import json,sys; d=json.loads(sys.stdin.read()); print(json.loads(d["body"])["text"])'; }
assert_last_contains() {
  last_message | grep -qF "$1" || fail "last telegram message does not contain '$1': $(last_message)"
}

log "creating user, registering agent"
SIGNUP="$WORK/signup.json"
curl -sf -X POST "$SUPABASE_URL/auth/v1/signup" \
  -H "apikey: $ANON_KEY" -H "Content-Type: application/json" \
  -d "{\"email\":\"alerts-$(date +%s)@e2e.local\",\"password\":\"e2e-password-123\"}" > "$SIGNUP"
JWT="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["access_token"])' "$SIGNUP")"
USER_ID="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["user"]["id"])' "$SIGNUP")"

TOKEN="rrt_$(python3 -c 'import secrets; print(secrets.token_urlsafe(32))')"
TOKEN_HASH="$(python3 -c 'import hashlib,sys; print(hashlib.sha256(sys.argv[1].encode()).hexdigest())' "$TOKEN")"
curl -sf -X POST "$SUPABASE_URL/rest/v1/registration_tokens" \
  -H "apikey: $ANON_KEY" -H "Authorization: Bearer $JWT" -H "Content-Type: application/json" \
  -d "{\"user_id\":\"$USER_ID\",\"token_hash\":\"$TOKEN_HASH\"}"

BIN="$WORK/restorable"
(cd "$ROOT/agent" && go build -o "$BIN" .)
CREDS="$WORK/creds.json"
"$BIN" register --url "$BASE_URL" --token "$TOKEN" --name alerts-e2e --credentials "$CREDS" > /dev/null

log "creating verified telegram channel (chat 42)"
curl -sf -X POST "$SUPABASE_URL/rest/v1/alert_channels" \
  -H "apikey: $ANON_KEY" -H "Authorization: Bearer $JWT" -H "Content-Type: application/json" \
  -H "Prefer: return=representation" \
  -d '{"user_id":"'$USER_ID'","type":"telegram","config":{"chat_id":"42"}}' > "$WORK/channel.json"
CHANNEL_ID="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))[0]["id"])' "$WORK/channel.json")"
curl -sf -X PATCH "$SUPABASE_URL/rest/v1/alert_channels?id=eq.$CHANNEL_ID" \
  -H "apikey: $ANON_KEY" -H "Authorization: Bearer $JWT" -H "Content-Type: application/json" \
  -d '{"verified":true}'

log "building fixture: repo whose recipe requires a missing path"
SRC="$WORK/src"; mkdir -p "$SRC"
echo "data" > "$SRC/data.txt"
export RESTIC_PASSWORD="e2e-password"
restic -r "$WORK/repo" init -q
restic -r "$WORK/repo" backup -q "$SRC"
SANDBOX="$WORK/sandboxes"; mkdir -p "$SANDBOX"
printf 'name: broken\nchecks:\n  - type: files\n    require:\n      - path: nope.txt\n' > "$WORK/recipe-broken.yaml"
printf 'name: good\nchecks:\n  - type: files\n    require:\n      - path: data.txt\n' > "$WORK/recipe-good.yaml"
printf "repo: $WORK/repo\nrecipes: [recipe-broken.yaml]\nsandbox: {dir: $SANDBOX}\n" > "$WORK/agent-broken.yaml"
printf "repo: $WORK/repo\nrecipes: [recipe-good.yaml]\nsandbox: {dir: $SANDBOX}\n" > "$WORK/agent-good.yaml"

log "case 1: forced failure fires a telegram alert (< 5 min)"
T0=$(date +%s)
"$BIN" test --config "$WORK/agent-broken.yaml" --credentials "$CREDS" > /dev/null 2>&1 || true
for i in $(seq 1 30); do [ "$(message_count)" -ge 1 ] && break; sleep 1; done
[ "$(message_count)" -eq 1 ] || fail "expected 1 telegram message, got $(message_count)"
echo "    alert arrived $(( $(date +%s) - T0 ))s after the failing run started"
[ $(( $(date +%s) - T0 )) -lt 300 ] || fail "alert took longer than 5 minutes"
assert_last_contains "Restore test failed"
python3 -c '
import json, sys
body = json.loads(json.loads(open(sys.argv[1]).readlines()[-1])["body"])
assert body["chat_id"] == "42", body
' "$MOCK_LOG" || fail "alert did not target chat 42"

log "case 2: second failure is suppressed (one alert per incident)"
"$BIN" test --config "$WORK/agent-broken.yaml" --credentials "$CREDS" > /dev/null 2>&1 || true
sleep 2
[ "$(message_count)" -eq 1 ] || fail "duplicate alert sent: $(message_count) messages"

log "case 3: recovery notice on the next pass"
"$BIN" test --config "$WORK/agent-good.yaml" --credentials "$CREDS" > /dev/null
for i in $(seq 1 30); do [ "$(message_count)" -ge 2 ] && break; sleep 1; done
[ "$(message_count)" -eq 2 ] || fail "expected recovery notice (2 messages), got $(message_count)"
assert_last_contains "passing again"

log "case 4: clock-shifted rows make the repo stale; cron fires"
# A second pass run gives staleness a cadence to infer from.
"$BIN" test --config "$WORK/agent-good.yaml" --credentials "$CREDS" > /dev/null
BEFORE_STALE="$(message_count)"
# Shift every run 10 days into the past (service role bypasses RLS).
curl -sf -X PATCH "$SUPABASE_URL/rest/v1/test_runs?user_id=eq.$USER_ID" \
  -H "apikey: $SERVICE_KEY" -H "Authorization: Bearer $SERVICE_KEY" -H "Content-Type: application/json" \
  -d '{"finished_at":"'"$(python3 -c 'import datetime; print((datetime.datetime.now(datetime.timezone.utc)-datetime.timedelta(days=10)).isoformat())')"'"}' > /dev/null
CRON_OUT="$(curl -sf "$BASE_URL/api/cron/alerts" -H "Authorization: Bearer $CRON_SECRET")"
echo "    cron: $CRON_OUT"
echo "$CRON_OUT" | grep -q '"stale_opened":1' || fail "cron did not open a stale incident: $CRON_OUT"
for i in $(seq 1 10); do [ "$(message_count)" -gt "$BEFORE_STALE" ] && break; sleep 1; done
assert_last_contains "gone quiet"

log "case 5: cron re-run does not duplicate the stale alert"
COUNT_BEFORE="$(message_count)"
CRON_OUT="$(curl -sf "$BASE_URL/api/cron/alerts" -H "Authorization: Bearer $CRON_SECRET")"
echo "$CRON_OUT" | grep -q '"stale_opened":0' || fail "cron duplicated the stale incident: $CRON_OUT"
[ "$(message_count)" -eq "$COUNT_BEFORE" ] || fail "duplicate stale alert message sent"

log "case 6: clock-shifted agent fires agent-silent; heartbeat resolves it"
curl -sf -X PATCH "$SUPABASE_URL/rest/v1/agents?user_id=eq.$USER_ID" \
  -H "apikey: $SERVICE_KEY" -H "Authorization: Bearer $SERVICE_KEY" -H "Content-Type: application/json" \
  -d '{"last_seen":"'"$(python3 -c 'import datetime; print((datetime.datetime.now(datetime.timezone.utc)-datetime.timedelta(days=2)).isoformat())')"'"}' > /dev/null
CRON_OUT="$(curl -sf "$BASE_URL/api/cron/alerts" -H "Authorization: Bearer $CRON_SECRET")"
echo "$CRON_OUT" | grep -q '"silent_opened":1' || fail "cron did not open an agent-silent incident: $CRON_OUT"
sleep 1
assert_last_contains "gone silent"
# The run daemon heartbeats at startup: run it briefly with a far-off schedule.
printf "repo: $WORK/repo\nschedule: \"0 3 1 1 *\"\nrecipes: [recipe-good.yaml]\nsandbox: {dir: $SANDBOX}\n" > "$WORK/agent-daemon.yaml"
"$BIN" run --config "$WORK/agent-daemon.yaml" --credentials "$CREDS" > /dev/null 2>&1 &
DAEMON_PID=$!
for i in $(seq 1 20); do
  last_message | grep -q "back online" && break
  sleep 1
done
kill "$DAEMON_PID" 2>/dev/null || true
assert_last_contains "back online"

log "case 7: cron endpoint rejects bad secrets"
CODE="$(curl -s -o /dev/null -w '%{http_code}' "$BASE_URL/api/cron/alerts" -H "Authorization: Bearer wrong")"
[ "$CODE" = "401" ] || fail "cron with bad secret returned $CODE, want 401"

log "all alerting e2e cases passed"
