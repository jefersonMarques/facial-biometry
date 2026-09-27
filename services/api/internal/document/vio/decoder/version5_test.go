package decoder

import "testing"

func TestDecodeVersion5Fields(t *testing.T) {
	plainText := []byte("ABC^DEF^1234")
	encoded := encodeVersion5FieldsForTest(plainText)

	decoded, err := decodeVersion5Fields(encoded)
	if err != nil {
		t.Fatalf("decodeVersion5Fields returned error: %v", err)
	}

	if string(decoded) != string(plainText) {
		t.Fatalf("unexpected decoded value: %q", decoded)
	}
}

func encodeVersion5FieldsForTest(input []byte) []byte {
	totalBits := len(input) * 6
	output := make([]byte, (totalBits+7)/8)

	for characterIndex, character := range input {
		value := character - 0x20
		for bitIndex := 0; bitIndex < 6; bitIndex++ {
			bit := (value >> (5 - bitIndex)) & 0x01
			absoluteBit := characterIndex*6 + bitIndex
			output[absoluteBit/8] |= bit << (7 - (absoluteBit % 8))
		}
	}

	return output
}
