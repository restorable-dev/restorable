#!/bin/sh
# Restorable installer — https://github.com/restorable-dev/restorable
#
# Downloads the latest release binary for this platform, verifies its
# checksum, and installs it. Safe to re-run to upgrade.
#
#   curl -fsSL https://raw.githubusercontent.com/restorable-dev/restorable/main/install.sh | sh
#
# Options (environment variables):
#   RESTORABLE_VERSION   version tag to install (default: latest release)
#   RESTORABLE_INSTALL   install directory (default: /usr/local/bin,
#                        falls back to ~/.local/bin without root)
set -eu

REPO="restorable-dev/restorable"
BASE_URL="${RESTORABLE_BASE_URL:-https://github.com/$REPO/releases/download}"

say()  { printf 'restorable install: %s\n' "$*"; }
fail() { printf 'restorable install: ERROR: %s\n' "$*" >&2; exit 1; }

# ── platform ─────────────────────────────────────────────────────────────────
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$OS" in
  linux|darwin) ;;
  *) fail "unsupported OS: $OS (linux and macOS only)" ;;
esac
case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) fail "unsupported architecture: $ARCH" ;;
esac
[ "$OS/$ARCH" = "darwin/amd64" ] && fail "no darwin/amd64 build; use an Apple Silicon Mac or a Linux box"

# ── version ──────────────────────────────────────────────────────────────────
VERSION="${RESTORABLE_VERSION:-}"
if [ -z "$VERSION" ]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name"' | head -1 | cut -d'"' -f4)" \
    || fail "could not determine the latest release"
  [ -n "$VERSION" ] || fail "could not determine the latest release"
fi
say "installing restorable $VERSION for $OS/$ARCH"

# ── download + verify ────────────────────────────────────────────────────────
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
ASSET="restorable_${VERSION}_${OS}_${ARCH}.tar.gz"
curl -fsSL -o "$TMP/$ASSET" "$BASE_URL/$VERSION/$ASSET" \
  || fail "download failed: $BASE_URL/$VERSION/$ASSET"
curl -fsSL -o "$TMP/SHA256SUMS" "$BASE_URL/$VERSION/SHA256SUMS" \
  || fail "checksum download failed"

cd "$TMP"
if command -v sha256sum >/dev/null 2>&1; then
  grep "$ASSET" SHA256SUMS | sha256sum -c - >/dev/null || fail "checksum verification FAILED"
elif command -v shasum >/dev/null 2>&1; then
  grep "$ASSET" SHA256SUMS | shasum -a 256 -c - >/dev/null || fail "checksum verification FAILED"
else
  fail "need sha256sum or shasum to verify the download"
fi
say "checksum verified"
tar -xzf "$ASSET"

# ── install ──────────────────────────────────────────────────────────────────
DEST="${RESTORABLE_INSTALL:-/usr/local/bin}"
if [ ! -w "$DEST" ] && [ -z "${RESTORABLE_INSTALL:-}" ]; then
  DEST="$HOME/.local/bin"
  mkdir -p "$DEST"
fi
install -m 0755 restorable "$DEST/restorable" 2>/dev/null \
  || { mkdir -p "$DEST" && cp restorable "$DEST/restorable" && chmod 0755 "$DEST/restorable"; } \
  || fail "could not install into $DEST (set RESTORABLE_INSTALL or use sudo)"

say "installed: $DEST/restorable"
case ":$PATH:" in
  *":$DEST:"*) ;;
  *) say "NOTE: $DEST is not on your PATH — add it, e.g.: export PATH=\"$DEST:\$PATH\"" ;;
esac
"$DEST/restorable" version || true
say "next: write agent.yaml and run 'restorable test' — see https://github.com/$REPO#quickstart-5-minutes"
