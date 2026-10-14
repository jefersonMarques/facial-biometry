package httpapi

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"faceproof/services/api/internal/domain"
)

func TestDecodeIdentityCompleteMultipart(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("sessionId", "session-123"); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("sessionToken", "token-456"); err != nil {
		t.Fatal(err)
	}

	manifest, err := json.Marshal(identityCompleteManifest{
		GuidedFrames: []identityCompleteManifestFrame{
			{
				Phase: "far",
				ClientQuality: &domain.ClientFrameQuality{
					Brightness: 0.50,
					Contrast:   0.10,
					Sharpness:  0.05,
				},
			},
			{
				Phase: "near",
				ClientQuality: &domain.ClientFrameQuality{
					Brightness: 0.55,
					Contrast:   0.12,
					Sharpness:  0.06,
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("manifest", string(manifest)); err != nil {
		t.Fatal(err)
	}

	jpegBytes := testJPEG(t)
	for index := 0; index < 2; index++ {
		part, err := writer.CreateFormFile("frame", "frame.jpg")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(jpegBytes); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/identity/complete", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()

	payload, err := decodeIdentityCompleteRequest(recorder, request)
	if err != nil {
		t.Fatal(err)
	}
	if payload.SessionID != "session-123" || payload.SessionToken != "token-456" {
		t.Fatalf("unexpected session metadata: %+v", payload)
	}
	if len(payload.GuidedFrames) != 2 {
		t.Fatalf("expected 2 frames, got %d", len(payload.GuidedFrames))
	}
	if payload.GuidedFrames[0].Phase != "far" || payload.GuidedFrames[1].Phase != "near" {
		t.Fatalf("unexpected frame phases: %+v", payload.GuidedFrames)
	}
	for _, frame := range payload.GuidedFrames {
		if len(frame.ImageBytes) == 0 {
			t.Fatal("expected binary JPEG bytes")
		}
		if frame.ImageBase64 != "" {
			t.Fatalf("multipart path must not recreate base64, got %q", frame.ImageBase64)
		}
	}
}

func TestDecodeIdentityCompleteJSONCompatibility(t *testing.T) {
	body := `{
		"sessionId":"session-json",
		"sessionToken":"token-json",
		"guidedFrames":[
			{"imageBase64":"data:image/jpeg;base64,test","phase":"far"}
		]
	}`
	request := httptest.NewRequest(http.MethodPost, "/v1/identity/complete", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	payload, err := decodeIdentityCompleteRequest(recorder, request)
	if err != nil {
		t.Fatal(err)
	}
	if payload.SessionID != "session-json" || len(payload.GuidedFrames) != 1 {
		t.Fatalf("unexpected JSON payload: %+v", payload)
	}
}

func testJPEG(t *testing.T) []byte {
	t.Helper()

	imageValue := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			imageValue.Set(x, y, color.RGBA{R: 120, G: 160, B: 200, A: 255})
		}
	}

	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, imageValue, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
