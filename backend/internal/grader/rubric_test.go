package grader

import (
	"math"
	"testing"
)

func scorePointer(score int) *int {
	return &score
}

func TestDifficultyWeights_AllDifficultiesUseFiveDimensions(t *testing.T) {
	difficulties := []string{"easy", "medium", "hard"}

	want := map[string]float64{
		"correctness":           0.25,
		"problem_decomposition": 0.25,
		"ai_collaboration":      0.25,
		"verification":          0.15,
		"communication":         0.10,
	}

	for _, difficulty := range difficulties {
		t.Run(difficulty, func(t *testing.T) {
			got := DifficultyWeights(difficulty)

			if len(got) != len(want) {
				t.Fatalf(
					"DifficultyWeights(%q) returned %d dimensions, want %d",
					difficulty,
					len(got),
					len(want),
				)
			}

			for dimension, wantWeight := range want {
				gotWeight, ok := got[dimension]
				if !ok {
					t.Errorf(
						"DifficultyWeights(%q) missing %q",
						difficulty,
						dimension,
					)
					continue
				}

				if math.Abs(gotWeight-wantWeight) > 0.000001 {
					t.Errorf(
						"DifficultyWeights(%q)[%q] = %v, want %v",
						difficulty,
						dimension,
						gotWeight,
						wantWeight,
					)
				}
			}
		})
	}
}

func TestDifficultyWeights_SumToOne(t *testing.T) {
	for _, difficulty := range []string{"easy", "medium", "hard"} {
		var total float64
		for _, weight := range DifficultyWeights(difficulty) {
			total += weight
		}

		if math.Abs(total-1.0) > 0.000001 {
			t.Errorf(
				"DifficultyWeights(%q) sum = %v, want 1",
				difficulty,
				total,
			)
		}
	}
}

func TestAggregateScores_RenormalizesNonNullDimensions(t *testing.T) {
	weights := DifficultyWeights("easy")

	results := map[string]DimensionScore{
		"correctness": {
			Dimension: "correctness",
			Score:     scorePointer(4),
		},
		"problem_decomposition": {
			Dimension: "problem_decomposition",
			Score:     nil,
		},
		"ai_collaboration": {
			Dimension: "ai_collaboration",
			Score:     nil,
		},
		"verification": {
			Dimension: "verification",
			Score:     scorePointer(3),
		},
		"communication": {
			Dimension: "communication",
			Score:     nil,
		},
	}

	finalScore, evaluated := aggregateScores(weights, results)

	// Correctness 4/5 = 80; verification 3/5 = 60.
	// (80*0.25 + 60*0.15) / (0.25+0.15) = 72.5.
	if math.Abs(finalScore-72.5) > 0.000001 {
		t.Fatalf("finalScore = %v, want 72.5", finalScore)
	}

	if len(evaluated) != 2 {
		t.Fatalf("evaluated dimensions = %v, want 2 dimensions", evaluated)
	}
	if evaluated[0] != "correctness" || evaluated[1] != "verification" {
		t.Fatalf(
			"evaluated dimensions = %v, want [correctness verification]",
			evaluated,
		)
	}
}

func TestAggregateScores_AllNullReturnsZero(t *testing.T) {
	weights := DifficultyWeights("hard")
	results := map[string]DimensionScore{}

	finalScore, evaluated := aggregateScores(weights, results)

	if finalScore != 0 {
		t.Fatalf("finalScore = %v, want 0", finalScore)
	}
	if len(evaluated) != 0 {
		t.Fatalf("evaluated dimensions = %v, want none", evaluated)
	}
}
