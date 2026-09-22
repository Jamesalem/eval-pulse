package storage

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func sampleJob(id string, created time.Time) *JobRecord {
	return &JobRecord{
		JobID:        id,
		Status:       StatusCompleted,
		Prompt:       "Benchmark prompt",
		TargetModels: []string{"gemini-1.5-flash", "gpt-4o-mini"},
		WinningModel: "gemini-1.5-flash",
		TotalCostUSD: 0.00045,
		CreatedAt:    created,
		Results: []ModelResultRecord{
			{Model: "gemini-1.5-flash", LatencyMs: 240, CostUSD: 0.00015, SemanticScore: 0.94, Graded: true},
			{Model: "gpt-4o-mini", LatencyMs: 310, CostUSD: 0.00030, SemanticScore: 0.91, Graded: true},
		},
	}
}

func TestHybridStore(t *testing.T) {
	store := NewHybridStore(nil)
	ctx := context.Background()

	if err := store.SaveJob(ctx, sampleJob("test-job-001", time.Now().UTC())); err != nil {
		t.Fatalf("Failed to save job: %v", err)
	}

	retrieved, err := store.GetJob(ctx, "test-job-001")
	if err != nil {
		t.Fatalf("Failed to retrieve job: %v", err)
	}
	if retrieved.JobID != "test-job-001" {
		t.Errorf("Expected jobID test-job-001, got %s", retrieved.JobID)
	}

	comparison, err := store.GetModelComparison(ctx)
	if err != nil {
		t.Fatalf("Failed to get model comparison: %v", err)
	}
	if len(comparison) != 2 {
		t.Fatalf("Expected 2 comparison records, got %d", len(comparison))
	}
}

func TestSaveJobRejectsMissingID(t *testing.T) {
	if err := NewHybridStore(nil).SaveJob(context.Background(), &JobRecord{}); err == nil {
		t.Fatal("expected error for job without ID")
	}
}

func TestGetJobNotFound(t *testing.T) {
	_, err := NewHybridStore(nil).GetJob(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// Regression test: saving the same job twice (queued -> completed) used to
// append its results to the aggregates again, double counting them.
func TestResavingJobDoesNotDoubleCount(t *testing.T) {
	store := NewHybridStore(nil)
	ctx := context.Background()
	job := sampleJob("job-1", time.Now().UTC())
	_ = store.SaveJob(ctx, job)
	_ = store.SaveJob(ctx, job)

	comparison, _ := store.GetModelComparison(ctx)
	for _, m := range comparison {
		if m.TotalEvals != 1 {
			t.Errorf("%s: TotalEvals = %d, want 1", m.Model, m.TotalEvals)
		}
	}
}

func TestListJobsNewestFirstWithLimit(t *testing.T) {
	store := NewHybridStore(nil)
	ctx := context.Background()
	base := time.Now().UTC()
	for i := 0; i < 5; i++ {
		_ = store.SaveJob(ctx, sampleJob(fmt.Sprintf("job-%d", i), base.Add(time.Duration(i)*time.Minute)))
	}

	jobs, err := store.ListJobs(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 3 {
		t.Fatalf("len = %d, want 3", len(jobs))
	}
	if jobs[0].JobID != "job-4" || jobs[2].JobID != "job-2" {
		t.Errorf("unexpected order: %s ... %s", jobs[0].JobID, jobs[2].JobID)
	}
}

func TestStoreReturnsCopies(t *testing.T) {
	store := NewHybridStore(nil)
	ctx := context.Background()
	_ = store.SaveJob(ctx, sampleJob("job-1", time.Now().UTC()))

	got, _ := store.GetJob(ctx, "job-1")
	got.Results[0].Model = "mutated"

	again, _ := store.GetJob(ctx, "job-1")
	if again.Results[0].Model == "mutated" {
		t.Fatal("caller mutation leaked into the store")
	}
}

func TestAggregateExcludesFailuresAndTracksGrading(t *testing.T) {
	jobs := []*JobRecord{{
		JobID: "j",
		Results: []ModelResultRecord{
			{Model: "m", LatencyMs: 100, SemanticScore: 0.95, Graded: true},
			{Model: "m", LatencyMs: 300, SemanticScore: 0.60, Graded: false},
			{Model: "m", ErrorMessage: "timeout"},
		},
	}}

	metrics := AggregateModelMetrics(jobs)
	if len(metrics) != 1 {
		t.Fatalf("len = %d, want 1", len(metrics))
	}
	m := metrics[0]
	if m.TotalEvals != 3 || m.FailedEvals != 1 || m.GradedEvals != 1 {
		t.Errorf("counts = total %d failed %d graded %d", m.TotalEvals, m.FailedEvals, m.GradedEvals)
	}
	if m.P50LatencyMs != 100 || m.P99LatencyMs != 300 {
		t.Errorf("percentiles p50=%d p99=%d; a failed call must not contribute a 0ms latency", m.P50LatencyMs, m.P99LatencyMs)
	}
	if m.AvgGradedScore != 0.95 || m.RegressionStatus != RegressionStable {
		t.Errorf("graded avg %v status %s", m.AvgGradedScore, m.RegressionStatus)
	}
}

func TestAggregateUngradedModel(t *testing.T) {
	metrics := AggregateModelMetrics([]*JobRecord{{Results: []ModelResultRecord{{Model: "m", LatencyMs: 10, SemanticScore: 0.7}}}})
	if metrics[0].RegressionStatus != RegressionUngraded {
		t.Errorf("status = %s, want ungraded", metrics[0].RegressionStatus)
	}
}
