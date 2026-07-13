#!/usr/bin/env bash
# Verifies the Telegram deep-link connect flow end to end, and that it is
# private per user (the webhook only stamps the link matching the token):
#   1. a user starts connect → a telegram_links row + t.me deep link
#   2. the bot webhook (with the right secret) stamps chat_id for that token
#   3. the webhook rejects a wrong secret
#   4. a different user's token is untouched (no cross-user exposure)
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:3100}"
SUPABASE_URL="${SUPABASE_URL:-http://127.0.0.1:54321}"
ANON_KEY="${SUPABASE_ANON_KEY:?SUPABASE_ANON_KEY must be set}"
SERVICE_KEY="${SUPABASE_SERVICE_ROLE_KEY:?SUPABASE_SERVICE_ROLE_KEY must be set}"
WH_SECRET="${TELEGRAM_WEBHOOK_SECRET:-local-dev-tg-webhook}"

WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT
log()  { printf '\n==> %s\n' "$*"; }
fail() { printf 'E2E FAIL: %s\n' "$*" >&2; exit 1; }

signup() { # email → prints "user_id token"
  curl -sf -X POST "$SUPABASE_URL/auth/v1/signup" -H "apikey: $ANON_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$1\",\"password\":\"e2e-password-123\"}" > "$WORK/u.json"
  echo "$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["user"]["id"])' "$WORK/u.json") \
$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["access_token"])' "$WORK/u.json")"
}

# mint a telegram_links token directly (mirrors startTelegramConnect) so the
# test does not need a browser.
mint_token() { # user_id jwt → prints token
  local tok="tg_$(python3 -c 'import secrets;print(secrets.token_urlsafe(24))')"
  curl -sf -X POST "$SUPABASE_URL/rest/v1/telegram_links" -H "apikey: $ANON_KEY" \
    -H "Authorization: Bearer $2" -H "Content-Type: application/json" \
    -d "{\"token\":\"$tok\",\"user_id\":\"$1\"}" >/dev/null
  echo "$tok"
}

webhook() { # token chat_id secret → http code
  curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE_URL/api/telegram/webhook" \
    -H "X-Telegram-Bot-Api-Secret-Token: $3" -H "Content-Type: application/json" \
    -d "{\"message\":{\"text\":\"/start $1\",\"chat\":{\"id\":$2,\"first_name\":\"Test\"}}}"
}

chat_of() { # token jwt → chat_id (or empty)
  curl -sf "$SUPABASE_URL/rest/v1/telegram_links?select=chat_id&token=eq.$1" \
    -H "apikey: $ANON_KEY" -H "Authorization: Bearer $2" \
    | python3 -c 'import json,sys;d=json.load(sys.stdin);print(d[0]["chat_id"] if d and d[0]["chat_id"] else "")'
}

log "creating users A and B"
STAMP="$(date +%s)"
read -r UA JA <<< "$(signup "tg-a-$STAMP@e2e.local")"
read -r UB JB <<< "$(signup "tg-b-$STAMP@e2e.local")"

TOKA="$(mint_token "$UA" "$JA")"
TOKB="$(mint_token "$UB" "$JB")"

log "case 1: wrong webhook secret is rejected (401)"
CODE="$(webhook "$TOKA" 111 "wrong-secret")"
[ "$CODE" = "401" ] || fail "bad secret returned $CODE, want 401"
[ -z "$(chat_of "$TOKA" "$JA")" ] || fail "chat_id was stamped despite bad secret"

log "case 2: correct webhook stamps A's chat_id"
CODE="$(webhook "$TOKA" 55501 "$WH_SECRET")"
[ "$CODE" = "200" ] || fail "valid webhook returned $CODE, want 200"
[ "$(chat_of "$TOKA" "$JA")" = "55501" ] || fail "A's chat_id not stamped"

log "case 3: B's token is untouched (no cross-user exposure)"
[ -z "$(chat_of "$TOKB" "$JB")" ] || fail "B's link changed by A's connect — cross-user leak!"

log "case 4: an unknown token is a harmless no-op (200, nothing created)"
CODE="$(webhook "nonexistent-token-xyz" 99999 "$WH_SECRET")"
[ "$CODE" = "200" ] || fail "unknown token returned $CODE, want 200"

log "all telegram-connect e2e cases passed"
