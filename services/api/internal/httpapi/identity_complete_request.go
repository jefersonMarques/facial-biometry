package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"faceproof/services/api/internal/domain"
)

const (
	maxIdentityCompleteBytes = 16 << 20
	maxIdentityFrameBytes    = 3 << 20
)

type identityCompleteManifest struct {
	Runtime         domain.RuntimeFingerprint       `json:"runtime"`
	CaptureProtocol domain.CaptureProtocolMetadata `json:"captureProtocol"`
	GuidedFrames    []identityCompleteManifestFrame `json:"guidedFrames"`
}

type identityCompleteManifestFrame struct {
	Phase         string                     `json:"phase"`
	ClientQuality *domain.ClientFrameQuality `json:"clientQuality,omitempty"`
}

func decodeIdentityCompleteRequest(
	writer http.ResponseWriter,
	request *http.Request,
) (identityCompleteRequest, error) {
	contentType := strings.ToLower(strings.TrimSpace(request.Header.Get("Content-Type")))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		return decodeIdentityCompleteMultipart(writer, request)
	}

	var payload identityCompleteRequest
	if err := decodeJSON(request, &payload, maxIdentityCompleteBytes); err != nil {
		return identityCompleteRequest{}, err
	}
	return payload, nil
}

func decodeIdentityCompleteMultipart(
	writer http.ResponseWriter,
	request *http.Request,
) (identityCompleteRequest, error) {
	request.Body = http.MaxBytesReader(
		writer,
		request.Body,
		maxIdentityCompleteBytes+identityMultipartOverhead,
	)
	if err := request.ParseMultipartForm(maxIdentityCompleteBytes); err != nil {
		return identityCompleteRequest{}, errors.New("invalid multipart identity capture")
	}
	if request.MultipartForm != nil {
		defer request.MultipartForm.RemoveAll()
	}

	sessionID := strings.TrimSpace(request.FormValue("sessionId"))
	sessionToken := strings.TrimSpace(request.FormValue("sessionToken"))
	manifestValue := strings.TrimSpace(request.FormValue("manifest"))
	if sessionID == "" || sessionToken == "" || manifestValue == "" {
		return identityCompleteRequest{}, errors.New("identity capture metadata is incomplete")
	}

	var manifest identityCompleteManifest
	if err := json.Unmarshal([]byte(manifestValue), &manifest); err != nil {
		return identityCompleteRequest{}, errors.New("identity capture manifest is invalid")
	}

	files := request.MultipartForm.File["frame"]
	if len(files) == 0 || len(files) != len(manifest.GuidedFrames) {
		return identityCompleteRequest{}, errors.New("identity capture frame count does not match manifest")
	}

	frames := make([]domain.GuidedCapturedFrame, len(files))
	for index, header := range files {
		file, err := header.Open()
		if err != nil {
			return identityCompleteRequest{}, errors.New("could not open identity capture frame")
		}

		data, readErr := io.ReadAll(io.LimitReader(file, maxIdentityFrameBytes+1))
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			return identityCompleteRequest{}, errors.New("could not read identity capture frame")
		}
		if len(data) == 0 || len(data) > maxIdentityFrameBytes {
			return identityCompleteRequest{}, errors.New("identity capture frame exceeds size limit")
		}
		if http.DetectContentType(data) != "image/jpeg" {
			return identityCompleteRequest{}, errors.New("identity capture frame must be JPEG")
		}

		meta := manifest.GuidedFrames[index]
		frames[index] = domain.GuidedCapturedFrame{
			ImageBytes:    data,
			Phase:         meta.Phase,
			ClientQuality: meta.ClientQuality,
		}
	}

	return identityCompleteRequest{
		SessionID:    sessionID,
		SessionToken: sessionToken,
		GuidedFrames: frames,
		Runtime:         manifest.Runtime,
		CaptureProtocol: manifest.CaptureProtocol,
	}, nil
}
