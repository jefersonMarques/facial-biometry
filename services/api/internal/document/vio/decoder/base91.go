package decoder

import "fmt"

const base91Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!#$%&()*+,./:;<=>?@[]^_`{|}~\""

func decodeBase91(input []byte) ([]byte, error) {
	lookup := [256]int{}
	for index := range lookup {
		lookup[index] = -1
	}
	for index, char := range []byte(base91Alphabet) {
		lookup[char] = index
	}

	output := make([]byte, 0, len(input))
	value := -1
	var buffer uint
	var bitCount uint

	for _, char := range input {
		decoded := lookup[char]
		if decoded < 0 {
			return nil, fmt.Errorf("caractere Base91 inválido: 0x%02x", char)
		}

		if value < 0 {
			value = decoded
			continue
		}

		value += decoded * 91
		buffer |= uint(value) << bitCount
		if value&8191 > 88 {
			bitCount += 13
		} else {
			bitCount += 14
		}

		for bitCount >= 8 {
			output = append(output, byte(buffer&255))
			buffer >>= 8
			bitCount -= 8
		}

		value = -1
	}

	if value >= 0 {
		buffer |= uint(value) << bitCount
		bitCount += 7
		for bitCount >= 8 {
			output = append(output, byte(buffer&255))
			buffer >>= 8
			bitCount -= 8
		}
	}

	return output, nil
}
