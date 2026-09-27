package decoder

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"time"
)

var ErrInvalidHeader = errors.New("cabeçalho do QR Code inválido")

func parseHeader(data []byte) (Header, error) {
	if header, ok := parseBinaryHeader(data); ok {
		return header, nil
	}

	if header, ok := parseTextHeader(data); ok {
		return header, nil
	}

	return Header{}, ErrInvalidHeader
}

func parseBinaryHeader(data []byte) (Header, bool) {
	if len(data) < 5 {
		return Header{}, false
	}

	timestamp := binary.BigEndian.Uint32(data[:4])
	version := data[4]
	if !isSupportedHeaderVersion(version) || !isPlausibleTimestamp(timestamp) {
		return Header{}, false
	}

	return Header{
		CreatedAt:  time.Unix(int64(timestamp), 0),
		Timestamp:  timestamp,
		Version:    version,
		Body:       data[5:],
		HeaderSize: 5,
		Format:     "binary",
	}, true
}

func parseTextHeader(data []byte) (Header, bool) {
	if len(data) < 10 {
		return Header{}, false
	}

	timestampValue, err := strconv.ParseUint(string(data[:8]), 16, 32)
	if err != nil {
		return Header{}, false
	}

	versionValue, err := strconv.ParseUint(string(data[8:10]), 16, 8)
	if err != nil {
		return Header{}, false
	}

	timestamp := uint32(timestampValue)
	version := uint8(versionValue)
	if !isSupportedHeaderVersion(version) || !isPlausibleTimestamp(timestamp) {
		return Header{}, false
	}

	return Header{
		CreatedAt:  time.Unix(int64(timestamp), 0),
		Timestamp:  timestamp,
		Version:    version,
		Body:       data[10:],
		HeaderSize: 10,
		Format:     "hex-text",
	}, true
}

func isSupportedHeaderVersion(version uint8) bool {
	return version >= 1 && version <= 6
}

func isPlausibleTimestamp(timestamp uint32) bool {
	createdAt := time.Unix(int64(timestamp), 0).UTC()
	min := time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC)
	max := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
	return !createdAt.Before(min) && createdAt.Before(max)
}

func headerSignedPrefix(header Header) []byte {
	prefix := make([]byte, 5)
	binary.BigEndian.PutUint32(prefix[:4], header.Timestamp)
	prefix[4] = header.Version
	return prefix
}

func headerTextSignedPrefix(header Header) []byte {
	return []byte(fmt.Sprintf("%08x%02x", header.Timestamp, header.Version))
}

func unsupportedVersionError(version uint8) error {
	return fmt.Errorf("versão QRCode%d não suportada", version)
}
