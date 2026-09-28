package decoder

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

func TestDecodeVersion1(t *testing.T) {
	header := Header{Timestamp: 1600000000, Version: 1, CreatedAt: time.Unix(1600000000, 0)}
	fields := []byte{'A', 0xAC, 'B'}
	image := []byte{1, 2, 3}
	signature := bytes.Repeat([]byte{9}, 256)
	templateText := "2"

	header.Body = []byte(fmt.Sprintf(
		"%s-%s-%s\\%s",
		templateText,
		encodeBase91ForTest(fields),
		encodeBase91ForTest(image),
		encodeBase91ForTest(signature),
	))

	payload, err := decodeVersion1(header)
	if err != nil {
		t.Fatalf("decodeVersion1 returned error: %v", err)
	}

	if payload.TemplateID != 2 {
		t.Fatalf("unexpected template: %d", payload.TemplateID)
	}
	if payload.FieldsText != "A¬B" {
		t.Fatalf("unexpected fields: %q", payload.FieldsText)
	}
	if !bytes.Equal(payload.Image, image) {
		t.Fatalf("unexpected image: %v", payload.Image)
	}
	if !bytes.Equal(payload.Signature, signature) {
		t.Fatalf("unexpected signature")
	}

	expectedSignedData := append(headerTextSignedPrefix(header), []byte(templateText)...)
	expectedSignedData = append(expectedSignedData, '-')
	expectedSignedData = append(expectedSignedData, fields...)
	expectedSignedData = append(expectedSignedData, '-')
	expectedSignedData = append(expectedSignedData, image...)
	if !bytes.Equal(payload.SignedData, expectedSignedData) {
		t.Fatalf("unexpected signed data")
	}
}

func TestDecodeVersion2(t *testing.T) {
	header := Header{Timestamp: 1600000000, Version: 2, CreatedAt: time.Unix(1600000000, 0)}
	encodedFields := encodePackedTextForTest("ABC^123", 6, []rune(version5Alphabet))
	signature := bytes.Repeat([]byte{7}, legacyRSASignatureLength)
	image := []byte{4, 5, 6, 7}

	body := make([]byte, 0)
	body = appendUint16(body, 4)
	body = appendUint16(body, uint16(len(encodedFields)))
	body = append(body, encodedFields...)
	body = append(body, signature...)
	body = append(body, image...)
	header.Body = body

	payload, err := decodeVersion2(header)
	if err != nil {
		t.Fatalf("decodeVersion2 returned error: %v", err)
	}

	decodedFields, err := decodeVersion2Fields(payload.EncodedFields)
	if err != nil {
		t.Fatalf("decodeVersion2Fields returned error: %v", err)
	}
	if decodedFields != "ABC^123" {
		t.Fatalf("unexpected fields: %q", decodedFields)
	}
	if payload.TemplateID != 4 || !bytes.Equal(payload.Image, image) {
		t.Fatalf("unexpected payload")
	}
}

func TestDecodeVersion3(t *testing.T) {
	header := Header{Timestamp: 1600000000, Version: 3, CreatedAt: time.Unix(1600000000, 0)}
	plainFields := []byte("ABC^123")
	encodedFields := []byte(encodeBase91ForTest(plainFields))
	signature := bytes.Repeat([]byte{5}, legacyRSASignatureLength)
	image := []byte{8, 9, 10}

	body := make([]byte, 0)
	body = appendUint16(body, 10)
	body = append(body, signature...)
	body = appendUint16(body, uint16(len(encodedFields)))
	body = append(body, encodedFields...)
	body = appendUint16(body, uint16(len(image)))
	body = append(body, image...)
	header.Body = body

	payload, err := decodeVersion3(header)
	if err != nil {
		t.Fatalf("decodeVersion3 returned error: %v", err)
	}

	decodedFields, err := decodeBase91(payload.EncodedFields)
	if err != nil {
		t.Fatalf("decodeBase91 returned error: %v", err)
	}
	if string(decodedFields) != string(plainFields) {
		t.Fatalf("unexpected fields: %q", decodedFields)
	}
	if payload.TemplateID != 10 || !bytes.Equal(payload.Image, image) {
		t.Fatalf("unexpected payload")
	}
}

func TestDecodeVersion4Fields(t *testing.T) {
	plainText := "ABC^1234"
	encoded := encodePackedTextForTest(plainText, 7, []rune(version4Alphabet))

	decoded, err := decodeVersion4Fields(encoded)
	if err != nil {
		t.Fatalf("decodeVersion4Fields returned error: %v", err)
	}
	if decoded != plainText {
		t.Fatalf("unexpected decoded value: %q", decoded)
	}
}

func encodePackedTextForTest(input string, width int, alphabet []rune) []byte {
	characters := []rune(input)
	totalBits := len(characters) * width
	output := make([]byte, (totalBits+7)/8)

	for characterIndex, character := range characters {
		value := -1
		for index, candidate := range alphabet {
			if candidate == character {
				value = index
				break
			}
		}
		if value < 0 {
			panic("test character not found in alphabet")
		}

		for bitIndex := 0; bitIndex < width; bitIndex++ {
			bit := (value >> (width - 1 - bitIndex)) & 0x01
			absoluteBit := characterIndex*width + bitIndex
			output[absoluteBit/8] |= byte(bit << (7 - (absoluteBit % 8)))
		}
	}

	return output
}

func encodeBase91ForTest(input []byte) string {
	var buffer uint
	var bitCount uint
	output := make([]byte, 0, len(input)*2)

	for _, value := range input {
		buffer |= uint(value) << bitCount
		bitCount += 8

		if bitCount <= 13 {
			continue
		}

		encoded := buffer & 8191
		if encoded > 88 {
			buffer >>= 13
			bitCount -= 13
		} else {
			encoded = buffer & 16383
			buffer >>= 14
			bitCount -= 14
		}

		output = append(output, base91Alphabet[encoded%91], base91Alphabet[encoded/91])
	}

	if bitCount > 0 {
		output = append(output, base91Alphabet[buffer%91])
		if bitCount > 7 || buffer > 90 {
			output = append(output, base91Alphabet[buffer/91])
		}
	}

	return string(output)
}
