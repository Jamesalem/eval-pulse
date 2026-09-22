package main

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/evalpulse/eval-pulse/internal/config"
	"github.com/evalpulse/eval-pulse/internal/models"
	"github.com/evalpulse/eval-pulse/internal/queue"
	"github.com/evalpulse/eval-pulse/internal/runner"
	"github.com/evalpulse/eval-pulse/internal/storage"
	"github.com/redis/go-redis/v9"
)

func newTestEngine(t *testing.T) (*WorkerEngine, *queue.Queue) {
	t.Helper()
	mr := miniredis.RunT(t)
	q := queue.NewQueue(mr.Addr(), "")
	t.Cleanup(func() { _ = q.Close() })

	cfg := &config.Config{StreamKey: "eval:jobs", DLQStreamKey: "eval:dlq", GroupName: "g", ConsumerName: "c1", WorkerConcurrency: 2, DefaultTimeoutSec: 5}
	if err := q.EnsureConsumerGroup(context.Background(), cfg.StreamKey, cfg.GroupName); err != nil {
		t.Fatal(err)
	}

	jobCtx, cancelJob := context.WithCancel(context.Background())
	t.Cleanup(cancelJob)
	return &WorkerEngine{
		cfg:       cfg,
		q:         q,
		store:     storage.NewHybridStore(q.Client()),
		runner:    &runner.Runner{PerModelTimeout: 5 * time.Second, NewProvider: models.NewMockProvider},
		semaphore: make(chan struct{}, cfg.WorkerConcurrency),
		ctx:       context.Background(),
		jobCtx:    jobCtx,
		cancelJob: cancelJob,
	}, q
}

func readOne(t *testing.T, w *WorkerEngine) (msgs int) {
	t.Helper()
	got, err := w.q.ReadJobs(context.Background(), w.cfg.StreamKey, w.cfg.GroupName, w.cfg.ConsumerName, 10, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range got {
		w.processMessage(m)
	}
	return len(got)
}

func TestWorkerProcessesPersistsAcksAndPurges(t *testing.T) {
	w, q := newTestEngine(t)
	ctx := context.Background()

	job := &queue.EvalJobPayload{
		JobID: "job-1", Prompt: "hello", GroundTruth: "hello",
		TargetModels: []string{"mock-a", "mock-b"}, ApiKeys: map[string]string{"openai": "sk-secret"},
		CreatedAt: time.Now().UTC(),
	}
	if _, err := q.EnqueueJob(ctx, w.cfg.StreamKey, job); err != nil {
		t.Fatal(err)
	}
	if n := readOne(t, w); n != 1 {
		t.Fatalf("read %d messages, want 1", n)
	}

	// The API's store (a separate instance) must see the completed result.
	apiStore := storage.NewHybridStore(q.Client())
	rec, err := apiStore.GetJob(ctx, "job-1")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != storage.StatusCompleted || len(rec.Results) != 2 || rec.WinningModel == "" {
		t.Fatalf("unexpected record %+v", rec)
	}

	if n, _ := q.Client().XLen(ctx, w.cfg.StreamKey).Result(); n != 0 {
		t.Errorf("processed message (with credentials) still in stream, len = %d", n)
	}
	pending, _ := q.Client().XPending(ctx, w.cfg.StreamKey, w.cfg.GroupName).Result()
	if pending.Count != 0 {
		t.Errorf("message left pending: %d", pending.Count)
	}
}

func TestWorkerDeadLettersMalformedPayload(t *testing.T) {
	w, q := newTestEngine(t)
	ctx := context.Background()

	if err := q.Client().XAdd(ctx, &redis.XAddArgs{Stream: w.cfg.StreamKey, Values: map[string]interface{}{"payload": "{not json"}}).Err(); err != nil {
		t.Fatal(err)
	}
	readOne(t, w)

	if n, _ := q.Client().XLen(ctx, w.cfg.DLQStreamKey).Result(); n != 1 {
		t.Fatalf("DLQ length = %d, want 1", n)
	}
	pending, _ := q.Client().XPending(ctx, w.cfg.StreamKey, w.cfg.GroupName).Result()
	if pending.Count != 0 {
		t.Errorf("malformed message should be acked after dead-lettering, pending = %d", pending.Count)
	}
}

func TestWorkerLeavesInterruptedJobPending(t *testing.T) {
	w, q := newTestEngine(t)
	ctx := context.Background()

	if _, err := q.EnqueueJob(ctx, w.cfg.StreamKey, &queue.EvalJobPayload{JobID: "job-2", Prompt: "p", TargetModels: []string{"mock-a"}}); err != nil {
		t.Fatal(err)
	}
	w.cancelJob() // simulate the drain deadline expiring mid-job
	readOne(t, w)

	pending, _ := q.Client().XPending(ctx, w.cfg.StreamKey, w.cfg.GroupName).Result()
	if pending.Count != 1 {
		t.Fatalf("interrupted job must stay pending for reclaim, pending = %d", pending.Count)
	}
	rec, _ := w.store.GetJob(ctx, "job-2")
	if rec == nil || rec.Status == storage.StatusCompleted {
		t.Fatalf("interrupted job must not be recorded as completed: %+v", rec)
	}
}
