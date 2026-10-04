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

func TestIdentityCaptureQualityUsesHardAndPreferredThresholds(t *testing.T) {
	result := domain.EngineResult{
		Quality: domain.EngineQuality{
			Score:        0.10,
			FacePresence: 1.0,
		},
		GuidedCapture: domain.EngineSignal{
			Score:  0.80,
			Status: "available",
		},
	}
	if !identityCaptureNeedsRecapture(result) {
		t.Fatal("expected very low-quality capture to require recapture")
	}

	result.Quality.Score = 0.36
	if identityCaptureNeedsRecapture(result) {
		t.Fatal("expected borderline quality to continue to facial matching")
	}
	if !identityCaptureIsBorderline(result) {
		t.Fatal("expected 0.36 quality to be marked borderline")
	}

	result.Quality.Score = 0.70
	result.GuidedCapture.Score = 0.75
	if identityCaptureIsBorderline(result) {
		t.Fatal("expected sufficiently good capture not to be borderline")
	}
}

func TestIdentityMatchRequestsRetryWhenBorderline(t *testing.T) {
	result := domain.EngineResult{
		Quality: domain.EngineQuality{
			Score:        0.36,
			FacePresence: 1.0,
		},
		GuidedCapture: domain.EngineSignal{
			Score:  0.80,
			Status: "available",
		},
	}

	const threshold = 0.363
	if !identityMatchNeedsRecapture(result, 0.42, threshold) {
		t.Fatal("expected borderline-quality near-threshold match to require recapture")
	}
	if identityMatchNeedsRecapture(result, 0.61, threshold) {
		t.Fatal("expected strong match to continue despite borderline image quality")
	}

	result.Quality.Score = 0.70
	if !identityMatchNeedsRecapture(result, 0.38, threshold) {
		t.Fatal("expected near-threshold match to require recapture even with good quality")
	}
	if identityMatchNeedsRecapture(result, -0.04, threshold) {
		t.Fatal("expected clearly different face to proceed to rejection instead of recapture")
	}
}

func TestNormalizeGuidedFramesRejectsFarAfterNear(t *testing.T) {
	frames := []domain.GuidedCapturedFrame{
		{ImageBase64: "data:image/jpeg;base64,AA==", Phase: "far"},
		{ImageBase64: "data:image/jpeg;base64,AA==", Phase: "far"},
		{ImageBase64: "data:image/jpeg;base64,AA==", Phase: "near"},
		{ImageBase64: "data:image/jpeg;base64,AA==", Phase: "far"},
		{ImageBase64: "data:image/jpeg;base64,AA==", Phase: "near"},
		{ImageBase64: "data:image/jpeg;base64,AA==", Phase: "near"},
	}

	if _, err := normalizeGuidedFrames(frames); err == nil {
		t.Fatal("expected far frame after near phase to be rejected")
	}
}
