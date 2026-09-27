package pdfintegrity

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalidPDF      = errors.New("invalid PDF")
	ErrToolUnavailable = errors.New("PDF validation tool unavailable")
)

const (
	defaultCommandTimeout = 20 * time.Second
	metadataTimeTolerance = 10 * time.Minute
)

type Inspector struct {
	pdfsigPath  string
	pdfinfoPath string
}

type Signature struct {
	FieldName             string    `json:"fieldName"`
	SignerCommonName      string    `json:"signerCommonName"`
	SignerDistinguishedName string  `json:"signerDistinguishedName"`
	SigningTime           time.Time `json:"signingTime"`
	HashAlgorithm         string    `json:"hashAlgorithm"`
	SignatureType         string    `json:"signatureType"`
	SignedRanges          string    `json:"signedRanges"`
	TotalDocumentSigned   bool      `json:"totalDocumentSigned"`
	SignatureValid        bool      `json:"signatureValid"`
	CertificateValidation string    `json:"certificateValidation"`
}

type Metadata struct {
	Title          string    `json:"title"`
	Creator        string    `json:"creator"`
	Producer       string    `json:"producer"`
	CreationTime   time.Time `json:"creationTime"`
	ModificationTime time.Time `json:"modificationTime"`
	JavaScript     string    `json:"javascript"`
	Encrypted      string    `json:"encrypted"`
	Suspects       string    `json:"suspects"`
	Form           string    `json:"form"`
	Pages          int       `json:"pages"`
	FileSize       int64     `json:"fileSize"`
	PDFVersion     string    `json:"pdfVersion"`
}

type Result struct {
	Valid       bool      `json:"valid"`
	Signature   Signature `json:"signature"`
	Metadata    Metadata  `json:"metadata"`
	Diagnostics []string  `json:"diagnostics"`
}

func NewInspector(pdfsigPath, pdfinfoPath string) *Inspector {
	return &Inspector{
		pdfsigPath:  strings.TrimSpace(pdfsigPath),
		pdfinfoPath: strings.TrimSpace(pdfinfoPath),
	}
}

func (inspector *Inspector) Ready() error {
	if _, err := resolveExecutable(inspector.pdfsigPath, "pdfsig"); err != nil {
		return err
	}
	if _, err := resolveExecutable(inspector.pdfinfoPath, "pdfinfo"); err != nil {
		return err
	}
	return nil
}

func (inspector *Inspector) Inspect(ctx context.Context, data []byte) (Result, error) {
	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		return Result{}, ErrInvalidPDF
	}

	pdfsig, err := resolveExecutable(inspector.pdfsigPath, "pdfsig")
	if err != nil {
		return Result{}, err
	}
	pdfinfo, err := resolveExecutable(inspector.pdfinfoPath, "pdfinfo")
	if err != nil {
		return Result{}, err
	}

	directory, err := os.MkdirTemp("", "faceproof-cnh-pdf-*")
	if err != nil {
		return Result{}, fmt.Errorf("create temporary PDF directory: %w", err)
	}
	defer os.RemoveAll(directory)

	path := filepath.Join(directory, "document.pdf")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return Result{}, fmt.Errorf("write temporary PDF: %w", err)
	}

	commandCtx, cancel := context.WithTimeout(ctx, defaultCommandTimeout)
	defer cancel()

	infoOutput, infoErr := run(commandCtx, pdfinfo, path)
	if infoErr != nil {
		return Result{}, fmt.Errorf("%w: pdfinfo failed", ErrInvalidPDF)
	}

	signatureOutput, signatureErr := run(commandCtx, pdfsig, path)
	if signatureErr != nil && strings.TrimSpace(signatureOutput) == "" {
		return Result{}, fmt.Errorf("%w: pdfsig failed", ErrInvalidPDF)
	}

	metadata := parsePDFInfo(signatureSafeString(infoOutput))
	signatures := parsePDFSig(signatureSafeString(signatureOutput))
	result := evaluate(data, metadata, signatures, time.Now().UTC())
	return result, nil
}

func run(ctx context.Context, executable string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, executable, args...)
	command.Env = append(os.Environ(), "LC_ALL=C", "LANG=C", "TZ=UTC")
	output, err := command.CombinedOutput()
	return string(output), err
}

func resolveExecutable(configured, fallback string) (string, error) {
	candidate := strings.TrimSpace(configured)
	if candidate == "" {
		candidate = fallback
	}

	if filepath.IsAbs(candidate) || strings.ContainsRune(candidate, filepath.Separator) {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			return "", fmt.Errorf("%w: %s", ErrToolUnavailable, fallback)
		}
		return candidate, nil
	}

	path, err := exec.LookPath(candidate)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrToolUnavailable, fallback)
	}
	return path, nil
}

func parsePDFSig(output string) []Signature {
	var signatures []Signature
	var current *Signature

	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "Signature #") && strings.HasSuffix(line, ":") {
			signatures = append(signatures, Signature{})
			current = &signatures[len(signatures)-1]
			continue
		}
		if current == nil || line == "" {
			continue
		}

		if line == "- Total document signed" || line == "Total document signed" {
			current.TotalDocumentSigned = true
			continue
		}

		line = strings.TrimPrefix(line, "- ")
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		switch key {
		case "Signature Field Name":
			current.FieldName = value
		case "Signer Certificate Common Name":
			current.SignerCommonName = value
		case "Signer full Distinguished Name":
			current.SignerDistinguishedName = value
		case "Signing Time":
			current.SigningTime = parsePDFSigTime(value)
		case "Signing Hash Algorithm":
			current.HashAlgorithm = value
		case "Signature Type":
			current.SignatureType = value
		case "Signed Ranges":
			current.SignedRanges = value
		case "Signature Validation":
			current.SignatureValid = strings.Contains(strings.ToLower(value), "signature is valid")
		case "Certificate Validation":
			current.CertificateValidation = value
		}
	}

	return signatures
}

func parsePDFInfo(output string) Metadata {
	var metadata Metadata

	for _, rawLine := range strings.Split(output, "\n") {
		key, value, found := strings.Cut(rawLine, ":")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		switch key {
		case "Title":
			metadata.Title = value
		case "Creator":
			metadata.Creator = value
		case "Producer":
			metadata.Producer = value
		case "CreationDate":
			metadata.CreationTime = parsePDFInfoTime(value)
		case "ModDate":
			metadata.ModificationTime = parsePDFInfoTime(value)
		case "JavaScript":
			metadata.JavaScript = value
		case "Encrypted":
			metadata.Encrypted = value
		case "Suspects":
			metadata.Suspects = value
		case "Form":
			metadata.Form = value
		case "Pages":
			metadata.Pages, _ = strconv.Atoi(value)
		case "File size":
			fields := strings.Fields(value)
			if len(fields) > 0 {
				metadata.FileSize, _ = strconv.ParseInt(fields[0], 10, 64)
			}
		case "PDF version":
			metadata.PDFVersion = value
		}
	}

	return metadata
}

func evaluate(data []byte, metadata Metadata, signatures []Signature, now time.Time) Result {
	result := Result{Metadata: metadata}
	var selected *Signature

	for index := range signatures {
		signature := signatures[index]
		if !signature.SignatureValid ||
			!signature.TotalDocumentSigned ||
			signature.SigningTime.IsZero() ||
			!acceptedHash(signature.HashAlgorithm) ||
			!acceptedSignatureType(signature.SignatureType) ||
			!acceptedCertificateStatus(signature.CertificateValidation) ||
			!officialCNHSigner(signature) ||
			signature.SigningTime.After(now.Add(5*time.Minute)) {
			continue
		}
		if selected == nil || signature.SigningTime.After(selected.SigningTime) {
			candidate := signature
			selected = &candidate
		}
	}

	if selected == nil {
		result.Diagnostics = append(result.Diagnostics, "no valid full-document DETRAN/SENATRAN signature was found")
		return result
	}
	result.Signature = *selected

	if !strings.EqualFold(strings.TrimSpace(metadata.Title), "CNH Digital") {
		result.Diagnostics = append(result.Diagnostics, "PDF title is not CNH Digital")
	}
	if !strings.EqualFold(strings.TrimSpace(metadata.Creator), "CDT") {
		result.Diagnostics = append(result.Diagnostics, "PDF creator is not CDT")
	}
	if !strings.EqualFold(strings.TrimSpace(metadata.Producer), "CDT") {
		result.Diagnostics = append(result.Diagnostics, "PDF producer is not CDT")
	}
	if !strings.EqualFold(strings.TrimSpace(metadata.JavaScript), "no") {
		result.Diagnostics = append(result.Diagnostics, "PDF contains JavaScript or JavaScript status is unknown")
	}
	if !strings.EqualFold(strings.TrimSpace(metadata.Encrypted), "no") {
		result.Diagnostics = append(result.Diagnostics, "PDF is encrypted or encryption status is unknown")
	}
	if strings.TrimSpace(metadata.Suspects) != "" && !strings.EqualFold(strings.TrimSpace(metadata.Suspects), "no") {
		result.Diagnostics = append(result.Diagnostics, "pdfinfo marked the PDF as suspicious")
	}
	if !strings.Contains(strings.ToLower(metadata.Form), "acroform") {
		result.Diagnostics = append(result.Diagnostics, "PDF signature form metadata is missing")
	}
	if metadata.Pages != 1 {
		result.Diagnostics = append(result.Diagnostics, "CNH Digital PDF must contain exactly one page")
	}
	if metadata.FileSize <= 0 || metadata.FileSize != int64(len(data)) {
		result.Diagnostics = append(result.Diagnostics, "PDF file size metadata is inconsistent")
	}
	if metadata.CreationTime.IsZero() || metadata.ModificationTime.IsZero() {
		result.Diagnostics = append(result.Diagnostics, "PDF creation/modification metadata is missing")
	} else {
		if durationAbs(selected.SigningTime.Sub(metadata.CreationTime)) > metadataTimeTolerance {
			result.Diagnostics = append(result.Diagnostics, "PDF creation time is inconsistent with signing time")
		}
		if durationAbs(selected.SigningTime.Sub(metadata.ModificationTime)) > metadataTimeTolerance {
			result.Diagnostics = append(result.Diagnostics, "PDF modification time is inconsistent with signing time")
		}
	}

	result.Valid = len(result.Diagnostics) == 0
	return result
}

func officialCNHSigner(signature Signature) bool {
	commonName := strings.ToUpper(strings.TrimSpace(signature.SignerCommonName))
	distinguishedName := strings.ToUpper(signature.SignerDistinguishedName)

	officialName := strings.Contains(commonName, "DETRAN") || strings.Contains(commonName, "SENATRAN")
	return officialName &&
		strings.Contains(distinguishedName, "SERPRO") &&
		strings.Contains(distinguishedName, "O=ICP-BRASIL") &&
		strings.Contains(distinguishedName, "C=BR")
}

func acceptedCertificateStatus(value string) bool {
	normalized := strings.ToLower(value)
	return strings.Contains(normalized, "trusted") || strings.Contains(normalized, "has expired")
}

func acceptedHash(value string) bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "SHA-256", "SHA-384", "SHA-512":
		return true
	default:
		return false
	}
}

func acceptedSignatureType(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(normalized, "pkcs7.detached") || strings.Contains(normalized, "cades.detached")
}

func parsePDFSigTime(value string) time.Time {
	parsed, err := time.ParseInLocation("Jan 02 2006 15:04:05", strings.TrimSpace(value), time.UTC)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func parsePDFInfoTime(value string) time.Time {
	parsed, err := time.Parse("Mon Jan _2 15:04:05 2006 MST", strings.TrimSpace(value))
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func durationAbs(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

func signatureSafeString(value string) string {
	return strings.ReplaceAll(value, "\x00", "")
}
