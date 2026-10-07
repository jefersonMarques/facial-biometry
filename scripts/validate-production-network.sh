#!/usr/bin/env bash
set -euo pipefail

fail() {
    echo "[FAIL] $*" >&2
    exit 1
}

ok() {
    echo "[ok] $*"
}

command -v ss >/dev/null 2>&1 || fail "ss is required"
command -v curl >/dev/null 2>&1 || fail "curl is required"

wait_for_http() {
    local url="$1"
    local label="$2"
    local max_attempts="${3:-60}"

    for ((attempt = 1; attempt <= max_attempts; attempt++)); do
        if curl --fail --silent --show-error \
            --connect-timeout 1 \
            --max-time 2 \
            "$url" >/dev/null 2>&1; then
            ok "$label is ready"
            return 0
        fi
        sleep 0.5
    done

    fail "$label did not become ready: $url"
}

current_listeners() {
    ss -ltnH
}

port_matches() {
    local port="$1"
    current_listeners |
        awk -v suffix=":$port" '$4 ~ suffix "$" {print $4}'
}

check_loopback_port() {
    local port="$1"
    local matches
    matches="$(port_matches "$port")"

    [ -n "$matches" ] || fail "port $port is not listening"

    while IFS= read -r address; do
        case "$address" in
            127.0.0.1:"$port"|"[::1]":"$port")
                ;;
            *)
                fail "port $port is exposed outside loopback: $address"
                ;;
        esac
    done <<< "$matches"

    ok "port $port is loopback-only"
}

port_is_listening() {
    local port="$1"
    [ -n "$(port_matches "$port")" ]
}

# The API loads/verifies the Model Pack and initializes the native Secure Core
# before ListenAndServe. Wait for readiness so validation cannot race startup.
wait_for_http     "http://127.0.0.1:8180/health"     "API"

wait_for_http     "http://127.0.0.1:5173/health"     "web gateway"

check_loopback_port 8180
check_loopback_port 5173

if port_is_listening 8090; then
    fail "port 8090 is active; Python biometric runtime must be absent in production"
fi
ok "Python biometric port 8090 is absent"

if port_is_listening 5174; then
    check_loopback_port 5174
else
    ok "admin port 5174 is not running"
fi

ok "API health is reachable through loopback"
ok "web gateway proxies health to the API"

if [ -n "${FACEPROOF_PUBLIC_URL:-}" ]; then
    case "$FACEPROOF_PUBLIC_URL" in
        https://*) ;;
        *) fail "FACEPROOF_PUBLIC_URL must use https://";;
    esac

    wait_for_http         "${FACEPROOF_PUBLIC_URL%/}/health"         "public HTTPS endpoint"
fi

echo "FaceProof production network validation passed."
