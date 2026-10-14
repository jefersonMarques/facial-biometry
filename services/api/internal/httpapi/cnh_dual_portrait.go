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
) (*identity.DualPortraitEvidence, domain.ReferenceResult) {
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
		PDFFaceSize:      finiteQualityScore(reference.Quality.FaceSize),
		PDFSharpness:     finiteQualityScore(reference.Quality.Sharpness),
		PDFBrightness:    finiteQualityScore(reference.Quality.Brightness),
	}
	if portrait.Status != cnh.DualPortraitReady {
		evidence.PDFVIO = &identity.PortraitPairEvidence{Status: "unavailable"}
		return evidence, domain.ReferenceResult{}
	}

	started := time.Now()
	photo := "data:" + portrait.MIME + ";base64," + base64.StdEncoding.EncodeToString(portrait.Photo)
	secondary, err := handler.engine.ExtractReference(ctx, photo)
	evidence.AnalysisDurationMS = time.Since(started).Milliseconds()
	if err != nil {
		evidence.Status = cnh.DualPortraitAnalysisFailed
		evidence.ErrorCode = "vio_reference_extraction_failed"
		evidence.PDFVIO = &identity.PortraitPairEvidence{
			Status: "analysis_failed",
			ErrorCode: "vio_reference_extraction_failed",
		}
		return evidence, domain.ReferenceResult{}
	}

	evidence.VIOQuality = finiteQualityScore(secondary.Quality.Score)
	evidence.VIOFaceSize = finiteQualityScore(secondary.Quality.FaceSize)
	evidence.VIOSharpness = finiteQualityScore(secondary.Quality.Sharpness)
	evidence.VIOBrightness = finiteQualityScore(secondary.Quality.Brightness)
	evidence.PDFVIO = comparePortraitPair(
		portraitEmbeddingSet(reference.EmbeddingModel, reference.Embedding, reference.Embeddings),
		portraitEmbeddingSet(secondary.EmbeddingModel, secondary.Embedding, secondary.Embeddings),
	)
	similarity, errorCode := compareCNHPortraitEmbeddings(reference, secondary)
	if errorCode != "" {
		evidence.Status = cnh.DualPortraitAnalysisFailed
		evidence.ErrorCode = errorCode
		return evidence, domain.ReferenceResult{}
	}

	evidence.Similarity = &similarity
	evidence.Status = cnh.DualPortraitCompared
	return evidence, secondary
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

type portraitEmbeddingSetData struct {
	Model    string
	Combined []float64
	Variants [][]float64
}

func portraitEmbeddingSet(model string, combined []float64, variants [][]float64) portraitEmbeddingSetData {
	if len(variants) == 0 && len(combined) > 0 {
		variants = [][]float64{combined}
	}
	return portraitEmbeddingSetData{Model: model, Combined: combined, Variants: variants}
}

func comparePortraitPair(reference, target portraitEmbeddingSetData) *identity.PortraitPairEvidence {
	evidence := &identity.PortraitPairEvidence{
		Status:            "analysis_failed",
		ReferenceVariants: len(reference.Variants),
		TargetVariants:    len(target.Variants),
	}
	if reference.Model == "" || !strings.EqualFold(reference.Model, target.Model) {
		evidence.ErrorCode = "embedding_model_mismatch"
		return evidence
	}
	if len(reference.Combined) == 0 || len(reference.Combined) != len(target.Combined) {
		evidence.ErrorCode = "embedding_dimensions_invalid"
		return evidence
	}
	combined := matching.CosineSimilarity(reference.Combined, target.Combined)
	if combined == -1 || math.IsNaN(combined) || math.IsInf(combined, 0) {
		evidence.ErrorCode = "embedding_comparison_invalid"
		return evidence
	}
	evidence.CombinedCosine = &combined

	robust, frameScores := matching.RobustCosineSimilarity(reference.Variants, target.Variants)
	if len(frameScores) == 0 || robust == -1 || math.IsNaN(robust) || math.IsInf(robust, 0) {
		evidence.ErrorCode = "robust_comparison_invalid"
		return evidence
	}
	evidence.RobustCosine = &robust
	evidence.FrameComparisons = len(frameScores)
	evidence.Status = "compared"
	return evidence
}
