package config

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	APIAddress              string
	EngineURL               string
	SecureCoreLibrary       string
	ModelPackPath           string
	ModelPackPublicKey      string
	ModelPackCacheDirectory string
	YUNetModelPath          string
	SFaceModelPath          string
	MiniFASNetModelPath     string
	AllowedOrigin           string
	SessionSecret           []byte
	TemplateKey             []byte
	TemplateDirectory       string
	SessionTTL              time.Duration
	MatchThreshold          float64
	LivenessThreshold       float64
	ReviewLivenessThreshold float64
	AllowReviewEnrollment   bool
	RequirePassivePAD       bool
	Debug                    bool

	RuntimeSDKVersion              string
	RuntimeLivenessCoreVersion     string
	RuntimeLivenessCoreWASMSHA256 string
	RuntimeMediaPipeVersion        string
	RuntimeMediaPipeVisionSHA256   string
	RuntimeFaceLandmarkerSHA256    string

	IdentityIssuerKey       []byte
	TenantRegistryPath      string
	IdentityStoreKey        []byte
	IdentityDirectory       string
	IdentityVerifyURL       string
	IdentityLinkTTL         time.Duration
	IdentityMaxPDFBytes     int64
	AdminKey                []byte
	AnalyticsDatabaseURL    string
	AnalyticsSubjectKey     []byte
	PDFSigPath              string
	PDFInfoPath             string
	BPGDecoderPath          string
}

func Load() (Config, error) {
	sessionSecret := []byte(os.Getenv("FACEPROOF_SESSION_SECRET"))
	if len(sessionSecret) < 32 {
		return Config{}, errors.New("FACEPROOF_SESSION_SECRET must contain at least 32 characters")
	}

	templateKeyEncoded := os.Getenv("FACEPROOF_TEMPLATE_KEY")
	templateKey, err := base64.StdEncoding.DecodeString(templateKeyEncoded)
	if err != nil || len(templateKey) != 32 {
		return Config{}, errors.New("FACEPROOF_TEMPLATE_KEY must be a Base64-encoded 32-byte key")
	}

	identityIssuerKey := []byte(strings.TrimSpace(os.Getenv("FACEPROOF_IDENTITY_ISSUER_KEY")))
	if len(identityIssuerKey) > 0 && len(identityIssuerKey) < 32 {
		return Config{}, errors.New("FACEPROOF_IDENTITY_ISSUER_KEY must contain at least 32 characters when configured")
	}
	adminKey := []byte(strings.TrimSpace(os.Getenv("FACEPROOF_ADMIN_KEY")))
	if len(adminKey) > 0 && len(adminKey) < 32 {
		return Config{}, errors.New("FACEPROOF_ADMIN_KEY must contain at least 32 characters when configured")
	}

	sessionTTLSeconds := envInt("FACEPROOF_SESSION_TTL_SECONDS", 120)
	identityTTLMinutes := envInt("FACEPROOF_IDENTITY_LINK_TTL_MINUTES", 60)
	identityMaxPDFMB := envInt("FACEPROOF_IDENTITY_MAX_PDF_MB", 8)
	if identityTTLMinutes <= 0 || identityMaxPDFMB <= 0 {
		return Config{}, errors.New("identity TTL and PDF size limits must be positive")
	}

	return Config{
		APIAddress:              envString("FACEPROOF_API_ADDR", ":8180"),
		EngineURL:               envString("FACEPROOF_ENGINE_URL", "http://127.0.0.1:8090"),
		SecureCoreLibrary:       strings.TrimSpace(os.Getenv("FACEPROOF_SECURE_CORE_LIBRARY")),
		ModelPackPath:           strings.TrimSpace(os.Getenv("FACEPROOF_MODEL_PACK")),
		ModelPackPublicKey:      strings.TrimSpace(os.Getenv("FACEPROOF_MODEL_PACK_PUBLIC_KEY")),
		ModelPackCacheDirectory: envString("FACEPROOF_MODEL_PACK_CACHE_DIR", "../../data/model-cache"),
		YUNetModelPath:          strings.TrimSpace(os.Getenv("FACEPROOF_YUNET_MODEL")),
		SFaceModelPath:          strings.TrimSpace(os.Getenv("FACEPROOF_SFACE_MODEL")),
		MiniFASNetModelPath:     strings.TrimSpace(os.Getenv("FACEPROOF_MINIFASNET_MODEL")),
		AllowedOrigin:           envString("FACEPROOF_ALLOWED_ORIGIN", "http://localhost:5173"),
		SessionSecret:           sessionSecret,
		TemplateKey:             templateKey,
		TemplateDirectory:       envString("FACEPROOF_TEMPLATE_DIR", "../../data/templates"),
		SessionTTL:              time.Duration(sessionTTLSeconds) * time.Second,
		MatchThreshold:          envFloat("FACEPROOF_MATCH_THRESHOLD", 0.363),
		LivenessThreshold:       envFloat("FACEPROOF_LIVENESS_THRESHOLD", 0.68),
		ReviewLivenessThreshold: envFloat("FACEPROOF_REVIEW_LIVENESS_THRESHOLD", 0.55),
		AllowReviewEnrollment:   envBool("FACEPROOF_ALLOW_REVIEW_ENROLLMENT", false),
		RequirePassivePAD:       envBool("FACEPROOF_REQUIRE_PASSIVE_PAD", true),
		Debug:                   envBool("FACEPROOF_DEBUG", false),
		RuntimeSDKVersion:              envString("FACEPROOF_RUNTIME_SDK_VERSION", "0.3.0"),
		RuntimeLivenessCoreVersion:     envString("FACEPROOF_RUNTIME_LIVENESS_CORE_VERSION", "0.1.0"),
		RuntimeLivenessCoreWASMSHA256: strings.ToLower(strings.TrimSpace(os.Getenv("FACEPROOF_RUNTIME_LIVENESS_WASM_SHA256"))),
		RuntimeMediaPipeVersion:        envString("FACEPROOF_RUNTIME_MEDIAPIPE_VERSION", "1.0.1"),
		RuntimeMediaPipeVisionSHA256:   envString("FACEPROOF_RUNTIME_MEDIAPIPE_VISION_SHA256", "d885630c297c0b20b1fe86096cb06291c4c8080876f27852e724f24ac603713f"),
		RuntimeFaceLandmarkerSHA256:    envString("FACEPROOF_RUNTIME_FACE_LANDMARKER_SHA256", "64184e229b263107bc2b804c6625db1341ff2bb731874b0bcc2fe6544e0bc9ff"),
		IdentityIssuerKey:       identityIssuerKey,
		TenantRegistryPath:      strings.TrimSpace(os.Getenv("FACEPROOF_TENANT_REGISTRY")),
		IdentityStoreKey:        deriveIdentityStoreKey(templateKey),
		IdentityDirectory:       envString("FACEPROOF_IDENTITY_DIR", "../../data/identity-checks"),
		IdentityVerifyURL:       envString("FACEPROOF_IDENTITY_VERIFY_URL", "http://localhost:5173/verify.html"),
		IdentityLinkTTL:         time.Duration(identityTTLMinutes) * time.Minute,
		IdentityMaxPDFBytes:     int64(identityMaxPDFMB) << 20,
		AdminKey:                adminKey,
		AnalyticsDatabaseURL:    strings.TrimSpace(os.Getenv("FACEPROOF_ANALYTICS_DATABASE_URL")),
		AnalyticsSubjectKey:     deriveAnalyticsSubjectKey(templateKey),
		PDFSigPath:              strings.TrimSpace(os.Getenv("FACEPROOF_PDFSIG_PATH")),
		PDFInfoPath:             strings.TrimSpace(os.Getenv("FACEPROOF_PDFINFO_PATH")),
		BPGDecoderPath:          strings.TrimSpace(os.Getenv("FACEPROOF_BPGDEC_PATH")),
	}, nil
}

func deriveIdentityStoreKey(master []byte) []byte {
	return deriveScopedKey(master, "faceproof/identity-store/v1")
}

func deriveAnalyticsSubjectKey(master []byte) []byte {
	return deriveScopedKey(master, "faceproof/analytics-subject/v1")
}

func deriveScopedKey(master []byte, scope string) []byte {
	mac := hmac.New(sha256.New, master)
	_, _ = mac.Write([]byte(scope))
	return mac.Sum(nil)
}

func envString(name string, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envFloat(name string, fallback float64) float64 {
	value := os.Getenv(name)
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
