package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/evalpulse/eval-pulse/internal/config"
	"github.com/evalpulse/eval-pulse/internal/models"
	"github.com/evalpulse/eval-pulse/internal/queue"
	"github.com/evalpulse/eval-pulse/internal/runner"
	"github.com/evalpulse/eval-pulse/internal/storage"
)

const version = "1.1.0"

func main() {
	cfg := config.Load()
	log.Printf("[EvalPulse API] Initializing on port %s...", cfg.APIPort)

	q := queue.NewQueue(cfg.RedisURL, cfg.RedisPassword)

	pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	redisErr := q.Ping(pingCtx)
	cancel()

	var store *storage.HybridStore
	if redisErr != nil {
		log.Printf("[EvalPulse API] Redis not reachable (%v). Running standalone with in-memory storage and local execution.", redisErr)
		store = storage.NewHybridStore(nil)
	} else {
		log.Printf("[EvalPulse API] Connected to Redis stream at %s", cfg.RedisURL)
		groupCtx, groupCancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := q.EnsureConsumerGroup(groupCtx, cfg.StreamKey, cfg.GroupName); err != nil {
			log.Printf("[EvalPulse API] Warning: %v", err)
		}
		groupCancel()
		// Share Redis with the worker so completed results are visible here.
		store = storage.NewHybridStore(q.Client())
	}

	srv := newServer(cfg, q, store, redisErr == nil)

	if cfg.SeedDemoData {
		go seedDemoData(store)
	}

	httpServer := &http.Server{
		Addr:              ":" + cfg.APIPort,
		Handler:           srv.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("[EvalPulse API] Server listening on http://localhost:%s", cfg.APIPort)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	exitCode := 0
	select {
	case err := <-serverErr:
		log.Printf("[EvalPulse API] Fatal server error: %v", err)
		exitCode = 1
	case <-quit:
		log.Println("[EvalPulse API] Shutting down gracefully...")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[EvalPulse API] HTTP shutdown error: %v", err)
	}
	srv.waitLocalJobs(shutdownCtx)
	_ = q.Close()
	log.Println("[EvalPulse API] Server stopped.")
	// Deferred calls do not run after os.Exit, so release the context first.
	shutdownCancel()
	os.Exit(exitCode)
}

// seedDemoData populates an empty store with sample benchmarks so the dashboard
// has something to show on first launch. It always uses simulated providers so
// it never spends real tokens or touches a local model daemon.
func seedDemoData(store *storage.HybridStore) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if !store.IsEmpty(ctx) {
		return
	}

	demo := &runner.Runner{PerModelTimeout: 5 * time.Second, NewProvider: models.NewMockProvider}
	modelsList := []string{"gemini-1.5-flash", "gpt-4o-mini", "claude-3-5-sonnet", "ollama:llama3"}
	prompts := []string{
		"Write a concurrent rate-limited worker pool in Go.",
		"Explain the difference between Redis Streams and Kafka.",
		"Implement a JSON schema validator for LLM structured outputs.",
	}

	now := time.Now().UTC()
	for i, prompt := range prompts {
		createdAt := now.Add(-time.Duration(len(prompts)-i) * 15 * time.Minute)
		record := demo.Run(ctx, &queue.EvalJobPayload{
			JobID:        fmt.Sprintf("job-seed-%02d", i+1),
			SuiteID:      "demo-seed-suite",
			Prompt:       prompt,
			TargetModels: modelsList,
			MaxTokens:    256,
			CreatedAt:    createdAt,
		})
		completedAt := createdAt.Add(2 * time.Second)
		record.CompletedAt = &completedAt
		if err := store.SaveJob(ctx, record); err != nil {
			log.Printf("[EvalPulse API] Demo seed failed: %v", err)
			return
		}
	}
	log.Printf("[EvalPulse API] Seeded %d demo benchmark runs (set SEED_DEMO_DATA=false to disable).", len(prompts))
}
