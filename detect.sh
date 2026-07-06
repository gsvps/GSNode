#!/usr/bin/env sh
# GSNode benchmark runner — quick or full detection
set -eu

BIN="${GSNODE_BIN:-gsnode}"
MODE="${1:-quick}"

if ! command -v "$BIN" >/dev/null 2>&1; then
  echo "gsnode not found. Install first:" >&2
  echo "  curl -fsSL https://github.com/gsvps/GSNode/raw/v0.1.0/install.sh | sh" >&2
  exit 1
fi

case "$MODE" in
  quick|q)
    echo "Running GSNode quick benchmark ..."
    exec "$BIN" -quick
    ;;
  full|run|f)
    echo "Running GSNode full benchmark ..."
    exec "$BIN" -run
    ;;
  *)
    echo "Usage: $0 [quick|full]" >&2
    echo "  quick  — fast check (default)" >&2
    echo "  full   — full benchmark" >&2
    exit 1
    ;;
esac
