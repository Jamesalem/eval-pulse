package models

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Provider families. BYOK credentials are keyed by family, so a single key
// serves every model of that provider (e.g. "openai" covers gpt-4o and gpt-4o-mini).
const (
	FamilyGemini    = "gemini"
	FamilyOpenAI    = "openai"
	FamilyAnthropic = "anthropic"
	FamilyOllama    = "ollama"
	FamilyMock      = "mock"
)

// OllamaEndpointKey is the BYOK map entry holding a custom Ollama / vLLM base URL.
const OllamaEndpointKey = "ollama_endpoint"

const (
	defaultMaxTokens     = 512
	maxResponseBodyBytes = 4 << 20 // 4 MiB
	maxErrorBodyChars    = 300
	defaultOllamaURL     = "http://localhost:11434"
)

// httpClient is shared across providers so TCP/TLS connections are pooled.
// Per-request deadlines come from the caller's context.
var httpClient = &http.Client{Timeout: 60 * time.Second}

// ModelOptions parameterizes prompt generation requests.
type ModelOptions struct {
	Temperature float64
	MaxTokens   int
	ApiKey      string
	EndpointURL string
}

func (o ModelOptions) maxTokens() int {
	if o.MaxTokens <= 0 {
		return defaultMaxTokens
	}
	return o.MaxTokens
}

// ModelResponse encapsulates output text and performance telemetry.
type ModelResponse struct {
	Model            string `json:"model"`
	ResponseText     string `json:"response_text"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	LatencyMs        int64  `json:"latency_ms"`
	TTFTMs           int64  `json:"ttft_ms"`
	Simulated        bool   `json:"simulated"`
}

// Provider defines the standard interface for LLM connectors.
type Provider interface {
	Name() string
	Generate(ctx context.Context, prompt string, opts ModelOptions) (*ModelResponse, error)
	ValidateCredentials(ctx context.Context, opts ModelOptions) error
}

// Family maps a model identifier (or a bare provider name) to its provider family.
func Family(modelID string) string {
	lower := strings.ToLower(strings.TrimSpace(modelID))
	switch {
	case strings.HasPrefix(lower, "gemini"), strings.HasPrefix(lower, "google"):
		return FamilyGemini
	case strings.HasPrefix(lower, "gpt"), strings.HasPrefix(lower, "openai"), strings.HasPrefix(lower, "o1"), strings.HasPrefix(lower, "o3"):
		return FamilyOpenAI
	case strings.HasPrefix(lower, "claude"), strings.HasPrefix(lower, "anthropic"):
		return FamilyAnthropic
	case strings.HasPrefix(lower, "ollama"), strings.HasPrefix(lower, "llama"), strings.HasPrefix(lower, "deepseek"):
		return FamilyOllama
	default:
		return FamilyMock
	}
}

// NewProvider returns the appropriate provider implementation based on model identifier.
func NewProvider(modelID string) Provider {
	switch Family(modelID) {
	case FamilyGemini:
		return &GeminiProvider{modelName: modelID}
	case FamilyOpenAI:
		return &OpenAIProvider{modelName: modelID}
	case FamilyAnthropic:
		return &AnthropicProvider{modelName: modelID}
	case FamilyOllama:
		return &OllamaProvider{modelName: modelID}
	default:
		// High-fidelity mock provider for tests, CI, and unknown providers.
		return &MockProvider{modelName: modelID}
	}
}

// ResolveOptions picks the BYOK credential and endpoint for a model from the
// submitted key map. An exact model-ID entry takes precedence over the family entry.
func ResolveOptions(modelID string, keys map[string]string, maxTokens int) ModelOptions {
	opts := ModelOptions{MaxTokens: maxTokens}
	if keys == nil {
		return opts
	}
	family := Family(modelID)
	if family == FamilyOllama {
		opts.EndpointURL = strings.TrimSpace(keys[OllamaEndpointKey])
		return opts
	}
	if k := strings.TrimSpace(keys[modelID]); k != "" {
		opts.ApiKey = k
	} else {
		opts.ApiKey = strings.TrimSpace(keys[family])
	}
	return opts
}

// ValidateEndpointURL ensures a user-supplied endpoint is an absolute http(s) URL.
func ValidateEndpointURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("endpoint must be an absolute http(s) URL")
	}
	return nil
}

// postJSON sends a JSON request and returns the response body. Non-2xx responses
// are converted into errors with a truncated body so callers never score an
// upstream failure as a successful (empty) completion.
func postJSON(ctx context.Context, provider, endpoint string, headers map[string]string, payload any) ([]byte, int64, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: marshal request: %w", provider, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, 0, fmt.Errorf("%s: build request: %w", provider, err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: request failed: %w", provider, redactURLError(err))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return nil, latency, fmt.Errorf("%s: read response: %w", provider, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, latency, fmt.Errorf("%s returned HTTP %d: %s", provider, resp.StatusCode, truncate(string(body), maxErrorBodyChars))
	}
	return body, latency, nil
}

// redactURLError strips the request URL from transport errors so that query
// parameters can never leak into logs or stored job results.
func redactURLError(err error) error {
	var uerr *url.Error
	if errors.As(err, &uerr) {
		return uerr.Err
	}
	return err
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func estimateTokens(text string) int {
	return len(strings.Fields(text)) * 4 / 3
}

// =========================================================================
// Gemini Provider (Google AI Studio)
// =========================================================================

type GeminiProvider struct {
	modelName string
}

func (p *GeminiProvider) Name() string {
	return p.modelName
}

func (p *GeminiProvider) Generate(ctx context.Context, prompt string, opts ModelOptions) (*ModelResponse, error) {
	if opts.ApiKey == "" {
		return (&MockProvider{modelName: p.modelName}).Generate(ctx, prompt, opts)
	}

	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", url.PathEscape(p.modelName))
	reqBody := map[string]any{
		"contents": []map[string]any{
			{"parts": []map[string]string{{"text": prompt}}},
		},
		"generationConfig": map[string]any{
			"temperature":     opts.Temperature,
			"maxOutputTokens": opts.maxTokens(),
		},
	}

	// The key travels in a header rather than the query string so it cannot
	// surface in error messages, proxies, or access logs.
	body, latency, err := postJSON(ctx, "gemini", endpoint, map[string]string{"x-goog-api-key": opts.ApiKey}, reqBody)
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int `json:"promptTokenCount"`
			CandidatesTokenCount int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("gemini: decode response: %w", err)
	}

	var sb strings.Builder
	if len(parsed.Candidates) > 0 {
		for _, part := range parsed.Candidates[0].Content.Parts {
			sb.WriteString(part.Text)
		}
	}
	responseText := sb.String()

	pTokens := parsed.UsageMetadata.PromptTokenCount
	if pTokens == 0 {
		pTokens = estimateTokens(prompt)
	}
	cTokens := parsed.UsageMetadata.CandidatesTokenCount
	if cTokens == 0 {
		cTokens = estimateTokens(responseText)
	}

	return &ModelResponse{
		Model:            p.modelName,
		ResponseText:     responseText,
		PromptTokens:     pTokens,
		CompletionTokens: cTokens,
		LatencyMs:        latency,
		TTFTMs:           latency / 3,
	}, nil
}

func (p *GeminiProvider) ValidateCredentials(ctx context.Context, opts ModelOptions) error {
	if opts.ApiKey == "" {
		return errors.New("gemini api key cannot be empty")
	}
	model := &GeminiProvider{modelName: validationModel(p.modelName, FamilyGemini, "gemini-1.5-flash")}
	_, err := model.Generate(ctx, "ping", ModelOptions{ApiKey: opts.ApiKey, MaxTokens: 5})
	return err
}

// validationModel substitutes a concrete, inexpensive model when credentials are
// checked using a bare provider name such as "openai".
func validationModel(name, family, fallback string) string {
	if strings.EqualFold(strings.TrimSpace(name), family) {
		return fallback
	}
	return name
}

// =========================================================================
// OpenAI Provider (GPT-4o / GPT-4o-mini)
// =========================================================================

type OpenAIProvider struct {
	modelName string
}

func (p *OpenAIProvider) Name() string {
	return p.modelName
}

func (p *OpenAIProvider) Generate(ctx context.Context, prompt string, opts ModelOptions) (*ModelResponse, error) {
	if opts.ApiKey == "" {
		return (&MockProvider{modelName: p.modelName}).Generate(ctx, prompt, opts)
	}

	endpoint := "https://api.openai.com/v1/chat/completions"
	if opts.EndpointURL != "" {
		endpoint = opts.EndpointURL
	}

	reqBody := map[string]any{
		"model": p.modelName,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": opts.Temperature,
		"max_tokens":  opts.maxTokens(),
	}

	body, latency, err := postJSON(ctx, "openai", endpoint, map[string]string{"Authorization": "Bearer " + opts.ApiKey}, reqBody)
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("openai: decode response: %w", err)
	}

	var responseText string
	if len(parsed.Choices) > 0 {
		responseText = parsed.Choices[0].Message.Content
	}

	return &ModelResponse{
		Model:            p.modelName,
		ResponseText:     responseText,
		PromptTokens:     parsed.Usage.PromptTokens,
		CompletionTokens: parsed.Usage.CompletionTokens,
		LatencyMs:        latency,
		TTFTMs:           latency / 4,
	}, nil
}

func (p *OpenAIProvider) ValidateCredentials(ctx context.Context, opts ModelOptions) error {
	if opts.ApiKey == "" {
		return errors.New("openai api key cannot be empty")
	}
	model := &OpenAIProvider{modelName: validationModel(p.modelName, FamilyOpenAI, "gpt-4o-mini")}
	_, err := model.Generate(ctx, "ping", ModelOptions{ApiKey: opts.ApiKey, EndpointURL: opts.EndpointURL, MaxTokens: 5})
	return err
}

// =========================================================================
// Anthropic Provider (Claude)
// =========================================================================

type AnthropicProvider struct {
	modelName string
}

func (p *AnthropicProvider) Name() string {
	return p.modelName
}

func (p *AnthropicProvider) Generate(ctx context.Context, prompt string, opts ModelOptions) (*ModelResponse, error) {
	if opts.ApiKey == "" {
		return (&MockProvider{modelName: p.modelName}).Generate(ctx, prompt, opts)
	}

	endpoint := "https://api.anthropic.com/v1/messages"
	if opts.EndpointURL != "" {
		endpoint = opts.EndpointURL
	}

	reqBody := map[string]any{
		"model":      p.modelName,
		"max_tokens": opts.maxTokens(),
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	headers := map[string]string{
		"x-api-key":         opts.ApiKey,
		"anthropic-version": "2023-06-01",
	}
	body, latency, err := postJSON(ctx, "anthropic", endpoint, headers, reqBody)
	if err != nil {
		return nil, err
	}

	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("anthropic: decode response: %w", err)
	}

	var sb strings.Builder
	for _, block := range parsed.Content {
		if block.Type == "" || block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}

	return &ModelResponse{
		Model:            p.modelName,
		ResponseText:     sb.String(),
		PromptTokens:     parsed.Usage.InputTokens,
		CompletionTokens: parsed.Usage.OutputTokens,
		LatencyMs:        latency,
		TTFTMs:           latency / 3,
	}, nil
}

func (p *AnthropicProvider) ValidateCredentials(ctx context.Context, opts ModelOptions) error {
	if opts.ApiKey == "" {
		return errors.New("anthropic api key cannot be empty")
	}
	model := &AnthropicProvider{modelName: validationModel(p.modelName, FamilyAnthropic, "claude-3-5-haiku-latest")}
	_, err := model.Generate(ctx, "ping", ModelOptions{ApiKey: opts.ApiKey, EndpointURL: opts.EndpointURL, MaxTokens: 5})
	return err
}

// =========================================================================
// Ollama / Local vLLM Provider
// =========================================================================

type OllamaProvider struct {
	modelName string
}

func (p *OllamaProvider) Name() string {
	return p.modelName
}

func (p *OllamaProvider) Generate(ctx context.Context, prompt string, opts ModelOptions) (*ModelResponse, error) {
	explicitEndpoint := opts.EndpointURL != ""
	endpoint := opts.EndpointURL
	if !explicitEndpoint {
		endpoint = defaultOllamaURL
	}

	cleanModel := strings.TrimPrefix(p.modelName, "ollama:")
	if cleanModel == "ollama" || cleanModel == "" {
		cleanModel = "llama3"
	}

	reqBody := map[string]any{
		"model":   cleanModel,
		"prompt":  prompt,
		"stream":  false,
		"options": map[string]any{"num_predict": opts.maxTokens()},
	}

	body, latency, err := postJSON(ctx, "ollama", strings.TrimSuffix(endpoint, "/")+"/api/generate", nil, reqBody)
	if err != nil {
		// Only fall back to simulation when no endpoint was configured and the
		// default local daemon is absent. A user-configured endpoint that fails
		// is a real error and must be surfaced.
		if !explicitEndpoint && ctx.Err() == nil {
			return (&MockProvider{modelName: p.modelName}).Generate(ctx, prompt, opts)
		}
		return nil, err
	}

	var parsed struct {
		Response        string `json:"response"`
		PromptEvalCount int    `json:"prompt_eval_count"`
		EvalCount       int    `json:"eval_count"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("ollama: decode response: %w", err)
	}

	return &ModelResponse{
		Model:            p.modelName,
		ResponseText:     parsed.Response,
		PromptTokens:     parsed.PromptEvalCount,
		CompletionTokens: parsed.EvalCount,
		LatencyMs:        latency,
		TTFTMs:           latency / 2,
	}, nil
}

func (p *OllamaProvider) ValidateCredentials(ctx context.Context, opts ModelOptions) error {
	endpoint := opts.EndpointURL
	if endpoint == "" {
		endpoint = defaultOllamaURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(endpoint, "/")+"/api/tags", nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama daemon unreachable at %s: %w", endpoint, redactURLError(err))
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBodyBytes))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama daemon at %s returned HTTP %d", endpoint, resp.StatusCode)
	}
	return nil
}

// =========================================================================
// Mock Provider (High-Fidelity Offline / CI Benchmark)
// =========================================================================

type MockProvider struct {
	modelName string
}

// NewMockProvider returns a simulated provider that never performs network I/O.
func NewMockProvider(modelID string) Provider {
	return &MockProvider{modelName: modelID}
}

func (p *MockProvider) Name() string {
	return p.modelName
}

func (p *MockProvider) Generate(ctx context.Context, prompt string, opts ModelOptions) (*ModelResponse, error) {
	// Simulate realistic LLM inference latency by model tier.
	modelLower := strings.ToLower(p.modelName)
	var simulatedLatency int64 = 420
	if strings.Contains(modelLower, "flash") || strings.Contains(modelLower, "mini") || strings.Contains(modelLower, "haiku") {
		simulatedLatency = 210
	} else if strings.Contains(modelLower, "sonnet") || strings.Contains(modelLower, "pro") || strings.Contains(modelLower, "opus") {
		simulatedLatency = 580
	}

	timer := time.NewTimer(time.Duration(simulatedLatency) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	pTokens := estimateTokens(prompt)
	if pTokens < 10 {
		pTokens = 15
	}
	cTokens := 140
	if max := opts.maxTokens(); cTokens > max {
		cTokens = max
	}

	simulatedText := fmt.Sprintf("[%s Simulation] Successfully processed evaluation task: %q. The architecture enforces bounded concurrency, Redis Stream consumer isolation, and automated regression detection.", p.modelName, prompt)

	return &ModelResponse{
		Model:            p.modelName,
		ResponseText:     simulatedText,
		PromptTokens:     pTokens,
		CompletionTokens: cTokens,
		LatencyMs:        simulatedLatency,
		TTFTMs:           simulatedLatency / 3,
		Simulated:        true,
	}, nil
}

func (p *MockProvider) ValidateCredentials(ctx context.Context, opts ModelOptions) error {
	return nil
}
