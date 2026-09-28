package decoder

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"unicode"
)

var ErrEmptyInput = errors.New("QR Code vazio")

type normalizedInput struct {
	Bytes    []byte
	Encoding string
}

func normalizeInput(value string) (normalizedInput, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return normalizedInput{}, ErrEmptyInput
	}

	if payload, ok := strings.CutPrefix(value, "hex:"); ok {
		decoded, err := decodeHex(payload)
		if err != nil {
			return normalizedInput{}, err
		}
		return normalizedInput{Bytes: decoded, Encoding: "hex"}, nil
	}

	if payload, ok := strings.CutPrefix(value, "base64:"); ok {
		decoded, err := decodeBase64(payload)
		if err != nil {
			return normalizedInput{}, err
		}
		return normalizedInput{Bytes: decoded, Encoding: "base64"}, nil
	}

	if decoded, err := decodeHex(value); err == nil && hasPlausibleHeader(decoded) {
		return normalizedInput{Bytes: decoded, Encoding: "hex"}, nil
	}

	if decoded, err := decodeBase64(value); err == nil && hasPlausibleHeader(decoded) {
		return normalizedInput{Bytes: decoded, Encoding: "base64"}, nil
	}

	return normalizedInput{Bytes: []byte(value), Encoding: "raw"}, nil
}

func decodeHex(value string) ([]byte, error) {
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || r == ':' || r == '-' {
			return -1
		}
		return r
	}, strings.TrimSpace(value))

	cleaned = strings.TrimPrefix(cleaned, "0x")
	if len(cleaned) < 10 || len(cleaned)%2 != 0 {
		return nil, errors.New("conteúdo hexadecimal inválido")
	}

	return hex.DecodeString(cleaned)
}

func decodeBase64(value string) ([]byte, error) {
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, strings.TrimSpace(value))

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

	return nil, errors.New("conteúdo Base64 inválido")
}

func hasPlausibleHeader(data []byte) bool {
	_, err := parseHeader(data)
	return err == nil
}
