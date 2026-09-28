package cnh

import (
	"crypto/x509"
	"fmt"
	"strings"
	"time"

	pdfer "github.com/benedoc-inc/pdfer/v2"

	"faceproof/services/api/internal/document/pdfanalysis"
)

func applyNativePDFSignatureValidation(pdf []byte, analysis *pdfanalysis.Result) error {
	if analysis == nil {
		return fmt.Errorf("nil PDF analysis")
	}

	signatures, err := pdfer.ValidateSignatures(pdf)
	if err != nil {
		return err
	}

	analysis.Signature.PresenceChecked = true
	analysis.Signature.ValidationChecked = true
	analysis.Signature.SignatureCount = len(signatures)

	if len(signatures) == 0 {
		analysis.Signature.Present = false
		analysis.Signature.CryptographicallyValid = false
		if analysis.Integrity.ByteRangeCount > 0 || analysis.Integrity.SignatureDictionaryDetected {
			analysis.SourceIntegrity.Status = "malformed_signature_structure"
		} else {
			analysis.SourceIntegrity.Status = "unsigned_pdf"
		}
		return nil
	}

	analysis.Signature.Present = true

	var selectedIndex = -1
	for index := range signatures {
		candidate := signatures[index]
		if candidate.Valid && byteRangeCoversWholeFile(candidate.ByteRange, len(pdf)) {
			selectedIndex = index
			break
		}
	}
	if selectedIndex == -1 {
		for index := range signatures {
			if signatures[index].Valid {
				selectedIndex = index
				break
			}
		}
	}
	if selectedIndex == -1 {
		selectedIndex = 0
	}

	selected := signatures[selectedIndex]
	analysis.Signature.CryptographicallyValid = selected.Valid
	analysis.Signature.Signer = strings.TrimSpace(selected.SignerName)
	if !selected.SignedAt.IsZero() {
		analysis.Signature.SigningTime = selected.SignedAt.UTC().Format("Jan 02 2006 15:04:05")
	}
	analysis.Signature.Type = "adbe.pkcs7.detached"
	analysis.Signature.HashAlgorithm = "SHA-256"

	if selected.Certificate != nil {
		applyNativeCertificate(selected.Certificate, selected.SignedAt, &analysis.Signature)
	}

	if selected.Valid {
		analysis.Signature.CertificateStatus = "native cryptographic validation"
	} else {
		analysis.Signature.CertificateStatus = "native cryptographic validation failed"
		if selected.Error != "" {
			analysis.Warnings = append(analysis.Warnings, "native PDF signature validation: "+selected.Error)
		}
	}

	analysis.Integrity.SignedRevisionCoversFile = byteRangeCoversWholeFile(selected.ByteRange, len(pdf))
	analysis.SourceIntegrity.Status = nativeSourceIntegrityStatus(analysis.Signature, analysis.Integrity)
	return nil
}

func applyNativeCertificate(cert *x509.Certificate, signedAt time.Time, target *pdfanalysis.Signature) {
	target.CertificateSubject = cert.Subject.String()
	target.CertificateIssuer = cert.Issuer.String()
	target.SignerDistinguishedName = cert.Subject.String()
	target.CertificateNotBefore = cert.NotBefore.UTC().Format(time.RFC3339)
	target.CertificateNotAfter = cert.NotAfter.UTC().Format(time.RFC3339)
	target.CertificateExpired = time.Now().UTC().After(cert.NotAfter)
	target.ICPBrasilDetected =
		containsCertificateMarker(cert.Subject.String(), "ICP-Brasil") ||
			containsCertificateMarker(cert.Issuer.String(), "ICP-Brasil")

	if !signedAt.IsZero() {
		target.SigningTimeValidityChecked = true
		target.SigningTimeWithinCertificateValidity =
			!signedAt.Before(cert.NotBefore) && !signedAt.After(cert.NotAfter)
	}
}

func containsCertificateMarker(value, marker string) bool {
	return strings.Contains(strings.ToUpper(value), strings.ToUpper(marker))
}

func byteRangeCoversWholeFile(byteRange [4]int64, fileSize int) bool {
	a, b, c, d := byteRange[0], byteRange[1], byteRange[2], byteRange[3]
	if a != 0 || b < 0 || c < 0 || d < 0 || a+b > c {
		return false
	}
	return c+d == int64(fileSize)
}

func nativeSourceIntegrityStatus(signature pdfanalysis.Signature, integrity pdfanalysis.Integrity) string {
	if !signature.PresenceChecked {
		return "signature_not_checked"
	}
	if !signature.Present && (integrity.ByteRangeCount > 0 || integrity.SignatureDictionaryDetected) {
		return "malformed_signature_structure"
	}
	if !signature.Present {
		return "unsigned_pdf"
	}
	if signature.ValidationChecked && !signature.CryptographicallyValid {
		return "invalid_signature"
	}
	if integrity.ActivePDFChangesAfterSignature {
		return "modified_after_signature"
	}
	if integrity.SignedRevisionCoversFile {
		return "signed_pdf_intact"
	}
	if integrity.SignedPDFRevisionComplete && integrity.NonPDFTrailingData {
		return "signed_pdf_with_external_trailing_data"
	}
	if integrity.UnsignedBytesAfterSignature {
		return "signed_pdf_partially_covered"
	}
	return "signed_pdf_validity_confirmed"
}
