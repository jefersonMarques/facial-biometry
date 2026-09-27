package qrscan

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalidPDF             = errors.New("PDF inválido")
	ErrPDFRendererUnavailable = errors.New("renderizador PDF indisponível")
)

const (
	maxPDFPages      = 20
	pdfRenderTimeout = 45 * time.Second
	pdfMaxDimension  = "3200"
)

// DecodePDF renders PDF pages sequentially and returns the first QR payload found.
// pdftoppm (Poppler) is intentionally used as an external renderer so the API does
// not need to load untrusted PDF parsers inside the application process.
func DecodePDF(ctx context.Context, data []byte) ([]byte, error) {
	renderer, err := exec.LookPath("pdftoppm")
	if err != nil {
		return nil, ErrPDFRendererUnavailable
	}
	return decodePDFWithRenderer(ctx, data, renderer)
}

func decodePDFWithRenderer(ctx context.Context, data []byte, renderer string) ([]byte, error) {
	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		return nil, ErrInvalidPDF
	}
	if strings.TrimSpace(renderer) == "" {
		return nil, ErrPDFRendererUnavailable
	}

	directory, err := os.MkdirTemp("", "rododata-pdf-*")
	if err != nil {
		return nil, ErrInvalidPDF
	}
	defer os.RemoveAll(directory)

	inputPath := filepath.Join(directory, "document.pdf")
	if err := os.WriteFile(inputPath, data, 0600); err != nil {
		return nil, ErrInvalidPDF
	}

	renderCtx, cancel := context.WithTimeout(ctx, pdfRenderTimeout)
	defer cancel()

	outputPrefix := filepath.Join(directory, "page")
	outputPath := outputPrefix + ".png"

	for pageNumber := 1; pageNumber <= maxPDFPages; pageNumber++ {
		_ = os.Remove(outputPath)

		command := exec.CommandContext(
			renderCtx,
			renderer,
			"-f", strconv.Itoa(pageNumber),
			"-l", strconv.Itoa(pageNumber),
			"-singlefile",
			"-scale-to", pdfMaxDimension,
			"-png",
			inputPath,
			outputPrefix,
		)
		output, err := command.CombinedOutput()
		if err != nil {
			if renderCtx.Err() != nil {
				return nil, ErrInvalidPDF
			}
			if pageNumber > 1 && isPDFPageOutOfRange(output) {
				break
			}
			return nil, ErrInvalidPDF
		}

		page, err := os.ReadFile(outputPath)
		if err != nil {
			return nil, ErrInvalidPDF
		}

		payload, err := DecodeImage(page)
		if err == nil {
			return payload, nil
		}
	}

	return nil, ErrQRCodeNotFound
}

func isPDFPageOutOfRange(output []byte) bool {
	message := strings.ToLower(string(output))
	return strings.Contains(message, "wrong page range") ||
		strings.Contains(message, "after the last page")
}
