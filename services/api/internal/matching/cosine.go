package matching

import (
	"math"
	"sort"
)

func CosineSimilarity(left []float64, right []float64) float64 {
	score, ok := cosineSimilarity(left, right)
	if !ok {
		return -1
	}
	return score
}

func RobustCosineSimilarity(
	referenceEmbeddings [][]float64,
	liveEmbeddings [][]float64,
) (float64, []float64) {
	if len(referenceEmbeddings) == 0 || len(liveEmbeddings) == 0 {
		return -1, nil
	}

	frameScores := make([]float64, 0, len(liveEmbeddings))
	for _, live := range liveEmbeddings {
		referenceScores := make([]float64, 0, len(referenceEmbeddings))
		for _, reference := range referenceEmbeddings {
			score, ok := cosineSimilarity(reference, live)
			if ok {
				referenceScores = append(referenceScores, score)
			}
		}
		if len(referenceScores) == 0 {
			continue
		}
		sort.Float64s(referenceScores)
		frameScores = append(frameScores, median(referenceScores))
	}

	if len(frameScores) == 0 {
		return -1, nil
	}

	var total float64
	for _, score := range frameScores {
		total += score
	}
	return total / float64(len(frameScores)), frameScores
}

func cosineSimilarity(left []float64, right []float64) (float64, bool) {
	if len(left) == 0 || len(left) != len(right) {
		return 0, false
	}

	var dot float64
	var leftNorm float64
	var rightNorm float64
	for index := range left {
		if math.IsNaN(left[index]) ||
			math.IsNaN(right[index]) ||
			math.IsInf(left[index], 0) ||
			math.IsInf(right[index], 0) {
			return 0, false
		}
		dot += left[index] * right[index]
		leftNorm += left[index] * left[index]
		rightNorm += right[index] * right[index]
	}

	if leftNorm <= 0 || rightNorm <= 0 {
		return 0, false
	}

	score := dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm))
	if math.IsNaN(score) || math.IsInf(score, 0) {
		return 0, false
	}
	if score > 1 && score < 1+1e-12 {
		score = 1
	}
	if score < -1 && score > -1-1e-12 {
		score = -1
	}
	if score < -1 || score > 1 {
		return 0, false
	}
	return score, true
}

func median(values []float64) float64 {
	length := len(values)
	if length == 0 {
		return -1
	}
	middle := length / 2
	if length%2 == 1 {
		return values[middle]
	}
	return (values[middle-1] + values[middle]) / 2
}
