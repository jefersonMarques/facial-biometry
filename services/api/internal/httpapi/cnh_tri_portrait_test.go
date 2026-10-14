package httpapi

import (
	"math"
	"testing"

	"faceproof/services/api/internal/identity"
)

func TestThreePortraitPairsKeepLowQualityVIOObservational(t *testing.T) {
	pdf := portraitEmbeddingSet("sface",
		[]float64{1, 0, 0},
		[][]float64{{1, 0, 0}, {0.9, 0.1, 0}},
	)
	vio := portraitEmbeddingSet("sface",
		[]float64{0.4, 0.916515138991168, 0},
		[][]float64{{0.4, 0.916515138991168, 0}, {0.5, 0.8660254037844386, 0}},
	)
	live := portraitEmbeddingSet("sface",
		[]float64{0.9, 0.4358898943540673, 0},
		[][]float64{{0.9, 0.4358898943540673, 0}, {0.85, 0.526782687642637, 0}},
	)

	tests := []struct {
		name   string
		left   portraitEmbeddingSetData
		right  portraitEmbeddingSetData
	}{
		{"pdf_vio", pdf, vio},
		{"vio_live", vio, live},
		{"pdf_live", pdf, live},
	}
	scores := map[string]float64{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := comparePortraitPair(test.left, test.right)
			if result.Status != "compared" ||
				result.CombinedCosine == nil ||
				result.RobustCosine == nil ||
				result.FrameComparisons != len(test.right.Variants) {
				t.Fatalf("incomplete comparison: %+v", result)
			}
			if math.IsNaN(*result.CombinedCosine) || math.IsNaN(*result.RobustCosine) {
				t.Fatal("invalid cosine emitted")
			}
			scores[test.name] = *result.CombinedCosine
		})
	}
	if scores["pdf_vio"] >= scores["pdf_live"] {
		t.Fatalf("expected VIO to have lower similarity without preventing analysis: %+v", scores)
	}
}

func TestComparePortraitPairFailureProducesNoSyntheticScore(t *testing.T) {
	base := portraitEmbeddingSet("sface", []float64{1, 0}, [][]float64{{1, 0}})
	tests := []struct {
		name  string
		right portraitEmbeddingSetData
		error string
	}{
		{
			name:  "different model",
			right: portraitEmbeddingSet("other", []float64{1, 0}, nil),
			error: "embedding_model_mismatch",
		},
		{
			name:  "invalid dimension",
			right: portraitEmbeddingSet("sface", []float64{1}, nil),
			error: "embedding_dimensions_invalid",
		},
		{
			name:  "invalid robust variants",
			right: portraitEmbeddingSet("sface", []float64{1, 0}, [][]float64{{0, 0}}),
			error: "robust_comparison_invalid",
		},
		{
			name:  "invalid combined",
			right: portraitEmbeddingSet("sface", []float64{0, 0}, [][]float64{{1, 0}}),
			error: "embedding_comparison_invalid",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := comparePortraitPair(base, test.right)
			if result.Status != "analysis_failed" || result.ErrorCode != test.error {
				t.Fatalf("unexpected error: %+v", result)
			}
			if result.ErrorCode != "robust_comparison_invalid" &&
				result.CombinedCosine != nil {
				t.Fatalf("untrusted score must not be emitted: %+v", result)
			}
		})
	}
}

func TestExpireIdentityCheckScrubsVIOTemporaryEmbeddings(t *testing.T) {
	check := identity.Check{
		ReferenceEmbedding:         []float64{1, 2},
		ReferenceEmbeddings:        [][]float64{{1, 2}},
		ReferenceEmbeddingModel:    "sface",
		VIOReferenceEmbedding:      []float64{3, 4},
		VIOReferenceEmbeddings:     [][]float64{{3, 4}},
		VIOReferenceEmbeddingModel: "sface",
	}
	if err := expireIdentityCheck(&check); err != nil {
		t.Fatal(err)
	}
	if check.Status != identity.StatusExpired ||
		len(check.ReferenceEmbedding) != 0 ||
		len(check.ReferenceEmbeddings) != 0 ||
		len(check.VIOReferenceEmbedding) != 0 ||
		len(check.VIOReferenceEmbeddings) != 0 ||
		check.VIOReferenceEmbeddingModel != "" {
		t.Fatalf("expired identity retains biometric templates: %+v", check)
	}
}
