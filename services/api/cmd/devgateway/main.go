package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	listenAddress := envString("FACEPROOF_WEB_ADDR", ":5173")
	apiURL := envString("FACEPROOF_DEV_API_URL", "http://127.0.0.1:8080")
	webDirectory := envString("FACEPROOF_WEB_DIR", "../../apps/web-sdk")

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
	apiProxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := apiProxy.Director
	apiProxy.Director = func(request *http.Request) {
		originalDirector(request)
		request.Host = target.Host
		request.Header.Set("X-Forwarded-Host", request.Host)
	}
	apiProxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, proxyErr error) {
		log.Printf("API proxy error for %s: %v", request.URL.Path, proxyErr)
		http.Error(writer, "FaceProof API unavailable", http.StatusBadGateway)
	}

	staticHandler := http.FileServer(http.Dir(absoluteWebDirectory))
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		setSecurityHeaders(writer)

		if request.URL.Path == "/v1" || strings.HasPrefix(request.URL.Path, "/v1/") {
			apiProxy.ServeHTTP(writer, request)
			return
		}

		writer.Header().Set("Cache-Control", "no-store")
		staticHandler.ServeHTTP(writer, request)
	})

	server := &http.Server{
		Addr:              listenAddress,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       40 * time.Second,
		WriteTimeout:      40 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("FaceProof web gateway listening on %s", listenAddress)
	log.Printf("Static files: %s", absoluteWebDirectory)
	log.Printf("API proxy: /v1/* -> %s", target)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func setSecurityHeaders(writer http.ResponseWriter) {
	writer.Header().Set("Referrer-Policy", "no-referrer")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("X-Frame-Options", "DENY")
	writer.Header().Set("Permissions-Policy", "camera=(self), microphone=()")
}

func envString(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}
