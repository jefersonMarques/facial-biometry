package cnh

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"faceproof/services/api/internal/document/pdfintegrity"
	"faceproof/services/api/internal/document/vio/decoder"
	"faceproof/services/api/internal/document/vio/imagepreview"
	"faceproof/services/api/internal/document/vio/qrscan"
)

var (
	ErrInvalidDocument      = errors.New("invalid CNH Digital")
	ErrDocumentTooOld       = errors.New("CNH Digital was generated before the minimum date")
	ErrDocumentMismatch     = errors.New("CNH Digital does not match the expected CPF")
	ErrPhotoUnavailable     = errors.New("CNH reference photo is unavailable")
	ErrDependencyUnavailable = errors.New("CNH validation dependency is unavailable")
)

type Service struct {
	inspector *pdfintegrity.Inspector
	vio       *decoder.Service
	preview   *imagepreview.Service
}

type Document struct {
	PDFSHA256            string
	PDFSigningTime       time.Time
	PDFSigner             string
	PDFCreator            string
	PDFProducer           string
	VIOTemplateID         uint16
	VIOCreatedAt          time.Time
	VIOSignatureAlgorithm string
	CPF                   string
	Photo                 []byte
	PhotoMIME             string
}

func NewService(inspector *pdfintegrity.Inspector, vio *decoder.Service, preview *imagepreview.Service) *Service {
	return &Service{inspector: inspector, vio: vio, preview: preview}
}

func (service *Service) Process(ctx context.Context, pdf []byte, expectedCPF string, minimumDocumentDate time.Time) (Document, error) {
	expectedCPF = NormalizeCPF(expectedCPF)
	if !ValidCPF(expectedCPF) {
		return Document{}, ErrDocumentMismatch
	}

	integrity, err := service.inspector.Inspect(ctx, pdf)
	if err != nil {
		if errors.Is(err, pdfintegrity.ErrToolUnavailable) {
			return Document{}, fmt.Errorf("%w: %v", ErrDependencyUnavailable, err)
		}
		return Document{}, fmt.Errorf("%w: %v", ErrInvalidDocument, err)
	}
	if !integrity.Valid {
		return Document{}, fmt.Errorf("%w: %s", ErrInvalidDocument, strings.Join(integrity.Diagnostics, "; "))
	}
	if integrity.Signature.SigningTime.Before(minimumDocumentDate) {
		return Document{}, ErrDocumentTooOld
	}

	qrPayload, err := qrscan.DecodePDF(ctx, pdf)
	if err != nil {
		if errors.Is(err, qrscan.ErrPDFRendererUnavailable) {
			return Document{}, fmt.Errorf("%w: %v", ErrDependencyUnavailable, err)
		}
		return Document{}, fmt.Errorf("%w: QR Code VIO não encontrado", ErrInvalidDocument)
	}

	result, err := service.vio.DecodeBytes(qrPayload, "identity-cnh-pdf")
	if err != nil {
		return Document{}, fmt.Errorf("%w: VIO decode failed", ErrInvalidDocument)
	}
	if !result.Technical.TemplateKnown ||
		!strings.EqualFold(strings.TrimSpace(result.TemplateName), "CNH") ||
		!strings.EqualFold(strings.TrimSpace(result.OwnerName), "SENATRAN") ||
		!result.SignatureValid {
		return Document{}, fmt.Errorf("%w: VIO signature or template is invalid", ErrInvalidDocument)
	}

	documentCPF := NormalizeCPF(fieldValue(result.Fields, "cpf"))
	if documentCPF == "" || documentCPF != expectedCPF {
		return Document{}, ErrDocumentMismatch
	}

	if service.preview == nil {
		return Document{}, ErrPhotoUnavailable
	}
	if err := service.preview.Attach(&result); err != nil {
		if errors.Is(err, imagepreview.ErrDecoderUnavailable) {
			return Document{}, fmt.Errorf("%w: BPG decoder unavailable", ErrDependencyUnavailable)
		}
		return Document{}, fmt.Errorf("%w: %v", ErrPhotoUnavailable, err)
	}
	if len(result.PreviewImage) == 0 || result.PreviewMIME == "" {
		return Document{}, ErrPhotoUnavailable
	}

	hash := sha256.Sum256(pdf)
	return Document{
		PDFSHA256:            hex.EncodeToString(hash[:]),
		PDFSigningTime:       integrity.Signature.SigningTime,
		PDFSigner:            integrity.Signature.SignerCommonName,
		PDFCreator:           integrity.Metadata.Creator,
		PDFProducer:          integrity.Metadata.Producer,
		VIOTemplateID:         result.TemplateID,
		VIOCreatedAt:          result.CreatedAt,
		VIOSignatureAlgorithm: result.SignatureAlgorithm,
		CPF:                   documentCPF,
		Photo:                 append([]byte(nil), result.PreviewImage...),
		PhotoMIME:             result.PreviewMIME,
	}, nil
}

func NormalizeCPF(value string) string {
	var builder strings.Builder
	for _, character := range value {
		if character >= '0' && character <= '9' {
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

func ValidCPF(value string) bool {
	value = NormalizeCPF(value)
	if len(value) != 11 {
		return false
	}
	allEqual := true
	for index := 1; index < len(value); index++ {
		if value[index] != value[0] {
			allEqual = false
			break
		}
	}
	if allEqual {
		return false
	}

	for digitIndex := 9; digitIndex < 11; digitIndex++ {
		sum := 0
		for index := 0; index < digitIndex; index++ {
			sum += int(value[index]-'0') * (digitIndex + 1 - index)
		}
		check := (sum * 10) % 11
		if check == 10 {
			check = 0
		}
		if check != int(value[digitIndex]-'0') {
			return false
		}
	}
	return true
}

func fieldValue(fields []decoder.FieldValue, name string) string {
	for _, field := range fields {
		if strings.EqualFold(strings.TrimSpace(field.Name), name) {
			return strings.TrimSpace(field.Value)
		}
	}
	return ""
}
