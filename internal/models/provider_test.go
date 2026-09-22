package models

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProviderFactory(t *testing.T) {
	tests := []struct {
		modelID string
		want    string
	}{
		{"gemini-1.5-pro", FamilyGemini},
		{"gpt-4o", FamilyOpenAI},
		{"claude-3-5-sonnet", FamilyAnthropic},
		{"ollama:llama3", FamilyOllama},
		{"custom-mock-model", FamilyMock},
	}

	for _, tt := range tests {
		p := NewProvider(tt.modelID)
		if p.Name() != tt.modelID {
			t.Errorf("Expected provider name %s, got %s", tt.modelID, p.Name())
		}
		var got string
		switch p.(type) {
		case *GeminiProvider:
			got = FamilyGemini
		case *OpenAIProvider:
			got = FamilyOpenAI
		case *AnthropicProvider:
			got = FamilyAnthropic
		case *OllamaProvider:
			got = FamilyOllama
		case *MockProvider:
			got = FamilyMock
		}
		if got != tt.want {
			t.Errorf("NewProvider(%q) built %s provider, want %s", tt.modelID, got, tt.want)
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
	if resp.CompletionTokens > 100 {
		t.Errorf("CompletionTokens %d exceeds MaxTokens", resp.CompletionTokens)
	}
	if !resp.Simulated {
		t.Error("mock responses must be flagged as simulated")
	}
}

func TestMockProviderHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (&MockProvider{modelName: "m"}).Generate(ctx, "x", ModelOptions{}); err == nil {
		t.Fatal("expected context error")
	}
}

// Regression test: the UI stores keys per provider family ("openai"), but the
// backend used to look them up by model ID ("gpt-4o-mini"), so BYOK keys were
// never applied and every run silently fell back to simulation.
func TestResolveOptionsUsesFamilyKeys(t *testing.T) {
	keys := map[string]string{
		"openai":          " sk-test ",
		"anthropic":       "sk-ant",
		"gpt-4o":          "sk-model-specific",
		OllamaEndpointKey: "http://gpu-box:11434",
	}
	cases := map[string]ModelOptions{
		"gpt-4o-mini":       {ApiKey: "sk-test", MaxTokens: 64},
		"gpt-4o":            {ApiKey: "sk-model-specific", MaxTokens: 64},
		"claude-3-5-sonnet": {ApiKey: "sk-ant", MaxTokens: 64},
		"gemini-1.5-flash":  {MaxTokens: 64},
		"ollama:llama3":     {EndpointURL: "http://gpu-box:11434", MaxTokens: 64},
	}
	for model, want := range cases {
		if got := ResolveOptions(model, keys, 64); got != want {
			t.Errorf("ResolveOptions(%q) = %+v, want %+v", model, got, want)
		}
	}
}

// Regression test: non-2xx responses used to return (response, nil), so a
// rejected API key was reported as a successful empty completion and
// credential validation always passed.
func TestOpenAIHTTPErrorIsReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer srv.Close()

	p := &OpenAIProvider{modelName: "gpt-4o-mini"}
	_, err := p.Generate(context.Background(), "hi", ModelOptions{ApiKey: "bad", EndpointURL: srv.URL})
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected HTTP 401 error, got %v", err)
	}
	if err := p.ValidateCredentials(context.Background(), ModelOptions{ApiKey: "bad", EndpointURL: srv.URL}); err == nil {
		t.Fatal("ValidateCredentials must fail for a rejected key")
	}
}

func TestOpenAISuccessParsesUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-ok" {
			t.Errorf("missing bearer token")
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"hello"}}],"usage":{"prompt_tokens":3,"completion_tokens":1}}`))
	}))
	defer srv.Close()

	resp, err := (&OpenAIProvider{modelName: "gpt-4o-mini"}).Generate(context.Background(), "hi", ModelOptions{ApiKey: "sk-ok", EndpointURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ResponseText != "hello" || resp.PromptTokens != 3 || resp.CompletionTokens != 1 || resp.Simulated {
		t.Errorf("unexpected response %+v", resp)
	}
}

func TestOllamaExplicitEndpointFailureIsNotMasked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := (&OllamaProvider{modelName: "ollama:llama3"}).Generate(context.Background(), "hi", ModelOptions{EndpointURL: srv.URL})
	if err == nil {
		t.Fatal("a configured endpoint failure must surface as an error, not a simulated response")
	}
}

func TestTransportErrorsDoNotLeakURL(t *testing.T) {
	_, _, err := postJSON(context.Background(), "test", "http://127.0.0.1:1/path?key=SECRET", nil, map[string]string{})
	if err == nil {
		t.Fatal("expected connection error")
	}
	if strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("error leaked the request URL: %v", err)
	}
}

func TestValidateEndpointURL(t *testing.T) {
	for _, ok := range []string{"", "http://localhost:11434", "https://api.example.com/v1"} {
		if err := ValidateEndpointURL(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"localhost:11434", "file:///etc/passwd", "ftp://x", "http://"} {
		if err := ValidateEndpointURL(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
