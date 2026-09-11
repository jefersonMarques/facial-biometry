package config

import (
	"encoding/base64"
	"errors"
	"os"
	"strconv"
	"time"
)

type Config struct {
	APIAddress              string
	EngineURL               string
	AllowedOrigin           string
	SessionSecret           []byte
	TemplateKey             []byte
	TemplateDirectory       string
	SessionTTL              time.Duration
	MatchThreshold          float64
	LivenessThreshold       float64
	ReviewLivenessThreshold float64
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

	sessionTTLSeconds := envInt("FACEPROOF_SESSION_TTL_SECONDS", 120)

	return Config{
		APIAddress:              envString("FACEPROOF_API_ADDR", ":8080"),
		EngineURL:               envString("FACEPROOF_ENGINE_URL", "http://127.0.0.1:8090"),
		AllowedOrigin:           envString("FACEPROOF_ALLOWED_ORIGIN", "http://localhost:5173"),
		SessionSecret:           sessionSecret,
		TemplateKey:             templateKey,
		TemplateDirectory:       envString("FACEPROOF_TEMPLATE_DIR", "../../data/templates"),
		SessionTTL:              time.Duration(sessionTTLSeconds) * time.Second,
		MatchThreshold:          envFloat("FACEPROOF_MATCH_THRESHOLD", 0.363),
		LivenessThreshold:       envFloat("FACEPROOF_LIVENESS_THRESHOLD", 0.68),
		ReviewLivenessThreshold: envFloat("FACEPROOF_REVIEW_LIVENESS_THRESHOLD", 0.55),
	}, nil
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
