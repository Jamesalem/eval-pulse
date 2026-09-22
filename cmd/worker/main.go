package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/evalpulse/eval-pulse/internal/config"
	"github.com/evalpulse/eval-pulse/internal/queue"
	"github.com/evalpulse/eval-pulse/internal/runner"
	"github.com/evalpulse/eval-pulse/internal/storage"
	"github.com/redis/go-redis/v9"
)

const (
	readBatchSize     = 5
	readBlock         = 2 * time.Second
	reclaimInterval   = 30 * time.Second
	persistTimeout    = 5 * time.Second
	redisConnAttempts = 10
)

type WorkerEngine struct {
	cfg       *config.Config
	q         *queue.Queue
	store     *storage.HybridStore
	runner    *runner.Runner
	semaphore chan struct{}
	wg        sync.WaitGroup

	// ctx stops intake of new messages on shutdown. In-flight jobs run on
	// jobCtx, which is only cancelled if the drain deadline passes.
	ctx       context.Context
	jobCtx    context.Context
	cancelJob context.CancelFunc
}

func main() {
	cfg := config.Load()
	log.Printf("[EvalPulse Worker] Starting distributed worker node [%s]...", cfg.ConsumerName)
	log.Printf("[EvalPulse Worker] Concurrency limit: %d | Stream: %s | Group: %s", cfg.WorkerConcurrency, cfg.StreamKey, cfg.GroupName)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	q := queue.NewQueue(cfg.RedisURL, cfg.RedisPassword)
	if err := waitForRedis(ctx, q, cfg.RedisURL); err != nil {
		log.Printf("[EvalPulse Worker] %v", err)
		_ = q.Close()
		os.Exit(1)
	}

	if err := q.EnsureConsumerGroup(ctx, cfg.StreamKey, cfg.GroupName); err != nil {
		log.Printf("[EvalPulse Worker] Note on consumer group: %v", err)
	}

	jobCtx, cancelJob := context.WithCancel(context.Background())
	defer cancelJob()

	engine := &WorkerEngine{
		cfg:       cfg,
		q:         q,
		store:     storage.NewHybridStore(q.Client()),
		runner:    runner.New(time.Duration(cfg.DefaultTimeoutSec) * time.Second),
		semaphore: make(chan struct{}, cfg.WorkerConcurrency),
		ctx:       ctx,
		jobCtx:    jobCtx,
		cancelJob: cancelJob,
	}

	log.Println("[EvalPulse Worker] Worker loop engaged. Listening for evaluation streams...")
	reclaimDone := make(chan struct{})
	go func() {
		defer close(reclaimDone)
		engine.reclaimLoop()
	}()
	engine.runLoop()
	// Both producers of new work must stop before draining the WaitGroup.
	<-reclaimDone

	log.Println("[EvalPulse Worker] Shutdown signal received. Draining active jobs...")
	engine.drain(time.Duration(cfg.DefaultTimeoutSec+5) * time.Second)
	_ = q.Close()
	log.Println("[EvalPulse Worker] Node shutdown complete. Exiting cleanly.")
}

func waitForRedis(ctx context.Context, q *queue.Queue, addr string) error {
	for i := 1; i <= redisConnAttempts; i++ {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := q.Ping(pingCtx)
		cancel()
		if err == nil {
			log.Printf("[EvalPulse Worker] Successfully connected to Redis at %s", addr)
			return nil
		}
		log.Printf("[EvalPulse Worker] Waiting for Redis (%d/%d): %v", i, redisConnAttempts, err)
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return fmt.Errorf("redis at %s unreachable after %d attempts; exiting so the orchestrator can restart this node", addr, redisConnAttempts)
}

// drain waits for in-flight jobs. If the deadline passes, remaining jobs are
// cancelled; their messages stay pending and are reclaimed by another worker.
func (w *WorkerEngine) drain(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		log.Println("[EvalPulse Worker] Drain deadline exceeded; cancelling remaining jobs (they will be reclaimed).")
		w.cancelJob()
		<-done
	}
}

func (w *WorkerEngine) runLoop() {
	backoff := time.Second
	for w.ctx.Err() == nil {
		msgs, err := w.q.ReadJobs(w.ctx, w.cfg.StreamKey, w.cfg.GroupName, w.cfg.ConsumerName, readBatchSize, readBlock)
		if err != nil {
			if w.ctx.Err() != nil {
				return
			}
			log.Printf("[EvalPulse Worker] Stream read failed (retrying in %v): %v", backoff, err)
			select {
			case <-time.After(backoff):
			case <-w.ctx.Done():
				return
			}
			backoff = min(backoff*2, 30*time.Second)
			continue
		}
		backoff = time.Second

		if !w.dispatch(msgs) {
			return
		}
	}
}

// reclaimLoop periodically takes over messages that another consumer received
// but never acknowledged (e.g. the node crashed mid-job).
func (w *WorkerEngine) reclaimLoop() {
	ticker := time.NewTicker(reclaimInterval)
	defer ticker.Stop()
	minIdle := time.Duration(w.cfg.ReclaimIdleSec) * time.Second

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
		}

		msgs, err := w.q.AutoclaimAbandoned(w.ctx, w.cfg.StreamKey, w.cfg.GroupName, w.cfg.ConsumerName, minIdle)
		if err != nil {
			if w.ctx.Err() == nil {
				log.Printf("[EvalPulse Worker] Reclaim scan failed: %v", err)
			}
			continue
		}
		if len(msgs) > 0 {
			log.Printf("[EvalPulse Worker] Reclaimed %d abandoned message(s)", len(msgs))
			w.dispatch(msgs)
		}
	}
}

// dispatch starts a bounded goroutine per message. It returns false if the
// worker is shutting down before all messages were dispatched; undispatched
// messages remain pending and will be reclaimed.
func (w *WorkerEngine) dispatch(msgs []redis.XMessage) bool {
	for _, msg := range msgs {
		select {
		case w.semaphore <- struct{}{}:
		case <-w.ctx.Done():
			return false
		}
		// select picks randomly when both cases are ready; re-check shutdown.
		if w.ctx.Err() != nil {
			<-w.semaphore
			return false
		}

		w.wg.Add(1)
		go func(m redis.XMessage) {
			defer func() {
				<-w.semaphore
				w.wg.Done()
			}()
			w.processMessage(m)
		}(msg)
	}
	return true
}

func (w *WorkerEngine) processMessage(msg redis.XMessage) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("[EvalPulse Worker] Panic processing message %s: %v", msg.ID, rec)
			w.deadLetter(msg, fmt.Sprintf("panic: %v", rec))
		}
	}()

	payloadRaw, ok := msg.Values["payload"].(string)
	if !ok {
		log.Printf("[EvalPulse Worker] Invalid message payload structure: %s", msg.ID)
		w.deadLetter(msg, "Missing payload field")
		return
	}

	var job queue.EvalJobPayload
	if err := json.Unmarshal([]byte(payloadRaw), &job); err != nil {
		log.Printf("[EvalPulse Worker] JSON deserialization failed for %s: %v", msg.ID, err)
		w.deadLetter(msg, "Malformed JSON")
		return
	}
	if job.JobID == "" || len(job.TargetModels) == 0 {
		w.deadLetter(msg, "Payload missing job_id or target_models")
		return
	}

	log.Printf("[EvalPulse Worker] Dequeued Job [%s] across %d models", job.JobID, len(job.TargetModels))
	w.markRunning(&job)

	record := w.runner.Run(w.jobCtx, &job)

	// Aborted by the drain deadline: leave the message pending so it is
	// reprocessed rather than persisting a partial result.
	if w.jobCtx.Err() != nil {
		log.Printf("[EvalPulse Worker] Job [%s] interrupted by shutdown; leaving for reclaim", job.JobID)
		return
	}

	persistCtx, cancel := context.WithTimeout(context.Background(), persistTimeout)
	defer cancel()

	if err := w.store.SaveJob(persistCtx, record); err != nil {
		// Without a persisted result the job must not be acked; it will be retried.
		log.Printf("[EvalPulse Worker] Failed to persist job [%s], leaving unacked: %v", job.JobID, err)
		return
	}

	w.ackAndDelete(persistCtx, msg.ID)
	log.Printf("[EvalPulse Worker] Finished Job [%s] status=%s in %v (Winner: %s | Cost: $%0.5f)",
		job.JobID, record.Status, time.Since(job.CreatedAt).Round(time.Millisecond), record.WinningModel, record.TotalCostUSD)
}

func (w *WorkerEngine) markRunning(job *queue.EvalJobPayload) {
	ctx, cancel := context.WithTimeout(context.Background(), persistTimeout)
	defer cancel()
	err := w.store.SaveJob(ctx, &storage.JobRecord{
		JobID:        job.JobID,
		SuiteID:      job.SuiteID,
		Status:       storage.StatusRunning,
		Prompt:       job.Prompt,
		GroundTruth:  job.GroundTruth,
		TargetModels: job.TargetModels,
		CreatedAt:    job.CreatedAt,
		Results:      []storage.ModelResultRecord{},
	})
	if err != nil {
		log.Printf("[EvalPulse Worker] Could not mark job [%s] running: %v", job.JobID, err)
	}
}

func (w *WorkerEngine) deadLetter(msg redis.XMessage, reason string) {
	ctx, cancel := context.WithTimeout(context.Background(), persistTimeout)
	defer cancel()
	if err := w.q.SendToDLQ(ctx, w.cfg.DLQStreamKey, msg, reason); err != nil {
		log.Printf("[EvalPulse Worker] Failed to dead-letter %s, leaving unacked: %v", msg.ID, err)
		return
	}
	w.ackAndDelete(ctx, msg.ID)
}

// ackAndDelete acknowledges the message and removes it from the stream so its
// BYOK credentials do not persist in Redis.
func (w *WorkerEngine) ackAndDelete(ctx context.Context, id string) {
	if err := w.q.AckJob(ctx, w.cfg.StreamKey, w.cfg.GroupName, id); err != nil {
		log.Printf("[EvalPulse Worker] Failed to ACK message %s: %v", id, err)
		return
	}
	if err := w.q.DeleteMessage(ctx, w.cfg.StreamKey, id); err != nil {
		log.Printf("[EvalPulse Worker] Failed to delete message %s: %v", id, err)
	}
}
