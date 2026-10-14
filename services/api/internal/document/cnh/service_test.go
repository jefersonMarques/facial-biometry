package cnh

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"faceproof/services/api/internal/document/pdfanalysis"
	"faceproof/services/api/internal/document/vio/decoder"
	"faceproof/services/api/internal/document/vio/imagepreview"
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

func TestCollectDualPortraitSignedPNG(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 18, 24))); err != nil {
		t.Fatal(err)
	}

	service := NewService(nil, imagepreview.NewService(nil))
	result := decoder.Result{
		SignatureValid: true,
		Image:          encoded.Bytes(),
		ImageFormat:    "PNG",
	}
	evidence := service.collectDualPortrait(&result, "signed_pdf_visual")
	if evidence.Status != DualPortraitReady {
		t.Fatalf("status = %q, want ready", evidence.Status)
	}
	if !evidence.PhotoPresent || evidence.MIME != "image/png" {
		t.Fatalf("unexpected portrait evidence: %+v", evidence)
	}
	if evidence.Width != 18 || evidence.Height != 24 {
		t.Fatalf("unexpected portrait dimensions: %dx%d", evidence.Width, evidence.Height)
	}
	if !bytes.Equal(evidence.Photo, encoded.Bytes()) {
		t.Fatal("decoded portrait differs from signed VIO image")
	}
}

func TestCollectDualPortraitUnavailableDoesNotFailDocument(t *testing.T) {
	service := NewService(nil, imagepreview.NewService(nil))
	cases := []struct {
		name       string
		result     decoder.Result
		source     string
		wantStatus string
	}{
		{
			name:       "missing image",
			result:     decoder.Result{SignatureValid: true},
			source:     "signed_pdf_visual",
			wantStatus: DualPortraitUnavailable,
		},
		{
			name:       "VIO reference already selected",
			result:     decoder.Result{SignatureValid: true},
			source:     "signed_vio_qr",
			wantStatus: DualPortraitNotApplicable,
		},
		{
			name:       "unvalidated signature",
			result:     decoder.Result{Image: []byte{1}, ImageFormat: "PNG"},
			source:     "signed_pdf_visual",
			wantStatus: DualPortraitAnalysisFailed,
		},
		{
			name:       "unsupported image format",
			result:     decoder.Result{SignatureValid: true, Image: []byte{1}, ImageFormat: "raw"},
			source:     "signed_pdf_visual",
			wantStatus: DualPortraitAnalysisFailed,
		},
		{
			name:       "missing BPG decoder",
			result:     decoder.Result{SignatureValid: true, Image: []byte{1, 2, 3}, ImageFormat: "BPG"},
			source:     "signed_pdf_visual",
			wantStatus: DualPortraitDecoderMissing,
		},
		{
			name:       "malformed PNG",
			result:     decoder.Result{SignatureValid: true, Image: []byte{1, 2, 3}, ImageFormat: "PNG"},
			source:     "signed_pdf_visual",
			wantStatus: DualPortraitAnalysisFailed,
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			result := test.result
			evidence := service.collectDualPortrait(&result, test.source)
			if evidence.Status != test.wantStatus {
				t.Fatalf("status = %q, want %q", evidence.Status, test.wantStatus)
			}
			if len(evidence.Photo) != 0 {
				t.Fatal("unavailable portrait must not expose image bytes")
			}
		})
	}
}

func TestCollectDualPortraitRejectsOversizedDimensions(t *testing.T) {
	var encoded bytes.Buffer
	largeImage := image.NewRGBA(image.Rect(0, 0, maxDualPortraitDimension+1, 3))
	if err := png.Encode(&encoded, largeImage); err != nil {
		t.Fatal(err)
	}

	service := NewService(nil, imagepreview.NewService(nil))
	result := decoder.Result{
		SignatureValid: true,
		Image:          encoded.Bytes(),
		ImageFormat:    "PNG",
	}
	evidence := service.collectDualPortrait(&result, "signed_pdf_visual")
	if evidence.Status != DualPortraitAnalysisFailed {
		t.Fatalf("status = %q, want analysis_failed", evidence.Status)
	}
	if len(evidence.Photo) != 0 {
		t.Fatal("oversized portrait must not reach native inference")
	}
}
