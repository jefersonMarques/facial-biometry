package decoder

import "fmt"

const legacyRSASignatureLength = 256

func decodeVersion3(header Header) (Payload, error) {
	cursor := newByteCursor(header.Body)

	templateID, err := cursor.readUint16()
	if err != nil {
		return Payload{}, err
	}

	signature, err := cursor.readBytes(legacyRSASignatureLength)
	if err != nil {
		return Payload{}, fmt.Errorf("assinatura QRCode3 inválida: %w", err)
	}

	fieldsLength, err := cursor.readUint16()
	if err != nil {
		return Payload{}, err
	}
	encodedFields, err := cursor.readBytes(int(fieldsLength))
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

	extra := append([]byte(nil), cursor.remaining()...)

	signedData := make([]byte, 0, 10+2+2+len(encodedFields)+2+len(image))
	signedData = append(signedData, headerTextSignedPrefix(header)...)
	signedData = appendUint16(signedData, templateID)
	signedData = appendUint16(signedData, fieldsLength)
	signedData = append(signedData, encodedFields...)
	signedData = appendUint16(signedData, imageLength)
	signedData = append(signedData, image...)

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
