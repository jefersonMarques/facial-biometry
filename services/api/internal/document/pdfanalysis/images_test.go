package pdfanalysis

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"strings"
	"testing"
)

func TestParsePDFImagesList(t *testing.T) {
	output := `page   num  type   width height color comp bpc  enc interp  object ID x-ppi y-ppi size ratio
--------------------------------------------------------------------------------------------
   1     0 image     963   680  rgb     3   8  jpeg   no        20  0   287   275  101K 5.4%
   1     1 image     640   480  rgb     3   8  image  no        21  0   72    72   900K 100%
`
	got := parsePDFImagesList(output)
	if len(got) != 2 {
		t.Fatalf("expected 2 image rows, got %d", len(got))
	}
	if got[0].Width != 963 || got[0].Height != 680 || got[0].Encoding != "jpeg" {
		t.Fatalf("unexpected first image: %#v", got[0])
	}
	if got[0].ObjectID != 20 || got[0].XPPI != 287 {
		t.Fatalf("unexpected object metadata: %#v", got[0])
	}
}

func TestJPEGMetadataFingerprint(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 3), G: uint8(y * 3), B: 100, A: 255})
		}
	}

	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, img, &jpeg.Options{Quality: 82}); err != nil {
		t.Fatal(err)
	}
	meta := parseJPEGMetadata(encoded.Bytes())
	if meta.QuantizationSHA256 == "" || meta.HuffmanSHA256 == "" || meta.MarkerProfileSHA256 == "" {
		t.Fatalf("expected JPEG fingerprints, got %#v", meta)
	}
}

func TestImageStructureFingerprintChangesWithLayout(t *testing.T) {
	base := []EmbeddedImageEvidence{{
		Index: 0, Page: 1, Type: "image", Width: 963, Height: 680,
		ColorSpace: "rgb", Components: 3, BitsPerComponent: 8,
		Encoding: "jpeg", Interpolation: "no", ObjectID: 20, ObjectGeneration: 0,
		XPPI: 287, YPPI: 275,
	}}
	first := imageStructureFingerprint(base, false)
	base[0].Width = 964
	second := imageStructureFingerprint(base, false)
	if first == second || first == "" || second == "" {
		t.Fatalf("expected distinct structural fingerprints: %q %q", first, second)
	}
}

func TestSelectCNHFrontCandidate(t *testing.T) {
	images := []EmbeddedImageEvidence{
		{Type: "image", Width: 120, Height: 120},
		{Type: "image", Width: 963, Height: 680},
		{Type: "image", Width: 800, Height: 1200},
	}
	if got := selectCNHFrontCandidate(images); got != 1 {
		t.Fatalf("expected candidate index 1, got %d", got)
	}
}

func TestPDFImagesListIgnoresHeaders(t *testing.T) {
	got := parsePDFImagesList("page num type width height color comp bpc enc interp object ID x-ppi y-ppi size ratio\n")
	if len(got) != 0 {
		t.Fatalf("expected no rows, got %#v", got)
	}
}

func TestStructuralFingerprintIncludesJPEGProfile(t *testing.T) {
	a := []EmbeddedImageEvidence{{
		Page: 1, Type: "image", Width: 963, Height: 680, ColorSpace: "rgb",
		Components: 3, BitsPerComponent: 8, Encoding: "jpeg",
		JPEG: &JPEGMetadata{QuantizationSHA256: strings.Repeat("a", 64)},
	}}
	b := []EmbeddedImageEvidence{{
		Page: 1, Type: "image", Width: 963, Height: 680, ColorSpace: "rgb",
		Components: 3, BitsPerComponent: 8, Encoding: "jpeg",
		JPEG: &JPEGMetadata{QuantizationSHA256: strings.Repeat("b", 64)},
	}}
	if imageStructureFingerprint(a, false) == imageStructureFingerprint(b, false) {
		t.Fatal("JPEG profile should affect structural fingerprint")
	}
}


func TestAttachRawPDFObjectHashes(t *testing.T) {
	pdf := []byte("%PDF-1.5\n20 0 obj\n<< /Type /XObject /Subtype /Image /Width 10 /Height 10 /Length 4 >>\nstream\nABCD\nendstream\nendobj\n%%EOF")
	images := []EmbeddedImageEvidence{{ObjectID: 20, ObjectGeneration: 0}}
	attachRawPDFObjectHashes(pdf, images)
	if images[0].RawStreamSHA256 == "" {
		t.Fatal("expected raw stream hash")
	}
	if images[0].ObjectDictionarySHA256 == "" {
		t.Fatal("expected object dictionary hash")
	}
}


func TestImageForensicIndicators(t *testing.T) {
	images := []EmbeddedImageEvidence{
		{Type: "image", Width: 1200, Height: 1600, Encoding: "jpeg"},
		{Type: "image", Width: 572, Height: 588, Encoding: "jpeg"},
		{Type: "image", Width: 963, Height: 680, Encoding: "jpeg"},
		{Type: "image", Width: 591, Height: 591, Encoding: "image"},
	}
	if got := countStandalonePortraitCandidates(images); got != 1 {
		t.Fatalf("expected one standalone portrait candidate, got %d", got)
	}
	if got := countSquareJPEGCandidates(images); got != 1 {
		t.Fatalf("expected one square JPEG candidate, got %d", got)
	}
}


func TestJPEGMetadataDetectsEditorMarkers(t *testing.T) {
	makeSegment := func(marker byte, payload []byte) []byte {
		length := len(payload) + 2
		return append([]byte{0xff, marker, byte(length >> 8), byte(length)}, payload...)
	}
	jpegData := []byte{0xff, 0xd8}
	jpegData = append(jpegData, makeSegment(0xe1, append([]byte("http://ns.adobe.com/xap/1.0/\x00"), []byte("<x:xmpmeta/>")...))...)
	jpegData = append(jpegData, makeSegment(0xed, []byte("Photoshop 3.0\x008BIM"))...)
	jpegData = append(jpegData, makeSegment(0xee, []byte("Adobe"))...)
	jpegData = append(jpegData, 0xff, 0xd9)

	meta := parseJPEGMetadata(jpegData)
	if !meta.XMPPresent || !meta.PhotoshopAPP13Present || !meta.AdobeAPP14Present {
		t.Fatalf("expected editor markers, got %#v", meta)
	}
}
