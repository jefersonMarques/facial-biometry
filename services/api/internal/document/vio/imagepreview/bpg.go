package imagepreview

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultTimeout      = 5 * time.Second
	maxDecodedImageSize = 4 << 20
)

var ErrDecoderUnavailable = errors.New("BPG decoder unavailable")

type BPGDecoder struct {
	path    string
	timeout time.Duration
}

func NewBPGDecoder(path string) *BPGDecoder {
	return &BPGDecoder{
		path:    strings.TrimSpace(path),
		timeout: defaultTimeout,
	}
}

func (decoder *BPGDecoder) Available() bool {
	_, err := decoder.executablePath()
	return err == nil
}

func (decoder *BPGDecoder) Decode(input []byte) ([]byte, error) {
	if len(input) == 0 {
		return nil, errors.New("empty BPG image")
	}

	executable, err := decoder.executablePath()
	if err != nil {
		return nil, err
	}

	tempDir, err := os.MkdirTemp("", "vio-bpg-*")
	if err != nil {
		return nil, fmt.Errorf("create BPG temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	inputPath := filepath.Join(tempDir, "input.bpg")
	outputPath := filepath.Join(tempDir, "output.png")

	if err := os.WriteFile(inputPath, input, 0o600); err != nil {
		return nil, fmt.Errorf("write BPG temporary file: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), decoder.timeout)
	defer cancel()

	command := exec.CommandContext(ctx, executable, "-b", "8", "-o", outputPath, inputPath)
	command.Stdout = io.Discard
	var stderr bytes.Buffer
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, errors.New("BPG decoder timeout")
		}
		return nil, fmt.Errorf("BPG decode failed: %s", sanitizeCommandError(stderr.String()))
	}

	file, err := os.Open(outputPath)
	if err != nil {
		return nil, fmt.Errorf("open decoded PNG: %w", err)
	}
	defer file.Close()

	pngBytes, err := io.ReadAll(io.LimitReader(file, maxDecodedImageSize+1))
	if err != nil {
		return nil, fmt.Errorf("read decoded PNG: %w", err)
	}
	if len(pngBytes) > maxDecodedImageSize {
		return nil, errors.New("decoded PNG exceeds size limit")
	}
	if len(pngBytes) < 8 || !bytes.Equal(pngBytes[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}) {
		return nil, errors.New("BPG decoder returned an invalid PNG")
	}

	return pngBytes, nil
}

func (decoder *BPGDecoder) executablePath() (string, error) {
	if decoder.path != "" {
		return resolveExecutableCandidate(decoder.path)
	}

	for _, candidate := range localDecoderCandidates() {
		if path, err := resolveExecutableCandidate(candidate); err == nil {
			return path, nil
		}
	}

	path, err := exec.LookPath("bpgdec")
	if err != nil {
		return "", ErrDecoderUnavailable
	}
	return path, nil
}

func (decoder *BPGDecoder) ResolvedPath() string {
	path, err := decoder.executablePath()
	if err != nil {
		return ""
	}
	return path
}

func resolveExecutableCandidate(candidate string) (string, error) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return "", ErrDecoderUnavailable
	}

	paths := []string{candidate}
	if !filepath.IsAbs(candidate) {
		if cwd, err := os.Getwd(); err == nil {
			paths = append(paths, filepath.Join(cwd, candidate))
		}
		if executablePath, err := os.Executable(); err == nil {
			paths = append(paths, filepath.Join(filepath.Dir(executablePath), candidate))
		}
	}

	for _, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		info, err := os.Stat(absPath)
		if err == nil && !info.IsDir() {
			return absPath, nil
		}
	}

	return "", ErrDecoderUnavailable
}

func localDecoderCandidates() []string {
	if runtime.GOOS == "windows" {
		return []string{
			filepath.Join("tools", "bpg", "bpgdec.exe"),
			"bpgdec.exe",
		}
	}

	return []string{
		filepath.Join("tools", "bpg", "bpgdec"),
		"bpgdec",
	}
}

func sanitizeCommandError(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "decoder process returned an error"
	}
	if len(value) > 240 {
		return value[:240]
	}
	return value
}

type BPGInfo struct {
	Width       int
	Height      int
	BitDepth    int
	PixelFormat string
	ColorSpace  string
	HasAlpha    bool
	Animation   bool
}

func InspectBPG(input []byte) (BPGInfo, error) {
	if len(input) < 8 {
		return BPGInfo{}, errors.New("invalid BPG header")
	}
	if !bytes.Equal(input[:4], []byte{'B', 'P', 'G', 0xFB}) {
		return BPGInfo{}, errors.New("invalid BPG magic")
	}

	headerInfo1 := input[4]
	headerInfo2 := input[5]
	offset := 6

	width, nextOffset, err := readUE7(input, offset)
	if err != nil {
		return BPGInfo{}, fmt.Errorf("read BPG width: %w", err)
	}
	height, _, err := readUE7(input, nextOffset)
	if err != nil {
		return BPGInfo{}, fmt.Errorf("read BPG height: %w", err)
	}
	if width == 0 || height == 0 {
		return BPGInfo{}, errors.New("invalid BPG dimensions")
	}

	pixelFormatCode := int((headerInfo1 >> 5) & 0x07)
	alpha1 := ((headerInfo1 >> 4) & 0x01) == 1
	alpha2 := ((headerInfo2 >> 2) & 0x01) == 1
	colorSpaceCode := int((headerInfo2 >> 4) & 0x0F)

	return BPGInfo{
		Width:       int(width),
		Height:      int(height),
		BitDepth:    int(headerInfo1&0x0F) + 8,
		PixelFormat: bpgPixelFormatName(pixelFormatCode),
		ColorSpace:  bpgColorSpaceName(colorSpaceCode),
		HasAlpha:    alpha1 || alpha2,
		Animation:   (headerInfo2 & 0x01) == 1,
	}, nil
}

func readUE7(input []byte, offset int) (uint32, int, error) {
	if offset < 0 || offset >= len(input) {
		return 0, offset, io.ErrUnexpectedEOF
	}

	var value uint32
	for index := offset; index < len(input); index++ {
		current := input[index]
		if value > (1<<25)-1 {
			return 0, index, errors.New("ue7 value overflow")
		}
		value = (value << 7) | uint32(current&0x7F)
		if current&0x80 == 0 {
			return value, index + 1, nil
		}
	}

	return 0, len(input), io.ErrUnexpectedEOF
}

func bpgPixelFormatName(value int) string {
	switch value {
	case 0:
		return "Grayscale"
	case 1:
		return "4:2:0 JPEG"
	case 2:
		return "4:2:2 JPEG"
	case 3:
		return "4:4:4"
	case 4:
		return "4:2:0 MPEG-2"
	case 5:
		return "4:2:2 MPEG-2"
	default:
		return fmt.Sprintf("Reserved (%d)", value)
	}
}

func bpgColorSpaceName(value int) string {
	switch value {
	case 0:
		return "YCbCr BT.601"
	case 1:
		return "RGB"
	case 2:
		return "YCgCo"
	case 3:
		return "YCbCr BT.709"
	case 4:
		return "YCbCr BT.2020"
	default:
		return fmt.Sprintf("Reserved (%d)", value)
	}
}
