#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

python3 models/download_models.py

if [ ! -d .venv ]; then
    python3 -m venv .venv
fi

"$ROOT/.venv/bin/python" -m pip install -r services/engine/requirements.txt

mkdir -p "$ROOT/.dev" "$ROOT/data/identity-checks"
chmod 700 "$ROOT/.dev" "$ROOT/data/identity-checks"

if [ ! -f "$ROOT/.dev/session-secret" ]; then
    python3 -c 'import secrets; print(secrets.token_urlsafe(48))' > "$ROOT/.dev/session-secret"
    chmod 600 "$ROOT/.dev/session-secret"
fi
if [ ! -f "$ROOT/.dev/template-key" ]; then
    python3 -c 'import os,base64; print(base64.b64encode(os.urandom(32)).decode())' > "$ROOT/.dev/template-key"
    chmod 600 "$ROOT/.dev/template-key"
fi
if [ ! -f "$ROOT/.dev/identity-issuer-key" ]; then
    python3 -c 'import secrets; print(secrets.token_urlsafe(48))' > "$ROOT/.dev/identity-issuer-key"
    chmod 600 "$ROOT/.dev/identity-issuer-key"
fi

export FACEPROOF_SESSION_SECRET="${FACEPROOF_SESSION_SECRET:-$(cat "$ROOT/.dev/session-secret")}"
export FACEPROOF_TEMPLATE_KEY="${FACEPROOF_TEMPLATE_KEY:-$(cat "$ROOT/.dev/template-key")}"
export FACEPROOF_TEMPLATE_DIR="${FACEPROOF_TEMPLATE_DIR:-$ROOT/data/templates}"
export FACEPROOF_IDENTITY_ISSUER_KEY="${FACEPROOF_IDENTITY_ISSUER_KEY:-$(cat "$ROOT/.dev/identity-issuer-key")}"
export FACEPROOF_IDENTITY_DIR="${FACEPROOF_IDENTITY_DIR:-$ROOT/data/identity-checks}"
export FACEPROOF_IDENTITY_VERIFY_URL="${FACEPROOF_IDENTITY_VERIFY_URL:-http://localhost:5173/verify.html}"
export FACEPROOF_YUNET_MODEL="${FACEPROOF_YUNET_MODEL:-$ROOT/models/yunet/face_detection_yunet_2023mar.onnx}"
export FACEPROOF_SFACE_MODEL="${FACEPROOF_SFACE_MODEL:-$ROOT/models/sface/face_recognition_sface_2021dec.onnx}"
export FACEPROOF_MINIFASNET_MODEL="${FACEPROOF_MINIFASNET_MODEL:-$ROOT/models/minifasnet/MiniFASNetV2.onnx}"
export FACEPROOF_ALLOW_REVIEW_ENROLLMENT="${FACEPROOF_ALLOW_REVIEW_ENROLLMENT:-true}"
export FACEPROOF_DEBUG="${FACEPROOF_DEBUG:-true}"

missing_required=()
for tool in pdfinfo pdftoppm; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        missing_required+=("$tool")
    fi
done
if [ "${#missing_required[@]}" -gt 0 ]; then
    echo "WARNING: IDENTITY CHECK unavailable until these tools are installed/configured: ${missing_required[*]}"
fi
if ! command -v pdfsig >/dev/null 2>&1; then
    echo "INFO: pdfsig not found. FaceProof will use native Go cryptographic PDF signature verification."
fi

for tool in pdfimages pdftotext; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        echo "WARNING: $tool not found; complementary PDF forensics will be limited."
    fi
done
if ! command -v openssl >/dev/null 2>&1; then
    echo "INFO: openssl not found. Native verification still checks certificate validity dates; openssl is complementary evidence."
fi
if ! command -v bpgdec >/dev/null 2>&1 && [ -z "${FACEPROOF_BPGDEC_PATH:-}" ]; then
    echo "INFO: bpgdec not found. CNH identity can still use the signed PDF portrait."
fi

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
    cd services/api
    FACEPROOF_WEB_DIR="$ROOT/apps/web-sdk" go run ./cmd/devgateway
) &
WEB_PID=$!

echo "FaceProof demo: http://localhost:5173"
echo "Identity verification: http://localhost:5173/verify.html"
echo "Identity issuer key: $ROOT/.dev/identity-issuer-key"
echo "Development mode: review enrollments are stored as provisional templates."
wait
