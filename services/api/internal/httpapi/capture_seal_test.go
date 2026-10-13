package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"faceproof/services/api/internal/domain"
)

func TestValidateCaptureSealAcceptsBoundFrameChain(t *testing.T) {
	session := domain.CaptureSession{
		ID:                     "session-1",
		CaptureRunID:           "123e4567-e89b-42d3-a456-426614174000",
		CaptureProtocolVersion: "2",
		CaptureChallenge:       "challenge-123",
	}
	frames := []domain.GuidedCapturedFrame{
		{ImageBytes: []byte("frame-a"), Phase: "far"},
		{ImageBytes: []byte("frame-b"), Phase: "near"},
	}

	seal := buildTestCaptureSeal(session, frames)
	if err := validateCaptureSeal(&seal, session, frames); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCaptureSealRejectsReplayFromAnotherSession(t *testing.T) {
	session := domain.CaptureSession{
		ID:                     "session-1",
		CaptureRunID:           "123e4567-e89b-42d3-a456-426614174000",
		CaptureProtocolVersion: "2",
		CaptureChallenge:       "challenge-123",
	}
	frames := []domain.GuidedCapturedFrame{
		{ImageBytes: []byte("frame-a"), Phase: "far"},
	}
	seal := buildTestCaptureSeal(session, frames)

	replayedSession := session
	replayedSession.ID = "session-2"
	replayedSession.CaptureChallenge = "challenge-456"

	err := validateCaptureSeal(&seal, replayedSession, frames)
	if err == nil || !strings.Contains(err.Error(), "challenge") {
		t.Fatalf("expected challenge mismatch, got %v", err)
	}
}

func TestValidateCaptureSealRejectsChangedFrame(t *testing.T) {
	session := domain.CaptureSession{
		ID:                     "session-1",
		CaptureRunID:           "123e4567-e89b-42d3-a456-426614174000",
		CaptureProtocolVersion: "2",
		CaptureChallenge:       "challenge-123",
	}
	frames := []domain.GuidedCapturedFrame{
		{ImageBytes: []byte("frame-a"), Phase: "far"},
	}
	seal := buildTestCaptureSeal(session, frames)
	frames[0].ImageBytes = []byte("tampered")

	err := validateCaptureSeal(&seal, session, frames)
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("expected frame checksum mismatch, got %v", err)
	}
}

func TestCaptureWatermarkTokenIsStableAndShort(t *testing.T) {
	first := captureWatermarkToken("challenge-123")
	second := captureWatermarkToken("challenge-123")
	if first != second || len(first) != 6 {
		t.Fatalf("unexpected watermark token: %q / %q", first, second)
	}
}

func buildTestCaptureSeal(
	session domain.CaptureSession,
	frames []domain.GuidedCapturedFrame,
) domain.CaptureSeal {
	hashes := make([]string, 0, len(frames))
	chain := captureSealInitialDigest(session)
	for index, frame := range frames {
		sum := sha256.Sum256(frame.ImageBytes)
		frameHash := hex.EncodeToString(sum[:])
		hashes = append(hashes, frameHash)
		chain = captureSealNextDigest(chain, index, frame.Phase, frameHash)
	}
	return domain.CaptureSeal{
		Version:     captureSealVersion,
		Challenge:   session.CaptureChallenge,
		FrameSHA256: hashes,
		ChainSHA256: chain,
	}
}
