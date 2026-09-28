package pdfanalysis

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const commandTimeout = 8 * time.Second

type Capabilities struct {
	PDFRender bool `json:"pdftoppm"`
	PDFInfo   bool `json:"pdfinfo"`
	PDFSig    bool `json:"pdfsig"`
	PDFImages bool `json:"pdfimages"`
	PDFText   bool `json:"pdftotext"`
	OpenSSL   bool `json:"openssl"`
}

func DetectCapabilities() Capabilities {
	_, pdfRenderErr := exec.LookPath("pdftoppm")
	_, pdfInfoErr := exec.LookPath("pdfinfo")
	_, pdfSigErr := exec.LookPath("pdfsig")
	_, pdfImagesErr := exec.LookPath("pdfimages")
	_, pdfTextErr := exec.LookPath("pdftotext")
	_, opensslErr := exec.LookPath("openssl")
	return Capabilities{
		PDFRender: pdfRenderErr == nil,
		PDFInfo:   pdfInfoErr == nil,
		PDFSig:    pdfSigErr == nil,
		PDFImages: pdfImagesErr == nil,
		PDFText:   pdfTextErr == nil,
		OpenSSL:   opensslErr == nil,
	}
}

func (c Capabilities) DeepAnalysisReady() bool {
	return c.PDFRender && c.PDFInfo && c.PDFSig && c.PDFImages && c.PDFText && c.OpenSSL
}

type Metadata struct {
	Title          string `json:"title,omitempty"`
	Creator        string `json:"creator,omitempty"`
	Producer       string `json:"producer,omitempty"`
	CreationDate   string `json:"creation_date,omitempty"`
	ModificationDate string `json:"modification_date,omitempty"`
	Pages          int    `json:"pages,omitempty"`
	PageSize       string `json:"page_size,omitempty"`
	Encrypted      bool   `json:"encrypted"`
	PDFVersion     string `json:"pdf_version,omitempty"`
}

type Signature struct {
	PresenceChecked          bool   `json:"presence_checked"`
	Present                  bool   `json:"present"`
	ValidationChecked        bool   `json:"validation_checked"`
	CryptographicallyValid   bool   `json:"cryptographically_valid"`
	Signer                   string `json:"signer,omitempty"`
	SignerDistinguishedName  string `json:"signer_distinguished_name,omitempty"`
	SigningTime              string `json:"signing_time,omitempty"`
	HashAlgorithm            string `json:"hash_algorithm,omitempty"`
	Type                     string `json:"type,omitempty"`
	CertificateStatus        string `json:"certificate_status,omitempty"`
	CertificateTrusted       bool   `json:"certificate_trusted"`
	CertificateExpired       bool   `json:"certificate_expired"`
	CertificateIssuer        string `json:"certificate_issuer,omitempty"`
	CertificateSubject       string `json:"certificate_subject,omitempty"`
	CertificateNotBefore     string `json:"certificate_not_before,omitempty"`
	CertificateNotAfter      string `json:"certificate_not_after,omitempty"`
	ICPBrasilDetected        bool   `json:"icp_brasil_detected"`
	SigningTimeValidityChecked bool `json:"signing_time_validity_checked"`
	SigningTimeWithinCertificateValidity bool `json:"signing_time_within_certificate_validity"`
	SignatureCount           int    `json:"signature_count,omitempty"`
}

type Integrity struct {
	FileSizeBytes                    int    `json:"file_size_bytes"`
	SHA256                           string `json:"sha256"`
	ByteRangeCount                   int    `json:"byte_range_count"`
	SignedRevisionCoversFile         bool   `json:"signed_revision_covers_file"`
	SignedPDFRevisionComplete        bool   `json:"signed_pdf_revision_complete"`
	UnsignedBytesAfterSignature     bool   `json:"unsigned_bytes_after_signature"`
	TrailingUnsignedBytes           int    `json:"trailing_unsigned_bytes"`
	TrailingDataType                string `json:"trailing_data_type,omitempty"`
	NonPDFTrailingData              bool   `json:"non_pdf_trailing_data"`
	ActivePDFChangesAfterSignature  bool   `json:"active_pdf_changes_after_signature"`
	IncrementalUpdateCount          int    `json:"incremental_update_count"`
	JavaScriptPresent               bool   `json:"javascript_present"`
	OpenActionPresent               bool   `json:"open_action_present"`
	EmbeddedFilesPresent            bool   `json:"embedded_files_present"`
	AcroFormPresent                 bool   `json:"acroform_present"`
	SignatureDictionaryDetected     bool   `json:"signature_dictionary_detected"`
}

type Consistency struct {
	CDTGeneratorDetected          bool   `json:"cdt_generator_detected"`
	CreationModificationSame      bool   `json:"creation_modification_same"`
	QRCreatedAt                   string `json:"qr_created_at,omitempty"`
	PDFCreatedAt                  string `json:"pdf_created_at,omitempty"`
	CreationDeltaSeconds          int64  `json:"creation_delta_seconds,omitempty"`
	PhotoComparisonAvailable      bool   `json:"photo_comparison_available"`
	TextEvidenceChecked           bool   `json:"text_evidence_checked"`
	VisibleDigitalSignatureClaim  bool   `json:"visible_digital_signature_claim"`
	SerproValidationReference     bool   `json:"serpro_validation_reference"`
	SignatureClaimMatchesPDF      bool   `json:"signature_claim_matches_pdf"`
}

type Photo struct {
	Available  bool   `json:"available"`
	Page       int    `json:"page,omitempty"`
	Method     string `json:"method,omitempty"`
	Confidence string `json:"confidence,omitempty"`
	MIME       string `json:"mime,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
	Bytes      []byte `json:"-"`
}

type SourceIntegrity struct {
	Status string `json:"status"`
}

type Result struct {
	Metadata        Metadata        `json:"metadata"`
	Signature       Signature       `json:"digital_signature"`
	Integrity       Integrity       `json:"integrity"`
	SourceIntegrity SourceIntegrity `json:"source_integrity"`
	Consistency     Consistency     `json:"consistency"`
	Images          ImageAnalysis   `json:"images"`
	Photo           Photo           `json:"-"`
	Warnings        []string        `json:"warnings,omitempty"`
}

type Assessment struct {
	Status   string   `json:"status"`
	Warnings []string `json:"warnings,omitempty"`
}

var byteRangePattern = regexp.MustCompile(`(?s)/ByteRange\s*\[\s*(\d+)\s+(\d+)\s+(\d+)\s+(\d+)\s*\]`)

func Analyze(ctx context.Context, pdf []byte, renderedPage []byte, pageNumber int, documentName string, qrCreatedAt time.Time) Result {
	result := Result{}
	result.Integrity = analyzeIntegrity(pdf)

	if len(pdf) < 5 || string(pdf[:5]) != "%PDF-" {
		result.Warnings = append(result.Warnings, "arquivo não possui cabeçalho PDF reconhecido")
		return result
	}

	dir, err := os.MkdirTemp("", "faceproof-pdf-analysis-*")
	if err != nil {
		result.Warnings = append(result.Warnings, "não foi possível preparar análise externa do PDF")
		return result
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "document.pdf")
	if err := os.WriteFile(path, pdf, 0600); err != nil {
		result.Warnings = append(result.Warnings, "não foi possível preparar análise externa do PDF")
		return result
	}

	if output, err := runTool(ctx, "pdfinfo", path); err == nil {
		result.Metadata = parsePDFInfo(output)
	} else if errors.Is(err, exec.ErrNotFound) {
		result.Warnings = append(result.Warnings, "pdfinfo indisponível; metadados avançados não analisados")
	} else {
		result.Warnings = append(result.Warnings, "pdfinfo não conseguiu analisar completamente o arquivo")
	}

	if output, err := runTool(ctx, "pdfsig", path); err == nil {
		result.Signature = mergeSignature(result.Signature, parsePDFSig(output))
	} else if errors.Is(err, exec.ErrNotFound) {
		result.Warnings = append(result.Warnings, "pdfsig indisponível; assinatura digital do PDF não validada")
	} else {
		if strings.TrimSpace(output) != "" {
			result.Signature = mergeSignature(result.Signature, parsePDFSig(output))
		}
		if !pdfsigNoSignatures(output) {
			result.Warnings = append(result.Warnings, "pdfsig não conseguiu validar completamente a assinatura do PDF")
		}
	}
	if result.Signature.Present {
		if cert, err := extractCertificateDetails(ctx, path, result.Signature.SigningTime); err == nil {
			result.Signature = mergeCertificateDetails(result.Signature, cert)
		} else if errors.Is(err, exec.ErrNotFound) {
			result.Warnings = append(result.Warnings, "openssl ou pdfsig -dump indisponível; período do certificado não analisado")
		}
	}

	result.Consistency = analyzeConsistency(result.Metadata, qrCreatedAt)
	if textEvidence, err := extractPDFTextEvidence(ctx, path); err == nil {
		result.Consistency.TextEvidenceChecked = true
		result.Consistency.VisibleDigitalSignatureClaim = textEvidence.SignatureClaim
		result.Consistency.SerproValidationReference = textEvidence.SerproReference
		result.Consistency.SignatureClaimMatchesPDF = !textEvidence.SignatureClaim || result.Signature.Present
	} else if errors.Is(err, exec.ErrNotFound) {
		result.Warnings = append(result.Warnings, "pdftotext indisponível; alegações textuais de assinatura não analisadas")
	}
	imageAnalysis, imagePhoto, imageErr := analyzeEmbeddedImages(ctx, path, pageNumber, isCNH(documentName))
	if imageErr == nil {
		result.Images = imageAnalysis
		if imagePhoto.Available {
			result.Photo = imagePhoto
			result.Consistency.PhotoComparisonAvailable = true
		}
	} else if errors.Is(imageErr, exec.ErrNotFound) {
		result.Warnings = append(result.Warnings, "pdfimages indisponível; fingerprint das imagens internas não analisado")
	} else {
		result.Warnings = append(result.Warnings, "estrutura das imagens internas do PDF não pôde ser analisada")
	}

	if isCNH(documentName) && !result.Photo.Available && renderedPage != nil {
		if photo, photoErr := extractCNHDigitalPhoto(renderedPage, pageNumber); photoErr == nil {
			result.Photo = photo
			result.Consistency.PhotoComparisonAvailable = true
		} else {
			result.Warnings = append(result.Warnings, "foto visual da CNH não pôde ser extraída do PDF")
		}
	}

	if result.Integrity.JavaScriptPresent || result.Integrity.OpenActionPresent {
		result.Warnings = append(result.Warnings, "PDF contém ação ativa ou JavaScript")
	}
	if result.Consistency.TextEvidenceChecked && result.Consistency.VisibleDigitalSignatureClaim &&
		result.Signature.PresenceChecked && !result.Signature.Present {
		result.Warnings = append(result.Warnings, "o conteúdo do documento afirma assinatura digital, mas o PDF não contém assinatura criptográfica")
	}
	if result.Signature.PresenceChecked && !result.Signature.Present &&
		(result.Integrity.ByteRangeCount > 0 || result.Integrity.SignatureDictionaryDetected) {
		result.Warnings = append(result.Warnings, "o PDF contém estruturas de assinatura, mas nenhuma assinatura digital válida foi reconhecida")
	}
	if result.Signature.Present && result.Integrity.ActivePDFChangesAfterSignature {
		result.Warnings = append(result.Warnings, "foram detectadas alterações PDF ativas após a última revisão assinada")
	} else if result.Signature.Present && result.Integrity.NonPDFTrailingData {
		result.Warnings = append(result.Warnings, "há dados externos ao PDF anexados após a revisão assinada")
	} else if result.Signature.Present && result.Integrity.UnsignedBytesAfterSignature {
		result.Warnings = append(result.Warnings, "foram encontrados bytes não classificados após a última revisão assinada")
	}
	if result.Signature.CertificateExpired && result.Signature.CryptographicallyValid {
		result.Warnings = append(result.Warnings, "o certificado está expirado na validação atual, mas a assinatura criptográfica do PDF é válida")
	}
	result.SourceIntegrity = SourceIntegrity{Status: sourceIntegrityStatus(result.Signature, result.Integrity)}
	return result
}

func sourceIntegrityStatus(signature Signature, integrity Integrity) string {
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

func Assess(qrSignatureValid bool, pdf *Result) Assessment {
	if !qrSignatureValid {
		return Assessment{Status: "invalid", Warnings: []string{"assinatura criptográfica do QR VIO não foi validada"}}
	}
	if pdf == nil {
		return Assessment{Status: "partially_verified", Warnings: []string{"entrada não era PDF; validação limitada ao QR VIO"}}
	}

	warnings := append([]string(nil), pdf.Warnings...)
	if pdf.Consistency.TextEvidenceChecked && pdf.Consistency.VisibleDigitalSignatureClaim &&
		pdf.Signature.PresenceChecked && !pdf.Signature.Present {
		return Assessment{Status: "inconsistent", Warnings: append(warnings, "o documento afirma assinatura digital, mas o PDF não contém assinatura criptográfica")}
	}
	if pdf.Signature.PresenceChecked && !pdf.Signature.Present &&
		(pdf.Integrity.ByteRangeCount > 0 || pdf.Integrity.SignatureDictionaryDetected) {
		return Assessment{Status: "inconsistent", Warnings: append(warnings, "o PDF contém estruturas de assinatura, mas nenhuma assinatura digital foi reconhecida")}
	}
	if pdf.Signature.Present && pdf.Signature.ValidationChecked && !pdf.Signature.CryptographicallyValid {
		return Assessment{Status: "inconsistent", Warnings: append(warnings, "assinatura digital do PDF é inválida")}
	}
	if pdf.Signature.Present && pdf.Integrity.ActivePDFChangesAfterSignature {
		return Assessment{Status: "inconsistent", Warnings: append(warnings, "foram detectadas alterações PDF ativas após a revisão assinada")}
	}
	if pdf.Signature.Present && pdf.Integrity.UnsignedBytesAfterSignature && pdf.Integrity.TrailingDataType == "binary_unknown" {
		return Assessment{Status: "partially_verified", Warnings: append(warnings, "há dados binários não classificados após o fim da revisão assinada")}
	}
	if certificateTrustFailure(pdf.Signature.CertificateStatus) {
		return Assessment{Status: "inconsistent", Warnings: append(warnings, "cadeia do certificado do PDF não foi considerada confiável")}
	}
	if pdf.Signature.SigningTimeValidityChecked && !pdf.Signature.SigningTimeWithinCertificateValidity {
		return Assessment{Status: "inconsistent", Warnings: append(warnings, "a assinatura do PDF foi feita fora do período de validade do certificado")}
	}
	if pdf.Signature.Present && pdf.Signature.CryptographicallyValid &&
		(pdf.Integrity.SignedRevisionCoversFile || (pdf.Integrity.SignedPDFRevisionComplete && pdf.Integrity.NonPDFTrailingData)) {
		if strings.TrimSpace(pdf.Signature.CertificateStatus) == "" {
			return Assessment{Status: "partially_verified", Warnings: append(warnings, "assinatura válida, mas a cadeia do certificado não teve status conclusivo")}
		}
		if pdf.Integrity.NonPDFTrailingData {
			warnings = append(warnings, "há dados externos ao PDF após a revisão assinada; eles não alteram a estrutura PDF assinada")
		}
		return Assessment{Status: "verified", Warnings: warnings}
	}
	if !pdf.Signature.Present {
		warnings = append(warnings, "PDF sem assinatura digital detectada")
	}
	return Assessment{Status: "partially_verified", Warnings: warnings}
}

func analyzeIntegrity(data []byte) Integrity {
	sum := sha256.Sum256(data)
	result := Integrity{
		FileSizeBytes:        len(data),
		SHA256:               hex.EncodeToString(sum[:]),
		JavaScriptPresent:    bytes.Contains(data, []byte("/JavaScript")) || bytes.Contains(data, []byte("/JS")),
		OpenActionPresent:    bytes.Contains(data, []byte("/OpenAction")) || bytes.Contains(data, []byte("/AA")),
		EmbeddedFilesPresent: bytes.Contains(data, []byte("/EmbeddedFiles")) || bytes.Contains(data, []byte("/Filespec")),
		AcroFormPresent:             bytes.Contains(data, []byte("/AcroForm")),
		SignatureDictionaryDetected: bytes.Contains(data, []byte("/Type /Sig")),
	}

	matches := byteRangePattern.FindAllSubmatch(data, -1)
	result.ByteRangeCount = len(matches)
	maxEnd := int64(0)
	for _, match := range matches {
		if len(match) != 5 {
			continue
		}
		offset2, err1 := strconv.ParseInt(string(match[3]), 10, 64)
		length2, err2 := strconv.ParseInt(string(match[4]), 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		if signedEnd := offset2 + length2; signedEnd > maxEnd {
			maxEnd = signedEnd
		}
	}

	if maxEnd > 0 && maxEnd <= int64(len(data)) {
		signedRevision := data[:maxEnd]
		trimmedSigned := bytes.TrimSpace(signedRevision)
		result.SignedPDFRevisionComplete = bytes.HasSuffix(trimmedSigned, []byte("%%EOF"))

		trailing := data[maxEnd:]
		trimmedTrailing := bytes.TrimSpace(trailing)
		result.SignedRevisionCoversFile = len(trimmedTrailing) == 0
		result.UnsignedBytesAfterSignature = !result.SignedRevisionCoversFile
		result.TrailingUnsignedBytes = len(trimmedTrailing)

		if len(trimmedTrailing) > 0 {
			result.TrailingDataType, result.NonPDFTrailingData, result.ActivePDFChangesAfterSignature = classifyTrailingData(trimmedTrailing)
		}
	}

	eofCount := bytes.Count(data, []byte("%%EOF"))
	if eofCount > 1 {
		result.IncrementalUpdateCount = eofCount - 1
	}
	if result.UnsignedBytesAfterSignature && result.IncrementalUpdateCount > 0 {
		result.ActivePDFChangesAfterSignature = true
		if result.TrailingDataType == "" || result.TrailingDataType == "binary_unknown" {
			result.TrailingDataType = "pdf_incremental_update"
		}
	}
	return result
}

func classifyTrailingData(data []byte) (kind string, nonPDF bool, activePDFChange bool) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return "", false, false
	}

	lower := bytes.ToLower(trimmed)
	if bytes.HasPrefix(lower, []byte("<?xml")) || bytes.HasPrefix(lower, []byte("<metadata")) || bytes.HasPrefix(lower, []byte("<metadados")) {
		return "non_pdf_xml", true, false
	}

	pdfTokens := [][]byte{
		[]byte(" obj"),
		[]byte("\nobj"),
		[]byte("\rxref"),
		[]byte("\nxref"),
		[]byte("trailer"),
		[]byte("startxref"),
		[]byte("/type"),
		[]byte("/page"),
		[]byte("/annots"),
		[]byte("/acroform"),
		[]byte("stream"),
		[]byte("%%eof"),
	}
	for _, token := range pdfTokens {
		if bytes.Contains(lower, token) {
			return "pdf_incremental_update", false, true
		}
	}

	printable := 0
	for _, value := range trimmed {
		if value == '\n' || value == '\r' || value == '\t' || (value >= 0x20 && value <= 0x7e) {
			printable++
		}
	}
	if printable == len(trimmed) {
		return "non_pdf_text", true, false
	}
	return "binary_unknown", false, false
}

func runTool(parent context.Context, name string, path string) (string, error) {
	binary, err := exec.LookPath(name)
	if err != nil {
		return "", exec.ErrNotFound
	}
	ctx, cancel := context.WithTimeout(parent, commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, path)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return string(output), ctx.Err()
	}
	if err != nil {
		return string(output), err
	}
	return string(output), nil
}

type certificateDetails struct {
	Issuer      string
	Subject     string
	NotBefore   string
	NotAfter    string
	ICPBrasil   bool
	PeriodChecked bool
	SigningTimeWithinPeriod bool
}

func extractCertificateDetails(parent context.Context, pdfPath string, signingTime string) (certificateDetails, error) {
	pdfsig, err := exec.LookPath("pdfsig")
	if err != nil {
		return certificateDetails{}, exec.ErrNotFound
	}
	openssl, err := exec.LookPath("openssl")
	if err != nil {
		return certificateDetails{}, exec.ErrNotFound
	}

	dir, err := os.MkdirTemp("", "faceproof-pdf-cert-*")
	if err != nil {
		return certificateDetails{}, err
	}
	defer os.RemoveAll(dir)

	copyPath := filepath.Join(dir, "document.pdf")
	data, err := os.ReadFile(pdfPath)
	if err != nil {
		return certificateDetails{}, err
	}
	if err := os.WriteFile(copyPath, data, 0600); err != nil {
		return certificateDetails{}, err
	}

	dumpCtx, cancel := context.WithTimeout(parent, commandTimeout)
	defer cancel()
	dump := exec.CommandContext(dumpCtx, pdfsig, "-dump", copyPath)
	dump.Dir = dir
	dump.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	if output, err := dump.CombinedOutput(); err != nil {
		if dumpCtx.Err() != nil {
			return certificateDetails{}, dumpCtx.Err()
		}
		return certificateDetails{}, fmt.Errorf("pdfsig -dump: %v: %s", err, strings.TrimSpace(string(output)))
	}

	signatures, err := filepath.Glob(filepath.Join(dir, "document.pdf.sig*"))
	if err != nil || len(signatures) == 0 {
		return certificateDetails{}, errors.New("assinatura PKCS#7 não encontrada")
	}
	sort.Strings(signatures)

	opensslCtx, cancelOpenSSL := context.WithTimeout(parent, commandTimeout)
	defer cancelOpenSSL()
	cmd := exec.CommandContext(opensslCtx, openssl, "pkcs7", "-inform", "DER", "-in", signatures[0], "-print_certs", "-text", "-noout")
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	output, err := cmd.CombinedOutput()
	if opensslCtx.Err() != nil {
		return certificateDetails{}, opensslCtx.Err()
	}
	if err != nil {
		return certificateDetails{}, fmt.Errorf("openssl pkcs7: %v: %s", err, strings.TrimSpace(string(output)))
	}

	details := parseCertificateText(string(output))
	details.ICPBrasil = containsFold(details.Issuer, "ICP-Brasil") || containsFold(details.Subject, "ICP-Brasil")
	if signedAt, ok := parsePDFSigTime(signingTime); ok {
		if notBefore, okBefore := parseCertificateTime(details.NotBefore); okBefore {
			if notAfter, okAfter := parseCertificateTime(details.NotAfter); okAfter {
				details.PeriodChecked = true
				details.SigningTimeWithinPeriod = !signedAt.Before(notBefore) && !signedAt.After(notAfter)
			}
		}
	}
	return details, nil
}

func parseCertificateText(output string) certificateDetails {
	var details certificateDetails
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case strings.HasPrefix(line, "Issuer:"):
			details.Issuer = strings.TrimSpace(strings.TrimPrefix(line, "Issuer:"))
		case strings.HasPrefix(line, "Subject:"):
			details.Subject = strings.TrimSpace(strings.TrimPrefix(line, "Subject:"))
		case strings.HasPrefix(line, "Not Before:"):
			details.NotBefore = strings.TrimSpace(strings.TrimPrefix(line, "Not Before:"))
		case strings.HasPrefix(line, "Not After :"):
			details.NotAfter = strings.TrimSpace(strings.TrimPrefix(line, "Not After :"))
		case strings.HasPrefix(line, "Not After:"):
			details.NotAfter = strings.TrimSpace(strings.TrimPrefix(line, "Not After:"))
		}
	}
	return details
}

func mergeCertificateDetails(signature Signature, details certificateDetails) Signature {
	signature.CertificateIssuer = details.Issuer
	signature.CertificateSubject = details.Subject
	signature.CertificateNotBefore = details.NotBefore
	signature.CertificateNotAfter = details.NotAfter
	signature.ICPBrasilDetected = details.ICPBrasil
	signature.SigningTimeValidityChecked = details.PeriodChecked
	signature.SigningTimeWithinCertificateValidity = details.SigningTimeWithinPeriod
	return signature
}

func parsePDFSigTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"Jan 02 2006 15:04:05", "Jan 2 2006 15:04:05", time.RFC3339} {
		if parsed, err := time.ParseInLocation(layout, value, time.UTC); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func parseCertificateTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"Jan 2 15:04:05 2006 MST", "Jan 02 15:04:05 2006 MST"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func certificateTrustFailure(status string) bool {
	value := strings.ToLower(strings.TrimSpace(status))
	return strings.Contains(value, "not trusted") ||
		strings.Contains(value, "untrusted") ||
		strings.Contains(value, "unknown issuer") ||
		strings.Contains(value, "issuer isn't trusted") ||
		strings.Contains(value, "issuer is not trusted")
}

func parsePDFInfo(output string) Metadata {
	values := parseColonLines(output)
	meta := Metadata{
		Title:            values["Title"],
		Creator:          values["Creator"],
		Producer:         values["Producer"],
		CreationDate:     values["CreationDate"],
		ModificationDate: values["ModDate"],
		PageSize:         values["Page size"],
		PDFVersion:       values["PDF version"],
	}
	if fields := strings.Fields(values["Pages"]); len(fields) > 0 {
		meta.Pages, _ = strconv.Atoi(fields[0])
	}
	meta.Encrypted = strings.EqualFold(strings.TrimSpace(values["Encrypted"]), "yes")
	return meta
}

func pdfsigNoSignatures(output string) bool {
	value := strings.ToLower(strings.TrimSpace(output))
	return strings.Contains(value, "does not contain any signatures") ||
		strings.Contains(value, "no signatures")
}

func parsePDFSig(output string) Signature {
	values := parseColonLines(output)
	signature := Signature{PresenceChecked: true}
	signature.SignatureCount = strings.Count(output, "Signature #")
	signature.Present = signature.SignatureCount > 0 || strings.Contains(output, "Signer Certificate")
	signature.Signer = firstNonEmpty(values,
		"Signer Certificate Common Name",
		"Signer Certificate Subject Common Name",
		"Signer",
	)
	signature.SignerDistinguishedName = firstNonEmpty(values,
		"Signer full Distinguished Name",
		"Signer Certificate Subject Distinguished Name",
	)
	signature.SigningTime = values["Signing Time"]
	signature.HashAlgorithm = firstNonEmpty(values, "Signing Hash Algorithm", "Hash Algorithm")
	signature.Type = firstNonEmpty(values, "Signature Type", "SubFilter")
	validation := firstNonEmpty(values, "Signature Validation", "Signature Validation Status")
	if validation != "" {
		signature.ValidationChecked = true
		lower := strings.ToLower(validation)
		signature.CryptographicallyValid = strings.Contains(lower, "valid") && !strings.Contains(lower, "invalid")
	}
	signature.CertificateStatus = firstNonEmpty(values, "Certificate Validation", "Certificate Validation Status")
	certLower := strings.ToLower(signature.CertificateStatus)
	signature.CertificateTrusted = strings.Contains(certLower, "trusted") && !strings.Contains(certLower, "not trusted")
	signature.CertificateExpired = strings.Contains(certLower, "expired")
	return signature
}

func mergeSignature(base Signature, parsed Signature) Signature {
	if parsed.PresenceChecked {
		base.PresenceChecked = true
	}
	if parsed.Present {
		base.Present = true
	}
	if parsed.SignatureCount > 0 {
		base.SignatureCount = parsed.SignatureCount
	}
	if parsed.ValidationChecked {
		base.ValidationChecked = true
		base.CryptographicallyValid = parsed.CryptographicallyValid
	}
	if parsed.Signer != "" {
		base.Signer = parsed.Signer
	}
	if parsed.SignerDistinguishedName != "" {
		base.SignerDistinguishedName = parsed.SignerDistinguishedName
	}
	if parsed.SigningTime != "" {
		base.SigningTime = parsed.SigningTime
	}
	if parsed.HashAlgorithm != "" {
		base.HashAlgorithm = parsed.HashAlgorithm
	}
	if parsed.Type != "" {
		base.Type = parsed.Type
	}
	if parsed.CertificateStatus != "" {
		base.CertificateStatus = parsed.CertificateStatus
		base.CertificateTrusted = parsed.CertificateTrusted
		base.CertificateExpired = parsed.CertificateExpired
	}
	return base
}

func parseColonLines(output string) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			values[key] = value
		}
	}
	return values
}

func firstNonEmpty(values map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(values[key]); value != "" {
			return value
		}
	}
	return ""
}

type pdfTextEvidence struct {
	SignatureClaim  bool
	SerproReference bool
}

func extractPDFTextEvidence(parent context.Context, pdfPath string) (pdfTextEvidence, error) {
	binary, err := exec.LookPath("pdftotext")
	if err != nil {
		return pdfTextEvidence{}, exec.ErrNotFound
	}
	ctx, cancel := context.WithTimeout(parent, commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, "-layout", pdfPath, "-")
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return pdfTextEvidence{}, ctx.Err()
	}
	if err != nil {
		return pdfTextEvidence{}, err
	}

	text := strings.ToLower(string(output))
	return pdfTextEvidence{
		SignatureClaim: strings.Contains(text, "documento assinado com certificado digital") ||
			strings.Contains(text, "assinado digitalmente"),
		SerproReference: strings.Contains(text, "assinador serpro") ||
			strings.Contains(text, "serpro.gov.br/assinador-digital"),
	}, nil
}

func analyzeConsistency(meta Metadata, qrCreatedAt time.Time) Consistency {
	result := Consistency{
		CDTGeneratorDetected:     containsFold(meta.Creator, "CDT") || containsFold(meta.Producer, "CDT"),
		CreationModificationSame: meta.CreationDate != "" && meta.CreationDate == meta.ModificationDate,
	}
	if !qrCreatedAt.IsZero() {
		result.QRCreatedAt = qrCreatedAt.Format(time.RFC3339)
	}
	if parsed, ok := parsePDFInfoDate(meta.CreationDate); ok {
		result.PDFCreatedAt = parsed.Format(time.RFC3339)
		if !qrCreatedAt.IsZero() {
			delta := parsed.Sub(qrCreatedAt)
			if delta < 0 {
				delta = -delta
			}
			result.CreationDeltaSeconds = int64(delta.Seconds())
		}
	}
	return result
}

func parsePDFInfoDate(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	layouts := []string{
		"Mon Jan 2 15:04:05 2006 MST",
		"Mon Jan 02 15:04:05 2006 MST",
		"Mon Jan 2 15:04:05 2006 -07",
		"Mon Jan 02 15:04:05 2006 -07",
		"Mon Jan 2 15:04:05 2006 -0700",
		"Mon Jan 02 15:04:05 2006 -0700",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func extractCNHDigitalPhotoFromPDF(parent context.Context, pdfPath string, pageNumber int) (Photo, error) {
	binary, err := exec.LookPath("pdfimages")
	if err != nil {
		return Photo{}, err
	}
	ctx, cancel := context.WithTimeout(parent, commandTimeout)
	defer cancel()

	dir := filepath.Dir(pdfPath)
	prefix := filepath.Join(dir, "embedded")
	cmd := exec.CommandContext(
		ctx,
		binary,
		"-f", strconv.Itoa(pageNumber),
		"-l", strconv.Itoa(pageNumber),
		"-all",
		pdfPath,
		prefix,
	)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	if output, err := cmd.CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return Photo{}, ctx.Err()
		}
		return Photo{}, fmt.Errorf("pdfimages: %v: %s", err, strings.TrimSpace(string(output)))
	}

	files, err := filepath.Glob(prefix + "-*")
	if err != nil {
		return Photo{}, err
	}
	sort.Strings(files)

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			continue
		}
		b := img.Bounds()
		width, height := b.Dx(), b.Dy()
		if width < 600 || height < 400 {
			continue
		}
		ratio := float64(width) / float64(height)
		if ratio < 1.20 || ratio > 1.60 {
			continue
		}

		x1 := b.Min.X + int(float64(width)*0.18)
		x2 := b.Min.X + int(float64(width)*0.37)
		y1 := b.Min.Y + int(float64(height)*0.38)
		y2 := b.Min.Y + int(float64(height)*0.78)
		if x2 <= x1 || y2 <= y1 || x2 > b.Max.X || y2 > b.Max.Y {
			continue
		}

		dst := image.NewRGBA(image.Rect(0, 0, x2-x1, y2-y1))
		draw.Draw(dst, dst.Bounds(), img, image.Point{X: x1, Y: y1}, draw.Src)
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, dst); err != nil {
			continue
		}
		return Photo{
			Available:  true,
			Page:       pageNumber,
			Method:     "pdfimages_cnh_front_crop_v1",
			Confidence: "layout",
			MIME:       "image/png",
			Bytes:      encoded.Bytes(),
		}, nil
	}
	return Photo{}, errors.New("imagem frontal da CNH não encontrada no PDF")
}

func extractCNHDigitalPhoto(pagePNG []byte, pageNumber int) (Photo, error) {
	img, _, err := image.Decode(bytes.NewReader(pagePNG))
	if err != nil {
		return Photo{}, err
	}
	b := img.Bounds()
	width, height := b.Dx(), b.Dy()
	if width < 400 || height < 500 {
		return Photo{}, fmt.Errorf("página renderizada pequena demais")
	}
	ratio := float64(width) / float64(height)
	if ratio < 0.62 || ratio > 0.82 {
		return Photo{}, fmt.Errorf("layout de página inesperado")
	}

	// Current CNH Digital PDF layout: portrait is in the upper-left document
	// panel. Coordinates are normalized so the crop scales with Poppler output.
	// This is visual evidence only; it is never used as proof of authenticity.
	x1 := b.Min.X + int(float64(width)*0.108)
	x2 := b.Min.X + int(float64(width)*0.205)
	y1 := b.Min.Y + int(float64(height)*0.155)
	y2 := b.Min.Y + int(float64(height)*0.255)
	if x2 <= x1 || y2 <= y1 || x2 > b.Max.X || y2 > b.Max.Y {
		return Photo{}, fmt.Errorf("recorte fora da página")
	}

	dst := image.NewRGBA(image.Rect(0, 0, x2-x1, y2-y1))
	draw.Draw(dst, dst.Bounds(), img, image.Point{X: x1, Y: y1}, draw.Src)

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, dst); err != nil {
		return Photo{}, err
	}
	photoBytes := encoded.Bytes()
	sum := sha256.Sum256(photoBytes)
	return Photo{
		Available:  true,
		Page:       pageNumber,
		Method:     "cnh_digital_page_crop_v1",
		Confidence: "heuristic",
		MIME:       "image/png",
		Width:      x2 - x1,
		Height:     y2 - y1,
		SHA256:     hex.EncodeToString(sum[:]),
		Bytes:      photoBytes,
	}, nil
}

func isCNH(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	return strings.Contains(normalized, "cnh") || strings.Contains(normalized, "habilita")
}

func containsFold(value, token string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(token))
}
