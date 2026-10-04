package main

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeadersBlockExternalRuntimeConnections(t *testing.T) {
	recorder := httptest.NewRecorder()

	setSecurityHeaders(recorder)

	csp := recorder.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "connect-src 'self'") {
		t.Fatalf("CSP must restrict runtime connections to self, got %q", csp)
	}
	if !strings.Contains(csp, "script-src 'self' 'wasm-unsafe-eval'") {
		t.Fatalf("CSP must allow only same-origin scripts plus WASM compilation, got %q", csp)
	}
	if strings.Contains(csp, "https:") || strings.Contains(csp, "*") {
		t.Fatalf("CSP must not whitelist arbitrary external origins, got %q", csp)
	}
	if got := recorder.Header().Get("Permissions-Policy"); got != "camera=(self), microphone=()" {
		t.Fatalf("unexpected Permissions-Policy: %q", got)
	}
}
