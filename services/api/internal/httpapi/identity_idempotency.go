package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"faceproof/services/api/internal/domain"
	"faceproof/services/api/internal/identity"
)

var identityIdempotencyKeyPattern = regexp.MustCompile(
	`^[A-Za-z0-9._:-]{8,128}$`,
)

type identityCompletionDigestFrame struct {
	Phase         string                     `json:"phase"`
	ClientQuality *domain.ClientFrameQuality `json:"clientQuality,omitempty"`
	SHA256        string                     `json:"sha256"`
}

type identityCompletionDigestPayload struct {
	SessionID       string                         `json:"sessionId"`
	Runtime         domain.RuntimeFingerprint      `json:"runtime"`
	CaptureProtocol domain.CaptureProtocolMetadata `json:"captureProtocol"`
	CaptureSeal     *domain.CaptureSeal             `json:"captureSeal,omitempty"`
	Geometry        *domain.GeometryTelemetry       `json:"geometry,omitempty"`
	Frames          []identityCompletionDigestFrame `json:"frames"`
}

func identityCompletionIdempotencyKey(
	request *http.Request,
	payload identityCompleteRequest,
) (string, error) {
	key := strings.TrimSpace(request.Header.Get("Idempotency-Key"))
	if key == "" {
		key = payload.CaptureProtocol.RunID
	}
	if !identityIdempotencyKeyPattern.MatchString(key) {
		return "", errors.New("invalid Idempotency-Key")
	}
	return key, nil
}

func identityCompletionRequestSHA256(
	payload identityCompleteRequest,
) (string, error) {
	frames := make([]identityCompletionDigestFrame, 0, len(payload.GuidedFrames))
	for _, frame := range payload.GuidedFrames {
		frameBytes, err := captureSealFrameBytes(frame)
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256(frameBytes)
		frames = append(frames, identityCompletionDigestFrame{
			Phase:         strings.TrimSpace(strings.ToLower(frame.Phase)),
			ClientQuality: frame.ClientQuality,
			SHA256:        hex.EncodeToString(sum[:]),
		})
	}

	canonical, err := json.Marshal(identityCompletionDigestPayload{
		SessionID:       payload.SessionID,
		Runtime:         payload.Runtime,
		CaptureProtocol: payload.CaptureProtocol,
		CaptureSeal:     payload.CaptureSeal,
		Geometry:        payload.Geometry,
		Frames:          frames,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

func newIdentityCompletionReceipt(
	sessionID string,
	idempotencyKey string,
	requestSHA256 string,
	response identityCompleteResponse,
) (*identity.CompletionReceipt, error) {
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}
	return &identity.CompletionReceipt{
		SessionID:      sessionID,
		IdempotencyKey: idempotencyKey,
		RequestSHA256:  requestSHA256,
		ResponseJSON:   string(responseJSON),
		CreatedAt:      time.Now().UTC(),
	}, nil
}

func (handler *Handler) replayIdentityCompletionIfAvailable(
	writer http.ResponseWriter,
	check identity.Check,
	idempotencyKey string,
	requestSHA256 string,
) bool {
	receipt := check.CompletionReceipt
	if receipt == nil {
		return false
	}
	if receipt.IdempotencyKey != idempotencyKey {
		return false
	}
	if receipt.RequestSHA256 != requestSHA256 {
		handler.writeError(
			writer,
			http.StatusConflict,
			"Idempotency-Key was already used with different capture content",
		)
		return true
	}
	if strings.TrimSpace(receipt.ResponseJSON) == "" {
		handler.writeError(
			writer,
			http.StatusConflict,
			"identity completion receipt is unavailable",
		)
		return true
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("X-FaceProof-Idempotency-Replayed", "true")
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte(receipt.ResponseJSON))
	return true
}
