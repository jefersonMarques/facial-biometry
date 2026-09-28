package imagepreview

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBPGDecoderUnavailable(t *testing.T) {
	decoder := NewBPGDecoder(filepath.Join(t.TempDir(), "missing-bpgdec"))
	if decoder.Available() {
		t.Fatal("expected decoder to be unavailable")
	}
}

func TestBPGDecoderUsesConfiguredExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX shell script")
	}

	tempDir := t.TempDir()
	executablePath := filepath.Join(tempDir, "bpgdec")
	pngBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Wl2bQAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatal(err)
	}
	pngPath := filepath.Join(tempDir, "fixture.png")
	if err := os.WriteFile(pngPath, pngBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	script := "#!/bin/sh\ncp \"" + pngPath + "\" \"$4\"\n"
	if err := os.WriteFile(executablePath, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}

	decoder := NewBPGDecoder(executablePath)
	decoded, err := decoder.Decode([]byte("BPG fixture"))
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(decoded) == 0 {
		t.Fatal("expected PNG output")
	}
}
