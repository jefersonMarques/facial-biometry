package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	environment := strings.ToLower(strings.TrimSpace(
		envString("FACEPROOF_ENV", "development"),
	))
	listenAddress := envString("FACEPROOF_WEB_ADDR", ":5173")
	apiURL := envString("FACEPROOF_DEV_API_URL", "http://127.0.0.1:8180")
	webDirectory := envString("FACEPROOF_WEB_DIR", "../../apps/web-sdk")
	gatewayRole := strings.ToLower(strings.TrimSpace(
		envString("FACEPROOF_GATEWAY_ROLE", "public"),
	))
	if gatewayRole != "public" && gatewayRole != "admin" {
		log.Fatalf("invalid FACEPROOF_GATEWAY_ROLE %q", gatewayRole)
	}

	absoluteWebDirectory, err := filepath.Abs(webDirectory)
	if err != nil {
		log.Fatal(err)
	}
	if info, err := os.Stat(absoluteWebDirectory); err != nil || !info.IsDir() {
		log.Fatalf("web directory unavailable: %s", absoluteWebDirectory)
	}

	target, err := url.Parse(apiURL)
	if err != nil {
		log.Fatal(err)
	}
	if err := validateProductionGateway(
		environment,
		listenAddress,
		target,
	); err != nil {
		log.Fatal(err)
	}
	apiProxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := apiProxy.Director
	apiProxy.Director = func(request *http.Request) {
		originalHost := request.Host
		originalDirector(request)
		request.Host = target.Host
		request.Header.Set("X-Forwarded-Host", originalHost)
	}
	apiProxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, proxyErr error) {
		log.Printf("API proxy error for %s: %v", request.URL.Path, proxyErr)
		http.Error(writer, "FaceProof API unavailable", http.StatusBadGateway)
	}

	staticHandler := http.FileServer(http.Dir(absoluteWebDirectory))
	handler := newGatewayHandler(
		gatewayRole,
		apiProxy,
		staticHandler,
	)

	server := &http.Server{
		Addr:              listenAddress,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       115 * time.Second,
		WriteTimeout:      115 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	log.Printf(
		"FaceProof %s gateway listening on %s",
		gatewayRole,
		listenAddress,
	)
	log.Printf("Static files: %s", absoluteWebDirectory)
	log.Printf("API proxy: /v1/* -> %s", target)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func newGatewayHandler(
	role string,
	apiProxy http.Handler,
	staticHandler http.Handler,
) http.Handler {
	return http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		setSecurityHeaders(writer)

		isAdminAPI := request.URL.Path == "/v1/admin" ||
			strings.HasPrefix(request.URL.Path, "/v1/admin/")
		if role != "admin" && isAdminAPI {
			http.NotFound(writer, request)
			return
		}

		if request.URL.Path == "/health" ||
			request.URL.Path == "/v1" ||
			strings.HasPrefix(request.URL.Path, "/v1/") {
			apiProxy.ServeHTTP(writer, request)
			return
		}

		writer.Header().Set("Cache-Control", "no-store")
		staticHandler.ServeHTTP(writer, request)
	})
}

func validateProductionGateway(
	environment string,
	listenAddress string,
	apiURL *url.URL,
) error {
	if environment != "production" {
		return nil
	}
	if !isLoopbackGatewayAddress(listenAddress) {
		return fmt.Errorf(
			"FACEPROOF_WEB_ADDR must bind to loopback in production, got %q",
			listenAddress,
		)
	}
	if apiURL == nil ||
		!strings.EqualFold(apiURL.Scheme, "http") ||
		apiURL.Hostname() == "" ||
		!isLoopbackHost(apiURL.Hostname()) {
		return fmt.Errorf(
			"FACEPROOF_DEV_API_URL must use an HTTP loopback upstream in production",
		)
	}
	return nil
}

func isLoopbackGatewayAddress(address string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil {
		return false
	}
	return isLoopbackHost(strings.Trim(host, "[]"))
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(strings.TrimSpace(host), "localhost") {
		return true
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	return ip != nil && ip.IsLoopback()
}

func setSecurityHeaders(writer http.ResponseWriter) {
	writer.Header().Set("Referrer-Policy", "no-referrer")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("X-Frame-Options", "DENY")
	writer.Header().Set("Permissions-Policy", "camera=(self), microphone=()")
	writer.Header().Set(
		"Content-Security-Policy",
		"default-src 'self'; "+
			"script-src 'self' 'wasm-unsafe-eval'; "+
			"connect-src 'self'; "+
			"img-src 'self' data: blob:; "+
			"media-src 'self' blob:; "+
			"worker-src 'self' blob:; "+
			"style-src 'self' 'unsafe-inline'; "+
			"font-src 'self'; "+
			"object-src 'none'; "+
			"base-uri 'none'; "+
			"frame-ancestors 'none'; "+
			"form-action 'self'",
	)
}

func envString(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}
