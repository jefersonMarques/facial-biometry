package httpapi

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"faceproof/services/api/internal/domain"
	"faceproof/services/api/internal/identity"
	"faceproof/services/api/internal/security"
	templaterepository "faceproof/services/api/internal/template"
)

const maxDemoReferencePhotoDataURLBytes = 4 << 20

var demoSubjectIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

func effectiveFlowType(check identity.Check) identity.FlowType {
	if check.FlowType == "" {
		return identity.FlowCNH
	}
	return check.FlowType
}

func normalizeDemoFlowType(value string) (identity.FlowType, bool) {
	switch identity.FlowType(strings.ToLower(strings.TrimSpace(value))) {
	case "", identity.FlowCNH:
		return identity.FlowCNH, true
	case identity.FlowFaceEnrollment:
		return identity.FlowFaceEnrollment, true
	case identity.FlowFaceVerification:
		return identity.FlowFaceVerification, true
	case identity.FlowPhotoVerification:
		return identity.FlowPhotoVerification, true
	default:
		return "", false
	}
}

func flowRequiresDocument(check identity.Check) bool {
	return effectiveFlowType(check) == identity.FlowCNH
}

func flowRequiresReference(check identity.Check) bool {
	switch effectiveFlowType(check) {
	case identity.FlowFaceEnrollment:
		return false
	default:
		return true
	}
}

func createDemoPhotoArtifacts(
	enabled bool,
	referencePhotoDataURL string,
) *identity.DemoArtifacts {
	if !enabled {
		return nil
	}
	referencePhotoDataURL = strings.TrimSpace(referencePhotoDataURL)
	if referencePhotoDataURL == "" {
		return &identity.DemoArtifacts{}
	}
	return &identity.DemoArtifacts{
		ReferencePhotoDataURL: referencePhotoDataURL,
	}
}

func validateDemoSubjectID(value string) (string, error) {
	value = strings.TrimSpace(value)
	if !demoSubjectIDPattern.MatchString(value) {
		return "", errors.New(
			"subjectId must use 1-128 letters, numbers, dot, underscore or hyphen",
		)
	}
	return value, nil
}

func validateDemoDisplayName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) > 120 {
		return "", errors.New("displayName must contain at most 120 characters")
	}
	return value, nil
}

func validateDemoReferencePhoto(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("referencePhotoDataUrl is required")
	}
	if len(value) > maxDemoReferencePhotoDataURLBytes {
		return "", errors.New("reference photo is too large")
	}
	if !strings.HasPrefix(value, "data:image/jpeg;base64,") &&
		!strings.HasPrefix(value, "data:image/png;base64,") &&
		!strings.HasPrefix(value, "data:image/webp;base64,") {
		return "", errors.New("reference photo must be JPEG, PNG or WebP")
	}
	return value, nil
}

func demoLinkTTL(defaultTTL time.Duration, requestedMinutes int) (time.Duration, error) {
	if requestedMinutes == 0 {
		return defaultTTL, nil
	}
	if requestedMinutes < 5 || requestedMinutes > 24*60 {
		return 0, errors.New("expiresInMinutes must be between 5 and 1440")
	}
	return time.Duration(requestedMinutes) * time.Minute, nil
}

func (handler *Handler) createDemoIdentityCheckRecord(
	ctx context.Context,
	payload createAdminIdentityCheckRequest,
) (createIdentityCheckResponse, error) {
	if !handler.config.DemoMode {
		return createIdentityCheckResponse{}, &identityCreateError{
			Status:  http.StatusForbidden,
			Message: "demo mode is disabled",
		}
	}
	if handler.identityChecks == nil || handler.engine == nil || handler.templates == nil {
		return createIdentityCheckResponse{}, &identityCreateError{
			Status:  http.StatusServiceUnavailable,
			Message: "demo biometric verification is unavailable",
		}
	}

	flowType, ok := normalizeDemoFlowType(payload.FlowType)
	if !ok || flowType == identity.FlowCNH {
		return createIdentityCheckResponse{}, &identityCreateError{
			Status:  http.StatusBadRequest,
			Message: "invalid demo flowType",
		}
	}

	subjectID, err := validateDemoSubjectID(payload.SubjectID)
	if err != nil {
		return createIdentityCheckResponse{}, &identityCreateError{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
		}
	}
	displayName, err := validateDemoDisplayName(payload.DisplayName)
	if err != nil {
		return createIdentityCheckResponse{}, &identityCreateError{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
		}
	}

	scenario, ok := normalizeAnalyticsScenario(payload.Scenario)
	if !ok {
		return createIdentityCheckResponse{}, &identityCreateError{
			Status:  http.StatusBadRequest,
			Message: "scenario is invalid",
		}
	}
	expectedDecision, ok := normalizeExpectedDecision(
		payload.ExpectedDecision,
		scenario,
	)
	if !ok {
		return createIdentityCheckResponse{}, &identityCreateError{
			Status:  http.StatusBadRequest,
			Message: "expectedDecision is invalid",
		}
	}

	campaignID := strings.TrimSpace(payload.CampaignID)
	campaignExists, err := handler.analyticsCampaignExists(ctx, campaignID)
	if err != nil {
		return createIdentityCheckResponse{}, &identityCreateError{
			Status:  http.StatusServiceUnavailable,
			Message: "analytics campaign lookup failed",
		}
	}
	if !campaignExists {
		return createIdentityCheckResponse{}, &identityCreateError{
			Status:  http.StatusBadRequest,
			Message: "campaignId is invalid or analytics is unavailable",
		}
	}

	linkTTL, err := demoLinkTTL(
		handler.config.IdentityLinkTTL,
		payload.ExpiresInMinutes,
	)
	if err != nil {
		return createIdentityCheckResponse{}, &identityCreateError{
			Status:  http.StatusBadRequest,
			Message: err.Error(),
		}
	}

	check := identity.Check{
		TenantID:         "internal",
		FlowType:         flowType,
		SubjectID:        subjectID,
		DisplayName:      displayName,
		CampaignID:       campaignID,
		Scenario:         scenario,
		ExpectedDecision: expectedDecision,
		Status:           identity.StatusBiometryPending,
	}

	switch flowType {
	case identity.FlowFaceEnrollment:
		// The live capture itself becomes the encrypted template reference.
	case identity.FlowFaceVerification:
		template, loadErr := handler.templates.Load(subjectID)
		if errors.Is(loadErr, templaterepository.ErrTemplateNotFound) {
			return createIdentityCheckResponse{}, &identityCreateError{
				Status:  http.StatusNotFound,
				Message: "subject has no facial enrollment",
			}
		}
		if loadErr != nil {
			return createIdentityCheckResponse{}, errors.New(
				"failed to load biometric enrollment",
			)
		}
		check.ReferenceEmbedding = append(
			[]float64(nil),
			template.Embedding...,
		)
		check.ReferenceEmbeddings = [][]float64{
			append([]float64(nil), template.Embedding...),
		}
		check.ReferenceEmbeddingModel = template.EmbeddingModel
		check.DemoArtifacts = createDemoPhotoArtifacts(
			handler.config.DemoMode,
			template.ReferencePhotoDataURL,
		)
	case identity.FlowPhotoVerification:
		referencePhoto, validationErr := validateDemoReferencePhoto(
			payload.ReferencePhotoDataURL,
		)
		if validationErr != nil {
			return createIdentityCheckResponse{}, &identityCreateError{
				Status:  http.StatusBadRequest,
				Message: validationErr.Error(),
			}
		}
		reference, referenceErr := handler.engine.ExtractReference(
			ctx,
			referencePhoto,
		)
		if referenceErr != nil {
			return createIdentityCheckResponse{}, &identityCreateError{
				Status:  http.StatusUnprocessableEntity,
				Message: "reference photo does not contain a usable face",
			}
		}
		check.ReferenceEmbedding = append(
			[]float64(nil),
			reference.Embedding...,
		)
		check.ReferenceEmbeddings = cloneEmbeddings(reference.Embeddings)
		if len(check.ReferenceEmbeddings) == 0 &&
			len(check.ReferenceEmbedding) > 0 {
			check.ReferenceEmbeddings = [][]float64{
				append([]float64(nil), check.ReferenceEmbedding...),
			}
		}
		check.ReferenceEmbeddingModel = reference.EmbeddingModel
		check.DemoArtifacts = createDemoPhotoArtifacts(
			handler.config.DemoMode,
			referencePhoto,
		)
	}

	idValue, err := security.RandomID(12)
	if err != nil {
		return createIdentityCheckResponse{}, errors.New(
			"failed to create demo biometric check",
		)
	}
	token, err := security.RandomID(32)
	if err != nil {
		return createIdentityCheckResponse{}, errors.New(
			"failed to create demo biometric check",
		)
	}

	now := time.Now().UTC()
	check.ID = "chk_" + idValue
	check.CreatedAt = now
	check.ExpiresAt = now.Add(linkTTL)

	if err := handler.identityChecks.Create(token, check); err != nil {
		return createIdentityCheckResponse{}, errors.New(
			"failed to persist demo biometric check",
		)
	}
	handler.recordAnalyticsCheckCreated(ctx, check)
	handler.recordAnalyticsEvent(ctx, check.ID, "demo_flow_created", map[string]any{
		"flowType":  flowType,
		"subjectId": subjectID,
	})

	baseURL := strings.Split(
		strings.TrimSpace(handler.config.IdentityVerifyURL),
		"#",
	)[0]
	return createIdentityCheckResponse{
		ID:               check.ID,
		VerificationURL:  baseURL + "#identity=" + token,
		ExpiresAt:        check.ExpiresAt,
		CampaignID:       check.CampaignID,
		Scenario:         check.Scenario,
		ExpectedDecision: check.ExpectedDecision,
		FlowType:         flowType,
		SubjectID:        subjectID,
		DisplayName:      displayName,
	}, nil
}


func demoCapturedPhotoDataURL(
	enabled bool,
	frames []domain.GuidedCapturedFrame,
	bestFrameIndex int,
) string {
	if !enabled ||
		bestFrameIndex < 0 ||
		bestFrameIndex >= len(frames) ||
		len(frames[bestFrameIndex].ImageBytes) == 0 {
		return ""
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(
		frames[bestFrameIndex].ImageBytes,
	)
}

func identityStatusFromDecision(decision string) identity.Status {
	switch decision {
	case "approved":
		return identity.StatusApproved
	case "review":
		return identity.StatusReview
	default:
		return identity.StatusRejected
	}
}

func (handler *Handler) completeDemoFaceEnrollment(
	writer http.ResponseWriter,
	request *http.Request,
	token string,
	check identity.Check,
	payload identityCompleteRequest,
	frames []domain.GuidedCapturedFrame,
	result domain.EngineResult,
) {
	if effectiveFlowType(check) != identity.FlowFaceEnrollment {
		handler.writeError(
			writer,
			http.StatusConflict,
			"identity check is not a face enrollment",
		)
		return
	}

	passivePADAvailable := result.PassivePAD.Status == "available"
	decision := handler.risk.EnrollmentDecision(
		result.LivenessScore,
		passivePADAvailable,
	)
	status := identityStatusFromDecision(decision)
	capturedPhotoDataURL := demoCapturedPhotoDataURL(
		handler.config.DemoMode,
		frames,
		result.BestFrameIndex,
	)

	templateStored := decision == "approved" ||
		(decision == "review" && handler.config.AllowReviewEnrollment)
	if templateStored {
		if err := handler.templates.Save(domain.BiometricTemplate{
			SubjectID:             check.SubjectID,
			Embedding:             append([]float64(nil), result.Embedding...),
			EmbeddingModel:        result.EmbeddingModel,
			ReferencePhotoDataURL: capturedPhotoDataURL,
			CreatedAt:             time.Now().UTC(),
		}); err != nil {
			_, _ = handler.identityChecks.Update(token, func(
				current *identity.Check,
			) error {
				if current.CaptureSessionID == payload.SessionID {
					current.CaptureSessionID = ""
					current.LastErrorCode = "template_store_failed"
				}
				return nil
			})
			handler.writeError(
				writer,
				http.StatusInternalServerError,
				"could not persist facial enrollment",
			)
			return
		}
	}

	completedAt := time.Now().UTC()
	updated, err := handler.identityChecks.Update(token, func(
		current *identity.Check,
	) error {
		if current.Status != identity.StatusBiometryPending ||
			current.CaptureSessionID != payload.SessionID {
			return errIdentityInvalidState
		}
		if current.BiometricSessions >= maxIdentityBiometricSessions {
			return errIdentityAttemptLimit
		}

		current.BiometricSessions++
		current.Status = status
		current.Decision = decision
		current.LivenessScore = result.LivenessScore
		current.FaceSimilarity = 0
		current.CompletedAt = &completedAt
		current.CaptureSessionID = ""
		current.LastErrorCode = ""

		runtimeCopy := payload.Runtime
		current.RuntimeFingerprint = &runtimeCopy
		protocolCopy := payload.CaptureProtocol
		current.CaptureProtocol = &protocolCopy

		if handler.config.DemoMode {
			if current.DemoArtifacts == nil {
				current.DemoArtifacts = &identity.DemoArtifacts{}
			}
			current.DemoArtifacts.ReferencePhotoDataURL =
				capturedPhotoDataURL
			current.DemoArtifacts.CapturedPhotoDataURL =
				capturedPhotoDataURL
		}
		return nil
	})
	if err != nil {
		handler.writeIdentityStateError(writer, err)
		return
	}

	handler.recordAnalyticsAuthoritativeState(request.Context(), updated)
	handler.recordAnalyticsEvent(
		request.Context(),
		updated.ID,
		"face_enrollment_completed",
		map[string]any{
			"decision":       decision,
			"templateStored": templateStored,
			"subjectId":      updated.SubjectID,
		},
	)
	handler.recordAnalyticsCompletion(
		request.Context(),
		updated,
		result,
		0,
		nil,
		payload.Geometry,
	)

	handler.writeJSON(writer, http.StatusOK, identityCompleteResponse{
		ID:             updated.ID,
		Status:         updated.Status,
		Decision:       decision,
		LivenessScore:  result.LivenessScore,
		Similarity:     0,
		BestFrameIndex: result.BestFrameIndex,
		MatchThreshold: handler.config.MatchThreshold,
		FlowType:       identity.FlowFaceEnrollment,
		DisplayName:    updated.DisplayName,
		TemplateStored: templateStored,
		Signals: signalResponse{
			PassivePAD:     result.PassivePAD,
			TemporalMotion: result.TemporalMotion,
			Illumination:   result.Illumination,
			GuidedCapture:  result.GuidedCapture,
		},
		Quality:      result.Quality,
		Diagnostics: handler.publicDiagnostics(
			request,
			result.Diagnostics,
		),
		NativeShadow: handler.publicNativeShadow(
			request,
			result.NativeShadow,
		),
	})
}
