#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD_DIR="${1:-$ROOT/build/liveness-wasm}"
PUBLISH_DIR="${2:-$ROOT/apps/web-sdk/wasm/liveness-core}"

if ! command -v emcmake >/dev/null 2>&1; then
  echo "Emscripten not found. Activate emsdk before running this script." >&2
  exit 1
fi

emcmake cmake -S "$ROOT/faceproof-liveness-core" -B "$BUILD_DIR" -DFACEPROOF_BUILD_TESTS=OFF -DCMAKE_BUILD_TYPE=Release
cmake --build "$BUILD_DIR" --config Release

test -s "$BUILD_DIR/faceproof-liveness-core.js"
test -s "$BUILD_DIR/faceproof-liveness-core.wasm"

mkdir -p "$PUBLISH_DIR"
cp "$BUILD_DIR/faceproof-liveness-core.js" "$PUBLISH_DIR/faceproof-liveness-core.js"
cp "$BUILD_DIR/faceproof-liveness-core.wasm" "$PUBLISH_DIR/faceproof-liveness-core.wasm"

echo "[ok] build: $BUILD_DIR/faceproof-liveness-core.js"
echo "[ok] build: $BUILD_DIR/faceproof-liveness-core.wasm"
echo "[ok] publish: $PUBLISH_DIR"
