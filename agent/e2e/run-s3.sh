#!/usr/bin/env bash
# Proves the agent works against an S3 backend (the most common remote restic
# repo — covers AWS S3, MinIO, Wasabi, and Backblaze B2's S3 API). Uses a
# throwaway MinIO container as the S3 endpoint.
#
# Requirements: go, restic, docker, aws CLI, python3.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
WORK="$(mktemp -d)"
NAME="restorable-s3-e2e-$$"
cleanup() { docker rm -f "$NAME" >/dev/null 2>&1 || true; rm -rf "$WORK"; }
trap cleanup EXIT

log()  { printf '\n==> %s\n' "$*"; }
fail() { printf 'E2E FAIL: %s\n' "$*" >&2; exit 1; }

log "starting MinIO (S3 endpoint)"
docker run -d --name "$NAME" -p 9101:9000 \
  -e MINIO_ROOT_USER=testaccess -e MINIO_ROOT_PASSWORD=testsecret123 \
  minio/minio server /data >/dev/null
for i in $(seq 1 30); do
  curl -sf -o /dev/null http://localhost:9101/minio/health/live && break
  sleep 1
  [ "$i" = 30 ] && fail "MinIO did not come up"
done

export AWS_ACCESS_KEY_ID=testaccess AWS_SECRET_ACCESS_KEY=testsecret123 AWS_DEFAULT_REGION=us-east-1
aws --endpoint-url http://localhost:9101 s3 mb s3://backups >/dev/null

log "building agent + fixture"
BIN="$WORK/restorable"
(cd "$ROOT" && go build -o "$BIN" .)
SRC="$WORK/src"; mkdir -p "$SRC/data"
echo "config" > "$SRC/config.txt"
for i in $(seq 1 20); do echo "f$i" > "$SRC/data/f$i.txt"; done

export RESTIC_REPOSITORY="s3:http://localhost:9101/backups" RESTIC_PASSWORD="s3-e2e-pw"
restic init -q
restic backup -q "$SRC"

log "restorable init writes a working config from the s3: repo"
OUT="$WORK/out"; mkdir -p "$OUT"
( cd "$OUT" && "$BIN" init --repo "$RESTIC_REPOSITORY" --yes )
[ -f "$OUT/agent.yaml" ] || fail "init did not write agent.yaml against s3:"

log "restorable test passes against the s3: repo"
"$BIN" test --config "$OUT/agent.yaml" >/dev/null || fail "test failed against s3: backend"

log "s3 backend e2e passed"
