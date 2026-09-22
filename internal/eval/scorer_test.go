package eval

import (
	"testing"
)

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		name             string
		model            string
		promptTokens     int
		completionTokens int
		expectedMin      float64
		expectedMax      float64
	}{
		{
			name:             "Gemini Flash Cost",
			model:            "gemini-1.5-flash",
			promptTokens:     1000,
			completionTokens: 2000,
			expectedMin:      0.0006,
			expectedMax:      0.0008,
		},
		{
			name:             "Claude 3.5 Sonnet Cost",
			model:            "claude-3-5-sonnet",
			promptTokens:     10000,
			completionTokens: 5000,
			expectedMin:      0.10,
			expectedMax:      0.11,
		},
		{
			name:             "Ollama Local Free Cost",
			model:            "ollama:llama3",
			promptTokens:     5000,
			completionTokens: 5000,
			expectedMin:      0.0,
			expectedMax:      0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := CalculateCost(tt.model, tt.promptTokens, tt.completionTokens)
			if cost < tt.expectedMin || cost > tt.expectedMax {
				t.Errorf("CalculateCost(%s) = %f; expected between %f and %f", tt.model, cost, tt.expectedMin, tt.expectedMax)
			}
		})
	}
}

func TestEvaluate(t *testing.T) {
	resp := "Package main implements a high concurrency worker pool in Go."
	truth := "package main implements a high concurrency worker pool in Go."

	score := Evaluate(resp, truth, "gemini-1.5-flash", 0.95, 5.0)

	if !score.ExactMatch {
		t.Errorf("Expected exact match to be true after case-normalization")
	}
	if score.SemanticScore < 0.99 {
		t.Errorf("Expected semantic score >= 0.99, got %f", score.SemanticScore)
	}
	if score.RegressionAlert {
		t.Errorf("Expected no regression alert for high score")
	}
}
