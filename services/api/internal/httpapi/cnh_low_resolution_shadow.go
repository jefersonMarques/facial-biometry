package httpapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"math"
	"time"

	"faceproof/services/api/internal/document/cnh"
	"faceproof/services/api/internal/domain"
	"faceproof/services/api/internal/identity"
)

const lowResolutionMaximumDuration = 4 * time.Second

// Interpoladores usados apenas no diagnóstico; o pipeline autoritativo não muda.
var lowResolutionMethods = []string{"nearest", "linear", "lanczos4"}

func (handler *Handler) analyzeCNHLowResolution(
	ctx context.Context,
	document cnh.Document,
	pdf domain.ReferenceResult,
	vio domain.ReferenceResult,
) ([]identity.LowResolutionEvidence, []identity.LowResolutionReference) {
	photo := document.DualPortrait
	if photo.Status != cnh.DualPortraitReady || len(photo.Photo) == 0 ||
		len(vio.Embedding) == 0 || photo.Width < 1 || photo.Height < 1 {
		return nil, nil
	}

	width, height := lowResolutionTargetSize(photo.Width, photo.Height)
	if width == photo.Width && height == photo.Height {
		return nil, nil
	}
	baseline := lowResolutionEvidence("production_cubic", photo.Width, photo.Height, width, height)
	baseline.Status = "compared"
	baseline.Quality = finiteQualityScore(vio.Quality.Score)
	baseline.PDFVIO = comparePortraitPair(
		portraitEmbeddingSet(pdf.EmbeddingModel, pdf.Embedding, pdf.Embeddings),
		portraitEmbeddingSet(vio.EmbeddingModel, vio.Embedding, vio.Embeddings),
	)
	evidence := []identity.LowResolutionEvidence{baseline}
	references := []identity.LowResolutionReference{lowResolutionReference("production_cubic", vio)}

	imageSource, _, err := image.Decode(bytes.NewReader(photo.Photo))
	if err != nil {
		return appendLowResolutionFailures(evidence, photo.Width, photo.Height, width, height, "image_decode_failed"), references
	}

	benchmarkContext, cancel := context.WithTimeout(ctx, lowResolutionMaximumDuration)
	defer cancel()
	for _, method := range lowResolutionMethods {
		entry := lowResolutionEvidence(method, photo.Width, photo.Height, width, height)
		if benchmarkContext.Err() != nil {
			entry.Status = "analysis_failed"
			entry.ErrorCode = "benchmark_timeout"
			evidence = append(evidence, entry)
			continue
		}
		started := time.Now()
		processed := resizeLowResolutionImage(imageSource, width, height, method)
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, processed); err != nil {
			entry.Status = "analysis_failed"
			entry.ErrorCode = "image_encode_failed"
			entry.DurationMS = time.Since(started).Milliseconds()
			evidence = append(evidence, entry)
			continue
		}
		dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes())
		secondary, err := handler.engine.ExtractReference(benchmarkContext, dataURL)
		entry.DurationMS = time.Since(started).Milliseconds()
		if err != nil {
			entry.Status = "analysis_failed"
			entry.ErrorCode = "reference_extraction_failed"
			evidence = append(evidence, entry)
			continue
		}
		entry.Quality = finiteQualityScore(secondary.Quality.Score)
		entry.PDFVIO = comparePortraitPair(
			portraitEmbeddingSet(pdf.EmbeddingModel, pdf.Embedding, pdf.Embeddings),
			portraitEmbeddingSet(secondary.EmbeddingModel, secondary.Embedding, secondary.Embeddings),
		)
		entry.Status = entry.PDFVIO.Status
		entry.ErrorCode = entry.PDFVIO.ErrorCode
		if entry.Status == "compared" {
			references = append(references, lowResolutionReference(method, secondary))
		}
		evidence = append(evidence, entry)
	}
	return evidence, references
}

func appendLowResolutionFailures(
	entries []identity.LowResolutionEvidence, sourceWidth, sourceHeight, width, height int, code string,
) []identity.LowResolutionEvidence {
	for _, method := range lowResolutionMethods {
		entry := lowResolutionEvidence(method, sourceWidth, sourceHeight, width, height)
		entry.Status = "analysis_failed"
		entry.ErrorCode = code
		entries = append(entries, entry)
	}
	return entries
}

func lowResolutionReference(method string, result domain.ReferenceResult) identity.LowResolutionReference {
	return identity.LowResolutionReference{
		Method: method, Embedding: append([]float64(nil), result.Embedding...),
		Embeddings: cloneEmbeddings(result.Embeddings), EmbeddingModel: result.EmbeddingModel,
	}
}

func lowResolutionEvidence(method string, sourceWidth, sourceHeight, width, height int) identity.LowResolutionEvidence {
	return identity.LowResolutionEvidence{
		Method: method, Status: "unavailable",
		SourceWidth: sourceWidth, SourceHeight: sourceHeight,
		PreparedWidth: width, PreparedHeight: height,
		ScaleX: float64(width) / float64(sourceWidth),
		ScaleY: float64(height) / float64(sourceHeight),
	}
}

func lowResolutionTargetSize(width, height int) (int, int) {
	short := math.Min(float64(width), float64(height))
	long := math.Max(float64(width), float64(height))
	if short >= 480 || long >= 1280 {
		return width, height
	}
	scale := 480.0 / short
	if long*scale > 1280 {
		scale = 1280.0 / long
	}
	return int(math.Max(1, math.RoundToEven(float64(width)*scale))),
		int(math.Max(1, math.RoundToEven(float64(height)*scale)))
}

type resizeWeights struct {
	indices []int
	weights []float64
}

func resizeKernel(method string, distance float64) float64 {
	distance = math.Abs(distance)
	switch method {
	case "nearest":
		if distance < 0.5 {
			return 1
		}
	case "linear":
		if distance < 1 {
			return 1 - distance
		}
	case "lanczos4":
		if distance < 4 {
			if distance == 0 {
				return 1
			}
			return 4 * math.Sin(math.Pi*distance) * math.Sin(math.Pi*distance/4) /
				(math.Pi * math.Pi * distance * distance)
		}
	}
	return 0
}

func weightsForResize(sourceLength, targetLength int, method string) []resizeWeights {
	radius := 4.0
	if method == "nearest" {
		radius = 0.5
	} else if method == "linear" {
		radius = 1
	}
	out := make([]resizeWeights, targetLength)
	for dest := range out {
		center := (float64(dest)+0.5)*float64(sourceLength)/float64(targetLength) - 0.5
		from := int(math.Floor(center - radius))
		to := int(math.Ceil(center + radius))
		sum := 0.0
		for src := from; src <= to; src++ {
			weight := resizeKernel(method, float64(src)-center)
			if weight == 0 {
				continue
			}
			index := src
			if index < 0 {
				index = 0
			}
			if index >= sourceLength {
				index = sourceLength - 1
			}
			out[dest].indices = append(out[dest].indices, index)
			out[dest].weights = append(out[dest].weights, weight)
			sum += weight
		}
		if sum == 0 {
			out[dest].indices = []int{int(math.Max(0, math.Min(float64(sourceLength-1), math.Round(center))))}
			out[dest].weights = []float64{1}
		} else {
			for i := range out[dest].weights {
				out[dest].weights[i] /= sum
			}
		}
	}
	return out
}

func resizeLowResolutionImage(source image.Image, width, height int, method string) *image.NRGBA {
	bounds := source.Bounds()
	pixels := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(pixels, pixels.Bounds(), source, bounds.Min, draw.Src)
	columns := weightsForResize(bounds.Dx(), width, method)
	rows := weightsForResize(bounds.Dy(), height, method)
	// Reamostragem separável preserva os detalhes existentes sem inferir novos rostos.
	horizontal := make([]float64, width*bounds.Dy()*4)
	for y := 0; y < bounds.Dy(); y++ {
		for x, kernel := range columns {
			for i, srcX := range kernel.indices {
				base := y*pixels.Stride + srcX*4
				destination := (y*width + x)*4
				for channel := 0; channel < 4; channel++ {
					horizontal[destination+channel] +=
						float64(pixels.Pix[base+channel]) * kernel.weights[i]
				}
			}
		}
	}
	output := image.NewNRGBA(image.Rect(0, 0, width, height))
	vertical := make([]float64, width*height*4)
	for y, kernel := range rows {
		for x := 0; x < width; x++ {
			for i, srcY := range kernel.indices {
				start := (srcY*width + x)*4
				end := y*output.Stride + x*4
				for channel := 0; channel < 4; channel++ {
					vertical[(y*width+x)*4+channel] +=
						horizontal[start+channel] * kernel.weights[i]
				}
			}
		}
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			for channel := 0; channel < 4; channel++ {
				output.Pix[y*output.Stride+x*4+channel] =
					normalizedPixel(vertical[(y*width+x)*4+channel])
			}
		}
	}
	return output
}

func normalizedPixel(value float64) uint8 {
	if math.IsNaN(value) {
		return 0
	}
	return uint8(math.Round(math.Max(0, math.Min(255, value))))
}

