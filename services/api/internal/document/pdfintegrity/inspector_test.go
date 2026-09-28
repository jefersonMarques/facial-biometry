package pdfintegrity

import (
	"testing"
	"time"
)

func TestEvaluateAcceptsExpectedCNHDigitalShape(t *testing.T) {
	signatures := parsePDFSig(`Digital Signature Info of: test.pdf
Signature #1:
  - Signature Field Name: Signature1
  - Signer Certificate Common Name: DETRAN PR
  - Signer full Distinguished Name: CN=DETRAN PR,OU=Autoridade Certificadora do SERPRO Final SSL,OU=ARSERPRO,O=ICP-Brasil,C=BR
  - Signing Time: Jan 05 2026 03:19:34
  - Signing Hash Algorithm: SHA-256
  - Signature Type: adbe.pkcs7.detached
  - Signed Ranges: [0 - 100], [200 - 300]
  - Total document signed
  - Signature Validation: Signature is Valid.
  - Certificate Validation: Certificate has Expired
`)
	metadata := parsePDFInfo(`Title:           CNH Digital
Creator:         CDT
Producer:        CDT
CreationDate:    Mon Jan  5 03:19:34 2026 UTC
ModDate:         Mon Jan  5 03:19:34 2026 UTC
Suspects:        no
Form:            AcroForm
JavaScript:      no
Pages:           1
Encrypted:       no
File size:       300 bytes
PDF version:     1.5
`)

	result := evaluate(make([]byte, 300), metadata, signatures, time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC))
	if !result.Valid {
		t.Fatalf("expected valid PDF, diagnostics: %v", result.Diagnostics)
	}
}

func TestEvaluateRejectsNonOfficialSigner(t *testing.T) {
	signature := Signature{
		SignerCommonName:        "EMPRESA EXEMPLO",
		SignerDistinguishedName: "CN=EMPRESA EXEMPLO,O=ICP-Brasil,C=BR",
		SigningTime:             time.Now().UTC().Add(-time.Hour),
		HashAlgorithm:           "SHA-256",
		SignatureType:           "adbe.pkcs7.detached",
		TotalDocumentSigned:     true,
		SignatureValid:          true,
		CertificateValidation:   "Certificate is Trusted.",
	}
	result := evaluate([]byte("0123456789"), Metadata{FileSize: 10}, []Signature{signature}, time.Now().UTC())
	if result.Valid {
		t.Fatal("expected non-official signer to be rejected")
	}
}
