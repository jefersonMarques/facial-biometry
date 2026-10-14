package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	maxImageSize   = 4 << 20
	maxRequestSize = 13 << 20
	maxDimension   = 4096
	defaultPort    = 8765
)

//go:embed index.html
var assets embed.FS

type benchmarkRunner func(context.Context, string, string, string) ([]byte, error)

type benchmarkApp struct {
	token string
	run   benchmarkRunner
	busy  chan struct{}
}

func main() {
	port := flag.Int("port", defaultPort, "local listen port")
	binary := flag.String("binary", "build/low-resolution-benchmark/faceproof_low_resolution_benchmark", "native benchmark executable")
	yunet := flag.String("yunet", "models/yunet/face_detection_yunet_2023mar.onnx", "YuNet model")
	sface := flag.String("sface", "models/sface/face_recognition_sface_2021dec.onnx", "SFace model")
	flag.Parse()

	if *port < 1024 || *port > 65535 {
		log.Fatal("invalid local port")
	}
	for _, path := range []string{*binary, *yunet, *sface} {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			log.Fatalf("required local file unavailable: %s", path)
		}
	}
	token, err := randomToken()
	if err != nil {
		log.Fatal(err)
	}
	app := &benchmarkApp{
		token: token,
		busy:  make(chan struct{}, 1),
		run: func(ctx context.Context, pdf, vio, live string) ([]byte, error) {
			command := exec.CommandContext(ctx, *binary, pdf, vio, live, *yunet, *sface)
			command.WaitDelay = time.Second
			return command.Output()
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", app.index)
	mux.HandleFunc("POST /benchmark", app.benchmark)

	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(*port))
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	log.Printf("Local-only benchmark frontend: http://127.0.0.1:%d", *port)
	log.Print("Files are deleted after each request. Never expose this port externally.")
	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      40 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    8192,
	}
	log.Fatal(server.Serve(listener))
}

func randomToken() (string, error) {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func (app *benchmarkApp) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	nonce, err := randomToken()
	if err != nil {
		http.Error(w, "local page unavailable", 500)
		return
	}
	template, err := assets.ReadFile("index.html")
	if err != nil {
		http.Error(w, "local page unavailable", 500)
		return
	}
	document := strings.ReplaceAll(string(template), "__NONCE__", nonce)
	document = strings.ReplaceAll(document, "__TOKEN__", app.token)
	securityHeaders(w)
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'nonce-"+nonce+"'; style-src 'unsafe-inline'; connect-src 'self'; img-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.WriteString(w, document)
}

func securityHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
}

func (app *benchmarkApp) benchmark(w http.ResponseWriter, r *http.Request) {
	securityHeaders(w)
	w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
	w.Header().Set("Content-Type", "application/json")
	if r.Header.Get("X-FaceProof-Benchmark-Token") != app.token || !sameOrigin(r) {
		http.Error(w, "{\"error\":\"unauthorized local request\"}", http.StatusForbidden)
		return
	}
	select {
	case app.busy <- struct{}{}:
		defer func() { <-app.busy }()
	default:
		http.Error(w, "{\"error\":\"another benchmark is running\"}", http.StatusTooManyRequests)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		http.Error(w, "{\"error\":\"invalid or oversized request\"}", http.StatusRequestEntityTooLarge)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	directory, err := os.MkdirTemp("", "faceproof-private-benchmark-*")
	if err != nil {
		http.Error(w, "{\"error\":\"local temporary directory unavailable\"}", 500)
		return
	}
	defer os.RemoveAll(directory)

	paths := make([]string, 0, 3)
	for _, field := range []string{"pdf", "vio", "live"} {
		path, loadErr := saveImage(r, field, directory)
		if loadErr != nil {
			http.Error(w, "{\"error\":\"three PNG/JPEG portrait files are required (max 4 MiB each)\"}", http.StatusBadRequest)
			return
		}
		paths = append(paths, path)
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	output, err := app.run(ctx, paths[0], paths[1], paths[2])
	if err != nil || len(output) > 128<<10 || len(output) == 0 || !json.Valid(output) {
		http.Error(w, "{\"error\":\"benchmark failed; verify face detection, models and image quality\"}", http.StatusUnprocessableEntity)
		return
	}
	var result struct {
		Mode     string          `json:"mode"`
		Variants json.RawMessage `json:"variants"`
	}
	if json.Unmarshal(output, &result) != nil || result.Mode != "offline_low_resolution_diagnostic" || len(result.Variants) == 0 {
		http.Error(w, "{"error":"unexpected native benchmark response"}", http.StatusBadGateway)
		return
	}
	_, _ = w.Write(output)
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	expected := "http://" + r.Host
	return origin == expected && (strings.HasPrefix(r.Host, "127.0.0.1:") || strings.HasPrefix(r.Host, "localhost:"))
}

func saveImage(r *http.Request, field, directory string) (string, error) {
	file, header, err := r.FormFile(field)
	if err != nil {
		return "", err
	}
	defer file.Close()
	if header.Size > maxImageSize {
		return "", errors.New("file too large")
	}
	// O nome do arquivo fornecido é ignorado; somente PNG/JPEG válido é aceito.
	data, err := io.ReadAll(io.LimitReader(file, maxImageSize+1))
	if err != nil || len(data) == 0 || len(data) > maxImageSize {
		return "", errors.New("invalid image size")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") ||
		cfg.Width < 1 || cfg.Height < 1 ||
		cfg.Width > maxDimension || cfg.Height > maxDimension {
		return "", errors.New("invalid image")
	}
	path := filepath.Join(directory, field+"."+format)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}
