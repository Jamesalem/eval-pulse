package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newRedisStore(t *testing.T, mr *miniredis.Miniredis) *HybridStore {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewHybridStore(client)
}

// Regression test for the core architecture bug: the API and worker each kept a
// private in-memory store, so results written by the worker were never visible
// to the API and jobs appeared "queued" forever.
func TestRedisStoreSharesJobsAcrossProcesses(t *testing.T) {
	mr := miniredis.RunT(t)
	api := newRedisStore(t, mr)
	worker := newRedisStore(t, mr)
	ctx := context.Background()

	created := time.Now().UTC()
	if err := api.SaveJob(ctx, &JobRecord{JobID: "job-1", Status: StatusQueued, CreatedAt: created}); err != nil {
		t.Fatal(err)
	}
	completed := sampleJob("job-1", created)
	if err := worker.SaveJob(ctx, completed); err != nil {
		t.Fatal(err)
	}

	got, err := api.GetJob(ctx, "job-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusCompleted || len(got.Results) != 2 {
		t.Fatalf("API sees stale job: status=%s results=%d", got.Status, len(got.Results))
	}

	jobs, err := api.ListJobs(ctx, 10)
	if err != nil || len(jobs) != 1 || jobs[0].Status != StatusCompleted {
		t.Fatalf("ListJobs = %v, %v", jobs, err)
	}

	metrics, _ := api.GetModelComparison(ctx)
	if len(metrics) != 2 {
		t.Fatalf("comparison should include worker results, got %d models", len(metrics))
	}
}

func TestRedisListOrderLimitAndTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	store := newRedisStore(t, mr)
	ctx := context.Background()

	base := time.Now().UTC()
	for i := 0; i < 5; i++ {
		if err := store.SaveJob(ctx, sampleJob(fmt.Sprintf("job-%d", i), base.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatal(err)
		}
	}

	jobs, err := store.ListJobs(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 || jobs[0].JobID != "job-4" || jobs[1].JobID != "job-3" {
		t.Fatalf("unexpected page: %+v", jobs)
	}
	if ttl := mr.TTL(jobKeyPrefix + "job-0"); ttl <= 0 {
		t.Errorf("job keys must expire, TTL = %v", ttl)
	}

	// A key that expired but is still indexed is skipped, not an error.
	mr.Del(jobKeyPrefix + "job-4")
	jobs, err = store.ListJobs(ctx, 2)
	if err != nil || len(jobs) != 1 || jobs[0].JobID != "job-3" {
		t.Fatalf("expired entry handling: %+v, %v", jobs, err)
	}
}

func TestRedisOutageFallsBackToMemory(t *testing.T) {
	mr := miniredis.RunT(t)
	store := newRedisStore(t, mr)
	ctx := context.Background()

	_ = store.SaveJob(ctx, sampleJob("job-1", time.Now().UTC()))
	mr.Close()

	got, err := store.GetJob(ctx, "job-1")
	if err != nil || got.JobID != "job-1" {
		t.Fatalf("expected memory fallback, got %v, %v", got, err)
	}
	jobs, err := store.ListJobs(ctx, 10)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("expected memory fallback list, got %v, %v", jobs, err)
	}
	if err := store.SaveJob(ctx, sampleJob("job-2", time.Now().UTC())); err == nil {
		t.Error("SaveJob should report the Redis failure")
	}
}

func TestIsEmpty(t *testing.T) {
	mr := miniredis.RunT(t)
	store := newRedisStore(t, mr)
	ctx := context.Background()
	if !store.IsEmpty(ctx) {
		t.Fatal("new store should be empty")
	}
	_ = store.SaveJob(ctx, sampleJob("job-1", time.Now().UTC()))
	if store.IsEmpty(ctx) {
		t.Fatal("store with a job should not be empty")
	}
}
