#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

NATIVE_SHADOW=false
NATIVE_CORE=true
PYTHON_ENGINE=false
ANALYTICS=false
for argument in "$@"; do
    case "$argument" in
        --native-shadow)
            NATIVE_SHADOW=true
            NATIVE_CORE=false
            PYTHON_ENGINE=true
            ;;
        --native-core)
            NATIVE_CORE=true
            PYTHON_ENGINE=false
            ;;
        --python-engine)
            NATIVE_CORE=false
            PYTHON_ENGINE=true
            ;;
        --analytics)
            ANALYTICS=true
            ;;
        *)
            echo "Unknown argument: $argument" >&2
            echo "Usage: scripts/run-dev.sh [--native-core|--python-engine|--native-shadow] [--analytics]" >&2
            exit 2
            ;;
    esac
done

if [ "$PYTHON_ENGINE" = true ]; then
    python3 models/download_models.py
fi

WEB_SDK_DIR="$ROOT/apps/web-sdk"
MEDIAPIPE_PACKAGE="$WEB_SDK_DIR/node_modules/@mediapipe/tasks-vision/package.json"
if [ ! -f "$MEDIAPIPE_PACKAGE" ]; then
    echo "Installing pinned web SDK dependencies..."
    (cd "$WEB_SDK_DIR" && npm install --no-audit --no-fund)
fi
python3 scripts/vendor-mediapipe.py

LIVENESS_RUNTIME_MANIFEST="$ROOT/apps/web-sdk/wasm/liveness-core/faceproof-liveness-core.manifest.json"
if [ ! -f "$LIVENESS_RUNTIME_MANIFEST" ]; then
    echo "FaceProof Liveness Core manifest not found. Run scripts/build-liveness-wasm.sh first." >&2
    exit 1
fi
export FACEPROOF_RUNTIME_SDK_VERSION="0.3.0"
export FACEPROOF_RUNTIME_LIVENESS_CORE_VERSION="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["version"])' "$LIVENESS_RUNTIME_MANIFEST")"
export FACEPROOF_RUNTIME_LIVENESS_WASM_SHA256="$(python3 -c 'import json,sys; d=json.load(open(sys.argv[1], encoding="utf-8")); print(next(x["sha256"] for x in d["files"] if x["path"]=="faceproof-liveness-core.wasm"))' "$LIVENESS_RUNTIME_MANIFEST")"
export FACEPROOF_RUNTIME_MEDIAPIPE_VERSION="1.0.1"

if [ "$PYTHON_ENGINE" = true ]; then
    if [ ! -d .venv ]; then
        python3 -m venv .venv
    fi

    "$ROOT/.venv/bin/python" -m pip install -r services/engine/requirements.txt
fi

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
if [ ! -f "$ROOT/.dev/admin-key" ]; then
    python3 -c 'import secrets; print(secrets.token_urlsafe(48))' > "$ROOT/.dev/admin-key"
    chmod 600 "$ROOT/.dev/admin-key"
fi

export FACEPROOF_SESSION_SECRET="${FACEPROOF_SESSION_SECRET:-$(cat "$ROOT/.dev/session-secret")}"
export FACEPROOF_TEMPLATE_KEY="${FACEPROOF_TEMPLATE_KEY:-$(cat "$ROOT/.dev/template-key")}"
export FACEPROOF_TEMPLATE_DIR="${FACEPROOF_TEMPLATE_DIR:-$ROOT/data/templates}"
export FACEPROOF_IDENTITY_ISSUER_KEY="${FACEPROOF_IDENTITY_ISSUER_KEY:-$(cat "$ROOT/.dev/identity-issuer-key")}"
export FACEPROOF_ADMIN_KEY="${FACEPROOF_ADMIN_KEY:-$(cat "$ROOT/.dev/admin-key")}"
export FACEPROOF_IDENTITY_DIR="${FACEPROOF_IDENTITY_DIR:-$ROOT/data/identity-checks}"
export FACEPROOF_IDENTITY_VERIFY_URL="${FACEPROOF_IDENTITY_VERIFY_URL:-http://localhost:5173/verify.html}"
export FACEPROOF_API_ADDR="${FACEPROOF_API_ADDR:-:8180}"
export FACEPROOF_DEV_API_URL="${FACEPROOF_DEV_API_URL:-http://127.0.0.1:8180}"
if [ "$PYTHON_ENGINE" = true ]; then
    export FACEPROOF_YUNET_MODEL="${FACEPROOF_YUNET_MODEL:-$ROOT/models/yunet/face_detection_yunet_2023mar.onnx}"
    export FACEPROOF_SFACE_MODEL="${FACEPROOF_SFACE_MODEL:-$ROOT/models/sface/face_recognition_sface_2021dec.onnx}"
    export FACEPROOF_MINIFASNET_MODEL="${FACEPROOF_MINIFASNET_MODEL:-$ROOT/models/minifasnet/MiniFASNetV2.onnx}"
fi

if [ "$NATIVE_CORE" = true ]; then
    NATIVE_CORE_MANIFEST="$ROOT/.dev/biometric-core.json"
    if [ ! -f "$NATIVE_CORE_MANIFEST" ]; then
        echo "FaceProof Secure Core not prepared. Run: bash scripts/build-biometric-core.sh" >&2
        exit 1
    fi

    export FACEPROOF_SECURE_CORE_LIBRARY="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["libraryPath"])' "$NATIVE_CORE_MANIFEST")"
    CORE_OPENCV_LIB_DIR="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["openCvLibDir"])' "$NATIVE_CORE_MANIFEST")"
    CORE_ONNXRUNTIME_LIB_DIR="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["onnxRuntimeLibDir"])' "$NATIVE_CORE_MANIFEST")"

    if [ ! -f "$FACEPROOF_SECURE_CORE_LIBRARY" ]; then
        echo "Secure Core library not found: $FACEPROOF_SECURE_CORE_LIBRARY" >&2
        exit 1
    fi

    DEFAULT_MODEL_PACK="$ROOT/.dev/model-packs/faceproof-standard-0.1.0.fpmp"
    DEFAULT_MODEL_PACK_PUBLIC_KEY="$ROOT/.dev/model-pack-public.key"

    if [ -z "${FACEPROOF_MODEL_PACK:-}" ]; then
        if [ ! -f "$DEFAULT_MODEL_PACK" ] || [ ! -f "$DEFAULT_MODEL_PACK_PUBLIC_KEY" ]; then
            echo "Preparing signed development Model Pack..."
            bash "$ROOT/scripts/build-model-pack.sh"
        fi
        export FACEPROOF_MODEL_PACK="$DEFAULT_MODEL_PACK"
    fi

    if [ ! -f "$FACEPROOF_MODEL_PACK" ]; then
        echo "FaceProof Model Pack not found: $FACEPROOF_MODEL_PACK" >&2
        exit 1
    fi

    if [ -z "${FACEPROOF_MODEL_PACK_PUBLIC_KEY:-}" ]; then
        if [ ! -f "$DEFAULT_MODEL_PACK_PUBLIC_KEY" ]; then
            echo "Model Pack public key not found: $DEFAULT_MODEL_PACK_PUBLIC_KEY" >&2
            exit 1
        fi
        export FACEPROOF_MODEL_PACK_PUBLIC_KEY="$(tr -d '\r\n' < "$DEFAULT_MODEL_PACK_PUBLIC_KEY")"
    fi

    export FACEPROOF_MODEL_PACK_CACHE_DIR="${FACEPROOF_MODEL_PACK_CACHE_DIR:-$ROOT/data/model-cache}"
    mkdir -p "$FACEPROOF_MODEL_PACK_CACHE_DIR"
    chmod 700 "$FACEPROOF_MODEL_PACK_CACHE_DIR"

    unset FACEPROOF_YUNET_MODEL FACEPROOF_SFACE_MODEL FACEPROOF_MINIFASNET_MODEL 2>/dev/null || true

    export LD_LIBRARY_PATH="$CORE_OPENCV_LIB_DIR:$CORE_ONNXRUNTIME_LIB_DIR${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
    echo "Secure Core C++ authority: ENABLED"
    echo "Signed Model Pack: $FACEPROOF_MODEL_PACK"
else
    unset FACEPROOF_SECURE_CORE_LIBRARY 2>/dev/null || true
fi

export FACEPROOF_NATIVE_SHADOW_ENABLED="false"
if [ "$NATIVE_SHADOW" = true ]; then
    NATIVE_SHADOW_MANIFEST="$ROOT/.dev/native-shadow.json"
    if [ ! -f "$NATIVE_SHADOW_MANIFEST" ]; then
        echo "Native biometric shadow not prepared. Run scripts/build-biometric-shadow.sh first." >&2
        exit 1
    fi

    export FACEPROOF_NATIVE_SHADOW_ENABLED="true"
    export FACEPROOF_BIOMETRIC_VISION_CLI="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["visionCli"])' "$NATIVE_SHADOW_MANIFEST")"
    export FACEPROOF_BIOMETRIC_REFERENCE_CLI="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["referenceCli"])' "$NATIVE_SHADOW_MANIFEST")"
    export FACEPROOF_BIOMETRIC_PAD_CLI="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["padCli"])' "$NATIVE_SHADOW_MANIFEST")"
    OPENCV_LIB_DIR="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["openCvLibDir"])' "$NATIVE_SHADOW_MANIFEST")"
    ONNXRUNTIME_LIB_DIR="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1], encoding="utf-8"))["onnxRuntimeLibDir"])' "$NATIVE_SHADOW_MANIFEST")"

    for executable in "$FACEPROOF_BIOMETRIC_VISION_CLI" "$FACEPROOF_BIOMETRIC_REFERENCE_CLI" "$FACEPROOF_BIOMETRIC_PAD_CLI"; do
        if [ ! -x "$executable" ]; then
            echo "Native shadow executable not found: $executable" >&2
            exit 1
        fi
    done

    export LD_LIBRARY_PATH="$OPENCV_LIB_DIR:$ONNXRUNTIME_LIB_DIR${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
    echo "Native biometric shadow: ENABLED (vision + PAD)"
fi

if [ "$ANALYTICS" = true ]; then
    ANALYTICS_DATABASE_URL_FILE="$ROOT/.dev/analytics-database-url"
    if [ -z "${FACEPROOF_ANALYTICS_DATABASE_URL:-}" ] && [ -f "$ANALYTICS_DATABASE_URL_FILE" ]; then
        export FACEPROOF_ANALYTICS_DATABASE_URL="$(cat "$ANALYTICS_DATABASE_URL_FILE")"
    fi
    source "$ROOT/scripts/start-analytics-postgres.sh"
fi

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

required_ports=(8180 5173)
if [ "$PYTHON_ENGINE" = true ]; then
    required_ports+=(8090)
fi
if [ "$ANALYTICS" = true ]; then
    required_ports+=(5174)
fi

occupied_ports=()
for port in "${required_ports[@]}"; do
    if ss -ltnH "sport = :$port" 2>/dev/null | grep -q .; then
        occupied_ports+=("$port")
    fi
done

if [ "${#occupied_ports[@]}" -gt 0 ]; then
    echo "FaceProof cannot start because these ports are already in use: ${occupied_ports[*]}" >&2
    echo "A previous development stack is probably still running." >&2
    echo "Stop the previous ./scripts/run-dev.sh process and try again." >&2
    echo "Diagnostic: ss -ltnp | grep -E ':(8090|8180|5173|5174)\\b'" >&2
    exit 1
fi

cleanup() {
    kill "${ENGINE_PID:-}" "${API_PID:-}" "${WEB_PID:-}" "${ADMIN_PID:-}" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

if [ "$PYTHON_ENGINE" = true ]; then
    (
        cd services/engine
        "$ROOT/.venv/bin/python" engine_server.py
    ) &
    ENGINE_PID=$!
fi

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

if [ "$ANALYTICS" = true ]; then
    (
        cd services/api
        FACEPROOF_WEB_ADDR="127.0.0.1:5174" FACEPROOF_WEB_DIR="$ROOT/apps/admin" go run ./cmd/devgateway
    ) &
    ADMIN_PID=$!
fi

echo "FaceProof API: http://localhost:8180"
echo "FaceProof demo: http://localhost:5173"
echo "Identity verification: http://localhost:5173/verify.html"
echo "Identity issuer key: $ROOT/.dev/identity-issuer-key"
if [ "$ANALYTICS" = true ]; then
    echo "Analytics panel: http://localhost:5174"
    echo "Admin key: $ROOT/.dev/admin-key"
fi
echo "Development mode: review enrollments are stored as provisional templates."
if [ "$NATIVE_CORE" = true ]; then
    echo "Runtime authority: FaceProof Secure Core C++"
    echo "Python biometric runtime: DISABLED"
else
    echo "Runtime authority: Python development/R&D engine"
fi
if [ "$NATIVE_SHADOW" = true ]; then
    echo "Native biometric shadow: active; Python remains authoritative."
fi
wait
