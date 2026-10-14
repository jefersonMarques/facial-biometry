package engine

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"faceproof/services/api/internal/domain"
)

func TestAnalyzeIdentitySendsBinaryMultipartFrames(t *testing.T) {
	expectedJPEG := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0xff, 0xd9}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !strings.HasPrefix(request.Header.Get("Content-Type"), "multipart/form-data;") {
			t.Fatalf("unexpected content type: %q", request.Header.Get("Content-Type"))
		}
		if err := request.ParseMultipartForm(2 << 20); err != nil {
			t.Fatal(err)
		}

		manifestValue := request.FormValue("manifest")
		if manifestValue == "" {
			t.Fatal("manifest is missing")
		}
		if strings.Contains(manifestValue, "imageBase64") {
			t.Fatal("multipart manifest must not contain base64 images")
		}

		var manifest struct {
			GuidedFrames []struct {
				Phase string `json:"phase"`
			} `json:"guidedFrames"`
		}
		if err := json.Unmarshal([]byte(manifestValue), &manifest); err != nil {
			t.Fatal(err)
		}
		if len(manifest.GuidedFrames) != 6 {
			t.Fatalf("expected 6 manifest frames, got %d", len(manifest.GuidedFrames))
		}

		files := request.MultipartForm.File["frame"]
		if len(files) != 6 {
			t.Fatalf("expected 6 binary frames, got %d", len(files))
		}
		for index, header := range files {
			if header.Header.Get("Content-Type") != "image/jpeg" {
				t.Fatalf("frame %d content type = %q", index, header.Header.Get("Content-Type"))
			}
			file, err := header.Open()
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(file)
			_ = file.Close()
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != string(expectedJPEG) {
				t.Fatalf("frame %d bytes changed in transport", index)
			}
		}

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"embedding":[0.1],"embeddingModel":"test"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL)
	frames := make([]domain.GuidedCapturedFrame, 6)
	for index := range frames {
		phase := "far"
		if index >= 3 {
			phase = "near"
		}
		frames[index] = domain.GuidedCapturedFrame{
			ImageBytes: append([]byte(nil), expectedJPEG...),
			Phase:      phase,
		}
	}

	result, err := client.AnalyzeIdentity(context.Background(), domain.EngineIdentityRequest{
		GuidedFrames: frames,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Embedding) != 1 {
		t.Fatalf("unexpected embedding: %+v", result.Embedding)
	}
}
