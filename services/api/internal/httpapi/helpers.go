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

func (handler *Handler) setCORS(writer http.ResponseWriter) {
	writer.Header().Set("Access-Control-Allow-Origin", handler.config.AllowedOrigin)
	writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
}

func (handler *Handler) writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
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
	return nil
}
