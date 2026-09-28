package httpapi

import (
	"testing"

	"faceproof/services/api/internal/domain"
)

func TestNormalizeGuidedFramesRequiresFarAndNearPhases(t *testing.T) {
	frames := []domain.GuidedCapturedFrame{
		{ImageBase64: "data:image/jpeg;base64,AA==", Phase: "far"},
		{ImageBase64: "data:image/jpeg;base64,AA==", Phase: "far"},
		{ImageBase64: "data:image/jpeg;base64,AA==", Phase: "far"},
		{ImageBase64: "data:image/jpeg;base64,AA==", Phase: "near"},
		{ImageBase64: "data:image/jpeg;base64,AA==", Phase: "near"},
		{ImageBase64: "data:image/jpeg;base64,AA==", Phase: "near"},
	}

	normalized, err := normalizeGuidedFrames(frames)
	if err != nil {
		t.Fatal(err)
	}
	if len(normalized) != 6 {
		t.Fatalf("normalized guided frames = %d", len(normalized))
	}
}

func TestNormalizeGuidedFramesRejectsSinglePhase(t *testing.T) {
	frames := []domain.GuidedCapturedFrame{
		{ImageBase64: "data:image/jpeg;base64,AA==", Phase: "far"},
		{ImageBase64: "data:image/jpeg;base64,AA==", Phase: "far"},
		{ImageBase64: "data:image/jpeg;base64,AA==", Phase: "far"},
		{ImageBase64: "data:image/jpeg;base64,AA==", Phase: "far"},
		{ImageBase64: "data:image/jpeg;base64,AA==", Phase: "far"},
		{ImageBase64: "data:image/jpeg;base64,AA==", Phase: "far"},
	}

	if _, err := normalizeGuidedFrames(frames); err == nil {
		t.Fatal("expected single-phase guided capture to be rejected")
	}
}

func TestIdentityCaptureQualityRequestsRecapture(t *testing.T) {
	result := domain.EngineResult{
		Quality: domain.EngineQuality{
			Score:        0.46,
			FacePresence: 1.0,
		},
		GuidedCapture: domain.EngineSignal{
			Score:  0.80,
			Status: "available",
		},
	}
	if !identityCaptureNeedsRecapture(result) {
		t.Fatal("expected low-quality capture to require recapture")
	}

	result.Quality.Score = 0.70
	result.GuidedCapture.Score = 0.75
	if identityCaptureNeedsRecapture(result) {
		t.Fatal("expected sufficiently good capture to continue")
	}
}
