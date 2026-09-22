package models

import (
	"context"
	"testing"
	"time"
)

func TestProviderFactory(t *testing.T) {
	tests := []struct {
		modelID      string
		expectedType string
	}{
		{"gemini-1.5-pro", "*models.GeminiProvider"},
		{"gpt-4o", "*models.OpenAIProvider"},
		{"claude-3-5-sonnet", "*models.AnthropicProvider"},
		{"ollama:llama3", "*models.OllamaProvider"},
		{"custom-mock-model", "*models.MockProvider"},
	}

	for _, tt := range tests {
		p := NewProvider(tt.modelID)
		if p.Name() != tt.modelID {
			t.Errorf("Expected provider name %s, got %s", tt.modelID, p.Name())
		}
	}
}

func TestMockProviderExecution(t *testing.T) {
	p := &MockProvider{modelName: "mock-llm"}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := p.Generate(ctx, "Hello world test prompt", ModelOptions{MaxTokens: 100})
	if err != nil {
		t.Fatalf("Unexpected mock error: %v", err)
	}

	if resp.Model != "mock-llm" {
		t.Errorf("Expected model name mock-llm, got %s", resp.Model)
	}
	if resp.PromptTokens == 0 || resp.CompletionTokens == 0 {
		t.Errorf("Expected non-zero token counts, got in=%d, out=%d", resp.PromptTokens, resp.CompletionTokens)
	}
	if resp.LatencyMs <= 0 {
		t.Errorf("Expected non-zero latency measurement")
	}
}
