#!/usr/bin/env sh
set -eu
VERSION="${GSPROBE_VERSION:-latest}"
INSTALL_DIR="${GSPROBE_INSTALL_DIR:-/usr/local/bin}"
BASE_URL="${GSPROBE_BASE_URL:-https://github.com/gsvps/gsprobe/releases}"
CURL_TIMEOUT_OPTS="--connect-timeout 10 --max-time 300"
ARCH="$(uname -m)"
case "$ARCH" in x86_64|amd64) ARCH=amd64;; aarch64|arm64) ARCH=arm64;; *) echo "unsupported architecture: $ARCH" >&2; exit 1;; esac
if [ "$VERSION" = "latest" ]; then
  DOWNLOAD_BASE="$BASE_URL/latest/download"
else
  DOWNLOAD_BASE="$BASE_URL/download/$VERSION"
fi
ASSET="gsprobe-linux-$ARCH"
URL="$DOWNLOAD_BASE/$ASSET"
SUM_URL="$URL.sha256"
TMP="$(mktemp)"; SUMFILE="$(mktemp)"
trap 'rm -f "$TMP" "$SUMFILE"' EXIT

echo "Downloading $URL"
curl -fL $CURL_TIMEOUT_OPTS --retry 3 "$URL" -o "$TMP"

echo "Fetching checksum $SUM_URL"
if ! curl -fsL $CURL_TIMEOUT_OPTS --retry 3 "$SUM_URL" -o "$SUMFILE"; then
  echo "checksum file not found at $SUM_URL, refusing to install" >&2
  exit 1
fi
EXPECTED="$(awk '{print $1}' "$SUMFILE")"
if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL="$(sha256sum "$TMP" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL="$(shasum -a 256 "$TMP" | awk '{print $1}')"
elif command -v openssl >/dev/null 2>&1; then
  ACTUAL="$(openssl dgst -sha256 "$TMP" | awk '{print $NF}')"
else
  echo "no sha256 tool available (need sha256sum, shasum, or openssl); refusing to install unverified binary" >&2
  exit 1
fi
if [ "$EXPECTED" != "$ACTUAL" ]; then
  echo "checksum mismatch: expected $EXPECTED, got $ACTUAL" >&2
  exit 1
fi
echo "Checksum OK: $ACTUAL"

chmod +x "$TMP"
mkdir -p "$INSTALL_DIR"
mv "$TMP" "$INSTALL_DIR/gsprobe"
echo "Installed: $INSTALL_DIR/gsprobe"
echo "Run: gsprobe -addr :8899"
