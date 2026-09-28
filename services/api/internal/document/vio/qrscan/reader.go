package qrscan

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"unicode"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

var (
	ErrEmptyImage     = errors.New("imagem do QR Code vazia")
	ErrInvalidImage   = errors.New("arquivo enviado não é uma imagem PNG, JPEG ou GIF válida")
	ErrQRCodeNotFound = errors.New("não foi possível localizar um QR Code na imagem")
	ErrEmptyQRPayload = errors.New("QR Code encontrado, mas sem payload utilizável")
	ErrInvalidBase64  = errors.New("imagem Base64 inválida")
)

func DecodeImage(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrEmptyImage
	}

	decodedImage, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidImage, err)
	}

	bitmap, err := gozxing.NewBinaryBitmapFromImage(decodedImage)
	if err != nil {
		return nil, fmt.Errorf("falha ao preparar imagem do QR Code: %w", err)
	}

	reader := qrcode.NewQRCodeReader()
	hints := map[gozxing.DecodeHintType]interface{}{
		gozxing.DecodeHintType_TRY_HARDER: true,
		gozxing.DecodeHintType_POSSIBLE_FORMATS: []gozxing.BarcodeFormat{
			gozxing.BarcodeFormat_QR_CODE,
		},
	}

	result, err := reader.Decode(bitmap, hints)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrQRCodeNotFound, err)
	}

	if payload := byteSegments(result); len(payload) > 0 {
		return payload, nil
	}

	if text := result.GetText(); text != "" {
		return []byte(text), nil
	}

	return nil, ErrEmptyQRPayload
}

func DecodeBase64Image(value string) ([]byte, error) {
	decoded, err := decodeBase64Image(value)
	if err != nil {
		return nil, err
	}

	return DecodeImage(decoded)
}

func byteSegments(result *gozxing.Result) []byte {
	metadata := result.GetResultMetadata()
	if len(metadata) == 0 {
		return nil
	}

	value, exists := metadata[gozxing.ResultMetadataType_BYTE_SEGMENTS]
	if !exists {
		return nil
	}

	segments, ok := value.([][]byte)
	if !ok {
		return nil
	}

	totalSize := 0
	for _, segment := range segments {
		totalSize += len(segment)
	}

	payload := make([]byte, 0, totalSize)
	for _, segment := range segments {
		payload = append(payload, segment...)
	}

	return payload
}

func decodeBase64Image(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, ErrEmptyImage
	}

	if comma := strings.IndexByte(value, ','); comma >= 0 {
		prefix := strings.ToLower(value[:comma])
		if strings.HasPrefix(prefix, "data:image/") && strings.Contains(prefix, ";base64") {
			value = value[comma+1:]
		}
	}

	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, value)

	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}

	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(cleaned)
		if err == nil {
			return decoded, nil
		}
	}

	return nil, ErrInvalidBase64
}
