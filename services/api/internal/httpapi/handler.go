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

const (
	captureDurationMS    = 2400
	sampleIntervalMS     = 120
	illuminationSettleMS = 65
	illuminationSteps    = 6
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
	SessionID            string    `json:"sessionId"`
	SessionToken         string    `json:"sessionToken"`
	Kind                 string    `json:"kind"`
	ExpiresAt            time.Time `json:"expiresAt"`
	CaptureDurationMS    int       `json:"captureDurationMs"`
	SampleIntervalMS     int       `json:"sampleIntervalMs"`
	IlluminationSettleMS int       `json:"illuminationSettleMs"`
	IlluminationPattern  []float64 `json:"illuminationPattern"`
}

type completeSessionRequest struct {
	SessionToken string                  `json:"sessionToken"`
	Frames       []domain.CapturedFrame  `json:"frames"`
	Metadata     *domain.CaptureMetadata `json:"metadata,omitempty"`
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
		risk: risk.NewEngine(
			configuration.LivenessThreshold,
			configuration.ReviewLivenessThreshold,
			configuration.MatchThreshold,
			configuration.RequirePassivePAD,
		),
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
	illuminationPattern, err := randomIlluminationPattern(illuminationSteps)
	if err != nil {
		handler.writeError(writer, http.StatusInternalServerError, "failed to create illumination challenge")
		return
	}

	now := time.Now().UTC()
	captureSession := domain.CaptureSession{
		ID:                   sessionID,
		SubjectID:            payload.SubjectID,
		Kind:                 kind,
		IlluminationPattern:  illuminationPattern,
		CaptureDurationMS:    captureDurationMS,
		SampleIntervalMS:     sampleIntervalMS,
		IlluminationSettleMS: illuminationSettleMS,
		CreatedAt:            now,
		ExpiresAt:            now.Add(handler.config.SessionTTL),
	}
	handler.sessions.Put(captureSession)

	token, err := handler.signer.Sign(captureSession.ID, captureSession.ExpiresAt)
	if err != nil {
		handler.writeError(writer, http.StatusInternalServerError, "failed to sign session")
		return
	}

	handler.writeJSON(writer, http.StatusCreated, createSessionResponse{
		SessionID:            captureSession.ID,
		SessionToken:         token,
		Kind:                 string(captureSession.Kind),
		ExpiresAt:            captureSession.ExpiresAt,
		CaptureDurationMS:    captureSession.CaptureDurationMS,
		SampleIntervalMS:     captureSession.SampleIntervalMS,
		IlluminationSettleMS: captureSession.IlluminationSettleMS,
		IlluminationPattern:  captureSession.IlluminationPattern,
	})
}

func (handler *Handler) completeSession(writer http.ResponseWriter, request *http.Request, sessionID string, expectedKind domain.SessionKind) {
	captureSession, err := handler.sessions.Get(sessionID)
	if err != nil {
		handler.writeError(writer, sessionErrorStatus(err), err.Error())
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

	normalizedFrames, err := normalizeCapturedFrames(payload.Frames, captureSession)
	if err != nil {
		handler.writeError(writer, http.StatusBadRequest, err.Error())
		return
	}

	captureSession, err = handler.sessions.Consume(sessionID)
	if err != nil {
		handler.writeError(writer, sessionErrorStatus(err), err.Error())
		return
	}

	result, err := handler.engine.Analyze(request.Context(), domain.EngineRequest{
		Frames:              normalizedFrames,
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

	passivePADAvailable := result.PassivePAD.Status == "available"
	if captureSession.Kind == domain.SessionKindEnrollment {
		response.Decision = handler.risk.EnrollmentDecision(result.LivenessScore, passivePADAvailable)
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
		response.Decision = handler.risk.VerificationDecision(result.LivenessScore, similarity, passivePADAvailable)
	}

	handler.writeJSON(writer, http.StatusOK, response)
}

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
			return nil, errors.New("frame timestamps must be strictly inc