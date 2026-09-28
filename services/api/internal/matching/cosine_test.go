package matching

import "testing"

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
