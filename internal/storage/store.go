package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Job lifecycle states.
const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// Regression status buckets for aggregated model metrics.
const (
	RegressionStable   = "stable"
	RegressionDegraded = "degraded"
	RegressionCritical = "critical"
	RegressionUngraded = "ungraded"
)

const (
	jobKeyPrefix = "eval:job:"
	jobIndexKey  = "eval:jobs:index"
	jobTTL       = 30 * 24 * time.Hour

	// maxIndexedJobs bounds the Redis recency index and the in-memory cache.
	maxIndexedJobs = 5000
	// comparisonWindow is how many recent jobs feed the model comparison metrics.
	comparisonWindow = 500
)

// ErrNotFound is returned when a job does not exist.
var ErrNotFound = errors.New("job not found")

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
	Graded           bool    `json:"graded"`
	Simulated        bool    `json:"simulated,omitempty"`
	SemanticScore    float64 `json:"semantic_score"`
	RegressionAlert  bool    `json:"regression_alert"`
	DriftPercent     float64 `json:"drift_percent"`
	ErrorMessage     string  `json:"error_message,omitempty"`
}

// Failed reports whether the model call errored.
func (r ModelResultRecord) Failed() bool {
	return r.ErrorMessage != ""
}

// JobRecord represents the full evaluation job and its multi-model outcome.
type JobRecord struct {
	JobID        string              `json:"job_id"`
	SuiteID      string              `json:"suite_id,omitempty"`
	Status       string              `json:"status"`
	Prompt       string              `json:"prompt"`
	GroundTruth  string              `json:"ground_truth,omitempty"`
	TargetModels []string            `json:"target_models"`
	WinningModel string              `json:"winning_model,omitempty"`
	TotalCostUSD float64             `json:"total_cost_usd"`
	CreatedAt    time.Time           `json:"created_at"`
	CompletedAt  *time.Time          `json:"completed_at,omitempty"`
	Results      []ModelResultRecord `json:"results"`
}

// ModelComparisonMetric summarizes aggregated performance for a model.
type ModelComparisonMetric struct {
	Model            string  `json:"model"`
	TotalEvals       int     `json:"total_evals"`
	FailedEvals      int     `json:"failed_evals"`
	GradedEvals      int     `json:"graded_evals"`
	P50LatencyMs     int64   `json:"p50_latency_ms"`
	P95LatencyMs     int64   `json:"p95_latency_ms"`
	P99LatencyMs     int64   `json:"p99_latency_ms"`
	AvgSemanticScore float64 `json:"avg_semantic_score"`
	AvgGradedScore   float64 `json:"avg_graded_score"`
	TotalCostUSD     float64 `json:"total_cost_usd"`
	RegressionStatus string  `json:"regression_status"`
}

// Store defines persistence operations for jobs and metrics.
type Store interface {
	SaveJob(ctx context.Context, job *JobRecord) error
	GetJob(ctx context.Context, jobID string) (*JobRecord, error)
	ListJobs(ctx context.Context, limit int) ([]*JobRecord, error)
	GetModelComparison(ctx context.Context) ([]ModelComparisonMetric, error)
}

// HybridStore persists jobs to Redis (the source of truth shared between the API
// and worker processes) and keeps a bounded in-memory copy that serves reads
// when Redis is absent or temporarily unavailable.
type HybridStore struct {
	redisClient *redis.Client

	mu         sync.RWMutex
	memoryJobs map[string]*JobRecord
}

var _ Store = (*HybridStore)(nil)

// NewHybridStore initializes a store. If redisClient is nil, operates purely in-memory.
func NewHybridStore(redisClient *redis.Client) *HybridStore {
	return &HybridStore{
		redisClient: redisClient,
		memoryJobs:  make(map[string]*JobRecord),
	}
}

// SaveJob persists a job record in memory and, when configured, in Redis.
func (s *HybridStore) SaveJob(ctx context.Context, job *JobRecord) error {
	if job == nil || job.JobID == "" {
		return errors.New("job record requires a job_id")
	}
	stored := cloneJob(job)

	s.mu.Lock()
	s.memoryJobs[stored.JobID] = stored
	s.evictOldestLocked()
	s.mu.Unlock()

	if s.redisClient == nil {
		return nil
	}

	data, err := json.Marshal(stored)
	if err != nil {
		return fmt.Errorf("marshal job %s: %w", job.JobID, err)
	}
	pipe := s.redisClient.TxPipeline()
	pipe.Set(ctx, jobKeyPrefix+stored.JobID, data, jobTTL)
	pipe.ZAdd(ctx, jobIndexKey, redis.Z{Score: float64(stored.CreatedAt.UnixMilli()), Member: stored.JobID})
	pipe.ZRemRangeByRank(ctx, jobIndexKey, 0, -maxIndexedJobs-1)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("persist job %s to redis: %w", job.JobID, err)
	}
	return nil
}

// GetJob retrieves a job record by ID. Redis is consulted first because another
// process (the worker) may have written a newer version than the local cache.
func (s *HybridStore) GetJob(ctx context.Context, jobID string) (*JobRecord, error) {
	if s.redisClient != nil {
		val, err := s.redisClient.Get(ctx, jobKeyPrefix+jobID).Bytes()
		switch {
		case err == nil:
			var fetched JobRecord
			if uerr := json.Unmarshal(val, &fetched); uerr == nil {
				return &fetched, nil
			}
		case !errors.Is(err, redis.Nil):
			log.Printf("[storage] redis GET %s failed, serving from memory: %v", jobID, err)
		}
	}

	s.mu.RLock()
	job, exists := s.memoryJobs[jobID]
	s.mu.RUnlock()
	if exists {
		return cloneJob(job), nil
	}
	return nil, ErrNotFound
}

// ListJobs returns the most recent evaluation jobs, newest first.
func (s *HybridStore) ListJobs(ctx context.Context, limit int) ([]*JobRecord, error) {
	if limit <= 0 {
		limit = 20
	}

	if s.redisClient != nil {
		jobs, err := s.listFromRedis(ctx, limit)
		if err == nil {
			return jobs, nil
		}
		log.Printf("[storage] redis list failed, serving from memory: %v", err)
	}

	s.mu.RLock()
	jobs := make([]*JobRecord, 0, len(s.memoryJobs))
	for _, j := range s.memoryJobs {
		jobs = append(jobs, j)
	}
	s.mu.RUnlock()

	sortNewestFirst(jobs)
	if len(jobs) > limit {
		jobs = jobs[:limit]
	}
	for i, j := range jobs {
		jobs[i] = cloneJob(j)
	}
	return jobs, nil
}

func (s *HybridStore) listFromRedis(ctx context.Context, limit int) ([]*JobRecord, error) {
	ids, err := s.redisClient.ZRevRange(ctx, jobIndexKey, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []*JobRecord{}, nil
	}

	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = jobKeyPrefix + id
	}
	vals, err := s.redisClient.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	jobs := make([]*JobRecord, 0, len(vals))
	for _, v := range vals {
		raw, ok := v.(string)
		if !ok {
			continue // expired between ZREVRANGE and MGET
		}
		var job JobRecord
		if json.Unmarshal([]byte(raw), &job) == nil {
			jobs = append(jobs, &job)
		}
	}
	return jobs, nil
}

// IsEmpty reports whether the store holds no jobs.
func (s *HybridStore) IsEmpty(ctx context.Context) bool {
	if s.redisClient != nil {
		if n, err := s.redisClient.ZCard(ctx, jobIndexKey).Result(); err == nil {
			return n == 0
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.memoryJobs) == 0
}

// GetModelComparison calculates latency percentiles and performance scores per
// model across the most recent jobs. Failed calls are counted but excluded from
// latency and score aggregates so they cannot skew percentiles toward zero.
func (s *HybridStore) GetModelComparison(ctx context.Context) ([]ModelComparisonMetric, error) {
	jobs, err := s.ListJobs(ctx, comparisonWindow)
	if err != nil {
		return nil, err
	}
	return AggregateModelMetrics(jobs), nil
}

// AggregateModelMetrics computes per-model comparison metrics from job records.
func AggregateModelMetrics(jobs []*JobRecord) []ModelComparisonMetric {
	type acc struct {
		latencies   []int64
		scoreSum    float64
		scoreCount  int
		gradedSum   float64
		gradedCount int
		failed      int
		cost        float64
	}
	byModel := make(map[string]*acc)

	for _, job := range jobs {
		for _, res := range job.Results {
			a, ok := byModel[res.Model]
			if !ok {
				a = &acc{}
				byModel[res.Model] = a
			}
			if res.Failed() {
				a.failed++
				continue
			}
			a.latencies = append(a.latencies, res.LatencyMs)
			a.scoreSum += res.SemanticScore
			a.scoreCount++
			a.cost += res.CostUSD
			if res.Graded {
				a.gradedSum += res.SemanticScore
				a.gradedCount++
			}
		}
	}

	metrics := make([]ModelComparisonMetric, 0, len(byModel))
	for model, a := range byModel {
		m := ModelComparisonMetric{
			Model:            model,
			TotalEvals:       len(a.latencies) + a.failed,
			FailedEvals:      a.failed,
			GradedEvals:      a.gradedCount,
			TotalCostUSD:     roundTo(a.cost, 6),
			RegressionStatus: RegressionUngraded,
		}
		if len(a.latencies) > 0 {
			sorted := append([]int64(nil), a.latencies...)
			sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
			m.P50LatencyMs = percentile(sorted, 50)
			m.P95LatencyMs = percentile(sorted, 95)
			m.P99LatencyMs = percentile(sorted, 99)
		}
		if a.scoreCount > 0 {
			m.AvgSemanticScore = roundTo(a.scoreSum/float64(a.scoreCount), 3)
		}
		if a.gradedCount > 0 {
			m.AvgGradedScore = roundTo(a.gradedSum/float64(a.gradedCount), 3)
			m.RegressionStatus = classifyRegression(m.AvgGradedScore)
		}
		metrics = append(metrics, m)
	}

	sort.Slice(metrics, func(i, j int) bool { return metrics[i].Model < metrics[j].Model })
	return metrics
}

func classifyRegression(score float64) string {
	switch {
	case score < 0.70:
		return RegressionCritical
	case score < 0.85:
		return RegressionDegraded
	default:
		return RegressionStable
	}
}

// percentile uses the nearest-rank method on an ascending slice.
func percentile(sorted []int64, p int) int64 {
	n := len(sorted)
	rank := int(math.Ceil(float64(p) / 100 * float64(n)))
	if rank < 1 {
		rank = 1
	}
	if rank > n {
		rank = n
	}
	return sorted[rank-1]
}

func roundTo(v float64, places int) float64 {
	f := math.Pow(10, float64(places))
	return math.Round(v*f) / f
}

func sortNewestFirst(jobs []*JobRecord) {
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})
}

// evictOldestLocked keeps the in-memory cache bounded. Caller must hold s.mu.
func (s *HybridStore) evictOldestLocked() {
	overflow := len(s.memoryJobs) - maxIndexedJobs
	if overflow <= 0 {
		return
	}
	jobs := make([]*JobRecord, 0, len(s.memoryJobs))
	for _, j := range s.memoryJobs {
		jobs = append(jobs, j)
	}
	sortNewestFirst(jobs)
	for _, j := range jobs[len(jobs)-overflow:] {
		delete(s.memoryJobs, j.JobID)
	}
}

// cloneJob returns a copy whose slices are not shared with the caller, so
// cached records cannot be mutated from outside the store.
func cloneJob(j *JobRecord) *JobRecord {
	c := *j
	c.TargetModels = append([]string(nil), j.TargetModels...)
	c.Results = append([]ModelResultRecord{}, j.Results...)
	if j.CompletedAt != nil {
		t := *j.CompletedAt
		c.CompletedAt = &t
	}
	return &c
}
