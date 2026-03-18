#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="$ROOT_DIR/bin"
UI_DIR="$ROOT_DIR/ui"

require_cmd() {
  local cmd="$1"
  local hint="$2"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "error: missing required build tool: $cmd" >&2
    echo "hint: $hint" >&2
    exit 1
  fi
}

require_cmd go "install Go 1.26+ and ensure 'go' is in PATH"
require_cmd node "install Node.js 22+ and ensure 'node' is in PATH"
require_cmd npm "install npm and ensure 'npm' is in PATH"

mkdir -p "$BIN_DIR"

echo "==> building admin"
go build -o "$BIN_DIR/admin" ./admin

echo "==> building engine"
go build -o "$BIN_DIR/engine" ./engine

if [ ! -d "$UI_DIR/node_modules" ]; then
  echo "error: frontend dependencies are not installed" >&2
  echo "hint: run 'cd ui && npm install' before executing ./build.sh" >&2
  exit 1
fi

echo "==> building ui"
(cd "$UI_DIR" && npm run build)

echo "build completed"
echo "artifacts:"
echo "  $BIN_DIR/admin"
echo "  $BIN_DIR/engine"
echo "  $UI_DIR/dist"
