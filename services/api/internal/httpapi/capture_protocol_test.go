package httpapi

import (
	"strings"
	"testing"
	"time"

	"faceproof/services/api/internal/domain"
)

func TestValidateCaptureProtocolAcceptsOrderedLifecycle(t *testing.T) {
	started := time.Now().UnixMilli()
	protocol := domain.CaptureProtocolMetadata{
		Version:             "1",
		RunID:               "123e4567-e89b-12d3-a456-426614174000",
		StartedAtUnixMS:     started,
		FarStartedAtUnixMS:  started,
		NearStartedAtUnixMS: started + 1_200,
		SubmittingAtUnixMS:  started + 2_600,
	}

	if err := validateCaptureProtocol(protocol); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCaptureProtocolRejectsOutOfOrderLifecycle(t *testing.T) {
	started := time.Now().UnixMilli()
	protocol := domain.CaptureProtocolMetadata{
		Version:             "1",
		RunID:               "123e4567-e89b-12d3-a456-426614174000",
		StartedAtUnixMS:     started,
		FarStartedAtUnixMS:  started + 1_000,
		NearStartedAtUnixMS: started + 900,
		SubmittingAtUnixMS:  started + 2_000,
	}

	err := validateCaptureProtocol(protocol)
	if err == nil || !strings.Contains(err.Error(), "out of order") {
		t.Fatalf("expected out-of-order lifecycle error, got %v", err)
	}
}

func TestValidateCaptureProtocolRejectsInvalidRunID(t *testing.T) {
	started := time.Now().UnixMilli()
	protocol := domain.CaptureProtocolMetadata{
		Version:             "1",
		RunID:               "not-a-run-id",
		StartedAtUnixMS:     started,
		FarStartedAtUnixMS:  started,
		NearStartedAtUnixMS: started + 1,
		SubmittingAtUnixMS:  started + 2,
	}

	err := validateCaptureProtocol(protocol)
	if err == nil || !strings.Contains(err.Error(), "runId") {
		t.Fatalf("expected invalid runId error, got %v", err)
	}
}
