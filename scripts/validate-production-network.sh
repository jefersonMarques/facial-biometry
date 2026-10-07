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

LISTENERS="$(ss -ltnH)"

check_loopback_port() {
    local port="$1"
    local matches
    matches="$(printf '%s
' "$LISTENERS" | awk -v suffix=":$port" '$4 ~ suffix "$" {print $4}')"

    [ -n "$matches" ] || fail "port $port is not listening"

    while IFS= read -r address; do
        case "$address" in
            127.0.0.1:"$port"|[::1]:"$port")
                ;;
            *)
                fail "port $port is exposed outside loopback: $address"
                ;;
        esac
    done <<< "$matches"

    ok "port $port is loopback-only"
}

check_loopback_port 8180
check_loopback_port 5173

if printf '%s\n' "$LISTENERS" | awk '$4 ~ /:8090$/ {found=1} END {exit !found}'; then
    fail "port 8090 is active; Python biometric runtime must be absent in production"
fi
ok "Python biometric port 8090 is absent"

if printf '%s
' "$LISTENERS" | awk '$4 ~ /:5174$/ {found=1} END {exit !found}'; then
    check_loopback_port 5174
else
    ok "admin port 5174 is not running"
fi

curl --fail --silent --show-error     http://127.0.0.1:8180/health >/dev/null
ok "API health is reachable only through loopback"

curl --fail --silent --show-error     http://127.0.0.1:5173/health >/dev/null
ok "web gateway proxies health to the API"

if [ -n "${FACEPROOF_PUBLIC_URL:-}" ]; then
    case "$FACEPROOF_PUBLIC_URL" in
        https://*) ;;
        *) fail "FACEPROOF_PUBLIC_URL must use https://";;
    esac

    curl --fail --silent --show-error         "${FACEPROOF_PUBLIC_URL%/}/health" >/dev/null
    ok "public HTTPS health endpoint is reachable"
fi

echo "FaceProof production network validation passed."
