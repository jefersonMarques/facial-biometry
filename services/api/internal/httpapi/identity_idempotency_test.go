package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"faceproof/services/api/internal/domain"
	"faceproof/services/api/internal/identity"
)

func TestIdentityCompletionDigestChangesWithFrameBytes(t *testing.T) {
	base := identityCompleteRequest{
		SessionID: "session-1",
		CaptureProtocol: domain.CaptureProtocolMetadata{
			Version: "2",
			RunID:   "123e4567-e89b-42d3-a456-426614174000",
		},
		GuidedFrames: []domain.GuidedCapturedFrame{
			{Phase: "far", ImageBytes: []byte("frame-a")},
		},
	}
	first, err := identityCompletionRequestSHA256(base)
	if err != nil {
		t.Fatal(err)
	}
	base.GuidedFrames[0].ImageBytes = []byte("frame-b")
	second, err := identityCompletionRequestSHA256(base)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("expected frame mutation to change request digest")
	}
}

func TestReplayIdentityCompletionReturnsStoredResponse(t *testing.T) {
	handler := &Handler{}
	recorder := httptest.NewRecorder()
	check := identity.Check{
		CompletionReceipt: &identity.CompletionReceipt{
			IdempotencyKey: "idem-12345678",
			RequestSHA256:  strings.Repeat("a", 64),
			ResponseJSON:   `{"decision":"approved"}`,
		},
	}

	if !handler.replayIdentityCompletionIfAvailable(
		recorder,
		check,
		"idem-12345678",
		strings.Repeat("a", 64),
	) {
		t.Fatal("expected stored completion to be replayed")
	}
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if recorder.Header().Get("X-FaceProof-Idempotency-Replayed") != "true" {
		t.Fatal("expected replay response header")
	}
}

func TestReplayIdentityCompletionRejectsChangedPayload(t *testing.T) {
	handler := &Handler{}
	recorder := httptest.NewRecorder()
	check := identity.Check{
		CompletionReceipt: &identity.CompletionReceipt{
			IdempotencyKey: "idem-12345678",
			RequestSHA256:  strings.Repeat("a", 64),
			ResponseJSON:   `{"decision":"approved"}`,
		},
	}

	if !handler.replayIdentityCompletionIfAvailable(
		recorder,
		check,
		"idem-12345678",
		strings.Repeat("b", 64),
	) {
		t.Fatal("expected idempotency conflict to be handled")
	}
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d", recorder.Code)
	}
}
