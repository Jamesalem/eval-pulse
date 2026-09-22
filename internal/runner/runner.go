// Package runner executes an evaluation job: it fans a prompt out to every
// target model in parallel, scores each response, and assembles the job record.
// It is shared by the worker daemon and the API's standalone fallback path.
package runner

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/evalpulse/eval-pulse/internal/eval"
	"github.com/evalpulse/eval-pulse/internal/models"
	"github.com/evalpulse/eval-pulse/internal/queue"
	"github.com/evalpulse/eval-pulse/internal/storage"
)

// ProviderFactory builds a provider for a model ID. Overridable in tests.
type ProviderFactory func(modelID string) models.Provider

// Runner executes evaluation jobs.
type Runner struct {
	// PerModelTimeout bounds each individual model call.
	PerModelTimeout time.Duration
	// NewProvider resolves providers; defaults to models.NewProvider.
	NewProvider ProviderFactory
}

// New returns a Runner with the given per-model timeout.
func New(perModelTimeout time.Duration) *Runner {
	if perModelTimeout <= 0 {
		perModelTimeout = 30 * time.Second
	}
	return &Runner{PerModelTimeout: perModelTimeout, NewProvider: models.NewProvider}
}

// Run evaluates the job against every target model concurrently. Results keep
// the order of job.TargetModels so output is deterministic. A panic inside a
// provider is contained to that model's result.
func (r *Runner) Run(ctx context.Context, job *queue.EvalJobPayload) *storage.JobRecord {
	results := make([]storage.ModelResultRecord, len(job.TargetModels))

	var wg sync.WaitGroup
	for i, modelName := range job.TargetModels {
		wg.Add(1)
		go func(i int, modelName string) {
			defer wg.Done()
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("[runner] panic evaluating %s for job %s: %v", modelName, job.JobID, rec)
					results[i] = storage.ModelResultRecord{Model: modelName, ErrorMessage: "internal error while evaluating model"}
				}
			}()
			results[i] = r.evaluateModel(ctx, job, modelName)
		}(i, modelName)
	}
	wg.Wait()

	return BuildRecord(job, results, time.Now().UTC())
}

func (r *Runner) evaluateModel(ctx context.Context, job *queue.EvalJobPayload, modelName string) storage.ModelResultRecord {
	factory := r.NewProvider
	if factory == nil {
		factory = models.NewProvider
	}
	provider := factory(modelName)
	opts := models.ResolveOptions(modelName, job.ApiKeys, job.MaxTokens)

	callCtx, cancel := context.WithTimeout(ctx, r.PerModelTimeout)
	defer cancel()

	resp, err := provider.Generate(callCtx, job.Prompt, opts)
	if err != nil {
		return storage.ModelResultRecord{Model: modelName, ErrorMessage: err.Error()}
	}
	if resp == nil {
		return storage.ModelResultRecord{Model: modelName, ErrorMessage: fmt.Sprintf("%s returned no response", modelName)}
	}

	sc := eval.Evaluate(resp.ResponseText, job.GroundTruth, modelName, eval.DefaultBaselineScore, eval.DefaultDriftThresholdPercent)
	if resp.Simulated {
		// Offline mock output says nothing about the real model's quality, so it
		// is never graded and can never trip the regression gate.
		sc.Graded, sc.RegressionAlert, sc.DriftPercent = false, false, 0
	}
	return storage.ModelResultRecord{
		Model:            modelName,
		Response:         resp.ResponseText,
		LatencyMs:        resp.LatencyMs,
		TTFTMs:           resp.TTFTMs,
		PromptTokens:     resp.PromptTokens,
		CompletionTokens: resp.CompletionTokens,
		CostUSD:          eval.CalculateCost(modelName, resp.PromptTokens, resp.CompletionTokens),
		ExactMatch:       sc.ExactMatch,
		Graded:           sc.Graded,
		Simulated:        resp.Simulated,
		SemanticScore:    sc.SemanticScore,
		RegressionAlert:  sc.RegressionAlert,
		DriftPercent:     sc.DriftPercent,
	}
}

// BuildRecord assembles the final job record. The winner is the successful
// result with the highest semantic score (ties broken by lower latency); failed
// calls never win. A job in which every model failed is marked failed.
func BuildRecord(job *queue.EvalJobPayload, results []storage.ModelResultRecord, completedAt time.Time) *storage.JobRecord {
	var (
		totalCost float64
		winner    *storage.ModelResultRecord
		succeeded int
	)
	for i := range results {
		res := &results[i]
		if res.Failed() {
			continue
		}
		succeeded++
		totalCost += res.CostUSD
		if winner == nil ||
			res.SemanticScore > winner.SemanticScore ||
			(res.SemanticScore == winner.SemanticScore && res.LatencyMs < winner.LatencyMs) {
			winner = res
		}
	}

	status := storage.StatusCompleted
	if succeeded == 0 && len(results) > 0 {
		status = storage.StatusFailed
	}

	record := &storage.JobRecord{
		JobID:        job.JobID,
		SuiteID:      job.SuiteID,
		Status:       status,
		Prompt:       job.Prompt,
		GroundTruth:  job.GroundTruth,
		TargetModels: job.TargetModels,
		TotalCostUSD: totalCost,
		CreatedAt:    job.CreatedAt,
		CompletedAt:  &completedAt,
		Results:      results,
	}
	if winner != nil {
		record.WinningModel = winner.Model
	}
	return record
}

// EstimateCostUSD is the worst-case spend for a job: every model producing
// maxTokens of output. Prompt tokens are approximated at ~4 characters/token.
func EstimateCostUSD(prompt string, targetModels []string, maxTokens int) float64 {
	promptTokens := len(prompt)/4 + 1
	var total float64
	for _, m := range targetModels {
		total += eval.CalculateCost(m, promptTokens, maxTokens)
	}
	return total
}
