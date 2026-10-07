package main

import (
	"net/http"
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

func TestPublicGatewayBlocksAdminAPI(t *testing.T) {
	t.Parallel()

	proxyCalled := false
	proxy := http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		proxyCalled = true
		writer.WriteHeader(http.StatusOK)
	})
	static := http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		writer.WriteHeader(http.StatusOK)
	})

	handler := newGatewayHandler("public", proxy, static)
	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/admin/summary",
		nil,
	)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", recorder.Code)
	}
	if proxyCalled {
		t.Fatal("public gateway must not proxy admin API")
	}
}

func TestAdminGatewayProxiesAdminAPI(t *testing.T) {
	t.Parallel()

	proxyCalled := false
	proxy := http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		proxyCalled = true
		writer.WriteHeader(http.StatusUnauthorized)
	})
	static := http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		writer.WriteHeader(http.StatusOK)
	})

	handler := newGatewayHandler("admin", proxy, static)
	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/admin/summary",
		nil,
	)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
	if !proxyCalled {
		t.Fatal("admin gateway must proxy admin API")
	}
}
