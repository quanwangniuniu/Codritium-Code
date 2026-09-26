package grader

import "google.golang.org/genai"

// Hand-written response schemas, one per dimension.
// Mirrors the OUTPUT JSON SCHEMA documented in rubric.go.
// Built manually (not via reflection over a struct) to avoid
// SDK auto-injection of propertyOrdering, which has been shown
// to cause near-empty JSON output on Gemini 2.5 Pro under long
// extraction prompts.

func schemaCorrectness() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"score": {
				Type:        genai.TypeInteger,
				Minimum:     genai.Ptr(1.0),
				Maximum:     genai.Ptr(5.0),
				Description: "1-5 score for correctness.",
			},
			"reasoning": {
				Type:      genai.TypeString,
				MaxLength: genai.Ptr[int64](1200),
			},
			"edge_cases_missed": {
				Type:  genai.TypeArray,
				Items: &genai.Schema{Type: genai.TypeString},
			},
			"test_pass_rate": {
				Type:    genai.TypeNumber,
				Minimum: genai.Ptr(0.0),
				Maximum: genai.Ptr(1.0),
			},
		},
		Required: []string{"score", "reasoning"},
	}
}

func schemaProblemDecomposition() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"score": {
				Type:        genai.TypeInteger,
				Minimum:     genai.Ptr(1.0),
				Maximum:     genai.Ptr(5.0),
				Nullable:    genai.Ptr(true),
				Description: "1-5 score, or null if not applicable.",
			},
			"reasoning": {
				Type:      genai.TypeString,
				MaxLength: genai.Ptr[int64](1200),
			},
			"first_prompt_intent": {
				Type: genai.TypeString,
				Enum: []string{"diagnose", "explore", "fix", "implement", "unknown"},
			},
			"clarifying_questions_count": {
				Type:     genai.TypeInteger,
				Nullable: genai.Ptr(true),
			},
		},
		Required: []string{"score", "reasoning"},
	}
}

func schemaAICollaboration() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"score": {
				Type:        genai.TypeInteger,
				Minimum:     genai.Ptr(1.0),
				Maximum:     genai.Ptr(5.0),
				Nullable:    genai.Ptr(true),
				Description: "1-5 score, or null if no prompt history.",
			},
			"reasoning": {
				Type:      genai.TypeString,
				MaxLength: genai.Ptr[int64](1200),
			},
			"nudges_count": {
				Type:     genai.TypeInteger,
				Nullable: genai.Ptr(true),
			},
			"architectural_decisions_owner": {
				Type: genai.TypeString,
				Enum: []string{"candidate", "ai", "mixed", "unknown"},
			},
		},
		Required: []string{"score", "reasoning"},
	}
}

func schemaVerification() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"score": {
				Type:    genai.TypeInteger,
				Minimum: genai.Ptr(1.0),
				Maximum: genai.Ptr(5.0),
			},
			"reasoning": {
				Type:      genai.TypeString,
				MaxLength: genai.Ptr[int64](1200),
			},
			"keyword_detection_hit": {
				Type: genai.TypeBoolean,
			},
			"artifact_types_present": {
				Type: genai.TypeArray,
				Items: &genai.Schema{
					Type: genai.TypeString,
					Enum: []string{"contract_table", "manual_scenario", "invariant_comments"},
				},
			},
			"oq12_partial_credit_applied": {
				Type: genai.TypeBoolean,
			},
		},
		Required: []string{"score", "reasoning"},
	}
}

func schemaCommunication() *genai.Schema {
	return &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"score": {
				Type:        genai.TypeInteger,
				Minimum:     genai.Ptr(1.0),
				Maximum:     genai.Ptr(5.0),
				Nullable:    genai.Ptr(true),
				Description: "1-5 score, or null if no narration channel.",
			},
			"reasoning": {
				Type:      genai.TypeString,
				MaxLength: genai.Ptr[int64](1200),
			},
			"before_prompt_narration_count": {
				Type:     genai.TypeInteger,
				Nullable: genai.Ptr(true),
			},
			"decision_explanations_count": {
				Type:     genai.TypeInteger,
				Nullable: genai.Ptr(true),
			},
		},
		Required: []string{"score", "reasoning"},
	}
}

// schemaForDim returns the response schema for a given dimension name.
func schemaForDim(name string) *genai.Schema {
	switch name {
	case "correctness":
		return schemaCorrectness()
	case "problem_decomposition":
		return schemaProblemDecomposition()
	case "ai_collaboration":
		return schemaAICollaboration()
	case "verification":
		return schemaVerification()
	case "communication":
		return schemaCommunication()
	}
	return nil
}
