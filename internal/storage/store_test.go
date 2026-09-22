package storage

import (
	"context"
	"testing"
	"time"
)

func TestHybridStore(t *testing.T) {
	store := NewHybridStore(nil)
	ctx := context.Background()

	job := &JobRecord{
		JobID:        "test-job-001",
		Status:       "completed",
		Prompt:       "Benchmark prompt",
		TargetModels: []string{"gemini-1.5-flash", "gpt-4o-mini"},
		WinningModel: "gemini-1.5-flash",
		TotalCostUSD: 0.00045,
		CreatedAt:    time.Now().UTC(),
		Results: []ModelResultRecord{
			{
				Model:         "gemini-1.5-flash",
				LatencyMs:     240,
				CostUSD:       0.00015,
				SemanticScore: 0.94,
			},
			{
				Model:         "gpt-4o-mini",
				LatencyMs:     310,
				CostUSD:       0.00030,
				SemanticScore: 0.91,
			},
		},
	}

	if err := store.SaveJob(ctx, job); err != nil {
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
