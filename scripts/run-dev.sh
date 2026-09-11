#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

python3 models/download_models.py

if [ ! -d .venv ]; then
    python3 -m venv .venv
fi

"$ROOT/.venv/bin/python" -m pip install -r services/engine/requirements.txt

mkdir -p "$ROOT/.dev"
chmod 700 "$ROOT/.dev"

if [ ! -f "$ROOT/.dev/session-secret" ]; then
    python3 -c 'import secrets; print(secrets.token_urlsafe(48))' > "$ROOT/.dev/session-secret"
    chmod 600 "$ROOT/.dev/session-secret"
fi
if [ ! -f "$ROOT/.dev/template-key" ]; then
    python3 -c 'import os,base64; print(base64.b64encode(os.urandom(32)).decode())' > "$ROOT/.dev/template-key"
    chmod 600 "$ROOT/.dev/template-key"
fi

export FACEPROOF_SESSION_SECRET="${FACEPROOF_SESSION_SECRET:-$(cat "$ROOT/.dev/session-secret")}"
export FACEPROOF_TEMPLATE_KEY="${FACEPROOF_TEMPLATE_KEY:-$(cat "$ROOT/.dev/template-key")}"
export FACEPROOF_TEMPLATE_DIR="${FACEPROOF_TEMPLATE_DIR:-$ROOT/data/templates}"
export FACEPROOF_YUNET_MODEL="${FACEPROOF_YUNET_MODEL:-$ROOT/models/yunet/face_detection_yunet_2023mar.onnx}"
export FACEPROOF_SFACE_MODEL="${FACEPROOF_SFACE_MODEL:-$ROOT/models/sface/face_recognition_sface_2021dec.onnx}"
export FACEPROOF_MINIFASNET_MODEL="${FACEPROOF_MINIFASNET_MODEL:-$ROOT/models/minifasnet/MiniFASNetV2.onnx}"

cleanup() {
    kill "${ENGINE_PID:-}" "${API_PID:-}" "${WEB_PID:-}" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

(
    cd services/engine
    "$ROOT/.venv/bin/python" engine_server.py
) &
ENGINE_PID=$!

(
    cd services/api
    go run ./cmd/server
) &
API_PID=$!

(
    cd apps/web-sdk
    python3 -m http.server 5173
) &
WEB_PID=$!

echo "FaceProof demo: http://localhost:5173"
wait
