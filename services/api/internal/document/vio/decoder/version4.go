package decoder

func decodeVersion4(header Header) (Payload, error) {
	return decodeLengthPrefixedEnvelope(header)
}
