package main

import (
	"net/http/httptest"
	"net/url"
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

func TestProductionGatewayRequiresLoopbackListenAddress(t *testing.T) {
	t.Parallel()

	apiURL, err := url.Parse("http://127.0.0.1:8180")
	if err != nil {
		t.Fatal(err)
	}

	if err := validateProductionGateway(
		"production",
		":5173",
		apiURL,
	); err == nil {
		t.Fatal("production gateway must reject wildcard listen address")
	}

	if err := validateProductionGateway(
		"production",
		"127.0.0.1:5173",
		apiURL,
	); err != nil {
		t.Fatalf("loopback production gateway rejected: %v", err)
	}
}

func TestProductionGatewayRequiresLoopbackAPIUpstream(t *testing.T) {
	t.Parallel()

	apiURL, err := url.Parse("http://10.0.0.5:8180")
	if err != nil {
		t.Fatal(err)
	}

	if err := validateProductionGateway(
		"production",
		"127.0.0.1:5173",
		apiURL,
	); err == nil {
		t.Fatal("production gateway must reject non-loopback API upstream")
	}
}

func TestDevelopmentGatewayKeepsCurrentFlexibility(t *testing.T) {
	t.Parallel()

	apiURL, err := url.Parse("http://10.0.0.5:8180")
	if err != nil {
		t.Fatal(err)
	}

	if err := validateProductionGateway(
		"development",
		":5173",
		apiURL,
	); err != nil {
		t.Fatalf("development gateway unexpectedly rejected: %v", err)
	}
}
