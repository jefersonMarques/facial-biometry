package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"faceproof/services/api/internal/document/cnh"
	"faceproof/services/api/internal/domain"
	"faceproof/services/api/internal/identity"
	"faceproof/services/api/internal/matching"
)

const (
	maxIdentityDocumentAttempts = 3
	maxIdentityBiometricSessions = 3
	maxIdentityCaptureSessions   = 12
	identityMultipartOverhead    = 1 << 20
)

var (
	errIdentityInvalidState = errors.New("identity check is not in the expected state")
	errIdentityExpired      = errors.New("identity check expired")
	errIdentityAttemptLimit = errors.New("identity check attempt limit reached")
)

type createIdentityCheckRequest struct {
	CPF                 string `json:"cpf"`
	MinimumDocumentDate string `json:"minimumDocumentDate"`
	ExpiresInMinutes    int    `json:"expiresInMinutes,omitempty"`
	CampaignID          string `json:"campaignId,omitempty"`
	Scenario            string `json:"scenario,omitempty"`
	ExpectedDecision    string `json:"expectedDecision,omitempty"`
}

type createIdentityCheckResponse struct {
	ID               string    `json:"id"`
	VerificationURL  string    `json:"verificationUrl"`
	ExpiresAt        time.Time `json:"expiresAt"`
	CampaignID       string    `json:"campaignId,omitempty"`
	Scenario         string    `json:"scenario,omitempty"`
	ExpectedDecision string    `json:"expectedDecision,omitempty"`
}

type identityStatusResponse struct {
	ID               string          `json:"id"`
	Status           identity.Status `json:"status"`
	ExpiresAt        time.Time       `json:"expiresAt"`
	DocumentAccepted bool            `json:"documentAccepted"`
	CanStartBiometry bool            `json:"canStartBiometry"`
	Decision         string          `json:"decision,omitempty"`
}

type identityDocumentDetails struct {
	SignatureValid        bool   `json:"signatureValid"`
	VIOSignatureValid     bool   `json:"vioSignatureValid"`
	CPFMatch              bool   `json:"cpfMatch"`
	FreshnessValid        bool   `json:"freshnessValid"`
	Name                  string `json:"name,omitempty"`
	CPF                   string `json:"cpf,omitempty"`
	BirthDate             string `json:"birthDate,omitempty"`
	Category              string `json:"category,omitempty"`
	ExpiryDate            string `json:"expiryDate,omitempty"`
	IssuingUF             string `json:"issuingUf,omitempty"`
	ReferencePhotoDataURL string `json:"referencePhotoDataUrl,omitempty"`
}

type identityDocumentResponse struct {
	Status   identity.Status          `json:"status"`
	Document identityDocumentDetails `json:"document"`
}

type identityCompleteRequest struct {
	SessionID       string                         `json:"sessionId"`
	SessionToken    string                         `json:"sessionToken"`
	GuidedFrames    []domain.GuidedCapturedFrame   `json:"guidedFrames"`
	Metadata        *domain.CaptureMetadata        `json:"metadata,omitempty"`
	Runtime         domain.RuntimeFingerprint      `json:"runtime"`
	CaptureProtocol domain.CaptureProtocolMetadata `json:"captureProtocol"`
	Geometry        *domain.GeometryTelemetry       `json:"geometry,omitempty"`
}

type issuerIdentityCheckResponse struct {
	ID                  string                     `json:"id"`
	Status              identity.Status            `json:"status"`
	MinimumDocumentDate time.Time                  `json:"minimumDocumentDate"`
	CreatedAt           time.Time                  `json:"createdAt"`
	ExpiresAt           time.Time                  `json:"expiresAt"`
	Decision            string                     `json:"decision,omitempty"`
	Document            *identity.DocumentEvidence `json:"document,omitempty"`
	LivenessScore       float64                    `json:"livenessScore,omitempty"`
	FaceSimilarity      float64                    `json:"faceSimilarity,omitempty"`
	CampaignID          string                     `json:"campaignId,omitempty"`
	Scenario            string                     `json:"scenario,omitempty"`
	ExpectedDecision    string                     `json:"expectedDecision,omitempty"`
	Runtime             *domain.RuntimeFingerprint      `json:"runtime,omitempty"`
	CaptureProtocol     *domain.CaptureProtocolMetadata `json:"captureProtocol,omitempty"`
}

type identityCompleteResponse struct {
	ID                string                  `json:"id"`
	Status            identity.Status         `json:"status"`
	Decision          string                  `json:"decision"`
	LivenessScore     float64                 `json:"livenessScore"`
	Similarity        float64                 `json:"similarity"`
	FrameSimilarities []float64               `json:"frameSimilarities,omitempty"`
	BestFrameIndex    int                     `json:"bestFrameIndex"`
	MatchThreshold    float64                 `json:"matchThreshold"`
	Signals           signalResponse          `json:"signals"`
	Quality           domain.EngineQuality            `json:"quality"`
	Diagnostics       []string                        `json:"diagnostics"`
	NativeShadow      *domain.NativeShadowComparison `json:"nativeShadow,omitempty"`
	Document          identityDocumentDetails         `json:"document"`
}

func (handler *Handler) createIdentityCheck(writer http.ResponseWriter, request *http.Request) {
	if handler.identityChecks == nil || handler.cnhDocuments == nil {
		handler.writeError(writer, http.StatusServiceUnavailable, "identity verification is unavailable")
		return
	}
	if len(handler.config.IdentityIssuerKey) == 0 {
		handler.writeError(writer, http.StatusServiceUnavailable, "identity issuer is not configured")
		return
	}
	if !handler.authorizeIdentityIssuer(request) {
		handler.writeError(writer, http.StatusUnauthorized, "invalid issuer credentials")
		return
	}

	var payload createIdentityCheckRequest
	if err := decodeJSON(request, &payload, 1<<20); err != nil {
		handler.writeError(writer, http.StatusBadRequest, err.Error())
		return
	}

	response, err := handler.createIdentityCheckRecord(request.Context(), payload)
	if err != nil {
		writeIdentityCreateError(handler, writer, err)
		return
	}
	handler.writeJSON(writer, http.StatusCreated, response)
}

func (handler *Handler) getIdentityCheck(writer http.ResponseWriter, request *http.Request) {
	check, _, ok := handler.loadPublicIdentityCheck(writer, request)
	if !ok {
		return
	}
	handler.recordAnalyticsLinkOpened(request.Context(), check.ID)
	handler.writeJSON(writer, http.StatusOK, identityStatusResponse{
		ID:               check.ID,
		Status:           check.Status,
		ExpiresAt:        check.ExpiresAt,
		DocumentAccepted: check.Document != nil,
		CanStartBiometry: check.Status == identity.StatusBiometryPending,
		Decision:         check.Decision,
	})
}

func (handler *Handler) uploadIdentityDocument(writer http.ResponseWriter, request *http.Request) {
	check, token, ok := handler.loadPublicIdentityCheck(writer, request)
	if !ok {
		return
	}
	if time.Now().After(check.ExpiresAt) {
		handler.writeError(writer, http.StatusGone, "identity check expired")
		return
	}

	pdf, err := readIdentityPDF(writer, request, handler.config.IdentityMaxPDFBytes)
	if err != nil {
		handler.writeError(writer, http.StatusBadRequest, err.Error())
		return
	}

	check, err = handler.identityChecks.Update(token, func(current *identity.Check) error {
		if current.Status != identity.StatusPendingDocument {
			return errIdentityInvalidState
		}
		if current.DocumentAttempts >= maxIdentityDocumentAttempts {
			return errIdentityAttemptLimit
		}
		current.DocumentAttempts++
		current.Status = identity.StatusProcessingDocument
		current.LastErrorCode = ""
		return nil
	})
	if err != nil {
		handler.writeIdentityStateError(writer, err)
		return
	}
	handler.recordAnalyticsEvent(request.Context(), check.ID, "document_upload_started", map[string]any{
		"attempt": check.DocumentAttempts,
		"bytes":   len(pdf),
	})

	document, documentErr := handler.cnhDocuments.Process(request.Context(), pdf, check.ExpectedCPF, check.MinimumDocumentDate)
	if documentErr != nil {
		handler.resetIdentityDocumentAttempt(token, documentErr)
		if failedCheck, loadErr := handler.identityChecks.Load(token); loadErr == nil {
			handler.recordAnalyticsDocumentFailure(request.Context(), failedCheck, documentErr)
		}
		switch {
		case errors.Is(documentErr, cnh.ErrDependencyUnavailable):
			handler.writeError(writer, http.StatusServiceUnavailable, documentErr.Error())
		case errors.Is(documentErr, cnh.ErrPhotoUnavailable):
			handler.writeError(writer, http.StatusUnprocessableEntity, "CNH reference photo is unavailable")
		case errors.Is(documentErr, cnh.ErrDocumentTooOld):
			handler.writeError(writer, http.StatusUnprocessableEntity, "CNH Digital was generated before the minimum date required for this verification")
		case errors.Is(documentErr, cnh.ErrDocumentMismatch):
			handler.writeError(writer, http.StatusUnprocessableEntity, "CNH Digital does not correspond to this verification")
		default:
			if handler.config.Debug {
				handler.writeError(writer, http.StatusUnprocessableEntity, documentErr.Error())
			} else {
				handler.writeError(writer, http.StatusUnprocessableEntity, "CNH Digital is invalid or could not be authenticated")
			}
		}
		return
	}

	referenceImage := "data:" + document.PhotoMIME + ";base64," + base64.StdEncoding.EncodeToString(document.Photo)
	reference, err := handler.engine.ExtractReference(request.Context(), referenceImage)
	if err != nil {
		failure := errors.New("reference_face_failed")
		handler.resetIdentityDocumentAttempt(token, failure)
		if failedCheck, loadErr := handler.identityChecks.Load(token); loadErr == nil {
			handler.recordAnalyticsDocumentFailure(request.Context(), failedCheck, failure)
		}
		handler.writeError(writer, http.StatusBadGateway, "could not analyze the CNH reference photo")
		return
	}

	updatedDocumentCheck, err := handler.identityChecks.Update(token, func(current *identity.Check) error {
		if current.Status != identity.StatusProcessingDocument {
			return errIdentityInvalidState
		}
		current.Document = &identity.DocumentEvidence{
			PDFSHA256:            document.PDFSHA256,
			PDFSigningTime:       document.PDFSigningTime,
			PDFSigner:            document.PDFSigner,
			PDFCreator:           document.PDFCreator,
			PDFProducer:           document.PDFProducer,
			PDFSourceIntegrity:    document.PDFSourceIntegrity,
			PDFSignatureAlgorithm: document.PDFSignatureAlgorithm,
			ReferencePhotoSource:     document.PhotoSource,
			ReferencePhotoMethod:     document.PhotoMethod,
			ReferencePhotoConfidence: document.PhotoConfidence,
			ReferencePhotoSHA256:     document.PhotoSHA256,
			ReferencePhotoWidth:      document.PhotoWidth,
			ReferencePhotoHeight:     document.PhotoHeight,
			VIOTemplateID:         document.VIOTemplateID,
			VIOCreatedAt:          document.VIOCreatedAt,
			VIOSignatureAlgorithm: document.VIOSignatureAlgorithm,
			Name:                  document.Name,
			BirthDate:             document.BirthDate,
			Category:              document.Category,
			ExpiryDate:            document.ExpiryDate,
			IssuingUF:             document.IssuingUF,
		}
		current.ReferenceEmbedding = append([]float64(nil), reference.Embedding...)
		current.ReferenceEmbeddings = cloneEmbeddings(reference.Embeddings)
		if len(current.ReferenceEmbeddings) == 0 && len(current.ReferenceEmbedding) > 0 {
			current.ReferenceEmbeddings = [][]float64{append([]float64(nil), current.ReferenceEmbedding...)}
		}
		current.ReferenceEmbeddingModel = reference.EmbeddingModel
		current.Status = identity.StatusBiometryPending
		current.LastErrorCode = ""
		return nil
	})
	if err != nil {
		handler.writeError(writer, http.StatusConflict, "identity check state changed")
		return
	}
	handler.recordAnalyticsDocumentAccepted(request.Context(), updatedDocumentCheck)

	response := identityDocumentResponse{
		Status: identity.StatusBiometryPending,
		Document: identityDocumentDetails{
			SignatureValid:        true,
			VIOSignatureValid:     true,
			CPFMatch:              true,
			FreshnessValid:        true,
			Name:                  document.Name,
			CPF:                   maskCPF(document.CPF),
			BirthDate:             document.BirthDate,
			Category:              document.Category,
			ExpiryDate:            document.ExpiryDate,
			IssuingUF:             document.IssuingUF,
			ReferencePhotoDataURL: referenceImage,
		},
	}
	handler.writeJSON(writer, http.StatusOK, response)
}

func (handler *Handler) issueIdentitySession(writer http.ResponseWriter, request *http.Request) {
	_, token, ok := handler.loadPublicIdentityCheck(writer, request)
	if !ok {
		return
	}

	var sessionResponse createSessionResponse
	updatedCheck, err := handler.identityChecks.Update(token, func(check *identity.Check) error {
		if time.Now().After(check.ExpiresAt) {
			return errIdentityExpired
		}
		if check.Status != identity.StatusBiometryPending || check.Document == nil ||
			(len(check.ReferenceEmbedding) == 0 && len(check.ReferenceEmbeddings) == 0) {
			return errIdentityInvalidState
		}
		if check.BiometricSessions >= maxIdentityBiometricSessions {
			return errIdentityAttemptLimit
		}
		if check.CaptureSessionsIssued >= maxIdentityCaptureSessions {
			return errIdentityAttemptLimit
		}

		response, err := handler.issueCaptureSession(check.ID, domain.SessionKindIdentity)
		if err != nil {
			return err
		}
		sessionResponse = response
		check.CaptureSessionID = response.SessionID
		check.CaptureSessionsIssued++
		return nil
	})
	if err != nil {
		handler.writeIdentityStateError(writer, err)
		return
	}
	handler.recordAnalyticsBiometrySession(request.Context(), updatedCheck.ID, sessionResponse.SessionID)

	handler.writeJSON(writer, http.StatusCreated, sessionResponse)
}

func (handler *Handler) completeIdentityCheck(writer http.ResponseWriter, request *http.Request) {
	check, token, ok := handler.loadPublicIdentityCheck(writer, request)
	if !ok {
		return
	}
	if time.Now().After(check.ExpiresAt) {
		handler.writeError(writer, http.StatusGone, "identity check expired")
		return
	}
	if check.Status != identity.StatusBiometryPending {
		handler.writeError(writer, http.StatusConflict, "identity check is not awaiting biometrics")
		return
	}

	payload, err := decodeIdentityCompleteRequest(writer, request)
	if err != nil {
		handler.writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	if payload.SessionID == "" || payload.SessionID != check.CaptureSessionID {
		handler.writeError(writer, http.StatusBadRequest, "identity capture session mismatch")
		return
	}
	if err := validateRuntimeFingerprint(payload.Runtime, handler.config); err != nil {
		handler.writeError(writer, http.StatusUpgradeRequired, err.Error())
		return
	}
	if err := validateCaptureProtocol(payload.CaptureProtocol); err != nil {
		handler.writeError(writer, http.StatusBadRequest, err.Error())
		return
	}

	captureSession, err := handler.sessions.Get(payload.SessionID)
	if err != nil {
		handler.writeError(writer, sessionErrorStatus(err), err.Error())
		return
	}
	if err := validateIdentityCaptureSession(captureSession, check.ID); err != nil {
		handler.writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateCaptureProtocolSession(payload.CaptureProtocol, captureSession); err != nil {
		handler.writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	if err := handler.signer.VerifyCapture(
		payload.SessionToken,
		payload.SessionID,
		payload.CaptureProtocol.RunID,
	); err != nil {
		handler.writeError(writer, http.StatusUnauthorized, err.Error())
		return
	}

	normalizedGuidedFrames, err := normalizeGuidedFrames(payload.GuidedFrames)
	if err != nil {
		handler.writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	captureSession, err = handler.sessions.Consume(payload.SessionID)
	if err != nil {
		handler.writeError(writer, sessionErrorStatus(err), err.Error())
		return
	}

	result, err := handler.engine.AnalyzeIdentity(request.Context(), domain.EngineIdentityRequest{
		GuidedFrames: normalizedGuidedFrames,
	})
	if err != nil {
		_, _ = handler.identityChecks.Update(token, func(current *identity.Check) error {
			if current.CaptureSessionID == payload.SessionID {
				current.CaptureSessionID = ""
			}
			return nil
		})
		handler.recordAnalyticsEvent(request.Context(), check.ID, "engine_failed", map[string]any{
			"sessionId": payload.SessionID,
		})
		handler.writeError(writer, http.StatusBadGateway, "biometric engine failed")
		return
	}
	handler.recordAnalyticsEvent(request.Context(), check.ID, "engine_completed", map[string]any{
		"sessionId":     payload.SessionID,
		"livenessScore": result.LivenessScore,
		"passivePad":    result.PassivePAD.Score,
		"quality":       result.Quality.Score,
		"nativeShadow":  nativeStatus(result.NativeShadow),
	})

	if identityCaptureNeedsRecapture(result) {
		_, _ = handler.identityChecks.Update(token, func(current *identity.Check) error {
			if current.CaptureSessionID == payload.SessionID {
				current.CaptureSessionID = ""
				current.LastErrorCode = "recapture_required"
			}
			return nil
		})
		handler.recordAnalyticsEvent(request.Context(), check.ID, "recapture_required", map[string]any{
			"reason":    "capture_quality",
			"sessionId": payload.SessionID,
		})
		handler.writeError(writer, http.StatusUnprocessableEntity, "capture quality insufficient; recapture required")
		return
	}

	check, err = handler.identityChecks.Load(token)
	if err != nil || check.Status != identity.StatusBiometryPending || check.CaptureSessionID != payload.SessionID {
		handler.writeError(writer, http.StatusConflict, "identity check state changed")
		return
	}
	if check.ReferenceEmbeddingModel == "" || check.ReferenceEmbeddingModel != result.EmbeddingModel {
		handler.writeError(writer, http.StatusConflict, "biometric model mismatch")
		return
	}

	referenceEmbeddings := check.ReferenceEmbeddings
	if len(referenceEmbeddings) == 0 && len(check.ReferenceEmbedding) > 0 {
		referenceEmbeddings = [][]float64{check.ReferenceEmbedding}
	}

	liveEmbeddings := make([][]float64, 0, len(result.FaceEmbeddings))
	for _, frame := range result.FaceEmbeddings {
		if frame.Phase == "near" && len(frame.Embedding) > 0 {
			liveEmbeddings = append(liveEmbeddings, frame.Embedding)
		}
	}
	if len(liveEmbeddings) == 0 && len(result.Embedding) > 0 {
		liveEmbeddings = [][]float64{result.Embedding}
	}

	similarity, frameSimilarities := matching.RobustCosineSimilarity(referenceEmbeddings, liveEmbeddings)
	handler.recordAnalyticsEvent(request.Context(), check.ID, "biometric_match_computed", map[string]any{
		"similarity":       similarity,
		"referenceFrames":  len(referenceEmbeddings),
		"liveFrames":       len(liveEmbeddings),
		"frameComparisons": len(frameSimilarities),
	})

	if identityMatchNeedsRecapture(result, similarity, handler.config.MatchThreshold) {
		_, _ = handler.identityChecks.Update(token, func(current *identity.Check) error {
			if current.CaptureSessionID == payload.SessionID {
				current.CaptureSessionID = ""
				current.LastErrorCode = "match_recapture_required"
			}
			return nil
		})
		handler.recordAnalyticsEvent(request.Context(), check.ID, "recapture_required", map[string]any{
			"reason":     "facial_match",
			"sessionId":  payload.SessionID,
			"similarity": similarity,
		})
		handler.writeError(writer, http.StatusUnprocessableEntity, "facial match inconclusive; recapture required")
		return
	}

	passivePADAvailable := result.PassivePAD.Status == "available"
	decision := handler.risk.VerificationDecision(result.LivenessScore, similarity, passivePADAvailable)
	status := identity.StatusRejected
	switch decision {
	case "approved":
		status = identity.StatusApproved
	case "review":
		status = identity.StatusReview
	}

	completedAt := time.Now().UTC()
	updated, err := handler.identityChecks.Update(token, func(current *identity.Check) error {
		if current.Status != identity.StatusBiometryPending || current.CaptureSessionID != payload.SessionID {
			return errIdentityInvalidState
		}
		if current.BiometricSessions >= maxIdentityBiometricSessions {
			return errIdentityAttemptLimit
		}
		current.BiometricSessions++
		current.Status = status
		current.Decision = decision
		current.LivenessScore = result.LivenessScore
		current.FaceSimilarity = similarity
		current.CompletedAt = &completedAt
		current.CaptureSessionID = ""
		runtimeCopy := payload.Runtime
		current.RuntimeFingerprint = &runtimeCopy
		protocolCopy := payload.CaptureProtocol
		current.CaptureProtocol = &protocolCopy
		current.ReferenceEmbedding = nil
		current.ReferenceEmbeddings = nil
		current.ReferenceEmbeddingModel = ""
		return nil
	})
	if err != nil {
		handler.writeError(writer, http.StatusConflict, "identity check state changed")
		return
	}
	handler.recordAnalyticsAuthoritativeState(request.Context(), updated)
	handler.recordAnalyticsEvent(request.Context(), check.ID, "identity_state_persisted", map[string]any{
		"status":         updated.Status,
		"decision":       updated.Decision,
		"faceSimilarity": similarity,
	})
	handler.recordAnalyticsCompletion(
		request.Context(),
		updated,
		result,
		similarity,
		frameSimilarities,
		payload.Geometry,
	)

	response := identityCompleteResponse{
		ID:             updated.ID,
		Status:         updated.Status,
		Decision:       decision,
		LivenessScore:  result.LivenessScore,
		Similarity:        similarity,
		FrameSimilarities: append([]float64(nil), frameSimilarities...),
		BestFrameIndex:    result.BestFrameIndex,
		MatchThreshold:    handler.config.MatchThreshold,
		Signals: signalResponse{
			PassivePAD:     result.PassivePAD,
			TemporalMotion: result.TemporalMotion,
			Illumination:   result.Illumination,
			GuidedCapture:  result.GuidedCapture,
		},
		Quality:      result.Quality,
		Diagnostics:  handler.publicDiagnostics(request, result.Diagnostics),
		NativeShadow: handler.publicNativeShadow(request, result.NativeShadow),
	}
	response.Document = identityDocumentDetailsFromEvidence(updated.Document, updated.ExpectedCPF)
	handler.writeJSON(writer, http.StatusOK, response)
}

func (handler *Handler) loadPublicIdentityCheck(writer http.ResponseWriter, request *http.Request) (identity.Check, string, bool) {
	if handler.identityChecks == nil || handler.cnhDocuments == nil {
		handler.writeError(writer, http.StatusServiceUnavailable, "identity verification is unavailable")
		return identity.Check{}, "", false
	}
	token := strings.TrimSpace(request.Header.Get("X-FaceProof-Identity-Token"))
	if token == "" {
		handler.writeError(writer, http.StatusUnauthorized, "identity token is required")
		return identity.Check{}, "", false
	}
	check, err := handler.identityChecks.Load(token)
	if errors.Is(err, identity.ErrNotFound) {
		handler.writeError(writer, http.StatusNotFound, "identity check not found")
		return identity.Check{}, "", false
	}
	if err != nil {
		handler.writeError(writer, http.StatusInternalServerError, "failed to load identity check")
		return identity.Check{}, "", false
	}

	if shouldExpireIdentityCheck(check, time.Now().UTC()) {
		check, err = handler.identityChecks.Update(token, expireIdentityCheck)
		if err != nil {
			handler.writeError(writer, http.StatusInternalServerError, "failed to expire identity check")
			return identity.Check{}, "", false
		}
		handler.recordAnalyticsStatus(request.Context(), check, "identity_expired")
	}
	return check, token, true
}

func (handler *Handler) authorizeIdentityIssuer(request *http.Request) bool {
	value := strings.TrimSpace(request.Header.Get("Authorization"))
	const prefix = "Bearer "
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	provided := []byte(strings.TrimSpace(strings.TrimPrefix(value, prefix)))
	return subtle.ConstantTimeCompare(provided, handler.config.IdentityIssuerKey) == 1
}

func (handler *Handler) resetIdentityDocumentAttempt(token string, failure error) {
	_, _ = handler.identityChecks.Update(token, func(check *identity.Check) error {
		check.CaptureSessionID = ""
		check.ReferenceEmbedding = nil
		check.ReferenceEmbeddings = nil
		check.ReferenceEmbeddingModel = ""
		check.Document = nil
		check.LastErrorCode = failure.Error()
		if check.DocumentAttempts >= maxIdentityDocumentAttempts {
			check.Status = identity.StatusRejected
			check.Decision = "rejected"
			if check.CompletedAt == nil {
				completedAt := time.Now().UTC()
				check.CompletedAt = &completedAt
			}
		} else {
			check.Status = identity.StatusPendingDocument
		}
		return nil
	})
}

func (handler *Handler) writeIdentityStateError(writer http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errIdentityExpired):
		handler.writeError(writer, http.StatusGone, "identity check expired")
	case errors.Is(err, errIdentityAttemptLimit):
		handler.writeError(writer, http.StatusTooManyRequests, "identity check attempt limit reached")
	case errors.Is(err, errIdentityInvalidState):
		handler.writeError(writer, http.StatusConflict, "identity check is not in the expected state")
	default:
		handler.writeError(writer, http.StatusInternalServerError, "identity check operation failed")
	}
}

func cloneEmbeddings(values [][]float64) [][]float64 {
	if len(values) == 0 {
		return nil
	}
	cloned := make([][]float64, 0, len(values))
	for _, value := range values {
		if len(value) == 0 {
			continue
		}
		cloned = append(cloned, append([]float64(nil), value...))
	}
	return cloned
}

func maskCPF(value string) string {
	value = cnh.NormalizeCPF(value)
	if len(value) != 11 {
		return ""
	}
	return "***.***.***-" + value[9:]
}

func identityDocumentDetailsFromEvidence(document *identity.DocumentEvidence, cpf string) identityDocumentDetails {
	details := identityDocumentDetails{
		SignatureValid:    true,
		VIOSignatureValid: true,
		CPFMatch:          true,
		FreshnessValid:    true,
		CPF:               maskCPF(cpf),
	}
	if document == nil {
		return details
	}
	details.Name = document.Name
	details.BirthDate = document.BirthDate
	details.Category = document.Category
	details.ExpiryDate = document.ExpiryDate
	details.IssuingUF = document.IssuingUF
	return details
}

func readIdentityPDF(writer http.ResponseWriter, request *http.Request, maxBytes int64) ([]byte, error) {
	request.Body = http.MaxBytesReader(writer, request.Body, maxBytes+identityMultipartOverhead)
	if err := request.ParseMultipartForm(maxBytes); err != nil {
		return nil, errors.New("invalid multipart request or PDF exceeds the size limit")
	}
	if request.MultipartForm != nil {
		defer request.MultipartForm.RemoveAll()
	}

	file, _, err := request.FormFile("file")
	if err != nil {
		return nil, errors.New("send the CNH Digital PDF in the file field")
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, errors.New("could not read the uploaded PDF")
	}
	if len(data) == 0 {
		return nil, errors.New("PDF is empty")
	}
	if int64(len(data)) > maxBytes {
		return nil, errors.New("PDF exceeds the size limit")
	}
	if len(data) < 5 || string(data[:5]) != "%PDF-" || http.DetectContentType(data) != "application/pdf" {
		return nil, errors.New("file is not a valid PDF")
	}

	expectedSHA256 := strings.TrimSpace(request.Header.Get("X-FaceProof-Upload-SHA256"))
	if expectedSHA256 != "" {
		sum := sha256.Sum256(data)
		actualSHA256 := hex.EncodeToString(sum[:])
		if !strings.EqualFold(expectedSHA256, actualSHA256) {
			return nil, errors.New("uploaded PDF bytes do not match browser checksum")
		}
	}
	return data, nil
}


func (handler *Handler) getIssuerIdentityCheck(writer http.ResponseWriter, request *http.Request, checkID string) {
	if handler.identityChecks == nil {
		handler.writeError(writer, http.StatusServiceUnavailable, "identity verification is unavailable")
		return
	}
	if !handler.authorizeIdentityIssuer(request) {
		handler.writeError(writer, http.StatusUnauthorized, "invalid issuer credentials")
		return
	}

	check, err := handler.identityChecks.LoadByID(checkID)
	if errors.Is(err, identity.ErrNotFound) {
		handler.writeError(writer, http.StatusNotFound, "identity check not found")
		return
	}
	if err != nil {
		handler.writeError(writer, http.StatusInternalServerError, "failed to load identity check")
		return
	}

	if shouldExpireIdentityCheck(check, time.Now().UTC()) {
		check, err = handler.identityChecks.UpdateByID(check.ID, expireIdentityCheck)
		if err != nil {
			handler.writeError(writer, http.StatusInternalServerError, "failed to expire identity check")
			return
		}
		handler.recordAnalyticsStatus(request.Context(), check, "identity_expired")
	}

	handler.writeJSON(writer, http.StatusOK, issuerIdentityCheckResponse{
		ID:                  check.ID,
		Status:              check.Status,
		MinimumDocumentDate: check.MinimumDocumentDate,
		CreatedAt:           check.CreatedAt,
		ExpiresAt:           check.ExpiresAt,
		Decision:            check.Decision,
		Document:            check.Document,
		LivenessScore:       check.LivenessScore,
		FaceSimilarity:      check.FaceSimilarity,
		CampaignID:          check.CampaignID,
		Scenario:            check.Scenario,
		ExpectedDecision:    check.ExpectedDecision,
		Runtime:             check.RuntimeFingerprint,
		CaptureProtocol:     check.CaptureProtocol,
	})
}


func shouldExpireIdentityCheck(check identity.Check, now time.Time) bool {
	if !now.After(check.ExpiresAt) {
		return false
	}
	switch check.Status {
	case identity.StatusApproved, identity.StatusReview, identity.StatusRejected, identity.StatusExpired:
		return false
	default:
		return true
	}
}

func expireIdentityCheck(check *identity.Check) error {
	check.Status = identity.StatusExpired
	check.CaptureSessionID = ""
	check.ReferenceEmbedding = nil
	check.ReferenceEmbeddings = nil
	check.ReferenceEmbeddingModel = ""
	return nil
}
