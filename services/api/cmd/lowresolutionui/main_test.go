package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func sampleImage(t *testing.T) []byte {
	t.Helper()
	picture := image.NewRGBA(image.Rect(0, 0, 45, 61))
	picture.Set(10, 10, color.RGBA{R: 50, G: 70, B: 100, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, picture); err != nil { t.Fatal(err) }
	return b.Bytes()
}

func multipartRequest(t *testing.T, content []byte) (*http.Request, error) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, name := range []string{"pdf", "vio", "live"} {
		file, err := writer.CreateFormFile(name, "source.png")
		if err != nil { return nil, err }
		if _, err := file.Write(content); err != nil { return nil, err }
	}
	if err := writer.Close(); err != nil { return nil, err }
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8765/benchmark", &body)
	request.Header.Set("Origin", "http://127.0.0.1:8765")
	request.Header.Set("X-FaceProof-Benchmark-Token", "test-token")
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request, nil
}

func TestBenchmarkDeletesPrivateImages(t *testing.T) {
	var temporary []string
	app := benchmarkApp{
		token: "test-token",
		busy: make(chan struct{}, 1),
		run: func(_ context.Context, pdf, vio, live string) ([]byte, error) {
			temporary = []string{pdf, vio, live}
			for _, path := range temporary {
				if _, err := os.Stat(path); err != nil { t.Fatalf("image unavailable during benchmark: %v", err) }
			}
			return []byte("{\"mode\":\"offline_low_resolution_diagnostic\",\"variants\":[{\"method\":\"production_cubic\"}]}"), nil
		},
	}
	request, err := multipartRequest(t, sampleImage(t))
	if err != nil { t.Fatal(err) }
	response := httptest.NewRecorder()
	app.benchmark(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d, message=%s", response.Code, response.Body.String())
	}
	var report map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &report); err != nil { t.Fatal(err) }
	if report["mode"] != "offline_low_resolution_diagnostic" { t.Fatalf("unexpected report: %v", report) }
	for _, path := range temporary {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("image retained after analysis: %s", path)
		}
	}
	if response.Header().Get("Cache-Control") != "no-store" { t.Fatal("sensitive metrics may be cached") }
}

func TestBenchmarkRejectsForeignOriginAndMissingToken(t *testing.T) {
	app := benchmarkApp{token: "test-token", busy: make(chan struct{}, 1)}
	for _, tc := range []struct { token, origin string }{
		{"", "http://127.0.0.1:8765"},
		{"test-token", "http://foreign.invalid"},
		{"test-token", ""},
	} {
		request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8765/benchmark", strings.NewReader("invalid"))
		request.Header.Set("Origin", tc.origin)
		request.Header.Set("X-FaceProof-Benchmark-Token", tc.token)
		response := httptest.NewRecorder()
		app.benchmark(response, request)
		if response.Code != http.StatusForbidden { t.Fatalf("status=%d, expected 403", response.Code) }
	}
}

func TestBenchmarkRejectsDocumentUpload(t *testing.T) {
	app := benchmarkApp{token: "test-token", busy: make(chan struct{}, 1)}
	request, err := multipartRequest(t, []byte("%PDF-1.7"))
	if err != nil { t.Fatal(err) }
	response := httptest.NewRecorder()
	app.benchmark(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, expected 400", response.Code)
	}
}
