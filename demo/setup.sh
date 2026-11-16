#!/usr/bin/env bash
# Builds the demo fixture: a small Vaultwarden-shaped data directory and a
# restic repository holding one good snapshot of it. Used by the demo workflow
# to record docs/assets/demo.gif and to produce docs/assets/sample-report.json.
#
#   demo/setup.sh <root>
#
# creates <root>/vaultwarden (the app data) and <root>/backups/restic (the
# repository). Needs restic and python3. RESTIC_PASSWORD must be set.
set -euo pipefail

ROOT="${1:?usage: demo/setup.sh <root>}"
: "${RESTIC_PASSWORD:?RESTIC_PASSWORD must be set}"

APP="$ROOT/vaultwarden"
REPO="$ROOT/backups/restic"
rm -rf "$APP" "$REPO"
mkdir -p "$APP/attachments/7f3c" "$APP/icon_cache" "$(dirname "$REPO")"

# A real SQLite database with enough rows to span many pages, so a partial
# copy of it is detectably broken rather than accidentally valid.
python3 - "$APP/db.sqlite3" <<'PY'
import sqlite3, sys, uuid
db = sqlite3.connect(sys.argv[1])
db.execute("create table users (uuid text primary key, email text, created_at text)")
db.execute("create table ciphers (uuid text primary key, user_uuid text, name text, data blob)")
for i in range(40):
    db.execute("insert into users values (?, ?, datetime('now'))", (str(uuid.uuid4()), f"user{i}@example.com"))
for i in range(2500):
    db.execute("insert into ciphers values (?, ?, ?, randomblob(64))", (str(uuid.uuid4()), "demo", f"login-{i}"))
db.commit()
db.close()
PY

openssl genrsa -out "$APP/rsa_key.pem" 2048 2>/dev/null
cat > "$APP/config.json" <<'EOF'
{"domain": "https://vault.example.com", "signups_allowed": false}
EOF
head -c 24000 /dev/urandom > "$APP/attachments/7f3c/receipt.pdf"
for site in github.com example.com wikipedia.org; do
  head -c 2000 /dev/urandom > "$APP/icon_cache/$site.png"
done

restic init -q -r "$REPO"
restic backup -q -r "$REPO" "$APP"
echo "fixture ready: app $APP, repo $REPO"
