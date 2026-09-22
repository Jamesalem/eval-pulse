package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/evalpulse/eval-pulse/internal/config"
	"github.com/evalpulse/eval-pulse/internal/eval"
	"github.com/evalpulse/eval-pulse/internal/models"
	"github.com/evalpulse/eval-pulse/internal/queue"
	"github.com/evalpulse/eval-pulse/internal/storage"
)

type WorkerEngine struct {
	cfg       *config.Config
	q         *queue.Queue
	store     *storage.HybridStore
	semaphore chan struct{}
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
}

func main() {
	cfg := config.Load()
	log.Printf("[EvalPulse Worker] Starting distributed worker node [%s]...", cfg.ConsumerName)
	log.Printf("[EvalPulse Worker] Concurrency limit: %d | Stream: %s | Group: %s", cfg.WorkerConcurrency, cfg.StreamKey, cfg.GroupName)

	q := queue.NewQueue(cfg.RedisURL, cfg.RedisPassword)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Verify Redis connection with retry loop
	for i := 1; i <= 10; i++ {
		pingCtx, pCancel := context.WithTimeout(ctx, 2*time.Second)
		err := q.Ping(pingCtx)
		pCancel()
		if err == nil {
			log.Printf("[EvalPulse Worker] Successfully connected to Redis at %s", cfg.RedisURL)
			break
		}
		log.Printf("[EvalPulse Worker] Waiting for Redis (%d/10): %v", i, err)
		time.Sleep(2 * time.Second)
	}

	if err := q.EnsureConsumerGroup(ctx, cfg.StreamKey, cfg.GroupName); err != nil {
		log.Printf("[EvalPulse Worker] Note on consumer group: %v", err)
	}

	store := storage.NewHybridStore(nil)

	engine := &WorkerEngine{
		cfg:       cfg,
		q:         q,
		store:     store,
		semaphore: make(chan struct{}, cfg.WorkerConcurrency),
		ctx:       ctx,
		cancel:    cancel,
	}

	// Trap graceful shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("[EvalPulse Worker] Shutdown signal received (SIGINT/SIGTERM). Commencing graceful drain...")
		engine.cancel()
	}()

	log.Println("[EvalPulse Worker] Worker loop engaged. Listening for evaluation streams...")
	engine.runLoop()

	log.Println("[EvalPulse Worker] Waiting for active worker goroutines to drain...")
	engine.wg.Wait()
	_ = q.Close()
	log.Println("[EvalPulse Worker] Node shutdown complete. Exiting cleanly.")
}

func (w *WorkerEngine) runLoop() {
	for {
		select {
		case <-w.ctx.Done():
			return
		default:
		}

		// Read messages from consumer group
		msgs, err := w.q.ReadJobs(w.ctx, w.cfg.StreamKey, w.cfg.GroupName, w.cfg.ConsumerName, 5, 2*time.Second)
		if err != nil {
			if w.ctx.Err() != nil {
				return
			}
			time.Sleep(1 * time.Second)
			continue
		}

		for _, msg := range msgs {
			// Acquire bounded semaphore
			select {
			case w.semaphore <- struct{}{}:
			case <-w.ctx.Done():
				return
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
	}
}

func (w *WorkerEngine) processMessage(msg redis.XMessage) {
	payloadRaw, ok := msg.Values["payload"].(string)
	if !ok {
		log.Printf("[EvalPulse Worker] Invalid message payload structure: %s", msg.ID)
		_ = w.q.SendToDLQ(w.ctx, w.cfg.DLQStreamKey, msg, "Missing payload field")
		_ = w.q.AckJob(w.ctx, w.cfg.StreamKey, w.cfg.GroupName, msg.ID)
		return
	}

	var job queue.EvalJobPayload
	if err := json.Unmarshal([]byte(payloadRaw), &job); err != nil {
		log.Printf("[EvalPulse Worker] JSON deserialization failed for %s: %v", msg.ID, err)
		_ = w.q.SendToDLQ(w.ctx, w.cfg.DLQStreamKey, msg, "Malformed JSON")
		_ = w.q.AckJob(w.ctx, w.cfg.StreamKey, w.cfg.GroupName, msg.ID)
		return
	}

	log.Printf("[EvalPulse Worker] Dequeued Job [%s] across %d models", job.JobID, len(job.TargetModels))

	// Fan out parallel evaluations across target models
	var fanWg sync.WaitGroup
	resultsChan := make(chan storage.ModelResultRecord, len(job.TargetModels))

	for _, modelName := range job.TargetModels {
		fanWg.Add(1)
		go func(m string) {
			defer fanWg.Done()

			provider := models.NewProvider(m)
			var apiKey string
			if job.ApiKeys != nil {
				apiKey = job.ApiKeys[m]
			}

			evalCtx, evalCancel := context.WithTimeout(w.ctx, time.Duration(w.cfg.DefaultTimeoutSec)*time.Second)
			defer evalCancel()

			resp, err := provider.Generate(evalCtx, job.Prompt, models.ModelOptions{
				MaxTokens: job.MaxTokens,
				ApiKey:    apiKey,
			})

			if err != nil {
				resultsChan <- storage.ModelResultRecord{
					Model:        m,
					ErrorMessage: err.Error(),
				}
				return
			}

			scoreCard := eval.Evaluate(resp.ResponseText, job.GroundTruth, m, 0.92, 5.0)
			cost := eval.CalculateCost(m, resp.PromptTokens, resp.CompletionTokens)

			resultsChan <- storage.ModelResultRecord{
				Model:            m,
				Response:         resp.ResponseText,
				LatencyMs:        resp.LatencyMs,
				TTFTMs:           resp.TTFTMs,
				PromptTokens:     resp.PromptTokens,
				CompletionTokens: resp.CompletionTokens,
				CostUSD:          cost,
				ExactMatch:       scoreCard.ExactMatch,
				SemanticScore:    scoreCard.SemanticScore,
				RegressionAlert:  scoreCard.RegressionAlert,
				DriftPercent:     scoreCard.DriftPercent,
			}
		}(modelName)
	}

	fanWg.Wait()
	close(resultsChan)

	var results []storage.ModelResultRecord
	var totalCost float64
	var winningModel string
	var bestScore float64 = -1.0

	for res := range resultsChan {
		totalCost += res.CostUSD
		if res.SemanticScore > bestScore {
			bestScore = res.SemanticScore
			winningModel = res.Model
		}
		results = append(results, res)
	}

	// Persist completed record
	record := &storage.JobRecord{
		JobID:        job.JobID,
		SuiteID:      job.SuiteID,
		Status:       "completed",
		Prompt:       job.Prompt,
		GroundTruth:  job.GroundTruth,
		TargetModels: job.TargetModels,
		WinningModel: winningModel,
		TotalCostUSD: totalCost,
		CreatedAt:    job.CreatedAt,
		CompletedAt:  time.Now().UTC(),
		Results:      results,
	}
	_ = w.store.SaveJob(w.ctx, record)

	// Acknowledge stream message
	if err := w.q.AckJob(w.ctx, w.cfg.StreamKey, w.cfg.GroupName, msg.ID); err != nil {
		log.Printf("[EvalPulse Worker] Failed to ACK message %s: %v", msg.ID, err)
	} else {
		log.Printf("[EvalPulse Worker] Completed and ACKed Job [%s] in %v (Winning: %s | Cost: $%0.5f)",
			job.JobID, time.Since(job.CreatedAt), winningModel, totalCost)
	}
}
