package httpapi

import (
	"testing"
	"time"

	"faceproof/services/api/internal/domain"
)

func TestNormalizeCapturedFramesDerivesChallengeFromTimestamp(t *testing.T) {
	captureSession := testCaptureSession()
	frames := make([]domain.CapturedFrame, 20)
	for index := range frames {
		frames[index] = domain.CapturedFrame{
			ImageBase64:    "data:image/jpeg;base64,test",
			TimestampMS:    int64(65 + index*120),
			ChallengeIndex: 0,
			ClientLight:    0,
		}
	}

	normalized, err := normalizeCapturedFrames(frames, captureSession)
	if err != nil {
		t.Fatal(err)
	}
	if normalized[19].ChallengeIndex == 0 {
		t.Fatal("expected server to derive later challenge indices from timestamps")
	}
	for _, frame := range normalized {
		expectedLight := captureSession.IlluminationPattern[frame.ChallengeIndex]
		if frame.ClientLight != expectedLight {
			t.Fatalf("expected server-owned light %.2f, got %.2f", expectedLight, frame.ClientLight)
		}
	}
}

func TestNormalizeCapturedFramesRejectsNonMonotonicTimestamps(t *testing.T) {
	captureSession := testCaptureSession()
	frames := make([]domain.CapturedFrame, 8)
	for index := range frames {
		frames[index] = domain.CapturedFrame{
			ImageBase64: "data:image/jpeg;base64,test",
			TimestampMS: int64(index * 300),
		}
	}
	frames[4].TimestampMS = frames[3].TimestampMS

	if _, err := normalizeCapturedFrames(frames, captureSession); err == nil {
		t.Fatal("expected non-monotonic timestamps to be rejected")
	}
}

func testCaptureSession() domain.CaptureSession {
	return domain.CaptureSession{
		ID:                   "session-1",
		CaptureDurationMS:    captureDurationMS,
		SampleIntervalMS:     sampleIntervalMS,
		IlluminationSettleMS: illuminationSettleMS,
		IlluminationPattern:  []float64{0.3, 0.8, 0.3, 0.8, 0.3, 0.8},
		ExpiresAt:            time.Now().Add(time.Minute),
	}
}
