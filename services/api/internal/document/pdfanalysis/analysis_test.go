package pdfanalysis

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"
)

func TestAnalyzeIntegrityDetectsSignedCoverage(t *testing.T) {
	prefix := []byte("%PDF-1.5\n1 0 obj<< /Type /Sig /ByteRange [0 10 20 30] >>endobj\n")
	data := append(prefix, bytes.Repeat([]byte("x"), 80)...)
	got := analyzeIntegrity(data)
	if got.ByteRangeCount != 1 {
		t.Fatalf("expected one ByteRange, got %d", got.ByteRangeCount)
	}
}

func TestParsePDFSig(t *testing.T) {
	output := `Signature #1:
  - Signer Certificate Common Name: DETRAN PR
  - Signer full Distinguished Name: CN=DETRAN PR,O=ICP-Brasil
  - Signing Time: Jan 05 2026 00:19:34
  - Signing Hash Algorithm: SHA256
  - Signature Type: adbe.pkcs7.detached
  - Signature Validation: Signature is Valid.
  - Certificate Validation: Certificate has Expired
`
	got := parsePDFSig(output)
	if !got.Present || !got.ValidationChecked || !got.CryptographicallyValid {
		t.Fatalf("unexpected signature result: %#v", got)
	}
	if got.Signer != "DETRAN PR" || got.HashAlgorithm != "SHA256" || !got.CertificateExpired {
		t.Fatalf("unexpected signature metadata: %#v", got)
	}
}

func TestAssessVerifiedPDF(t *testing.T) {
	pdf := &Result{
		Signature: Signature{PresenceChecked: true, Present: true, ValidationChecked: true, CryptographicallyValid: true, CertificateStatus: "Certificate has Expired"},
		Integrity: Integrity{SignedRevisionCoversFile: true},
	}
	got := Assess(true, pdf)
	if got.Status != "verified" {
		t.Fatalf("expected verified, got %#v", got)
	}
}

func TestExtractCNHDigitalPhoto(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 768, 1024))
	for y := 0; y < 1024; y++ {
		for x := 0; x < 768; x++ {
			img.Set(x, y, color.RGBA{R: 240, G: 240, B: 240, A: 255})
		}
	}
	var input bytes.Buffer
	if err := png.Encode(&input, img); err != nil {
		t.Fatal(err)
	}
	photo, err := extractCNHDigitalPhoto(input.Bytes(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if !photo.Available || photo.MIME != "image/png" || len(photo.Bytes) == 0 {
		t.Fatalf("unexpected photo: %#v", photo)
	}
}

func TestAnalyzeConsistency(t *testing.T) {
	qrTime := time.Date(2026, 1, 5, 3, 19, 34, 0, time.UTC)
	got := analyzeConsistency(Metadata{
		Creator:          "CDT",
		Producer:         "CDT",
		CreationDate:     "Mon Jan 5 00:19:34 2026 -03",
		ModificationDate: "Mon Jan 5 00:19:34 2026 -03",
	}, qrTime)
	if !got.CDTGeneratorDetected || !got.CreationModificationSame {
		t.Fatalf("unexpected consistency: %#v", got)
	}
}


func TestParseCertificateTextAndSigningPeriod(t *testing.T) {
	text := `Certificate:
    Data:
        Issuer: C=BR, O=ICP-Brasil, OU=SERPRO, CN=Autoridade Certificadora do SERPRO Final SSL
        Validity
            Not Before: Sep 19 15:03:22 2025 GMT
            Not After : Sep 19 15:03:22 2026 GMT
        Subject: C=BR, O=ICP-Brasil, CN=DETRAN PR
`
	details := parseCertificateText(text)
	details.ICPBrasil = containsFold(details.Issuer, "ICP-Brasil") || containsFold(details.Subject, "ICP-Brasil")
	signedAt, okSigned := parsePDFSigTime("Jan 05 2026 03:19:34")
	notBefore, okBefore := parseCertificateTime(details.NotBefore)
	notAfter, okAfter := parseCertificateTime(details.NotAfter)
	if !okSigned || !okBefore || !okAfter {
		t.Fatal("expected certificate dates to parse")
	}
	if signedAt.Before(notBefore) || signedAt.After(notAfter) {
		t.Fatal("signing time should be inside certificate validity")
	}
	if !details.ICPBrasil || !strings.Contains(details.Issuer, "SERPRO") {
		t.Fatalf("unexpected certificate details: %#v", details)
	}
}


func TestClassifyTrailingXMLAsExternalData(t *testing.T) {
	kind, nonPDF, active := classifyTrailingData([]byte("<?xml version=\"1.0\"?><metadados><metadado nome=\"size\">111254</metadado></metadados>"))
	if kind != "non_pdf_xml" || !nonPDF || active {
		t.Fatalf("unexpected trailing classification: kind=%q nonPDF=%v active=%v", kind, nonPDF, active)
	}
}

func TestClassifyTrailingPDFUpdateAsActiveChange(t *testing.T) {
	kind, nonPDF, active := classifyTrailingData([]byte("20 0 obj\n<< /Type /Annot >>\nendobj\nstartxref\n123\n%%EOF"))
	if kind != "pdf_incremental_update" || nonPDF || !active {
		t.Fatalf("unexpected trailing classification: kind=%q nonPDF=%v active=%v", kind, nonPDF, active)
	}
}

func TestAssessAcceptsSignedPDFWithExternalTrailingData(t *testing.T) {
	pdf := &Result{
		Signature: Signature{
			Present: true, ValidationChecked: true, CryptographicallyValid: true,
			CertificateStatus: "Certificate has Expired",
		},
		Integrity: Integrity{
			SignedPDFRevisionComplete:   true,
			UnsignedBytesAfterSignature: true,
			TrailingUnsignedBytes:       180,
			TrailingDataType:            "non_pdf_xml",
			NonPDFTrailingData:          true,
		},
	}
	got := Assess(true, pdf)
	if got.Status != "verified" {
		t.Fatalf("expected verified with warning, got %#v", got)
	}
	if len(got.Warnings) == 0 {
		t.Fatal("expected warning about external trailing data")
	}
}

func TestAssessRejectsActivePDFChangeAfterSignature(t *testing.T) {
	pdf := &Result{
		Signature: Signature{
			Present: true, ValidationChecked: true, CryptographicallyValid: true,
			CertificateStatus: "Certificate has Expired",
		},
		Integrity: Integrity{
			SignedPDFRevisionComplete:       true,
			UnsignedBytesAfterSignature:     true,
			TrailingDataType:                "pdf_incremental_update",
			ActivePDFChangesAfterSignature:  true,
		},
	}
	got := Assess(true, pdf)
	if got.Status != "inconsistent" {
		t.Fatalf("expected inconsistent, got %#v", got)
	}
}

func TestSourceIntegrityStatuses(t *testing.T) {
	valid := Signature{PresenceChecked: true, Present: true, ValidationChecked: true, CryptographicallyValid: true}
	cases := []struct {
		name      string
		signature Signature
		integrity Integrity
		expected  string
	}{
		{name: "unchecked", expected: "signature_not_checked"},
		{name: "unsigned", signature: Signature{PresenceChecked: true}, expected: "unsigned_pdf"},
		{name: "malformed", signature: Signature{PresenceChecked: true}, integrity: Integrity{SignatureDictionaryDetected: true}, expected: "malformed_signature_structure"},
		{name: "intact", signature: valid, integrity: Integrity{SignedRevisionCoversFile: true}, expected: "signed_pdf_intact"},
		{name: "external-data", signature: valid, integrity: Integrity{SignedPDFRevisionComplete: true, UnsignedBytesAfterSignature: true, NonPDFTrailingData: true}, expected: "signed_pdf_with_external_trailing_data"},
		{name: "modified", signature: valid, integrity: Integrity{ActivePDFChangesAfterSignature: true}, expected: "modified_after_signature"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sourceIntegrityStatus(tc.signature, tc.integrity); got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}


func TestAssessRejectsVisibleSignatureClaimWithoutPDFSignature(t *testing.T) {
	pdf := &Result{
		Signature: Signature{PresenceChecked: true},
		Consistency: Consistency{
			TextEvidenceChecked:          true,
			VisibleDigitalSignatureClaim: true,
			SignatureClaimMatchesPDF:     false,
		},
		SourceIntegrity: SourceIntegrity{Status: "unsigned_pdf"},
	}
	got := Assess(true, pdf)
	if got.Status != "inconsistent" {
		t.Fatalf("expected inconsistent, got %#v", got)
	}
}


func TestAssessRejectsMalformedSignatureStructure(t *testing.T) {
	pdf := &Result{
		Signature: Signature{PresenceChecked: true},
		Integrity: Integrity{SignatureDictionaryDetected: true, ByteRangeCount: 1},
	}
	got := Assess(true, pdf)
	if got.Status != "inconsistent" {
		t.Fatalf("expected inconsistent, got %#v", got)
	}
}
