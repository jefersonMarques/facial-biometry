#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BUILD_DIR="${1:-$ROOT/build/liveness-wasm}"

if ! command -v emcmake >/dev/null 2>&1; then
  echo "Emscripten not found. Activate emsdk before running this script." >&2
  exit 1
fi

emcmake cmake -S "$ROOT/faceproof-liveness-core" -B "$BUILD_DIR" -DFACEPROOF_BUILD_TESTS=OFF -DCMAKE_BUILD_TYPE=Release
cmake --build "$BUILD_DIR" --config Release

test -s "$BUILD_DIR/faceproof-liveness-core.js"
test -s "$BUILD_DIR/faceproof-liveness-core.wasm"

echo "[ok] $BUILD_DIR/faceproof-liveness-core.js"
echo "[ok] $BUILD_DIR/faceproof-liveness-core.wasm"
