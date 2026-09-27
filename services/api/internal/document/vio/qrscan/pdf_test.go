package qrscan

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const pdfTestQRPNG = "iVBORw0KGgoAAAANSUhEUgAAASIAAAEiAQAAAAB1xeIbAAABjklEQVR4nO2ZTW6EMAxGnwtSlyDNAeYo4WY9U28AR+kNyLJSqq+LBDrVqGoX5W9wFgjMk/LJcYxjTPw+hqc/QOCUU0455dTeKSujBmKNdXGydJvqOgUVJEkjmF3fDagkSfpOra/rFFQsMa5+elO2wca6Hpmq7yyxRsTlZnTqJ8pexkrWrTjjeakp7hsBETRcU7bcHrr2qv7YVPH9YABUWHirE/Bh2+o6A5V9/xXjGlqsbIMtdZ2Hsi5X9TXQpFzsWPcV/PtWf1SKXMdrnO8kqW8SBKVS+Pd7VX9sinKEaiTCWImQT1SV1FMe3ffLULPvEznas9ubsgDu+wWpkml6oCSeHPyl0HTfL0jdZnlJIozMO2Bemb2qPzY15Zw8KklKJfgBj/sVqLmPydCC2TUhzYln7+oPSk09hdhCeAWDGhHNewqrUzcpaGjnlvL2us5AmVmNdY1kXXyeOprb63pE6q6PSbwg4iUReu8prEENZmbWls+sddla+T/DJam7Pma5pPluG11OOeWUU079L/UJaXjl3vPHDTcAAAAASUVORK5CYII="
const pdfTestBlankPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAusB9Y9ZxZsAAAAASUVORK5CYII="

func TestDecodePDFWithRendererStopsAfterFindingQR(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("renderer fixture uses a POSIX shell script")
	}
	rendererPath, callLog := preparePDFRenderer(t, "3")

	payload, err := decodePDFWithRenderer(context.Background(), []byte("%PDF-1.4\n% test"), rendererPath)
	if err != nil {
		t.Fatalf("decode PDF: %v", err)
	}
	if string(payload) != "PDF-VIO-TEST" {
		t.Fatalf("unexpected payload %q", payload)
	}

	calls := readRendererCalls(t, callLog)
	if len(calls) != 3 {
		t.Fatalf("expected renderer to stop after page 3, got calls %v", calls)
	}
}

func TestDecodePDFWithRendererScansTwentyPages(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("renderer fixture uses a POSIX shell script")
	}
	rendererPath, callLog := preparePDFRenderer(t, "20")

	payload, err := decodePDFWithRenderer(context.Background(), []byte("%PDF-1.4\n% test"), rendererPath)
	if err != nil {
		t.Fatalf("decode PDF: %v", err)
	}
	if string(payload) != "PDF-VIO-TEST" {
		t.Fatalf("unexpected payload %q", payload)
	}

	calls := readRendererCalls(t, callLog)
	if len(calls) != maxPDFPages || calls[len(calls)-1] != "20" {
		t.Fatalf("expected pages 1..20, got calls %v", calls)
	}
}

func TestDecodePDFWithRendererRejectsInvalidPDF(t *testing.T) {
	_, err := decodePDFWithRenderer(context.Background(), []byte("not-a-pdf"), "/bin/true")
	if !errors.Is(err, ErrInvalidPDF) {
		t.Fatalf("expected ErrInvalidPDF, got %v", err)
	}
}

func preparePDFRenderer(t *testing.T, qrPage string) (string, string) {
	t.Helper()

	directory := t.TempDir()
	qrPath := filepath.Join(directory, "qr.png")
	blankPath := filepath.Join(directory, "blank.png")
	callLog := filepath.Join(directory, "calls.log")

	writeBase64Fixture(t, qrPath, pdfTestQRPNG)
	writeBase64Fixture(t, blankPath, pdfTestBlankPNG)

	rendererPath := filepath.Join(directory, "pdftoppm-test")
	renderer := `#!/bin/sh
page=""
previous=""
last=""
for arg in "$@"; do
  if [ "$previous" = "-f" ]; then
    page="$arg"
  fi
  previous="$arg"
  last="$arg"
done
printf '%s\n' "$page" >> "$PDF_CALL_LOG"
if [ "$page" = "$PDF_QR_PAGE" ]; then
  cp "$PDF_QR_FIXTURE" "${last}.png"
else
  cp "$PDF_BLANK_FIXTURE" "${last}.png"
fi
`
	if err := os.WriteFile(rendererPath, []byte(renderer), 0700); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PDF_QR_FIXTURE", qrPath)
	t.Setenv("PDF_BLANK_FIXTURE", blankPath)
	t.Setenv("PDF_QR_PAGE", qrPage)
	t.Setenv("PDF_CALL_LOG", callLog)

	return rendererPath, callLog
}

func writeBase64Fixture(t *testing.T, path, encoded string) {
	t.Helper()

	fixture, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, fixture, 0600); err != nil {
		t.Fatal(err)
	}
}

func readRendererCalls(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(string(data))
}
