#!/usr/bin/env bash
# Scripted local smoke test for the built app-cleaner CLI binary.
# Not part of `go test`: exercises the real compiled binary against a
# throwaway $HOME, proving the engine's scanners.DefaultRoots() (which
# reads os.UserHomeDir() -> $HOME on macOS/Linux) is injectable via plain
# environment override — no test-only seam needed for this smoke.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN="$ROOT_DIR/bin/app-cleaner"

if [ ! -x "$BIN" ]; then
  echo "error: $BIN not found — run 'task build:cli' first" >&2
  exit 1
fi

VERSION_OUT="$("$BIN" --version)"
if [ -z "$VERSION_OUT" ]; then
  echo "error: --version produced no output" >&2
  exit 1
fi
echo "version: $VERSION_OUT"

SMOKE_HOME="$(mktemp -d)"
trap 'rm -rf "$SMOKE_HOME"' EXIT

mkdir -p "$SMOKE_HOME/.Trash"
echo "junk" > "$SMOKE_HOME/.Trash/old-file.txt"
mkdir -p "$SMOKE_HOME/Library/Caches/com.example.app"
echo "cache-data" > "$SMOKE_HOME/Library/Caches/com.example.app/blob.bin"

SCAN_JSON="$(HOME="$SMOKE_HOME" "$BIN" scan --categories trash,system-cache --json)"

if ! command -v jq >/dev/null 2>&1; then
  echo "warning: jq not found, skipping JSON assertions (raw output below)" >&2
  echo "$SCAN_JSON"
else
  # scan --json (Task 3 shape): {categories:[{id, itemCount, ...}]} with NO
  # per-item `items` array in non-verbose mode, so assert on itemCount.
  TRASH_ITEMS="$(echo "$SCAN_JSON" | jq '[.categories[] | select(.id=="trash") | .itemCount] | add // 0')"
  if [ "$TRASH_ITEMS" -lt 1 ]; then
    echo "error: expected at least 1 trash item in scan --json against $SMOKE_HOME, got $TRASH_ITEMS" >&2
    echo "$SCAN_JSON" >&2
    exit 1
  fi
  echo "ok: scan --json found $TRASH_ITEMS trash item(s) under a scratch HOME"
fi

echo "smoke test passed"
