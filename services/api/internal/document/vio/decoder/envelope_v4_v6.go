package decoder

func decodeLengthPrefixedEnvelope(header Header) (Payload, error) {
	cursor := newByteCursor(header.Body)

	templateID, err := cursor.readUint16()
	if err != nil {
		return Payload{}, err
	}

	signatureLength, err := cursor.readUint16()
	if err != nil {
		return Payload{}, err
	}
	signature, err := cursor.readBytes(int(signatureLength))
	if err != nil {
		return Payload{}, err
	}

	imageLength, err := cursor.readUint16()
	if err != nil {
		return Payload{}, err
	}
	image, err := cursor.readBytes(int(imageLength))
	if err != nil {
		return Payload{}, err
	}

	fieldsLength, err := cursor.readUint16()
	if err != nil {
		return Payload{}, err
	}
	encodedFields, err := cursor.readBytes(int(fieldsLength))
	if err != nil {
		return Payload{}, err
	}

	extra := append([]byte(nil), cursor.remaining()...)

	signedData := make([]byte, 0, 5+2+2+len(image)+2+len(encodedFields))
	signedData = append(signedData, headerSignedPrefix(header)...)
	signedData = appendUint16(signedData, templateID)
	signedData = appendUint16(signedData, imageLength)
	signedData = append(signedData, image...)
	signedData = appendUint16(signedData, fieldsLength)
	signedData = append(signedData, encodedFields...)

	return Payload{
		TemplateID:    templateID,
		Signature:     append([]byte(nil), signature...),
		Image:         append([]byte(nil), image...),
		EncodedFields: append([]byte(nil), encodedFields...),
		Extra:         extra,
		SignedData:    signedData,
		ImageIsBPG:    true,
	}, nil
}
