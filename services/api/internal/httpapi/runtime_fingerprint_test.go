package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"faceproof/services/api/internal/config"
	"faceproof/services/api/internal/domain"
)

func TestValidateRuntimeFingerprintAcceptsSupportedRelease(t *testing.T) {
	fingerprint := testRuntimeFingerprint("wasmhash")
	configuration := config.Config{
		RuntimeSDKVersion:              fingerprint.SDKVersion,
		RuntimeLivenessCoreVersion:     fingerprint.LivenessCoreVersion,
		RuntimeLivenessCoreWASMSHA256: fingerprint.LivenessCoreWASMSHA256,
		RuntimeMediaPipeVersion:        fingerprint.MediaPipeVersion,
		RuntimeMediaPipeVisionSHA256:   fingerprint.MediaPipeVisionSHA256,
		RuntimeFaceLandmarkerSHA256:    fingerprint.FaceLandmarkerSHA256,
	}

	if err := validateRuntimeFingerprint(fingerprint, configuration); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRuntimeFingerprintRejectsUnsupportedSDK(t *testing.T) {
	fingerprint := testRuntimeFingerprint("wasmhash")
	configuration := config.Config{
		RuntimeSDKVersion: "9.9.9",
	}

	err := validateRuntimeFingerprint(fingerprint, configuration)
	if err == nil || !strings.Contains(err.Error(), "SDK version") {
		t.Fatalf("expected unsupported SDK version, got %v", err)
	}
}

func TestValidateRuntimeFingerprintRejectsModifiedRuntimeID(t *testing.T) {
	fingerprint := testRuntimeFingerprint("wasmhash")
	fingerprint.RuntimeID = strings.Repeat("0", 64)

	err := validateRuntimeFingerprint(fingerprint, config.Config{})
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("expected runtime checksum error, got %v", err)
	}
}

func testRuntimeFingerprint(seed string) domain.RuntimeFingerprint {
	hash := func(value string) string {
		sum := sha256.Sum256([]byte(value))
		return hex.EncodeToString(sum[:])
	}

	fingerprint := domain.RuntimeFingerprint{
		SDKVersion:              "0.3.0",
		LivenessCoreVersion:     "0.1.0",
		LivenessCoreWASMSHA256: hash(seed + "/wasm"),
		MediaPipeVersion:        "1.0.1",
		MediaPipeVisionSHA256:   hash(seed + "/vision"),
		FaceLandmarkerSHA256:    hash(seed + "/face"),
	}
	canonical := strings.Join([]string{
		fingerprint.SDKVersion,
		fingerprint.LivenessCoreVersion,
		fingerprint.LivenessCoreWASMSHA256,
		fingerprint.MediaPipeVersion,
		fingerprint.MediaPipeVisionSHA256,
		fingerprint.FaceLandmarkerSHA256,
	}, "|")
	fingerprint.RuntimeID = hash(canonical)
	return fingerprint
}
