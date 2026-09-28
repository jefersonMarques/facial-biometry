package decoder

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrTemplateNotFound    = errors.New("template Vio não encontrado")
	ErrCertificateNotFound = errors.New("certificado Vio não encontrado")
)

type TemplateField struct {
	Name  string
	Label string
}

type Template struct {
	ID               uint16
	Name             string
	OwnerName        string
	Fields           []TemplateField
	CertificateGroup string
}

type Certificate struct {
	ID        string
	PublicKey string
}

type Repository interface {
	FindTemplate(id uint16) (Template, error)
	FindCertificates(groupID string, unixSeconds int64) ([]Certificate, error)
}

type SignatureVerifier interface {
	Verify(data []byte, signature []byte, certificate Certificate) (algorithm string, err error)
}

type Service struct {
	repository Repository
	verifier   SignatureVerifier
}

func NewService(repository Repository, verifier SignatureVerifier) *Service {
	return &Service{repository: repository, verifier: verifier}
}

func (service *Service) Decode(value string) (Result, error) {
	normalized, err := normalizeInput(value)
	if err != nil {
		return Result{}, err
	}

	return service.DecodeBytes(normalized.Bytes, normalized.Encoding)
}

func (service *Service) DecodeBytes(data []byte, inputEncoding string) (Result, error) {
	header, err := parseHeader(data)
	if err != nil {
		return Result{}, err
	}

	payload, err := decodePayload(header)
	if err != nil {
		return Result{}, err
	}

	fieldsText, separator, err := decodeFields(header.Version, payload)
	if err != nil {
		return Result{}, fmt.Errorf("falha ao decodificar campos QRCode%d: %w", header.Version, err)
	}
	values := splitFieldValues(fieldsText, separator)

	template, templateKnown, err := service.findTemplate(payload.TemplateID)
	if err != nil {
		return Result{}, err
	}

	fields, rawFields, unmappedFields := mapFieldValues(template.Fields, values)
	image, imageFormat := prepareImage(payload)

	result := Result{
		InputEncoding:     inputEncoding,
		Version:           header.Version,
		TemplateID:        payload.TemplateID,
		TemplateName:      template.Name,
		OwnerName:         template.OwnerName,
		CreatedAt:         header.CreatedAt,
		Fields:            fields,
		RawFields:         rawFields,
		DecodedFieldsText: fieldsText,
		RawQRPayload:      append([]byte(nil), data...),
		HeaderBytes:       append([]byte(nil), data[:header.HeaderSize]...),
		Body:              append([]byte(nil), header.Body...),
		Signature:         append([]byte(nil), payload.Signature...),
		EncodedFields:     append([]byte(nil), payload.EncodedFields...),
		SignedData:        append([]byte(nil), payload.SignedData...),
		Image:             image,
		EmbeddedImage:     append([]byte(nil), payload.Image...),
		ImageFormat:       imageFormat,
		Extra:             append([]byte(nil), payload.Extra...),
		Technical: TechnicalData{
			HeaderFormat:        header.Format,
			Timestamp:           header.Timestamp,
			TemplateKnown:       templateKnown,
			TemplateFieldCount:  len(template.Fields),
			DecodedFieldCount:   len(values),
			UnmappedFieldCount:  unmappedFields,
			CertificateGroupID:  template.CertificateGroup,
			QRPayloadSize:       len(data),
			QRPayloadSHA256:     sha256Hex(data),
			HeaderSize:          header.HeaderSize,
			BodySize:            len(header.Body),
			SignedDataSize:      len(payload.SignedData),
			SignedDataSHA256:    sha256Hex(payload.SignedData),
			SignatureSize:       len(payload.Signature),
			SignatureSHA256:     sha256Hex(payload.Signature),
			EncodedFieldsSize:   len(payload.EncodedFields),
			EmbeddedImageSize:   len(payload.Image),
			EmbeddedImageSHA256: sha256Hex(payload.Image),
			ImageFileSize:       len(image),
			ImageSHA256:         sha256Hex(image),
			ExtraSize:           len(payload.Extra),
			FieldCount:          len(fields),
		},
	}

	if !templateKnown || template.CertificateGroup == "" {
		return result, nil
	}

	certificates, err := service.repository.FindCertificates(template.CertificateGroup, header.CreatedAt.Unix())
	if err != nil {
		if errors.Is(err, ErrCertificateNotFound) {
			return result, nil
		}
		return Result{}, err
	}
	result.Technical.CertificateCandidates = len(certificates)

	for _, certificate := range certificates {
		algorithm, verifyErr := service.verifier.Verify(payload.SignedData, payload.Signature, certificate)
		if verifyErr != nil {
			continue
		}

		result.SignatureValid = true
		result.SignatureAlgorithm = algorithm
		result.CertificateID = certificate.ID
		return result, nil
	}

	return result, nil
}

func (service *Service) findTemplate(templateID uint16) (Template, bool, error) {
	template, err := service.repository.FindTemplate(templateID)
	if err == nil {
		return template, true, nil
	}
	if !errors.Is(err, ErrTemplateNotFound) {
		return Template{}, false, err
	}

	return Template{
		ID:        templateID,
		Name:      fmt.Sprintf("Template %d não catalogado", templateID),
		OwnerName: "Emissor não identificado",
	}, false, nil
}

func decodePayload(header Header) (Payload, error) {
	switch header.Version {
	case 1:
		return decodeVersion1(header)
	case 2:
		return decodeVersion2(header)
	case 3:
		return decodeVersion3(header)
	case 4:
		return decodeVersion4(header)
	case 5:
		return decodeVersion5(header)
	case 6:
		return decodeVersion6(header)
	default:
		return Payload{}, unsupportedVersionError(header.Version)
	}
}

func decodeFields(version uint8, payload Payload) (string, string, error) {
	switch version {
	case 1:
		return payload.FieldsText, "¬", nil
	case 2:
		text, err := decodeVersion2Fields(payload.EncodedFields)
		return text, "^", err
	case 3:
		decoded, err := decodeBase91(payload.EncodedFields)
		if err != nil {
			return "", "", err
		}
		return latin1ToUTF8(decoded), "^", nil
	case 4:
		text, err := decodeVersion4Fields(payload.EncodedFields)
		return text, "^", err
	case 5:
		decoded, err := decodeVersion5Fields(payload.EncodedFields)
		if err != nil {
			return "", "", err
		}
		return latin1ToUTF8(decoded), "^", nil
	case 6:
		decoded, err := decodeBase91(payload.EncodedFields)
		if err != nil {
			return "", "", err
		}
		return latin1ToUTF8(decoded), "^", nil
	default:
		return "", "", unsupportedVersionError(version)
	}
}

func splitFieldValues(text string, separator string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(text, separator)
}

func prepareImage(payload Payload) ([]byte, string) {
	if len(payload.Image) == 0 {
		return nil, "none"
	}

	if payload.ImageIsBPG {
		image := make([]byte, 0, len(payload.Image)+3)
		image = append(image, 'B', 'P', 'G')
		image = append(image, payload.Image...)
		return image, "BPG"
	}

	return append([]byte(nil), payload.Image...), "raw"
}

func mapFieldValues(definitions []TemplateField, values []string) ([]FieldValue, []RawFieldValue, int) {
	fields := make([]FieldValue, 0, len(definitions))
	for index, definition := range definitions {
		value := ""
		if index < len(values) {
			value = values[index]
		}

		fields = append(fields, FieldValue{
			Name:  definition.Name,
			Label: definition.Label,
			Value: value,
		})
	}

	rawFields := make([]RawFieldValue, 0, len(values))
	unmapped := 0
	for index, value := range values {
		field := RawFieldValue{
			Index: index,
			Name:  fmt.Sprintf("field_%d", index+1),
			Label: fmt.Sprintf("Campo %d", index+1),
			Value: value,
		}

		if index < len(definitions) {
			field.Name = definitions[index].Name
			field.Label = definitions[index].Label
			field.Mapped = true
		} else {
			field.Label = fmt.Sprintf("Campo %d (sem definição no template)", index+1)
			unmapped++
		}

		rawFields = append(rawFields, field)
	}

	return fields, rawFields, unmapped
}

func latin1ToUTF8(input []byte) string {
	runes := make([]rune, len(input))
	for index, value := range input {
		runes[index] = rune(value)
	}
	return string(runes)
}

func IsEmptyInput(err error) bool {
	return errors.Is(err, ErrEmptyInput)
}

func sha256Hex(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
