package eval

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

// DefaultBaselineScore is the reference semantic score that drift is measured against.
const DefaultBaselineScore = 0.92

// DefaultDriftThresholdPercent is the drift beyond which a result raises a regression alert.
const DefaultDriftThresholdPercent = 5.0

// RateCard defines input and output cost per 1,000,000 tokens in USD.
type RateCard struct {
	InputPerMillion  float64
	OutputPerMillion float64
}

// ModelPricing maps known provider models to their official pricing rate cards.
var ModelPricing = map[string]RateCard{
	// Google Gemini
	"gemini-1.5-pro":   {InputPerMillion: 3.50, OutputPerMillion: 10.50},
	"gemini-1.5-flash": {InputPerMillion: 0.075, OutputPerMillion: 0.30},
	"gemini":           {InputPerMillion: 0.075, OutputPerMillion: 0.30},

	// OpenAI
	"gpt-4o":      {InputPerMillion: 2.50, OutputPerMillion: 10.00},
	"gpt-4o-mini": {InputPerMillion: 0.15, OutputPerMillion: 0.60},
	"openai":      {InputPerMillion: 0.15, OutputPerMillion: 0.60},

	// Anthropic Claude
	"claude-3-5-sonnet": {InputPerMillion: 3.00, OutputPerMillion: 15.00},
	"claude-3-5-haiku":  {InputPerMillion: 0.80, OutputPerMillion: 4.00},
	"anthropic":         {InputPerMillion: 3.00, OutputPerMillion: 15.00},

	// Local Ollama / Self-hosted vLLM
	"ollama":   {InputPerMillion: 0.00, OutputPerMillion: 0.00},
	"llama3":   {InputPerMillion: 0.00, OutputPerMillion: 0.00},
	"deepseek": {InputPerMillion: 0.00, OutputPerMillion: 0.00},
}

// ScoreCard holds the multi-dimensional evaluation results for a model's output.
type ScoreCard struct {
	ExactMatch      bool    `json:"exact_match"`
	Graded          bool    `json:"graded"`
	SemanticScore   float64 `json:"semantic_score"`
	CostUSD         float64 `json:"cost_usd"`
	RegressionAlert bool    `json:"regression_alert"`
	DriftPercent    float64 `json:"drift_percent"`
}

// defaultRateCard applies to models without a known rate card.
var defaultRateCard = RateCard{InputPerMillion: 1.00, OutputPerMillion: 3.00}

// pricingKeys lists ModelPricing keys longest-first so the most specific entry
// wins (e.g. "gpt-4o-mini" must not be billed as "gpt-4o"). Iterating the map
// directly would pick a random match on every call.
var pricingKeys = func() []string {
	keys := make([]string, 0, len(ModelPricing))
	for k := range ModelPricing {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})
	return keys
}()

// LookupRateCard returns the most specific rate card matching a model identifier.
func LookupRateCard(modelName string) RateCard {
	lower := strings.ToLower(modelName)
	for _, key := range pricingKeys {
		if strings.Contains(lower, key) {
			return ModelPricing[key]
		}
	}
	return defaultRateCard
}

// CalculateCost computes the dollar cost of an evaluation using Cline-style token pricing.
func CalculateCost(modelName string, promptTokens, completionTokens int) float64 {
	if promptTokens < 0 {
		promptTokens = 0
	}
	if completionTokens < 0 {
		completionTokens = 0
	}
	card := LookupRateCard(modelName)

	inputCost := (float64(promptTokens) / 1_000_000.0) * card.InputPerMillion
	outputCost := (float64(completionTokens) / 1_000_000.0) * card.OutputPerMillion
	return inputCost + outputCost
}

// Evaluate performs multi-metric evaluation comparing model response with ground truth.
// Without a ground truth there is no reference to regress against, so the result
// is marked ungraded and never raises a regression alert.
func Evaluate(response, groundTruth, modelName string, baselineScore, thresholdPercent float64) ScoreCard {
	normResp := normalizeText(response)
	normTruth := normalizeText(groundTruth)

	// 1. Exact Match heuristic
	exactMatch := false
	if normTruth != "" && (normResp == normTruth || strings.Contains(normResp, normTruth)) {
		exactMatch = true
	}

	// 2. Semantic Similarity heuristic (Token-vector cosine distance)
	semanticScore := computeSemanticSimilarity(normResp, normTruth)
	if normTruth == "" {
		// In exploratory benchmarks without ground truth, score based on structural richness & non-emptiness
		semanticScore = math.Min(1.0, 0.70+float64(len(strings.Fields(response)))*0.002)
	}

	// 3. Regression Detection (graded runs only)
	var isRegressed bool
	var driftPercent float64
	if normTruth != "" && baselineScore > 0 {
		driftPercent = ((baselineScore - semanticScore) / baselineScore) * 100.0
		if driftPercent > thresholdPercent {
			isRegressed = true
		}
	}

	return ScoreCard{
		ExactMatch:      exactMatch,
		Graded:          normTruth != "",
		SemanticScore:   math.Round(semanticScore*1000) / 1000,
		RegressionAlert: isRegressed,
		DriftPercent:    math.Round(driftPercent*10) / 10,
	}
}

// normalizeText trims whitespace and lowers case for deterministic comparison.
func normalizeText(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// computeSemanticSimilarity calculates token bag-of-words cosine similarity.
func computeSemanticSimilarity(s1, s2 string) float64 {
	if s1 == "" || s2 == "" {
		return 0.0
	}

	words1 := tokenize(s1)
	words2 := tokenize(s2)

	freq1 := make(map[string]float64)
	freq2 := make(map[string]float64)
	allWords := make(map[string]struct{})

	for _, w := range words1 {
		freq1[w]++
		allWords[w] = struct{}{}
	}
	for _, w := range words2 {
		freq2[w]++
		allWords[w] = struct{}{}
	}

	var dotProduct, norm1, norm2 float64
	for w := range allWords {
		v1 := freq1[w]
		v2 := freq2[w]
		dotProduct += v1 * v2
		norm1 += v1 * v1
		norm2 += v2 * v2
	}

	if norm1 == 0 || norm2 == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(norm1) * math.Sqrt(norm2))
}

func tokenize(s string) []string {
	f := func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	}
	return strings.FieldsFunc(s, f)
}
