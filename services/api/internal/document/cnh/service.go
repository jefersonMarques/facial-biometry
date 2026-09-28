package cnh

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"faceproof/services/api/internal/document/pdfanalysis"
	"faceproof/services/api/internal/document/vio/decoder"
	"faceproof/services/api/internal/document/vio/imagepreview"
	"faceproof/services/api/internal/document/vio/qrscan"
)

var (
	ErrInvalidDocument       = errors.New("invalid CNH Digital")
	ErrDocumentTooOld        = errors.New("CNH Digital was generated before the minimum date")
	ErrDocumentMismatch      = errors.New("CNH Digital does not match the expected CPF")
	ErrPhotoUnavailable      = errors.New("CNH reference photo is unavailable")
	ErrDependencyUnavailable = errors.New("CNH validation dependency is unavailable")
)

type Service struct {
	vio     *decoder.Service
	preview *imagepreview.Service
}

type Document struct {
	PDFSHA256             string
	PDFSigningTime        time.Time
	PDFSigner             string
	PDFCreator            string
	PDFProducer           string
	PDFSourceIntegrity    string
	PDFSignatureAlgorithm string
	VIOTemplateID         uint16
	VIOCreatedAt          time.Time
	VIOSignatureAlgorithm string
	CPF                    string
	Photo                  []byte
	PhotoMIME              string
	PhotoSource            string
	PhotoWidth             int
	PhotoHeight            int
}

func NewService(vio *decoder.Service, preview *imagepreview.Service) *Service {
	return &Service{vio: vio, preview: preview}
}

func (service *Service) Process(ctx context.Context, pdf []byte, expectedCPF string, minimumDocumentDate time.Time) (Document, error) {
	expectedCPF = NormalizeCPF(expectedCPF)
	if !ValidCPF(expectedCPF) {
		return Document{}, ErrDocumentMismatch
	}

	if len(pdf) < 5 || string(pdf[:5]) != "%PDF-" {
		return Document{}, fmt.Errorf("%w: invalid PDF header", ErrInvalidDocument)
	}

	capabilities := pdfanalysis.DetectCapabilities()
	missing := requiredMissingCapabilities(capabilities)
	if len(missing) > 0 {
		return Document{}, fmt.Errorf("%w: missing %s", ErrDependencyUnavailable, strings.Join(missing, ", "))
	}

	decoded, err := qrscan.DecodePDFDetailed(ctx, pdf)
	if err != nil {
		if errors.Is(err, qrscan.ErrPDFRendererUnavailable) {
			return Document{}, fmt.Errorf("%w: pdftoppm", ErrDependencyUnavailable)
		}
		return Document{}, fmt.Errorf("%w: VIO QR Code not found", ErrInvalidDocument)
	}

	result, err := service.vio.DecodeBytes(decoded.Payload, "identity-cnh-pdf")
	if err != nil {
		return Document{}, fmt.Errorf("%w: VIO decode failed", ErrInvalidDocument)
	}
	if !result.Technical.TemplateKnown ||
		!strings.EqualFold(strings.TrimSpace(result.TemplateName), "CNH") ||
		!strings.EqualFold(strings.TrimSpace(result.OwnerName), "SENATRAN") ||
		!result.SignatureValid {
		return Document{}, fmt.Errorf("%w: VIO signature or CNH template is invalid", ErrInvalidDocument)
	}

	analysis := pdfanalysis.Analyze(
		ctx,
		pdf,
		decoded.PageImage,
		decoded.PageNumber,
		result.TemplateName,
		result.CreatedAt,
	)
	assessment := pdfanalysis.Assess(result.SignatureValid, &analysis)
	if assessment.Status != "verified" {
		return Document{}, fmt.Errorf("%w: PDF authenticity status %s", ErrInvalidDocument, assessment.Status)
	}
	if analysis.SourceIntegrity.Status != "signed_pdf_intact" {
		return Document{}, fmt.Errorf("%w: PDF source integrity status %s", ErrInvalidDocument, analysis.SourceIntegrity.Status)
	}
	if !officialPDFSigner(analysis.Signature) {
		return Document{}, fmt.Errorf("%w: PDF signer is not an expected DETRAN/SENATRAN ICP-Brasil signer", ErrInvalidDocument)
	}
	if analysis.Metadata.Pages != 1 || analysis.Metadata.Encrypted {
		return Document{}, fmt.Errorf("%w: unexpected PDF structure", ErrInvalidDocument)
	}
	if analysis.Integrity.JavaScriptPresent ||
		analysis.Integrity.OpenActionPresent ||
		analysis.Integrity.EmbeddedFilesPresent {
		return Document{}, fmt.Errorf("%w: active or embedded PDF content detected", ErrInvalidDocument)
	}

	signingTime, ok := parseSigningTime(analysis.Signature.SigningTime)
	if !ok {
		return Document{}, fmt.Errorf("%w: PDF signing time unavailable", ErrInvalidDocument)
	}
	if signingTime.Before(minimumDocumentDate) {
		return Document{}, ErrDocumentTooOld
	}

	documentCPF := NormalizeCPF(fieldValue(result.Fields, "cpf"))
	if documentCPF == "" || documentCPF != expectedCPF {
		return Document{}, ErrDocumentMismatch
	}

	photo, photoMIME, photoSource, photoWidth, photoHeight, err := service.referencePhoto(&result, analysis)
	if err != nil {
		return Document{}, err
	}

	return Document{
		PDFSHA256:             analysis.Integrity.SHA256,
		PDFSigningTime:        signingTime,
		PDFSigner:             analysis.Signature.Signer,
		PDFCreator:            analysis.Metadata.Creator,
		PDFProducer:           analysis.Metadata.Producer,
		PDFSourceIntegrity:    analysis.SourceIntegrity.Status,
		PDFSignatureAlgorithm: analysis.Signature.HashAlgorithm,
		VIOTemplateID:         result.TemplateID,
		VIOCreatedAt:          result.CreatedAt,
		VIOSignatureAlgorithm: result.SignatureAlgorithm,
		CPF:                    documentCPF,
		Photo:                  photo,
		PhotoMIME:              photoMIME,
		PhotoSource:            photoSource,
		PhotoWidth:             photoWidth,
		PhotoHeight:            photoHeight,
	}, nil
}

func (service *Service) referencePhoto(
	result *decoder.Result,
	analysis pdfanalysis.Result,
) ([]byte, string, string, int, int, error) {
	if analysis.Photo.Available && len(analysis.Photo.Bytes) > 0 && analysis.Photo.MIME != "" {
		return append([]byte(nil), analysis.Photo.Bytes...),
			analysis.Photo.MIME,
			"signed_pdf_visual",
			analysis.Photo.Width,
			analysis.Photo.Height,
			nil
	}

	if service.preview != nil {
		if err := service.preview.Attach(result); err == nil && len(result.PreviewImage) > 0 && result.PreviewMIME != "" {
			return append([]byte(nil), result.PreviewImage...),
				result.PreviewMIME,
				"signed_vio_qr",
				result.Technical.PreviewWidth,
				result.Technical.PreviewHeight,
				nil
		} else if err != nil && !errors.Is(err, imagepreview.ErrDecoderUnavailable) {
			return nil, "", "", 0, 0, fmt.Errorf("%w: %v", ErrPhotoUnavailable, err)
		}
	}

	return nil, "", "", 0, 0, fmt.Errorf("%w: signed PDF portrait unavailable and VIO BPG preview unavailable", ErrPhotoUnavailable)
}

func requiredMissingCapabilities(capabilities pdfanalysis.Capabilities) []string {
	var missing []string
	if !capabilities.PDFRender {
		missing = append(missing, "pdftoppm")
	}
	if !capabilities.PDFInfo {
		missing = append(missing, "pdfinfo")
	}
	if !capabilities.PDFSig {
		missing = append(missing, "pdfsig")
	}
	return missing
}

func officialPDFSigner(signature pdfanalysis.Signature) bool {
	signer := strings.ToUpper(strings.TrimSpace(signature.Signer))
	distinguishedName := strings.ToUpper(strings.TrimSpace(signature.SignerDistinguishedName))
	certificateSubject := strings.ToUpper(strings.TrimSpace(signature.CertificateSubject))
	certificateIssuer := strings.ToUpper(strings.TrimSpace(signature.CertificateIssuer))

	authority := strings.Contains(signer, "DETRAN") ||
		strings.Contains(signer, "SENATRAN") ||
		strings.Contains(certificateSubject, "DETRAN") ||
		strings.Contains(certificateSubject, "SENATRAN")

	icpBrasil := signature.ICPBrasilDetected ||
		strings.Contains(distinguishedName, "ICP-BRASIL") ||
		strings.Contains(certificateSubject, "ICP-BRASIL") ||
		strings.Contains(certificateIssuer, "ICP-BRASIL")

	serpro := strings.Contains(distinguishedName, "SERPRO") ||
		strings.Contains(certificateSubject, "SERPRO") ||
		strings.Contains(certificateIssuer, "SERPRO")

	return authority && icpBrasil && serpro
}

func parseSigningTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}

	for _, layout := range []string{
		"Jan 02 2006 15:04:05",
		"Jan 2 2006 15:04:05",
		time.RFC3339,
	} {
		if parsed, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
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
