package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// ModelResultRecord represents the evaluated output and metrics for a single model in a job.
type ModelResultRecord struct {
	Model            string  `json:"model"`
	Response         string  `json:"response"`
	LatencyMs        int64   `json:"latency_ms"`
	TTFTMs           int64   `json:"ttft_ms"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	CostUSD          float64 `json:"cost_usd"`
	ExactMatch       bool    `json:"exact_match"`
	SemanticScore    float64 `json:"semantic_score"`
	RegressionAlert  bool    `json:"regression_alert"`
	DriftPercent     float64 `json:"drift_percent"`
	ErrorMessage     string  `json:"error_message,omitempty"`
}

// JobRecord represents the full evaluation job and its multi-model outcome.
type JobRecord struct {
	JobID        string              `json:"job_id"`
	SuiteID      string              `json:"suite_id,omitempty"`
	Status       string              `json:"status"` // queued, running, completed, failed
	Prompt       string              `json:"prompt"`
	GroundTruth  string              `json:"ground_truth,omitempty"`
	TargetModels []string            `json:"target_models"`
	WinningModel string              `json:"winning_model,omitempty"`
	TotalCostUSD float64             `json:"total_cost_usd"`
	CreatedAt    time.Time           `json:"created_at"`
	CompletedAt  time.Time           `json:"completed_at,omitempty"`
	Results      []ModelResultRecord `json:"results"`
}

// ModelComparisonMetric summarizes aggregated performance for a model.
type ModelComparisonMetric struct {
	Model            string  `json:"model"`
	TotalEvals       int     `json:"total_evals"`
	P50LatencyMs     int64   `json:"p50_latency_ms"`
	P95LatencyMs     int64   `json:"p95_latency_ms"`
	P99LatencyMs     int64   `json:"p99_latency_ms"`
	AvgSemanticScore float64 `json:"avg_semantic_score"`
	TotalCostUSD     float64 `json:"total_cost_usd"`
	RegressionStatus string  `json:"regression_status"` // stable, degraded, critical
}

// Store defines persistence operations for jobs and metrics.
type Store interface {
	SaveJob(ctx context.Context, job *JobRecord) error
	GetJob(ctx context.Context, jobID string) (*JobRecord, error)
	ListJobs(ctx context.Context, limit int) ([]*JobRecord, error)
	GetModelComparison(ctx context.Context) ([]ModelComparisonMetric, error)
}

// HybridStore coordinates Redis persistence with thread-safe in-memory caching.
type HybridStore struct {
	redisClient *redis.Client
	mu          sync.RWMutex
	memoryJobs  map[string]*JobRecord
	latencies   map[string][]int64
	scores      map[string][]float64
	costs       map[string]float64
}

// NewHybridStore initializes a store. If redisClient is nil, operates purely in-memory.
func NewHybridStore(redisClient *redis.Client) *HybridStore {
	return &HybridStore{
		redisClient: redisClient,
		memoryJobs:  make(map[string]*JobRecord),
		latencies:   make(map[string][]int64),
		scores:      make(map[string][]float64),
		costs:       make(map[string]float64),
	}
}

// SaveJob persists a job record in memory and optionally in Redis.
func (s *HybridStore) SaveJob(ctx context.Context, job *JobRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.memoryJobs[job.JobID] = job

	// Update aggregated metrics per model
	for _, res := range job.Results {
		s.latencies[res.Model] = append(s.latencies[res.Model], res.LatencyMs)
		s.scores[res.Model] = append(s.scores[res.Model], res.SemanticScore)
		s.costs[res.Model] += res.CostUSD
	}

	if s.redisClient != nil {
		data, err := json.Marshal(job)
		if err == nil {
			s.redisClient.Set(ctx, fmt.Sprintf("eval:job:%s", job.JobID), data, 30*24*time.Hour)
		}
	}

	return nil
}

// GetJob retrieves a job record by ID.
func (s *HybridStore) GetJob(ctx context.Context, jobID string) (*JobRecord, error) {
	s.mu.RLock()
	job, exists := s.memoryJobs[jobID]
	s.mu.RUnlock()

	if exists {
		return job, nil
	}

	if s.redisClient != nil {
		val, err := s.redisClient.Get(ctx, fmt.Sprintf("eval:job:%s", jobID)).Result()
		if err == nil {
			var fetched JobRecord
			if json.Unmarshal([]byte(val), &fetched) == nil {
				return &fetched, nil
			}
		}
	}

	return nil, fmt.Errorf("job %s not found", jobID)
}

// ListJobs returns the most recent evaluation jobs.
func (s *HybridStore) ListJobs(ctx context.Context, limit int) ([]*JobRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*JobRecord, 0, len(s.memoryJobs))
	for _, j := range s.memoryJobs {
		jobs = append(jobs, j)
	}

	// Sort newest first
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})

	if limit > 0 && len(jobs) > limit {
		jobs = jobs[:limit]
	}

	return jobs, nil
}

// GetModelComparison calculates latency percentiles and performance scores per model.
func (s *HybridStore) GetModelComparison(ctx context.Context) ([]ModelComparisonMetric, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var metrics []ModelComparisonMetric

	for model, latList := range s.latencies {
		if len(latList) == 0 {
			continue
		}

		sortedLats := make([]int64, len(latList))
		copy(sortedLats, latList)
		sort.Slice(sortedLats, func(i, j int) bool { return sortedLats[i] < sortedLats[j] })

		n := len(sortedLats)
		p50 := sortedLats[n*50/100]
		p95 := sortedLats[n*95/100]
		p99 := sortedLats[n*99/100]

		var totalScore float64
		for _, sc := range s.scores[model] {
			totalScore += sc
		}
		avgScore := totalScore / float64(len(s.scores[model]))

		regStatus := "stable"
		if avgScore < 0.85 {
			regStatus = "degraded"
		}
		if avgScore < 0.70 {
			regStatus = "critical"
		}

		metrics = append(metrics, ModelComparisonMetric{
			Model:            model,
			TotalEvals:       n,
			P50LatencyMs:     p50,
			P95LatencyMs:     p95,
			P99LatencyMs:     p99,
			AvgSemanticScore: float64(int(avgScore*1000)) / 1000.0,
			TotalCostUSD:     float64(int(s.costs[model]*10000)) / 10000.0,
			RegressionStatus: regStatus,
		})
	}

	// Sort by model name
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].Model < metrics[j].Model
	})

	return metrics, nil
}
