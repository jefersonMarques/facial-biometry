package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"faceproof/services/api/internal/domain"
	"faceproof/services/api/internal/identity"
)

const (
	maxIdentityGuideRequestBytes       = 1 << 20
	hardMinimumCaptureQuality          = 0.30
	preferredCaptureQuality            = 0.50
	hardMinimumFacePresence            = 0.65
	preferredFacePresence              = 0.80
	hardMinimumGuidedCaptureScore      = 0.30
	preferredGuidedCaptureScore        = 0.45
	identityMatchRetryMargin           = 0.05
	borderlineQualityStrongMatchMargin = 0.12
)

type identityGuideRequest struct {
	ImageBase64 string `json:"imageBase64"`
}

func (handler *Handler) guideIdentityFace(writer http.ResponseWriter, request *http.Request) {
	check, _, ok := handler.loadPublicIdentityCheck(writer, request)
	if !ok {
		return
	}
	if time.Now().After(check.ExpiresAt) {
		handler.writeError(writer, http.StatusGone, "identity check expired")
		return
	}
	if check.Status != identity.StatusBiometryPending || check.Document == nil || len(check.ReferenceEmbedding) == 0 {
		handler.writeError(writer, http.StatusConflict, "identity check is not awaiting biometrics")
		return
	}
	if handler.engine == nil {
		handler.writeError(writer, http.StatusServiceUnavailable, "biometric engine is unavailable")
		return
	}

	var payload identityGuideRequest
	if err := decodeJSON(request, &payload, maxIdentityGuideRequestBytes); err != nil {
		handler.writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	payload.ImageBase64 = strings.TrimSpace(payload.ImageBase64)
	if payload.ImageBase64 == "" {
		handler.writeError(writer, http.StatusBadRequest, "imageBase64 is required")
		return
	}

	result, err := handler.engine.Guide(request.Context(), payload.ImageBase64)
	if err != nil {
		handler.writeError(writer, http.StatusBadGateway, "biometric guide failed")
		return
	}
	handler.writeJSON(writer, http.StatusOK, result)
}

func normalizeGuidedFrames(frames []domain.GuidedCapturedFrame) ([]domain.GuidedCapturedFrame, error) {
	if len(frames) < 6 || len(frames) > 12 {
		return nil, errors.New("guided capture must contain between 6 and 12 frames")
	}

	normalized := make([]domain.GuidedCapturedFrame, len(frames))
	farCount := 0
	nearCount := 0
	for index, frame := range frames {
		frame.ImageBase64 = strings.TrimSpace(frame.ImageBase64)
		frame.Phase = strings.ToLower(strings.TrimSpace(frame.Phase))
		if frame.ImageBase64 == "" {
			return nil, errors.New("guided capture contains an empty frame")
		}
		switch frame.Phase {
		case "far":
			farCount++
		case "near":
			nearCount++
		default:
			return nil, errors.New("guided capture phase must be far or near")
		}
		normalized[index] = frame
	}
	if farCount < 3 || nearCount < 3 {
		return nil, errors.New("guided capture must contain at least three far and three near frames")
	}
	return normalized, nil
}

func identityCaptureNeedsRecapture(result domain.EngineResult) bool {
	return result.Quality.Score < hardMinimumCaptureQuality ||
		result.Quality.FacePresence < hardMinimumFacePresence ||
		result.GuidedCapture.Score < hardMinimumGuidedCaptureScore
}

func identityCaptureIsBorderline(result domain.EngineResult) bool {
	return result.Quality.Score < preferredCaptureQuality ||
		result.Quality.FacePresence < preferredFacePresence ||
		result.GuidedCapture.Score < preferredGuidedCaptureScore
}

func identityMatchNeedsRecapture(result domain.EngineResult, similarity float64, threshold float64) bool {
	if threshold <= 0 {
		return false
	}

	nearThreshold := similarity >= threshold*0.90 &&
		similarity < threshold+identityMatchRetryMargin
	if nearThreshold {
		return true
	}

	return identityCaptureIsBorderline(result) &&
		similarity >= threshold*0.90 &&
		similarity < threshold+borderlineQualityStrongMatchMargin
}
