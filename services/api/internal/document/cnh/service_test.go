package cnh

import (
	"testing"

	"faceproof/services/api/internal/document/pdfanalysis"
	"faceproof/services/api/internal/document/vio/decoder"
)

func TestValidCPF(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{"390.533.447-05", true},
		{"39053344705", true},
		{"11111111111", false},
		{"39053344706", false},
		{"123", false},
	}

	for _, test := range tests {
		if got := ValidCPF(test.value); got != test.valid {
			t.Fatalf("ValidCPF(%q) = %v, want %v", test.value, got, test.valid)
		}
	}
}


func TestOfficialPDFSignerRequiresAuthorityAndICPBrasil(t *testing.T) {
	valid := pdfanalysis.Signature{
		Signer:             "DETRAN PR",
		CertificateSubject: "C=BR,O=ICP-Brasil,CN=DETRAN PR",
		ICPBrasilDetected:  true,
	}
	if !officialPDFSigner(valid) {
		t.Fatal("expected DETRAN ICP-Brasil signer to be accepted")
	}

	withoutICP := valid
	withoutICP.ICPBrasilDetected = false
	withoutICP.CertificateSubject = "C=BR,CN=DETRAN PR"
	if officialPDFSigner(withoutICP) {
		t.Fatal("expected signer without ICP-Brasil evidence to be rejected")
	}

	unrelated := valid
	unrelated.Signer = "Example Company"
	unrelated.CertificateSubject = "C=BR,O=ICP-Brasil,CN=Example Company"
	if officialPDFSigner(unrelated) {
		t.Fatal("expected unrelated ICP-Brasil signer to be rejected")
	}
}

func TestForensicRejectionReasonRejectsPostSignatureChanges(t *testing.T) {
	analysis := pdfanalysis.Result{
		SourceIntegrity: pdfanalysis.SourceIntegrity{Status: "modified_after_signature"},
	}
	if got := forensicRejectionReason(analysis); got == "" {
		t.Fatal("expected modified PDF to be rejected")
	}
}

func TestReferencePhotoPrefersCryptographicallyIntactSignedPDF(t *testing.T) {
	service := NewService(nil, nil)
	analysis := pdfanalysis.Result{
		Signature:       pdfanalysis.Signature{CryptographicallyValid: true},
		SourceIntegrity: pdfanalysis.SourceIntegrity{Status: "signed_pdf_intact"},
		Photo: pdfanalysis.Photo{
			Available:  true,
			Method:     "pdfimages_structural_candidate_crop_v2",
			Confidence: "layout",
			MIME:       "image/png",
			SHA256:     "abc123",
			Width:      250,
			Height:     337,
			Bytes:      []byte{1, 2, 3},
		},
	}

	photo, evidence, err := service.referencePhoto(&decoder.Result{}, analysis)
	if err != nil {
		t.Fatal(err)
	}
	if string(photo) != string([]byte{1, 2, 3}) {
		t.Fatalf("unexpected photo bytes: %v", photo)
	}
	if evidence.Source != "signed_pdf_visual" ||
		evidence.Method != "pdfimages_structural_candidate_crop_v2" ||
		evidence.SHA256 != "abc123" ||
		evidence.Width != 250 ||
		evidence.Height != 337 {
		t.Fatalf("unexpected photo evidence: %#v", evidence)
	}
}
