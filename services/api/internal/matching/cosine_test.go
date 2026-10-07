package matching

import (
	"math"
	"testing"
)

func TestCosineSimilarity(t *testing.T) {
	if score := CosineSimilarity([]float64{1, 0}, []float64{1, 0}); score < 0.9999 {
		t.Fatalf("expected near 1, got %f", score)
	}
	if score := CosineSimilarity([]float64{1, 0}, []float64{0, 1}); score > 0.0001 {
		t.Fatalf("expected near 0, got %f", score)
	}
}


func TestRobustCosineSimilarityUsesReferenceMedianAndLiveMean(t *testing.T) {
	references := [][]float64{
		{1, 0},
		{0.98, 0.20},
		{0.99, -0.10},
	}
	live := [][]float64{
		{1, 0},
		{0.97, 0.24},
		{0.96, -0.28},
	}

	score, frameScores := RobustCosineSimilarity(references, live)
	if len(frameScores) != 3 {
		t.Fatalf("expected 3 frame scores, got %d", len(frameScores))
	}
	if score < 0.90 {
		t.Fatalf("expected strong robust score, got %f", score)
	}
}

func TestRobustCosineSimilaritySkipsInvalidDimensions(t *testing.T) {
	references := [][]float64{
		{1, 0},
		{1, 0, 0},
	}
	live := [][]float64{
		{1, 0},
	}

	score, frameScores := RobustCosineSimilarity(references, live)
	if len(frameScores) != 1 {
		t.Fatalf("expected one valid frame score, got %d", len(frameScores))
	}
	if score < 0.9999 {
		t.Fatalf("expected valid comparison to survive invalid dimension, got %f", score)
	}
}

func TestRobustCosineSimilarityReturnsNoFramesWhenAllComparisonsInvalid(t *testing.T) {
	score, frameScores := RobustCosineSimilarity(
		[][]float64{{1, 0, 0}},
		[][]float64{{1, 0}},
	)
	if score != -1 {
		t.Fatalf("expected sentinel -1 when no valid comparisons exist, got %f", score)
	}
	if len(frameScores) != 0 {
		t.Fatalf("expected no frame scores, got %d", len(frameScores))
	}
}

func TestRobustCosineSimilarityRejectsNonFiniteVectors(t *testing.T) {
	score, frameScores := RobustCosineSimilarity(
		[][]float64{{1, math.NaN()}},
		[][]float64{{1, 0}},
	)
	if score != -1 || len(frameScores) != 0 {
		t.Fatalf("expected invalid non-finite comparison, got score=%f frames=%d", score, len(frameScores))
	}
}

func TestRobustCosineSimilarityKeepsLegitimateNegativeOne(t *testing.T) {
	score, frameScores := RobustCosineSimilarity(
		[][]float64{{1, 0}},
		[][]float64{{-1, 0}},
	)
	if len(frameScores) != 1 {
		t.Fatalf("expected legitimate cosine -1 to remain a valid comparison")
	}
	if math.Abs(score+1) > 1e-12 {
		t.Fatalf("expected -1 cosine, got %f", score)
	}
}
