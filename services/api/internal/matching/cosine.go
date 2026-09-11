package matching

import "math"

func CosineSimilarity(left []float64, right []float64) float64 {
	if len(left) == 0 || len(left) != len(right) {
		return -1
	}

	var dot float64
	var leftNorm float64
	var rightNorm float64
	for index := range left {
		dot += left[index] * right[index]
		leftNorm += left[index] * left[index]
		rightNorm += right[index] * right[index]
	}

	if leftNorm == 0 || rightNorm == 0 {
		return -1
	}
	return dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm))
}
