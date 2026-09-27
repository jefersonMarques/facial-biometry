package decoder

import "fmt"

func decodeVersion2(header Header) (Payload, error) {
	cursor := newByteCursor(header.Body)

	templateID, err := cursor.readUint16()
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

	signature, err := cursor.readBytes(legacyRSASignatureLength)
	if err != nil {
		return Payload{}, fmt.Errorf("assinatura QRCode2 inválida: %w", err)
	}

	image := append([]byte(nil), cursor.remaining()...)

	signedData := make([]byte, 0, 10+2+2+len(encodedFields)+len(image))
	signedData = append(signedData, headerTextSignedPrefix(header)...)
	signedData = appendUint16(signedData, templateID)
	signedData = appendUint16(signedData, fieldsLength)
	signedData = append(signedData, encodedFields...)
	signedData = append(signedData, image...)

	return Payload{
		TemplateID:    templateID,
		Signature:     append([]byte(nil), signature...),
		Image:         image,
		EncodedFields: append([]byte(nil), encodedFields...),
		SignedData:    signedData,
		ImageIsBPG:    true,
	}, nil
}
