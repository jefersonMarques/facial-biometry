package httpapi

import (
	"math"
	"testing"

	"faceproof/services/api/internal/domain"
)

func TestCompareCNHPortraitEmbeddings(t *testing.T) {
	tests := []struct {
		name          string
		left          domain.ReferenceResult
		right         domain.ReferenceResult
		expectedScore float64
		expectedError string
	}{
		{
			name:          "same portrait",
			left:          domain.ReferenceResult{EmbeddingModel: "sface", Embedding: []float64{1, 0, 0}},
			right:         domain.ReferenceResult{EmbeddingModel: "sface", Embedding: []float64{1, 0, 0}},
			expectedScore: 1,
		},
		{
			name:          "different portrait still produces observational score",
			left:          domain.ReferenceResult{EmbeddingModel: "sface", Embedding: []float64{1, 0}},
			right:         domain.ReferenceResult{EmbeddingModel: "sface", Embedding: []float64{0, 1}},
			expectedScore: 0,
		},
		{
			name:          "model mismatch",
			left:          domain.ReferenceResult{EmbeddingModel: "sface", Embedding: []float64{1, 0}},
			right:         domain.ReferenceResult{EmbeddingModel: "other", Embedding: []float64{1, 0}},
			expectedError: "embedding_model_mismatch",
		},
		{
			name:          "embedding dimension mismatch",
			left:          domain.ReferenceResult{EmbeddingModel: "sface", Embedding: []float64{1, 0}},
			right:         domain.ReferenceResult{EmbeddingModel: "sface", Embedding: []float64{1}},
			expectedError: "embedding_dimensions_invalid",
		},
		{
			name:          "zero embedding",
			left:          domain.ReferenceResult{EmbeddingModel: "sface", Embedding: []float64{0, 0}},
			right:         domain.ReferenceResult{EmbeddingModel: "sface", Embedding: []float64{1, 0}},
			expectedError: "embedding_comparison_invalid",
		},
		{
			name:          "nonfinite embedding",
			left:          domain.ReferenceResult{EmbeddingModel: "sface", Embedding: []float64{math.NaN(), 0}},
			right:         domain.ReferenceResult{EmbeddingModel: "sface", Embedding: []float64{1, 0}},
			expectedError: "embedding_comparison_invalid",
		},
		{
			name:          "cosine sentinel is invalid",
			left:          domain.ReferenceResult{EmbeddingModel: "sface", Embedding: []float64{1, 0}},
			right:         domain.ReferenceResult{EmbeddingModel: "sface", Embedding: []float64{-1, 0}},
			expectedError: "embedding_comparison_invalid",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			score, errorCode := compareCNHPortraitEmbeddings(test.left, test.right)
			if errorCode != test.expectedError {
				t.Fatalf("error code = %q, want %q", errorCode, test.expectedError)
			}
			if errorCode == "" && math.Abs(score-test.expectedScore) > 1e-10 {
				t.Fatalf("similarity = %v, want %v", score, test.expectedScore)
			}
		})
	}
}

func TestFiniteQualityScore(t *testing.T) {
	if score := finiteQualityScore(0); score == nil || *score != 0 {
		t.Fatal("zero is a valid finite quality measurement")
	}
	if score := finiteQualityScore(0.82); score == nil || *score != 0.82 {
		t.Fatal("valid quality measurement was lost")
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if finiteQualityScore(value) != nil {
			t.Fatal("nonfinite quality must never be persisted")
		}
	}
}
