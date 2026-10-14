package httpapi

import (
	"image"
	"image/color"
	"math"
	"testing"

	"faceproof/services/api/internal/domain"
	"faceproof/services/api/internal/identity"
)

func TestLowResolutionTargetSizeMatchesSecureCore(t *testing.T) {
	tests := []struct {
		width, height int
		wantWidth, wantHeight int
	}{
		{45, 61, 480, 651},
		{61, 45, 651, 480},
		{480, 651, 480, 651},
		{1600, 1600, 1600, 1600},
	}
	for _, test := range tests {
		width, height := lowResolutionTargetSize(test.width, test.height)
		if width != test.wantWidth || height != test.wantHeight {
			t.Fatalf("%dx%d => %dx%d, expected %dx%d",
				test.width, test.height, width, height, test.wantWidth, test.wantHeight)
		}
	}
}

func TestLowResolutionInterpolationIsDeterministic(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 3, 3))
	for y := 0; y < 3; y++ {
		for x := 0; x < 3; x++ {
			source.SetNRGBA(x, y, color.NRGBA{
				R: uint8(20 + x*70),
				G: uint8(20 + y*70),
				B: uint8(140 + x*15),
				A: 255,
			})
		}
	}
	center := map[string]color.NRGBA{}
	for _, method := range lowResolutionMethods {
		first := resizeLowResolutionImage(source, 21, 21, method)
		second := resizeLowResolutionImage(source, 21, 21, method)
		if first.Bounds() != image.Rect(0, 0, 21, 21) {
			t.Fatalf("%s returned wrong dimensions", method)
		}
		if string(first.Pix) != string(second.Pix) {
			t.Fatalf("%s interpolation is not deterministic", method)
		}
		center[method] = first.NRGBAAt(10, 10)
		if center[method].A != 255 {
			t.Fatalf("%s interpolation changed opacity", method)
		}
		if method != "lanczos4" &&
			first.NRGBAAt(0, 0).R != source.NRGBAAt(0, 0).R {
			t.Fatalf("%s unexpectedly changed a border pixel", method)
		}
	}
	for _, method := range []string{"linear", "lanczos4"} {
		if center[method].R == 0 {
			t.Fatalf("%s produced an all-black reference", method)
		}
	}
}

func TestFinishCNHLowResolutionKeepsAllMethodsSeparate(t *testing.T) {
	pdfVIO := &identity.PortraitPairEvidence{Status: "compared"}
	check := identity.Check{
		Document: &identity.DocumentEvidence{
			LowResolutionBenchmark: []identity.LowResolutionEvidence{
				{Method: "production_cubic", Status: "compared", PDFVIO: pdfVIO},
				{Method: "nearest", Status: "compared"},
				{Method: "linear", Status: "analysis_failed", ErrorCode: "reference_extraction_failed"},
			},
		},
		LowResolutionReferences: []identity.LowResolutionReference{
			{
				Method: "production_cubic",
				EmbeddingModel: "sface",
				Embedding: []float64{1, 0, 0},
				Embeddings: [][]float64{{1, 0, 0}},
			},
			{
				Method: "nearest",
				EmbeddingModel: "sface",
				Embedding: []float64{0.8, 0.6, 0},
				Embeddings: [][]float64{{0.8, 0.6, 0}},
			},
		},
	}
	result := domain.EngineResult{
		EmbeddingModel: "sface",
		Embedding: []float64{1, 0, 0},
	}
	completed := finishCNHLowResolution(check, result, [][]float64{{1, 0, 0}})
	if len(completed) != 3 || completed[0].VIOLive == nil ||
		completed[1].VIOLive == nil || completed[2].VIOLive != nil {
		t.Fatalf("unexpected completed benchmark: %+v", completed)
	}
	if completed[0].VIOLive.Status != "compared" ||
		math.Abs(*completed[0].VIOLive.RobustCosine-1) > 1e-9 {
		t.Fatalf("baseline score invalid: %+v", completed[0].VIOLive)
	}
	if completed[1].VIOLive.Status != "compared" ||
		math.Abs(*completed[1].VIOLive.RobustCosine-0.8) > 1e-9 {
		t.Fatalf("nearest score invalid: %+v", completed[1].VIOLive)
	}
	if check.Document.LowResolutionBenchmark[0].VIOLive != nil {
		t.Fatal("completion mutated original document evidence")
	}
}

func TestLowResolutionReferenceNotStoredInAnalyticsEvidence(t *testing.T) {
	entry := lowResolutionReference("nearest", domain.ReferenceResult{
		Embedding: []float64{0.5, 0.25},
		Embeddings: [][]float64{{0.5, 0.25}},
		EmbeddingModel: "sface",
	})
	if entry.Method != "nearest" || len(entry.Embedding) != 2 {
		t.Fatal("temporary reference unavailable")
	}
	evidence := lowResolutionEvidence("nearest", 45, 61, 480, 651)
	if evidence.SourceWidth != 45 || evidence.ScaleX != 480.0/45.0 {
		t.Fatalf("incorrect nonbiometric metadata: %+v", evidence)
	}
}
