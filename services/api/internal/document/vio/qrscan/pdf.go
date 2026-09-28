package qrscan

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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
	pdfMaxDimension  = "2400"
)

type PDFDecodeResult struct {
	Payload    []byte
	PageImage  []byte
	PageNumber int
}

// DecodePDF renders PDF pages sequentially and returns the first QR payload found.
// pdftoppm (Poppler) is intentionally used as an external renderer so the API does
// not need to load untrusted PDF parsers inside the application process.
func DecodePDF(ctx context.Context, data []byte) ([]byte, error) {
	result, err := DecodePDFDetailed(ctx, data)
	if err != nil {
		return nil, err
	}
	return result.Payload, nil
}

// DecodePDFDetailed returns the QR payload plus the rendered page where it was
// found. The rendered page can be reused by higher layers for visual evidence,
// avoiding a second PDF rendering pass.
func DecodePDFDetailed(ctx context.Context, data []byte) (PDFDecodeResult, error) {
	renderer, err := exec.LookPath("pdftoppm")
	if err != nil {
		return PDFDecodeResult{}, ErrPDFRendererUnavailable
	}
	return decodePDFWithRendererDetailed(ctx, data, renderer)
}

func decodePDFWithRenderer(ctx context.Context, data []byte, renderer string) ([]byte, error) {
	result, err := decodePDFWithRendererDetailed(ctx, data, renderer)
	if err != nil {
		return nil, err
	}
	return result.Payload, nil
}

func decodePDFWithRendererDetailed(ctx context.Context, data []byte, renderer string) (PDFDecodeResult, error) {
	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		return PDFDecodeResult{}, ErrInvalidPDF
	}
	if strings.TrimSpace(renderer) == "" {
		return PDFDecodeResult{}, ErrPDFRendererUnavailable
	}

	directory, err := os.MkdirTemp("", "faceproof-pdf-*")
	if err != nil {
		return PDFDecodeResult{}, ErrInvalidPDF
	}
	defer os.RemoveAll(directory)

	inputPath := filepath.Join(directory, "document.pdf")
	if err := os.WriteFile(inputPath, data, 0600); err != nil {
		return PDFDecodeResult{}, ErrInvalidPDF
	}

	renderCtx, cancel := context.WithTimeout(ctx, pdfRenderTimeout)
	defer cancel()

	outputPrefix := filepath.Join(directory, "page")
	outputPath := outputPrefix + ".png"

	pdfImages, _ := exec.LookPath("pdfimages")

	for pageNumber := 1; pageNumber <= maxPDFPages; pageNumber++ {
		// CNH Digital PDFs normally embed the QR Code as its own raster image.
		// Prefer the original embedded pixels before rendering the whole page:
		// dense VIO QRs can become harder to decode after PDF antialiasing.
		if strings.TrimSpace(pdfImages) != "" {
			payload, found, err := decodeEmbeddedPDFImages(renderCtx, pdfImages, inputPath, directory, pageNumber)
			if err == nil && found {
				page, _, _ := renderPDFPage(renderCtx, renderer, inputPath, outputPrefix, outputPath, pageNumber)
				return PDFDecodeResult{
					Payload:    payload,
					PageImage:  page,
					PageNumber: pageNumber,
				}, nil
			}
			// Extraction is an optimization. Rendering remains the compatibility
			// fallback for flattened/scanned PDFs and unusual generators.
		}

		page, output, err := renderPDFPage(renderCtx, renderer, inputPath, outputPrefix, outputPath, pageNumber)
		if err != nil {
			if renderCtx.Err() != nil {
				return PDFDecodeResult{}, ErrInvalidPDF
			}
			if pageNumber > 1 && isPDFPageOutOfRange(output) {
				break
			}
			return PDFDecodeResult{}, ErrInvalidPDF
		}

		payload, err := DecodeImage(page)
		if err == nil {
			return PDFDecodeResult{
				Payload:    payload,
				PageImage:  append([]byte(nil), page...),
				PageNumber: pageNumber,
			}, nil
		}
	}

	return PDFDecodeResult{}, ErrQRCodeNotFound
}

func decodeEmbeddedPDFImages(
	ctx context.Context,
	pdfImages string,
	inputPath string,
	directory string,
	pageNumber int,
) ([]byte, bool, error) {
	prefix := filepath.Join(directory, fmt.Sprintf("embedded-%03d", pageNumber))
	command := exec.CommandContext(
		ctx,
		pdfImages,
		"-f", strconv.Itoa(pageNumber),
		"-l", strconv.Itoa(pageNumber),
		"-png",
		inputPath,
		prefix,
	)
	output, err := command.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return nil, false, ctx.Err()
		}
		if pageNumber > 1 && isPDFPageOutOfRange(output) {
			return nil, false, nil
		}
		return nil, false, err
	}

	files, err := filepath.Glob(prefix + "-*.png")
	if err != nil {
		return nil, false, err
	}
	sort.Strings(files)
	for _, path := range files {
		imageBytes, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		payload, err := DecodeImage(imageBytes)
		if err == nil {
			return payload, true, nil
		}
	}
	return nil, false, nil
}

func renderPDFPage(
	ctx context.Context,
	renderer string,
	inputPath string,
	outputPrefix string,
	outputPath string,
	pageNumber int,
) ([]byte, []byte, error) {
	_ = os.Remove(outputPath)
	command := exec.CommandContext(
		ctx,
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
		return nil, output, err
	}
	page, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, output, err
	}
	return page, output, nil
}

func isPDFPageOutOfRange(output []byte) bool {
	message := strings.ToLower(string(output))
	return strings.Contains(message, "wrong page range") ||
		strings.Contains(message, "after the last page")
}
