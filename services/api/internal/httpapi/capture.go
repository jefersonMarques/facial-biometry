package httpapi

import (
	"crypto/rand"
	"errors"
	"strings"

	"faceproof/services/api/internal/domain"
)

func normalizeCapturedFrames(frames []domain.CapturedFrame, captureSession domain.CaptureSession) ([]domain.CapturedFrame, error) {
	if len(frames) < 8 || len(frames) > 32 {
		return nil, errors.New("capture must contain between 8 and 32 frames")
	}
	if captureSession.CaptureDurationMS <= 0 || captureSession.SampleIntervalMS <= 0 || len(captureSession.IlluminationPattern) < 4 {
		return nil, errors.New("capture session configuration is invalid")
	}

	normalized := make([]domain.CapturedFrame, len(frames))
	seenChallenges := make(map[int]struct{}, len(captureSession.IlluminationPattern))
	var previousTimestamp int64 = -1

	for index, frame := range frames {
		if strings.TrimSpace(frame.ImageBase64) == "" {
			return nil, errors.New("capture contains an empty frame")
		}
		if frame.TimestampMS < 0 || frame.TimestampMS <= previousTimestamp {
			return nil, errors.New("frame timestamps must be strictly increasing")
		}
		previousTimestamp = frame.TimestampMS

		challengeIndex := expectedChallengeIndex(frame.TimestampMS, captureSession)
		frame.ChallengeIndex = challengeIndex
		frame.ClientLight = captureSession.IlluminationPattern[challengeIndex]
		normalized[index] = frame
		seenChallenges[challengeIndex] = struct{}{}
	}

	firstTimestamp := normalized[0].TimestampMS
	lastTimestamp := normalized[len(normalized)-1].TimestampMS
	maxStartDelay := int64(captureSession.SampleIntervalMS * 3)
	maxEndTimestamp := int64(captureSession.CaptureDurationMS + captureSession.SampleIntervalMS*3)
	minimumCoverage := int64(float64(captureSession.CaptureDurationMS) * 0.60)

	if firstTimestamp > maxStartDelay {
		return nil, errors.New("capture starts too late")
	}
	if lastTimestamp > maxEndTimestamp {
		return nil, errors.New("capture duration exceeds session limits")
	}
	if lastTimestamp-firstTimestamp < minimumCoverage {
		return nil, errors.New("capture duration is too short")
	}

	minimumChallengeCoverage := 4
	if len(captureSession.IlluminationPattern) < minimumChallengeCoverage {
		minimumChallengeCoverage = len(captureSession.IlluminationPattern)
	}
	if len(seenChallenges) < minimumChallengeCoverage {
		return nil, errors.New("capture does not cover enough illumination challenges")
	}

	return normalized, nil
}

func expectedChallengeIndex(timestampMS int64, captureSession domain.CaptureSession) int {
	effectiveTimestamp := timestampMS - int64(captureSession.IlluminationSettleMS)
	if effectiveTimestamp < 0 {
		effectiveTimestamp = 0
	}

	index := int(effectiveTimestamp * int64(len(captureSession.IlluminationPattern)) / int64(captureSession.CaptureDurationMS))
	if index < 0 {
		return 0
	}
	if index >= len(captureSession.IlluminationPattern) {
		return len(captureSession.IlluminationPattern) - 1
	}
	return index
}

func randomIlluminationPattern(length int) ([]float64, error) {
	if length <= 0 {
		return nil, errors.New("illumination challenge length must be positive")
	}

	lowLevels := []float64{0.22, 0.30, 0.38}
	highLevels := []float64{0.72, 0.82, 0.90}
	buffer := make([]byte, length+1)
	if _, err := rand.Read(buffer); err != nil {
		return nil, err
	}

	startHigh := buffer[0]&1 == 1
	pattern := make([]float64, length)
	for index := range pattern {
		useHigh := (index%2 == 0) == startHigh
		levels := lowLevels
		if useHigh {
			levels = highLevels
		}
		pattern[index] = levels[int(buffer[index+1])%len(levels)]
	}

	return pattern, nil
}
