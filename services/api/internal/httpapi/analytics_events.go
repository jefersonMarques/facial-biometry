package httpapi

import (
	"context"
	"log"
	"strings"
	"time"

	"faceproof/services/api/internal/analytics"
	"faceproof/services/api/internal/domain"
	"faceproof/services/api/internal/identity"
)

const defaultAnalyticsScenario = "unknown"

var allowedAnalyticsScenarios = map[string]bool{
	"unknown":       true,
	"genuine_live":  true,
	"impostor_live": true,
	"printed_photo": true,
	"screen_photo":  true,
	"replay_video":  true,
}

var allowedExpectedDecisions = map[string]bool{
	"":         true,
	"approved": true,
	"review":   true,
	"rejected": true,
}

func normalizeAnalyticsScenario(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = defaultAnalyticsScenario
	}
	return value, allowedAnalyticsScenarios[value]
}

func normalizeExpectedDecision(value, scenario string) (string, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		switch scenario {
		case "genuine_live":
			value = "approved"
		case "impostor_live", "printed_photo", "screen_photo", "replay_video":
			value = "rejected"
		}
	}
	return value, allowedExpectedDecisions[value]
}

func (handler *Handler) analyticsCampaignExists(ctx context.Context, campaignID string) (bool, error) {
	campaignID = strings.TrimSpace(campaignID)
	if campaignID == "" {
		return true, nil
	}
	if handler.analytics == nil {
		return false, nil
	}
	return handler.analytics.CampaignExists(ctx, campaignID)
}

func (handler *Handler) recordAnalyticsCheckCreated(ctx context.Context, check identity.Check) {
	if handler.analytics == nil {
		return
	}
	if err := handler.analytics.RecordCheckCreated(ctx, analytics.CheckCreated{
		CheckID:          check.ID,
		SubjectCPF:       check.ExpectedCPF,
		CampaignID:       check.CampaignID,
		Scenario:         check.Scenario,
		ExpectedDecision: check.ExpectedDecision,
		Status:           string(check.Status),
		CreatedAt:        check.CreatedAt,
		ExpiresAt:        check.ExpiresAt,
	}); err != nil {
		handler.logAnalyticsError("record check", err)
		return
	}
	handler.recordAnalyticsEvent(ctx, check.ID, "check_created", map[string]any{
		"scenario":         check.Scenario,
		"expectedDecision": check.ExpectedDecision,
		"campaignId":       check.CampaignID,
	})
}

func (handler *Handler) recordAnalyticsLinkOpened(ctx context.Context, checkID string) {
	if handler.analytics == nil {
		return
	}
	if err := handler.analytics.MarkStarted(ctx, checkID); err != nil {
		handler.logAnalyticsError("mark check started", err)
	}
	if err := handler.analytics.RecordEventOnce(
		ctx,
		checkID,
		"link_opened",
		"link_opened",
		nil,
	); err != nil {
		handler.logAnalyticsError("record link opened", err)
	}
}

func (handler *Handler) recordAnalyticsEvent(
	ctx context.Context,
	checkID string,
	eventType string,
	payload any,
) {
	if handler.analytics == nil {
		return
	}
	if err := handler.analytics.RecordEvent(ctx, checkID, eventType, payload); err != nil {
		handler.logAnalyticsError("record "+eventType, err)
	}
}

func (handler *Handler) recordAnalyticsDocumentAccepted(ctx context.Context, check identity.Check) {
	if handler.analytics == nil || check.Document == nil {
		return
	}
	document := check.Document
	if err := handler.analytics.RecordDocumentAccepted(
		ctx,
		check.ID,
		string(check.Status),
		check.DocumentAttempts,
		analytics.DocumentSnapshot{
			PDFSHA256:                document.PDFSHA256,
			PDFSigningTime:           document.PDFSigningTime,
			PDFSourceIntegrity:       document.PDFSourceIntegrity,
			PDFSignatureAlgorithm:    document.PDFSignatureAlgorithm,
			VIOTemplateID:            document.VIOTemplateID,
			VIOCreatedAt:             document.VIOCreatedAt,
			VIOSignatureAlgorithm:    document.VIOSignatureAlgorithm,
			ReferencePhotoSource:     document.ReferencePhotoSource,
			ReferencePhotoMethod:     document.ReferencePhotoMethod,
			ReferencePhotoConfidence: document.ReferencePhotoConfidence,
			ReferencePhotoSHA256:     document.ReferencePhotoSHA256,
			ReferencePhotoWidth:      document.ReferencePhotoWidth,
			ReferencePhotoHeight:     document.ReferencePhotoHeight,
		},
	); err != nil {
		handler.logAnalyticsError("record document accepted", err)
	}
	handler.recordAnalyticsEvent(ctx, check.ID, "document_accepted", map[string]any{
		"attempt":              check.DocumentAttempts,
		"sourceIntegrity":      document.PDFSourceIntegrity,
		"referencePhotoSource": document.ReferencePhotoSource,
	})
}

func (handler *Handler) recordAnalyticsDocumentFailure(
	ctx context.Context,
	check identity.Check,
	failure error,
) {
	if handler.analytics == nil {
		return
	}
	if err := handler.analytics.RecordDocumentFailure(
		ctx,
		check.ID,
		string(check.Status),
		check.DocumentAttempts,
		failure.Error(),
	); err != nil {
		handler.logAnalyticsError("record document rejection", err)
	}
}

func (handler *Handler) recordAnalyticsBiometrySession(
	ctx context.Context,
	checkID string,
	sessionID string,
) {
	if handler.analytics == nil {
		return
	}
	if err := handler.analytics.RecordBiometrySession(ctx, checkID, sessionID); err != nil {
		handler.logAnalyticsError("record biometric session", err)
	}
}

func (handler *Handler) recordAnalyticsStatus(
	ctx context.Context,
	check identity.Check,
	eventType string,
) {
	if handler.analytics == nil {
		return
	}
	if err := handler.analytics.RecordStatus(
		ctx,
		check.ID,
		string(check.Status),
		check.Decision,
		check.CompletedAt,
	); err != nil {
		handler.logAnalyticsError("record identity status", err)
	}
	if eventType != "" {
		handler.recordAnalyticsEvent(ctx, check.ID, eventType, map[string]any{
			"status":   check.Status,
			"decision": check.Decision,
		})
	}
}

func (handler *Handler) recordAnalyticsCompletion(
	ctx context.Context,
	check identity.Check,
	result domain.EngineResult,
	similarity float64,
	frameSimilarities []float64,
	geometry *domain.GeometryTelemetry,
) {
	if handler.analytics == nil || check.CompletedAt == nil {
		return
	}

	var geometrySnapshot *analytics.GeometrySnapshot
	if geometry != nil {
		geometrySnapshot = &analytics.GeometrySnapshot{
			RunID:             geometry.RunID,
			Status:            geometry.Status,
			SampleCount:       geometry.SampleCount,
			FarSamples:        geometry.FarSamples,
			NearSamples:       geometry.NearSamples,
			ScaleRatio:        geometry.ScaleRatio,
			TransitionScore:   geometry.TransitionScore,
			PerspectiveChange: geometry.PerspectiveChange,
			DepthChange:       geometry.DepthChange,
			PhaseStability:    geometry.PhaseStability,
			EvidenceScore:     geometry.EvidenceScore,
			WASMStatus:        geometry.WASMStatus,
			WASMMaxDelta:      geometry.WASMMaxDelta,
		}
	}

	var nativeSnapshot *analytics.NativeShadowSnapshot
	if result.NativeShadow != nil {
		shadow := result.NativeShadow
		nativeSnapshot = &analytics.NativeShadowSnapshot{
			Status:                     shadow.Status,
			PythonFrames:               shadow.PythonFrames,
			NativeFrames:               shadow.NativeFrames,
			BBoxMaxDeltaPx:             shadow.BBoxMaxDeltaPx,
			ConfidenceMaxDelta:         shadow.ConfidenceMaxDelta,
			QualityMaxDelta:            shadow.QualityMaxDelta,
			PadComparedFrames:          shadow.PadComparedFrames,
			PassivePadMaxDelta:         shadow.PassivePadMaxDelta,
			SelectedEmbeddingMinCosine: shadow.SelectedEmbeddingMinCosine,
			SelectedEmbeddingMaxDelta:  shadow.SelectedEmbeddingMaxDelta,
			CombinedEmbeddingCosine:    shadow.CombinedEmbeddingCosine,
			CombinedEmbeddingMaxDelta:  shadow.CombinedEmbeddingMaxDelta,
		}
	}

	if err := handler.analytics.RecordCompletion(ctx, analytics.CompletionSnapshot{
		CheckID:                 check.ID,
		Status:                  string(check.Status),
		Decision:                check.Decision,
		CompletedAt:             *check.CompletedAt,
		DocumentAttempts:        check.DocumentAttempts,
		BiometricSessions:       check.BiometricSessions,
		LivenessScore:           result.LivenessScore,
		FaceSimilarity:          similarity,
		MatchThreshold:          handler.config.MatchThreshold,
		LivenessThreshold:       handler.config.LivenessThreshold,
		ReviewLivenessThreshold: handler.config.ReviewLivenessThreshold,
		RequirePassivePAD:       handler.config.RequirePassivePAD,
		FrameSimilarities:       append([]float64(nil), frameSimilarities...),
		PassivePADScore:         result.PassivePAD.Score,
		TemporalMotionScore:     result.TemporalMotion.Score,
		IlluminationScore:       result.Illumination.Score,
		GuidedCaptureScore:      result.GuidedCapture.Score,
		QualityScore:            result.Quality.Score,
		EmbeddingModel:          result.EmbeddingModel,
		Geometry:                geometrySnapshot,
		NativeShadow:            nativeSnapshot,
		Runtime:                 check.RuntimeFingerprint,
		CaptureProtocol:         check.CaptureProtocol,
		Diagnostics:             append([]string(nil), result.Diagnostics...),
	}); err != nil {
		handler.logAnalyticsError("record biometric completion", err)
		return
	}

	handler.recordAnalyticsEvent(ctx, check.ID, "identity_completed", map[string]any{
		"status":         check.Status,
		"decision":       check.Decision,
		"faceSimilarity": similarity,
		"livenessScore":  result.LivenessScore,
		"passivePad":     result.PassivePAD.Score,
		"quality":        result.Quality.Score,
		"nativeShadow":   nativeStatus(result.NativeShadow),
	})
}

func (handler *Handler) logAnalyticsError(operation string, err error) {
	log.Printf("analytics %s failed: %v", operation, err)
}

func nativeStatus(shadow *domain.NativeShadowComparison) string {
	if shadow == nil {
		return ""
	}
	return shadow.Status
}

func completedNow(check *identity.Check) {
	now := time.Now().UTC()
	check.CompletedAt = &now
}
