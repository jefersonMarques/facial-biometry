package httpapi

import (
	"context"
	"encoding/base64"
	"math"
	"strings"
	"time"

	"faceproof/services/api/internal/document/cnh"
	"faceproof/services/api/internal/domain"
	"faceproof/services/api/internal/identity"
	"faceproof/services/api/internal/matching"
)

func (handler *Handler) analyzeCNHDualPortrait(
	ctx context.Context,
	document cnh.Document,
	reference domain.ReferenceResult,
) *identity.DualPortraitEvidence {
	portrait := document.DualPortrait
	evidence := &identity.DualPortraitEvidence{
		Status:            portrait.Status,
		PDFQuality:        finiteQualityScore(reference.Quality.Score),
		VIOPhotoPresent:   portrait.PhotoPresent,
		VIOPhotoValidated: portrait.Status == cnh.DualPortraitReady,
		VIOPhotoWidth:     portrait.Width,
		VIOPhotoHeight:    portrait.Height,
		DecodeDurationMS:  portrait.DecodeDurationMS,
		EmbeddingModel:    reference.EmbeddingModel,
	}
	if portrait.Status != cnh.DualPortraitReady {
		return evidence
	}

	started := time.Now()
	photo := "data:" + portrait.MIME + ";base64," + base64.StdEncoding.EncodeToString(portrait.Photo)
	secondary, err := handler.engine.ExtractReference(ctx, photo)
	evidence.AnalysisDurationMS = time.Since(started).Milliseconds()
	if err != nil {
		evidence.Status = cnh.DualPortraitAnalysisFailed
		evidence.ErrorCode = "vio_reference_extraction_failed"
		return evidence
	}

	evidence.VIOQuality = finiteQualityScore(secondary.Quality.Score)
	similarity, errorCode := compareCNHPortraitEmbeddings(reference, secondary)
	if errorCode != "" {
		evidence.Status = cnh.DualPortraitAnalysisFailed
		evidence.ErrorCode = errorCode
		return evidence
	}

	evidence.Similarity = &similarity
	evidence.Status = cnh.DualPortraitCompared
	return evidence
}

func compareCNHPortraitEmbeddings(
	reference domain.ReferenceResult,
	secondary domain.ReferenceResult,
) (float64, string) {
	if reference.EmbeddingModel == "" ||
		!strings.EqualFold(reference.EmbeddingModel, secondary.EmbeddingModel) {
		return 0, "embedding_model_mismatch"
	}
	if len(reference.Embedding) == 0 ||
		len(reference.Embedding) != len(secondary.Embedding) {
		return 0, "embedding_dimensions_invalid"
	}

	score := matching.CosineSimilarity(reference.Embedding, secondary.Embedding)
	if score == -1 || math.IsNaN(score) || math.IsInf(score, 0) {
		return 0, "embedding_comparison_invalid"
	}
	return score, ""
}

func finiteQualityScore(score float64) *float64 {
	if math.IsNaN(score) || math.IsInf(score, 0) {
		return nil
	}
	return &score
}
