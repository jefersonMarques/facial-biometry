package httpapi

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"faceproof/services/api/internal/domain"
)

const captureSealVersion = "1"

func captureWatermarkToken(challenge string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(challenge)))
	return strings.ToUpper(hex.EncodeToString(sum[:3]))
}

func validateCaptureSeal(
	seal *domain.CaptureSeal,
	captureSession domain.CaptureSession,
	frames []domain.GuidedCapturedFrame,
) error {
	if captureSession.CaptureProtocolVersion == "1" {
		return nil
	}
	if captureSession.CaptureProtocolVersion != "2" {
		return errors.New("capture seal protocol is unsupported")
	}
	if seal == nil {
		return errors.New("capture seal is required")
	}
	if seal.Version != captureSealVersion {
		return errors.New("capture seal version is unsupported")
	}
	if captureSession.CaptureChallenge == "" ||
		seal.Challenge != captureSession.CaptureChallenge {
		return errors.New("capture seal challenge mismatch")
	}
	if len(frames) == 0 || len(seal.FrameSHA256) != len(frames) {
		return errors.New("capture seal frame count mismatch")
	}

	chain := captureSealInitialDigest(captureSession)
	for index, frame := range frames {
		frameBytes, err := captureSealFrameBytes(frame)
		if err != nil {
			return err
		}
		frameDigest := sha256.Sum256(frameBytes)
		frameHash := hex.EncodeToString(frameDigest[:])
		if !strings.EqualFold(frameHash, seal.FrameSHA256[index]) {
			return fmt.Errorf("capture seal frame %d checksum mismatch", index)
		}
		chain = captureSealNextDigest(
			chain,
			index,
			frame.Phase,
			frameHash,
		)
	}

	if !strings.EqualFold(chain, seal.ChainSHA256) {
		return errors.New("capture seal chain mismatch")
	}
	return nil
}

func captureSealInitialDigest(captureSession domain.CaptureSession) string {
	value := strings.Join([]string{
		"faceproof-capture-seal-v1",
		captureSession.ID,
		captureSession.CaptureRunID,
		captureSession.CaptureChallenge,
	}, "|")
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func captureSealNextDigest(
	previous string,
	index int,
	phase string,
	frameSHA256 string,
) string {
	value := fmt.Sprintf(
		"%s|%d|%s|%s",
		previous,
		index,
		strings.TrimSpace(strings.ToLower(phase)),
		strings.ToLower(frameSHA256),
	)
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func captureSealFrameBytes(frame domain.GuidedCapturedFrame) ([]byte, error) {
	if len(frame.ImageBytes) > 0 {
		return frame.ImageBytes, nil
	}
	value := strings.TrimSpace(frame.ImageBase64)
	comma := strings.IndexByte(value, ',')
	if comma < 0 {
		return nil, errors.New("capture seal frame bytes are unavailable")
	}
	decoded, err := base64.StdEncoding.DecodeString(value[comma+1:])
	if err != nil || len(decoded) == 0 {
		return nil, errors.New("capture seal frame bytes are invalid")
	}
	return decoded, nil
}
