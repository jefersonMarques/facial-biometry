package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"faceproof/services/api/internal/config"
	"faceproof/services/api/internal/domain"
)

func validateRuntimeFingerprint(
	fingerprint domain.RuntimeFingerprint,
	configuration config.Config,
) error {
	fingerprint.SDKVersion = strings.TrimSpace(fingerprint.SDKVersion)
	fingerprint.LivenessCoreVersion = strings.TrimSpace(fingerprint.LivenessCoreVersion)
	fingerprint.LivenessCoreWASMSHA256 = normalizeRuntimeSHA256(fingerprint.LivenessCoreWASMSHA256)
	fingerprint.MediaPipeVersion = strings.TrimSpace(fingerprint.MediaPipeVersion)
	fingerprint.MediaPipeVisionSHA256 = normalizeRuntimeSHA256(fingerprint.MediaPipeVisionSHA256)
	fingerprint.FaceLandmarkerSHA256 = normalizeRuntimeSHA256(fingerprint.FaceLandmarkerSHA256)
	fingerprint.RuntimeID = normalizeRuntimeSHA256(fingerprint.RuntimeID)

	if fingerprint.SDKVersion == "" ||
		fingerprint.LivenessCoreVersion == "" ||
		fingerprint.MediaPipeVersion == "" ||
		fingerprint.LivenessCoreWASMSHA256 == "" ||
		fingerprint.MediaPipeVisionSHA256 == "" ||
		fingerprint.FaceLandmarkerSHA256 == "" ||
		fingerprint.RuntimeID == "" {
		return errors.New("runtime fingerprint is incomplete")
	}

	canonical := strings.Join([]string{
		fingerprint.SDKVersion,
		fingerprint.LivenessCoreVersion,
		fingerprint.LivenessCoreWASMSHA256,
		fingerprint.MediaPipeVersion,
		fingerprint.MediaPipeVisionSHA256,
		fingerprint.FaceLandmarkerSHA256,
	}, "|")
	sum := sha256.Sum256([]byte(canonical))
	if fingerprint.RuntimeID != hex.EncodeToString(sum[:]) {
		return errors.New("runtime fingerprint checksum mismatch")
	}

	if !runtimeValueMatches(configuration.RuntimeSDKVersion, fingerprint.SDKVersion) {
		return errors.New("unsupported FaceProof SDK version")
	}
	if !runtimeValueMatches(configuration.RuntimeLivenessCoreVersion, fingerprint.LivenessCoreVersion) {
		return errors.New("unsupported FaceProof liveness core version")
	}
	if !runtimeHashMatches(configuration.RuntimeLivenessCoreWASMSHA256, fingerprint.LivenessCoreWASMSHA256) {
		return errors.New("unsupported FaceProof liveness WASM build")
	}
	if !runtimeValueMatches(configuration.RuntimeMediaPipeVersion, fingerprint.MediaPipeVersion) {
		return errors.New("unsupported MediaPipe version")
	}
	if !runtimeHashMatches(configuration.RuntimeMediaPipeVisionSHA256, fingerprint.MediaPipeVisionSHA256) {
		return errors.New("unsupported MediaPipe runtime build")
	}
	if !runtimeHashMatches(configuration.RuntimeFaceLandmarkerSHA256, fingerprint.FaceLandmarkerSHA256) {
		return errors.New("unsupported Face Landmarker model")
	}

	return nil
}

func normalizeRuntimeSHA256(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != sha256.Size*2 {
		return ""
	}
	if _, err := hex.DecodeString(value); err != nil {
		return ""
	}
	return value
}

func runtimeValueMatches(expected, actual string) bool {
	expected = strings.TrimSpace(expected)
	return expected == "" || expected == actual
}

func runtimeHashMatches(expected, actual string) bool {
	expected = normalizeRuntimeSHA256(expected)
	return expected == "" || expected == actual
}
