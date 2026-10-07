package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"faceproof/services/api/internal/session"
)

func sessionErrorStatus(err error) int {
	if errors.Is(err, session.ErrSessionCompleted) {
		return http.StatusConflict
	}
	return http.StatusBadRequest
}

func (handler *Handler) setSecurityHeaders(writer http.ResponseWriter) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Pragma", "no-cache")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("X-Frame-Options", "DENY")
	writer.Header().Set("Referrer-Policy", "no-referrer")
	writer.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), payment=()")
}

func (handler *Handler) setCORS(writer http.ResponseWriter) {
	writer.Header().Set("Access-Control-Allow-Origin", handler.config.AllowedOrigin)
	writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-FaceProof-Identity-Token")
	writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	writer.Header().Set("Vary", "Origin")
}

func (handler *Handler) writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("Cache-Control", "no-store")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

func (handler *Handler) writeError(writer http.ResponseWriter, status int, message string) {
	handler.writeJSON(writer, status, map[string]string{"error": message})
}

func splitPath(path string) []string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func decodeJSON(request *http.Request, target any, maxBytes int64) error {
	defer request.Body.Close()

	body, err := io.ReadAll(io.LimitReader(request.Body, maxBytes+1))
	if err != nil {
		return err
	}
	if int64(len(body)) > maxBytes {
		return errors.New("request body is too large")
	}

	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body contains trailing JSON values")
		}
		return err
	}
	return nil
}
