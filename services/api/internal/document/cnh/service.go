package cnh

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	Name                  string
	CPF                   string
	BirthDate             string
	Category              string
	ExpiryDate            string
	IssuingUF             string
	Photo                 []byte
	PhotoMIME             string
	PhotoSource           string
	PhotoMethod           string
	PhotoConfidence       string
	PhotoSHA256           string
	PhotoWidth            int
	PhotoHeight           int
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
	if missing := requiredMissingCapabilities(capabilities); len(missing) > 0 {
		return Document{}, fmt.Errorf("%w: missing %s", ErrDependencyUnavailable, strings.Join(missing, ", "))
	}

	decoded, qrErr := qrscan.DecodePDFDetailed(ctx, pdf)
	if qrErr != nil {
		if errors.Is(qrErr, qrscan.ErrPDFRendererUnavailable) {
			return Document{}, fmt.Errorf("%w: pdftoppm", ErrDependencyUnavailable)
		}

		analysis := pdfanalysis.Analyze(ctx, pdf, nil, 1, "CNH", time.Time{})
		if !capabilities.PDFSig || !analysis.Signature.ValidationChecked {
			if nativeErr := applyNativePDFSignatureValidation(pdf, &analysis); nativeErr != nil {
				return Document{}, fmt.Errorf("%w: native PDF signature validation failed", ErrInvalidDocument)
			}
		}
		if reason := forensicRejectionReason(analysis); reason != "" {
			return Document{}, fmt.Errorf("%w: %s", ErrInvalidDocument, reason)
		}
		return Document{}, fmt.Errorf("%w: %s", ErrInvalidDocument, qrIssue(qrErr, nil))
	}

	if service.vio == nil {
		return Document{}, fmt.Errorf("%w: VIO decoder is unavailable", ErrDependencyUnavailable)
	}
	result, vioErr := service.vio.DecodeBytes(decoded.Payload, "identity-cnh-pdf")
	if vioErr != nil {
		analysis := pdfanalysis.Analyze(ctx, pdf, decoded.PageImage, decoded.PageNumber, "CNH", time.Time{})
		if !capabilities.PDFSig || !analysis.Signature.ValidationChecked {
			if nativeErr := applyNativePDFSignatureValidation(pdf, &analysis); nativeErr != nil {
				return Document{}, fmt.Errorf("%w: native PDF signature validation failed", ErrInvalidDocument)
			}
		}
		if reason := forensicRejectionReason(analysis); reason != "" {
			return Document{}, fmt.Errorf("%w: %s", ErrInvalidDocument, reason)
		}
		return Document{}, fmt.Errorf("%w: %s", ErrInvalidDocument, qrIssue(nil, vioErr))
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
	if !capabilities.PDFSig || !analysis.Signature.ValidationChecked {
		if nativeErr := applyNativePDFSignatureValidation(pdf, &analysis); nativeErr != nil {
			return Document{}, fmt.Errorf("%w: native PDF signature validation failed", ErrInvalidDocument)
		}
	}

	assessment := pdfanalysis.Assess(result.SignatureValid, &analysis)
	if assessment.Status != "verified" {
		return Document{}, fmt.Errorf("%w: PDF authenticity status %s", ErrInvalidDocument, assessment.Status)
	}
	if reason := forensicRejectionReason(analysis); reason != "" {
		return Document{}, fmt.Errorf("%w: %s", ErrInvalidDocument, reason)
	}
	if !officialPDFSigner(analysis.Signature) {
		return Document{}, fmt.Errorf("%w: PDF signer is not an expected DETRAN/SENATRAN ICP-Brasil signer", ErrInvalidDocument)
	}

	signingTime, ok := parseSigningTime(analysis.Signature.SigningTime)
	if !ok {
		return Document{}, fmt.Errorf("%w: PDF signing time unavailable", ErrInvalidDocument)
	}
	if signingTime.After(time.Now().UTC().Add(5 * time.Minute)) {
		return Document{}, fmt.Errorf("%w: PDF signing time is in the future", ErrInvalidDocument)
	}
	if signingTime.Before(minimumDocumentDate) {
		return Document{}, ErrDocumentTooOld
	}

	if analysis.Signature.CertificateExpired {
		if !analysis.Signature.SigningTimeValidityChecked {
			if !capabilities.OpenSSL {
				return Document{}, fmt.Errorf("%w: missing openssl for expired signer validation", ErrDependencyUnavailable)
			}
			return Document{}, fmt.Errorf("%w: signer certificate validity at signing time could not be verified", ErrInvalidDocument)
		}
		if !analysis.Signature.SigningTimeWithinCertificateValidity {
			return Document{}, fmt.Errorf("%w: PDF was signed outside the certificate validity period", ErrInvalidDocument)
		}
	}

	documentCPF := NormalizeCPF(fieldValue(result.Fields, "cpf"))
	if documentCPF == "" || documentCPF != expectedCPF {
		return Document{}, ErrDocumentMismatch
	}

	photo, photoEvidence, err := service.referencePhoto(&result, analysis)
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
		Name:                  firstNonEmptyField(result.Fields, "nome", "nome_civil"),
		CPF:                   documentCPF,
		BirthDate:             fieldValue(result.Fields, "data_nascimento"),
		Category:              fieldValue(result.Fields, "categoria"),
		ExpiryDate:            fieldValue(result.Fields, "data_validade"),
		IssuingUF:             fieldValue(result.Fields, "uf_emissao"),
		Photo:                 photo,
		PhotoMIME:             photoEvidence.MIME,
		PhotoSource:           photoEvidence.Source,
		PhotoMethod:           photoEvidence.Method,
		PhotoConfidence:       photoEvidence.Confidence,
		PhotoSHA256:           photoEvidence.SHA256,
		PhotoWidth:            photoEvidence.Width,
		PhotoHeight:           photoEvidence.Height,
	}, nil
}

type photoEvidence struct {
	Source     string
	Method     string
	Confidence string
	MIME       string
	SHA256     string
	Width      int
	Height     int
}

func (service *Service) referencePhoto(
	result *decoder.Result,
	analysis pdfanalysis.Result,
) ([]byte, photoEvidence, error) {
	if analysis.SourceIntegrity.Status == "signed_pdf_intact" &&
		analysis.Signature.CryptographicallyValid &&
		analysis.Photo.Available &&
		len(analysis.Photo.Bytes) > 0 &&
		analysis.Photo.MIME != "" {
		return append([]byte(nil), analysis.Photo.Bytes...), photoEvidence{
			Source:     "signed_pdf_visual",
			Method:     analysis.Photo.Method,
			Confidence: analysis.Photo.Confidence,
			MIME:       analysis.Photo.MIME,
			SHA256:     analysis.Photo.SHA256,
			Width:      analysis.Photo.Width,
			Height:     analysis.Photo.Height,
		}, nil
	}

	if service.preview != nil {
		if err := service.preview.Attach(result); err == nil && len(result.PreviewImage) > 0 && result.PreviewMIME != "" {
			return append([]byte(nil), result.PreviewImage...), photoEvidence{
				Source:     "signed_vio_qr",
				Method:     "vio_embedded_portrait",
				Confidence: "cryptographic",
				MIME:       result.PreviewMIME,
				SHA256:     hashBytesHex(result.PreviewImage),
				Width:      result.Technical.PreviewWidth,
				Height:     result.Technical.PreviewHeight,
			}, nil
		} else if err != nil && !errors.Is(err, imagepreview.ErrDecoderUnavailable) {
			return nil, photoEvidence{}, fmt.Errorf("%w: %v", ErrPhotoUnavailable, err)
		}
	}

	return nil, photoEvidence{}, fmt.Errorf("%w: signed PDF portrait unavailable and VIO portrait fallback unavailable", ErrPhotoUnavailable)
}

func forensicRejectionReason(analysis pdfanalysis.Result) string {
	switch analysis.SourceIntegrity.Status {
	case "invalid_signature":
		return "PDF digital signature is invalid"
	case "malformed_signature_structure":
		return "PDF signature structure is inconsistent"
	case "modified_after_signature":
		return "PDF contains active changes after the signed revision"
	case "unsigned_pdf":
		return "PDF is not digitally signed"
	case "signed_pdf_partially_covered":
		return "PDF contains unsigned bytes after the signed revision"
	case "signature_not_checked":
		return "PDF signature could not be checked"
	case "signed_pdf_validity_confirmed":
		return "PDF signature is valid but complete-file coverage was not confirmed"
	case "unknown":
		return "PDF authenticity could not be established"
	}

	if analysis.SourceIntegrity.Status != "signed_pdf_intact" {
		return "PDF source integrity is not intact"
	}
	if analysis.Metadata.Pages != 1 || analysis.Metadata.Encrypted {
		return "unexpected PDF structure"
	}
	if analysis.Integrity.JavaScriptPresent ||
		analysis.Integrity.OpenActionPresent ||
		analysis.Integrity.EmbeddedFilesPresent {
		return "active or embedded PDF content detected"
	}
	if analysis.Consistency.TextEvidenceChecked &&
		analysis.Consistency.VisibleDigitalSignatureClaim &&
		analysis.Signature.PresenceChecked &&
		!analysis.Signature.Present {
		return "document claims a digital signature but no valid PDF signature was found"
	}
	return ""
}

func qrIssue(qrErr error, vioErr error) string {
	switch {
	case vioErr != nil && errors.Is(vioErr, decoder.ErrInvalidHeader):
		return "QR Code is incompatible with the expected VIO format"
	case vioErr != nil:
		return "QR Code could not be validated as a compatible VIO document"
	case qrErr != nil && errors.Is(qrErr, qrscan.ErrQRCodeNotFound):
		return "no compatible VIO QR Code was found in the PDF"
	case qrErr != nil:
		return "PDF QR Code could not be validated"
	default:
		return "document could not be validated through VIO"
	}
}

func requiredMissingCapabilities(capabilities pdfanalysis.Capabilities) []string {
	var missing []string
	if !capabilities.PDFRender {
		missing = append(missing, "pdftoppm")
	}
	if !capabilities.PDFInfo {
		missing = append(missing, "pdfinfo")
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

	return authority && icpBrasil
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

func firstNonEmptyField(fields []decoder.FieldValue, names ...string) string {
	for _, name := range names {
		if value := fieldValue(fields, name); value != "" {
			return value
		}
	}
	return ""
}


func hashBytesHex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}
