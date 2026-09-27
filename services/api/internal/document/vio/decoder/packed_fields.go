package decoder

import (
	"fmt"
	"strings"
)

const version4Alphabet = " ABCÇDEFGHIJKLMNOPQRSTUVWXYZabcçdefghijklmnopqrstuvwxyz0123456789áàéíóúüñÁÀÉÍÓÚÜÑÃãÂâÔôÕõ=+-/\\*_|()[]{}<>#%&@'\".:;,!?$\n~^êÊºª§"
const version5Alphabet = " !\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_"

func decodeVersion4Fields(input []byte) (string, error) {
	return decodePackedAlphabet(input, 7, []rune(version4Alphabet), false)
}

func decodeVersion5Fields(input []byte) ([]byte, error) {
	text, err := decodePackedAlphabet(input, 6, []rune(version5Alphabet), false)
	if err != nil {
		return nil, err
	}
	return []byte(text), nil
}

func decodeVersion2Fields(input []byte) (string, error) {
	text, err := decodePackedAlphabet(input, 6, []rune(version5Alphabet), false)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(text), nil
}

func decodePackedAlphabet(input []byte, width int, alphabet []rune, trimPadding bool) (string, error) {
	if len(input) == 0 {
		return "", nil
	}
	if width <= 0 || width > 8 {
		return "", fmt.Errorf("largura de bits inválida: %d", width)
	}

	totalBits := len(input) * 8
	output := make([]rune, 0, totalBits/width)

	for bitOffset := 0; bitOffset+width <= totalBits; bitOffset += width {
		value := 0
		for bitIndex := 0; bitIndex < width; bitIndex++ {
			absoluteBit := bitOffset + bitIndex
			sourceByte := input[absoluteBit/8]
			sourceShift := 7 - (absoluteBit % 8)
			value = (value << 1) | int((sourceByte>>sourceShift)&0x01)
		}

		if value < 0 || value >= len(alphabet) {
			return "", fmt.Errorf("símbolo compactado fora do alfabeto: %d", value)
		}
		output = append(output, alphabet[value])
	}

	result := string(output)
	if trimPadding {
		result = strings.TrimSpace(result)
	}
	return result, nil
}
