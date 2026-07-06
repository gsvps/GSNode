#!/usr/bin/env sh
# GSNode installer — https://github.com/gsvps/GSNode
set -eu

INSTALL_DIR="${GSNODE_INSTALL_DIR:-/usr/local/bin}"
BASE_URL="${GSNODE_BASE_URL:-https://github.com/gsvps/GSNode/raw/main/bin}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  linux)  ;;
  darwin) OS=darwin ;;
  *)
    echo "unsupported OS: $OS (use manual download from bin/)" >&2
    exit 1
    ;;
esac

case "$ARCH" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  i686|i386|x86) ARCH=386 ;;
  *)
    echo "unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

NAME="gsnode-${OS}-${ARCH}"
URL="$BASE_URL/$NAME"

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required" >&2
  exit 1
fi

TMP="$(mktemp)"
trap 'rm -f "$TMP"' EXIT

echo "Downloading $URL ..."
curl -fL --retry 3 --connect-timeout 30 "$URL" -o "$TMP"

chmod +x "$TMP"
mkdir -p "$INSTALL_DIR"
mv "$TMP" "$INSTALL_DIR/gsnode"

echo ""
echo "GSNode installed: $INSTALL_DIR/gsnode"
echo ""
echo "  Quick check : gsnode -quick"
echo "  Full check  : gsnode -run"
echo "  Web console : gsnode -addr :8899"
echo ""
