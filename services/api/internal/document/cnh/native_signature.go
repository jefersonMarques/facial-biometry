package cnh

import (
	"crypto/x509"
	"encoding/asn1"
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fullsailor/pkcs7"

	"faceproof/services/api/internal/document/pdfanalysis"
)

var (
	nativeByteRangePattern = regexp.MustCompile(`/ByteRange\s*\[\s*(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s*\]`)
	nativeContentsPattern  = regexp.MustCompile(`(?s)/Contents\s*<([0-9A-Fa-f\s]+)>`)
	signingTimeOID         = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 9, 5}
)

type nativePDFSignature struct {
	ByteRange   [4]int64
	Valid       bool
	Error       string
	Certificate *x509.Certificate
	SignedAt    time.Time
}

func applyNativePDFSignatureValidation(pdf []byte, analysis *pdfanalysis.Result) error {
	if analysis == nil {
		return fmt.Errorf("nil PDF analysis")
	}

	signatures, err := validateNativePDFSignatures(pdf)
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

	selected := signatures[0]
	for _, candidate := range signatures {
		if candidate.Valid && byteRangeCoversWholeFile(candidate.ByteRange, len(pdf)) {
			selected = candidate
			break
		}
		if candidate.Valid && !selected.Valid {
			selected = candidate
		}
	}

	analysis.Signature.CryptographicallyValid = selected.Valid
	analysis.Signature.Type = "adbe.pkcs7.detached"
	analysis.Signature.HashAlgorithm = "CMS/PKCS#7"

	if !selected.SignedAt.IsZero() {
		analysis.Signature.SigningTime = selected.SignedAt.UTC().Format("Jan 02 2006 15:04:05")
	}

	if selected.Certificate != nil {
		analysis.Signature.Signer = strings.TrimSpace(selected.Certificate.Subject.CommonName)
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

func validateNativePDFSignatures(pdf []byte) ([]nativePDFSignature, error) {
	byteRanges := nativeByteRangePattern.FindAllSubmatchIndex(pdf, -1)
	if len(byteRanges) == 0 {
		return nil, nil
	}

	contents := nativeContentsPattern.FindAllSubmatchIndex(pdf, -1)
	if len(contents) == 0 {
		return nil, fmt.Errorf("signature /Contents not found")
	}

	results := make([]nativePDFSignature, 0, len(byteRanges))
	for index, match := range byteRanges {
		if len(match) != 10 {
			continue
		}

		var br [4]int64
		for valueIndex := 0; valueIndex < 4; valueIndex++ {
			value, err := strconv.ParseInt(string(pdf[match[2+valueIndex*2]:match[3+valueIndex*2]]), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid ByteRange: %w", err)
			}
			br[valueIndex] = value
		}

		contentMatch := closestContentsAfter(contents, match[1], nextByteRangeStart(byteRanges, index, len(pdf)))
		if contentMatch == nil {
			return nil, fmt.Errorf("signature /Contents not associated with ByteRange")
		}

		hexValue := regexp.MustCompile(`\s+`).ReplaceAll(pdf[contentMatch[2]:contentMatch[3]], nil)
		cms, err := hex.DecodeString(string(hexValue))
		if err != nil {
			results = append(results, nativePDFSignature{ByteRange: br, Error: "invalid signature hex"})
			continue
		}

		result := nativePDFSignature{ByteRange: br}
		if err := validateNativeCMS(pdf, br, cms, &result); err != nil {
			result.Error = err.Error()
		} else {
			result.Valid = true
		}
		results = append(results, result)
	}

	return results, nil
}

func validateNativeCMS(pdf []byte, br [4]int64, cms []byte, result *nativePDFSignature) error {
	if err := validateByteRangeBounds(br, len(pdf)); err != nil {
		return err
	}

	p7, err := pkcs7.Parse(cms)
	if err != nil {
		return fmt.Errorf("PKCS7/CMS parse failed: %w", err)
	}

	signedData := make([]byte, 0, br[1]+br[3])
	signedData = append(signedData, pdf[br[0]:br[0]+br[1]]...)
	signedData = append(signedData, pdf[br[2]:br[2]+br[3]]...)
	p7.Content = signedData

	if err := p7.Verify(); err != nil {
		return fmt.Errorf("PKCS7/CMS signature verification failed: %w", err)
	}

	result.Certificate = p7.GetOnlySigner()
	var signedAt time.Time
	if err := p7.UnmarshalSignedAttribute(signingTimeOID, &signedAt); err == nil {
		result.SignedAt = signedAt.UTC()
	}
	return nil
}

func validateByteRangeBounds(br [4]int64, fileSize int) error {
	a, b, c, d := br[0], br[1], br[2], br[3]
	if a < 0 || b < 0 || c < 0 || d < 0 ||
		a+b > int64(fileSize) ||
		c+d > int64(fileSize) ||
		a+b > c {
		return fmt.Errorf("invalid signature ByteRange")
	}
	return nil
}

func closestContentsAfter(contents [][]int, after int, before int) []int {
	for _, match := range contents {
		if len(match) < 4 {
			continue
		}
		if match[0] >= after && match[0] < before {
			return match
		}
	}
	for _, match := range contents {
		if len(match) >= 4 && match[0] >= after && match[0]-after <= 65536 {
			return match
		}
	}
	return nil
}

func nextByteRangeStart(matches [][]int, index int, fallback int) int {
	if index+1 < len(matches) {
		return matches[index+1][0]
	}
	return fallback
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
	if signature.CryptographicallyValid && integrity.SignedRevisionCoversFile {
		return "signed_pdf_intact"
	}
	if signature.CryptographicallyValid && integrity.SignedPDFRevisionComplete && integrity.NonPDFTrailingData {
		return "signed_pdf_with_external_trailing_data"
	}
	if signature.CryptographicallyValid && integrity.UnsignedBytesAfterSignature {
		return "signed_pdf_partially_covered"
	}
	if signature.CryptographicallyValid {
		return "signed_pdf_validity_confirmed"
	}
	return "unknown"
}
