package grader

import "sort"

// aggregateScores calculates a 0-100 final score from the non-null dimensions.
// Dimensions without sufficient evidence are excluded, and the remaining
// weights are renormalized to sum to one.
func aggregateScores(
	weights map[string]float64,
	results map[string]DimensionScore,
) (float64, []string) {
	validWeights := make(map[string]float64)
	evaluated := make([]string, 0, len(weights))

	for dimension, weight := range weights {
		score, ok := results[dimension]
		if !ok || score.Score == nil {
			continue
		}

		validWeights[dimension] = weight
		evaluated = append(evaluated, dimension)
	}

	sort.Strings(evaluated)

	var totalWeight float64
	for _, weight := range validWeights {
		totalWeight += weight
	}

	if totalWeight == 0 {
		return 0, evaluated
	}

	var finalScore float64
	for dimension, weight := range validWeights {
		finalScore += float64(*results[dimension].Score) * (weight / totalWeight)
	}

	return finalScore * 20, evaluated
}
