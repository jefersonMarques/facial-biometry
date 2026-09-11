package httpapi

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"faceproof/services/api/internal/config"
	"faceproof/services/api/internal/domain"
	"faceproof/services/api/internal/engine"
	"faceproof/services/api/internal/matching"
	"faceproof/services/api/internal/risk"
	"faceproof/services/api/internal/security"
	"faceproof/services/api/internal/session"
	templaterepository "faceproof/services/api/internal/template"
)

type Handler struct {
	config    config.Config
	sessions  *session.Store
	signer    *security.Signer
	engine    *engine.Client
	templates *templaterepository.Repository
	risk      *risk.Engine
}

type createSessionRequest struct {
	SubjectID string `json:"subjectId"`
}

type createSessionResponse struct {
	SessionID           string    `json:"sessionId"`
	SessionToken        string    `json:"sessionToken"`
	Kind                string    `json:"kind"`
	ExpiresAt           time.Time `json:"expiresAt"`
	CaptureDurationMS   int       `json:"captureDurationMs"`
	SampleIntervalMS    int       `json:"sampleIntervalMs"`
	IlluminationPattern []float64 `json:"illuminationPattern"`
}

type completeSessionRequest struct {
	SessionToken string                 `json:"sessionToken"`
	Frames       []domain.CapturedFrame `json:"frames"`
}

type completeSessionResponse struct {
	SessionID           string               `json:"sessionId"`
	SubjectID           string               `json:"subjectId"`
	Kind                string               `json:"kind"`
	Decision            string               `json:"decision"`
	LivenessScore       float64              `json:"livenessScore"`
	Similarity          *float64             `json:"similarity,omitempty"`
	MatchThreshold      *float64             `json:"matchThreshold,omitempty"`
	TemplateStored      bool                 `json:"templateStored,omitempty"`
	TemplateProvisional bool                 `json:"templateProvisional,omitempty"`
	Signals             signalResponse       `json:"signals"`
	Quality             domain.EngineQuality `json:"quality"`
	Diagnostics         []string             `json:"diagnostics"`
}

type signalResponse struct {
	PassivePAD     domain.EngineSignal `json:"passivePad"`
	TemporalMotion domain.EngineSignal `json:"temporalMotion"`
	Illumination   domain.EngineSignal `json:"illumination"`
}

func NewHandler(configuration config.Config, sessions *session.Store, signer *security.Signer, engineClient *engine.Client, templates *templaterepository.Repository) *Handler {
	return &Handler{
		config:    configuration,
		sessions:  sessions,
		signer:    signer,
		engine:    engineClient,
		templates: templates,
		risk:      risk.NewEngine(configuration.LivenessThreshold, configuration.ReviewLivenessThreshold, configuration.MatchThreshold),
	}
}

func (handler *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	handler.setCORS(writer)
	if request.Method == http.MethodOptions {
		writer.WriteHeader(http.StatusNoContent)
		return
	}

	if request.URL.Path == "/health" && request.Method == http.MethodGet {
		handler.writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
		return
	}

	if request.URL.Path == "/v1/biometric/enrollments" && request.Method == http.MethodPost {
		handler.createSession(writer, request, domain.SessionKindEnrollment)
		return
	}
	if request.URL.Path == "/v1/biometric/verifications" && request.Method == http.MethodPost {
		handler.createSession(writer, request, domain.SessionKindVerification)
		return
	}

	segments := splitPath(request.URL.Path)
	if len(segments) == 5 && segments[0] == "v1" && segments[1] == "biometric" && segments[4] == "complete" && request.Method == http.MethodPost {
		kind := domain.SessionKind("")
		switch segments[2] {
		case "enrollments":
			kind = domain.SessionKindEnrollment
		case "verifications":
			kind = domain.SessionKindVerification
		default:
			handler.writeError(writer, http.StatusNotFound, "route not found")
			return
		}
		handler.completeSession(writer, request, segments[3], kind)
		return
	}

	handler.writeError(writer, http.StatusNotFound, "route not found")
}

func (handler *Handler) createSession(writer http.ResponseWriter, request *http.Request, kind domain.SessionKind) {
	var payload createSessionRequest
	if err := decodeJSON(request, &payload, 1<<20); err != nil {
		handler.writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	payload.SubjectID = strings.TrimSpace(payload.SubjectID)
	if payload.SubjectID == "" {
		handler.writeError(writer, http.StatusBadRequest, "subjectId is required")
		return
	}
	if kind == domain.SessionKindVerification {
		if _, err := handler.templates.Load(payload.SubjectID); err != nil {
			if errors.Is(err, templaterepository.ErrTemplateNotFound) {
				handler.writeError(writer, http.StatusNotFound, "subject has no enrollment")
				return
			}
			handler.writeError(writer, http.StatusInternalServerError, "failed to load biometric template")
			return
		}
	}

	sessionID, err := security.RandomID(18)
	if err != nil {
		handler.writeError(writer, http.StatusInternalServerError, "failed to create session")
		return
	}
	now := time.Now().UTC()
	captureSession := domain.CaptureSession{
		ID:                  sessionID,
		SubjectID:           payload.SubjectID,
		Kind:                kind,
		IlluminationPattern: randomIlluminationPattern(6),
		CreatedAt:           now,
		ExpiresAt:           now.Add(handler.config.SessionTTL),
	}
	handler.sessions.Put(captureSession)

	token, err := handler.signer.Sign(captureSession.ID, captureSession.ExpiresAt)
	if err != nil {
		handler.writeError(writer, http.StatusInternalServerError, "failed to sign session")
		return
	}

	handler.writeJSON(writer, http.StatusCreated, createSessionResponse{
		SessionID:           captureSession.ID,
		SessionToken:        token,
		Kind:                string(captureSession.Kind),
		ExpiresAt:           captureSession.ExpiresAt,
		CaptureDurationMS:   2400,
		SampleIntervalMS:    120,
		IlluminationPattern: captureSession.IlluminationPattern,
	})
}

func (handler *Handler) completeSession(writer http.ResponseWriter, request *http.Request, sessionID string, expectedKind domain.SessionKind) {
	captureSession, err := handler.sessions.Get(sessionID)
	if err != nil {
		handler.writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	if captureSession.Kind != expectedKind {
		handler.writeError(writer, http.StatusBadRequest, "session kind mismatch")
		return
	}

	var payload completeSessionRequest
	if err := decodeJSON(request, &payload, 16<<20); err != nil {
		handler.writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	if err := handler.signer.Verify(payload.SessionToken, sessionID); err != nil {
		handler.writeError(writer, http.StatusUnauthorized, err.Error())
		return
	}
	if len(payload.Frames) < 8 || len(payload.Frames) > 32 {
		handler.writeError(writer, http.StatusBadRequest, "capture must contain between 8 and 32 frames")
		return
	}

	result, err := handler.engine.Analyze(request.Context(), domain.EngineRequest{
		Frames:              payload.Frames,
		IlluminationPattern: captureSession.IlluminationPattern,
	})
	if err != nil {
		handler.writeError(writer, http.StatusBadGateway, "biometric engine failed: "+err.Error())
		return
	}

	response := completeSessionResponse{
		SessionID:     captureSession.ID,
		SubjectID:     captureSession.SubjectID,
		Kind:          string(captureSession.Kind),
		LivenessScore: result.LivenessScore,
		Signals: signalResponse{
			PassivePAD:     result.PassivePAD,
			TemporalMotion: result.TemporalMotion,
			Illumination:   result.Illumination,
		},
		Quality:     result.Quality,
		Diagnostics: append([]string(nil), result.Diagnostics...),
	}

	if captureSession.Kind == domain.SessionKindEnrollment {
		response.Decision = handler.risk.EnrollmentDecision(result.LivenessScore)
		shouldStoreTemplate := response.Decision == "approved" || (response.Decision == "review" && handler.config.AllowReviewEnrollment)
		if shouldStoreTemplate {
			biometricTemplate := domain.BiometricTemplate{
				SubjectID:      captureSession.SubjectID,
				Embedding:      result.Embedding,
				EmbeddingModel: result.EmbeddingModel,
				CreatedAt:      time.Now().UTC(),
			}
			if err := handler.templates.Save(biometricTemplate); err != nil {
				handler.writeError(writer, http.StatusInternalServerError, "failed to store biometric template")
				return
			}
			response.TemplateStored = true
			response.TemplateProvisional = response.Decision == "review"
			if response.TemplateProvisional {
				response.Diagnostics = append(response.Diagnostics, "review enrollment template stored because development mode is enabled")
			}
		}
	} else {
		biometricTemplate, err := handler.templates.Load(captureSession.SubjectID)
		if err != nil {
			handler.writeError(writer, http.StatusInternalServerError, "failed to load biometric template")
			return
		}
		similarity := matching.CosineSimilarity(biometricTemplate.Embedding, result.Embedding)
		threshold := handler.config.MatchThreshold
		response.Similarity = &similarity
		response.MatchThreshold = &threshold
		response.Decision = handler.risk.VerificationDecision(result.LivenessScore, similarity)
	}

	if err := handler.sessions.MarkCompleted(sessionID); err != nil {
		handler.writeError(writer, http.StatusInternalServerError, "failed to complete session")
		return
	}
	handler.writeJSON(writer, http.StatusOK, response)
}

func randomIlluminationPattern(length int) []float64 {
	pattern := make([]float64, length)
	buffer := make([]byte, length)
	if _, err := rand.Read(buffer); err != nil {
		for index := range pattern {
			pattern[index] = 0.5
		}
		return pattern
	}
	for index, value := range buffer {
		pattern[index] = 0.22 + (float64(value)/255.0)*0.68
	}
	return pattern
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
