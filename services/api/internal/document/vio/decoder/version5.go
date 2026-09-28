package decoder

func decodeVersion5(header Header) (Payload, error) {
	return decodeLengthPrefixedEnvelope(header)
}
