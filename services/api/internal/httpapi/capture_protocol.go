package httpapi

import (
	"errors"
	"regexp"
	"time"

	"faceproof/services/api/internal/domain"
)

var captureRunIDPattern = regexp.MustCompile(
	`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`,
)

const (
	maxCaptureProtocolDuration = 10 * time.Minute
	minimumServerCaptureAge    = 1200 * time.Millisecond
)

func validateCaptureProtocol(protocol domain.CaptureProtocolMetadata) error {
	if protocol.Version != "1" {
		return errors.New("unsupported capture protocol version")
	}
	if !captureRunIDPattern.MatchString(protocol.RunID) {
		return errors.New("capture protocol runId is invalid")
	}
	if protocol.StartedAtUnixMS <= 0 ||
		protocol.FarStartedAtUnixMS <= 0 ||
		protocol.NearStartedAtUnixMS <= 0 ||
		protocol.SubmittingAtUnixMS <= 0 {
		return errors.New("capture protocol timestamps are incomplete")
	}
	if protocol.StartedAtUnixMS > protocol.FarStartedAtUnixMS ||
		protocol.FarStartedAtUnixMS > protocol.NearStartedAtUnixMS ||
		protocol.NearStartedAtUnixMS > protocol.SubmittingAtUnixMS {
		return errors.New("capture protocol timestamps are out of order")
	}

	durationMS := protocol.SubmittingAtUnixMS - protocol.StartedAtUnixMS
	if durationMS < 0 || time.Duration(durationMS)*time.Millisecond > maxCaptureProtocolDuration {
		return errors.New("capture protocol duration is invalid")
	}
	return nil
}

func validateCaptureProtocolSession(
	protocol domain.CaptureProtocolMetadata,
	captureSession domain.CaptureSession,
) error {
	if captureSession.CaptureRunID == "" || captureSession.CaptureProtocolVersion == "" {
		return errors.New("capture session protocol binding is missing")
	}
	if protocol.RunID != captureSession.CaptureRunID {
		return errors.New("capture protocol runId does not belong to this session")
	}
	if protocol.Version != captureSession.CaptureProtocolVersion {
		return errors.New("capture protocol version does not belong to this session")
	}
	return nil
}

func validateCaptureSessionServerTiming(
	captureSession domain.CaptureSession,
	now time.Time,
) error {
	if captureSession.CreatedAt.IsZero() || captureSession.ExpiresAt.IsZero() {
		return errors.New("capture session timing is incomplete")
	}
	if now.Before(captureSession.CreatedAt) {
		return errors.New("capture session server timing is invalid")
	}
	if now.Sub(captureSession.CreatedAt) < minimumServerCaptureAge {
		return errors.New("capture session completed too quickly")
	}
	if now.After(captureSession.ExpiresAt) {
		return errors.New("capture session expired")
	}
	return nil
}
