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

python3 - "$BUILD_DIR" <<'PY'
import hashlib
import json
import pathlib
import sys

build_dir = pathlib.Path(sys.argv[1])
files = []
for name in ("faceproof-liveness-core.js", "faceproof-liveness-core.wasm"):
    path = build_dir / name
    files.append({
        "path": name,
        "bytes": path.stat().st_size,
        "sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
    })

manifest = {
    "schemaVersion": 1,
    "version": "0.1.0",
    "files": files,
}
(build_dir / "faceproof-liveness-core.manifest.json").write_text(
    json.dumps(manifest, indent=2, sort_keys=True) + "\n",
    encoding="utf-8",
)
PY

cp "$BUILD_DIR/faceproof-liveness-core.manifest.json" "$PUBLISH_DIR/faceproof-liveness-core.manifest.json"

echo "[ok] build: $BUILD_DIR/faceproof-liveness-core.js"
echo "[ok] build: $BUILD_DIR/faceproof-liveness-core.wasm"
echo "[ok] manifest: $BUILD_DIR/faceproof-liveness-core.manifest.json"
echo "[ok] publish: $PUBLISH_DIR"
