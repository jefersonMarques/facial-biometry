package decoder

func decodeVersion6(header Header) (Payload, error) {
	return decodeLengthPrefixedEnvelope(header)
}
