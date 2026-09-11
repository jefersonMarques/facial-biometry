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
