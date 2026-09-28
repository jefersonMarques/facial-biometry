package decoder

import (
	"fmt"
	"strconv"
	"strings"
)

func decodeVersion1(header Header) (Payload, error) {
	parts := strings.Split(string(header.Body), "\\")
	if len(parts) != 2 {
		return Payload{}, fmt.Errorf("QRCode1 inválido: esperado bloco de dados e assinatura")
	}

	signature, err := decodeBase91([]byte(parts[1]))
	if err != nil {
		return Payload{}, fmt.Errorf("assinatura QRCode1 inválida: %w", err)
	}

	dataParts := strings.Split(parts[0], "-")
	if len(dataParts) != 3 {
		return Payload{}, fmt.Errorf("QRCode1 inválido: estrutura de dados inesperada")
	}

	templateValue, err := strconv.ParseUint(dataParts[0], 16, 16)
	if err != nil {
		return Payload{}, fmt.Errorf("template QRCode1 inválido: %w", err)
	}

	fieldsBytes, err := decodeOptionalBase91(dataParts[1])
	if err != nil {
		return Payload{}, fmt.Errorf("campos QRCode1 inválidos: %w", err)
	}

	image, err := decodeOptionalBase91(dataParts[2])
	if err != nil {
		return Payload{}, fmt.Errorf("imagem QRCode1 inválida: %w", err)
	}

	signedData := make([]byte, 0, 10+len(dataParts[0])+len(fieldsBytes)+len(image)+2)
	signedData = append(signedData, headerTextSignedPrefix(header)...)
	signedData = append(signedData, dataParts[0]...)
	signedData = append(signedData, '-')
	signedData = append(signedData, fieldsBytes...)
	signedData = append(signedData, '-')
	signedData = append(signedData, image...)

	return Payload{
		TemplateID: uint16(templateValue),
		Signature:  signature,
		Image:      image,
		FieldsText: latin1ToUTF8(fieldsBytes),
		SignedData: signedData,
		ImageIsBPG: false,
	}, nil
}

func decodeOptionalBase91(value string) ([]byte, error) {
	if value == "" {
		return nil, nil
	}
	return decodeBase91([]byte(value))
}
