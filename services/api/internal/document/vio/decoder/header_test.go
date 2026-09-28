package decoder

import (
	"encoding/binary"
	"testing"
	"time"
)

func TestParseBinaryHeader(t *testing.T) {
	timestamp := uint32(time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC).Unix())
	input := make([]byte, 6)
	binary.BigEndian.PutUint32(input[:4], timestamp)
	input[4] = 6
	input[5] = 0xAA

	header, err := parseHeader(input)
	if err != nil {
		t.Fatalf("parseHeader() error = %v", err)
	}

	if header.Version != 6 {
		t.Fatalf("Version = %d, want 6", header.Version)
	}
	if len(header.Body) != 1 || header.Body[0] != 0xAA {
		t.Fatalf("Body = %x, want aa", header.Body)
	}
}
