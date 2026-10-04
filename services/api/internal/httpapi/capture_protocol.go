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

const maxCaptureProtocolDuration = 10 * time.Minute

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
