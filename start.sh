#!/usr/bin/env sh
# GSNode Web UI server
set -eu

BIN="${GSNODE_BIN:-gsnode}"
ADDR="${GSNODE_ADDR:-:8899}"
DATA="${GSNODE_DATA:-./data}"

if ! command -v "$BIN" >/dev/null 2>&1; then
  echo "gsnode not found. Install first:" >&2
  echo "  curl -fsSL https://github.com/gsvps/GSNode/raw/main/install.sh | sh" >&2
  exit 1
fi

mkdir -p "$DATA"
echo "Starting GSNode Web UI on http://0.0.0.0${ADDR} (data: $DATA)"
exec "$BIN" -addr "$ADDR" -data "$DATA"
