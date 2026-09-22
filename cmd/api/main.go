package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/evalpulse/eval-pulse/internal/config"
	"github.com/evalpulse/eval-pulse/internal/eval"
	"github.com/evalpulse/eval-pulse/internal/models"
	"github.com/evalpulse/eval-pulse/internal/queue"
	"github.com/evalpulse/eval-pulse/internal/storage"
)

type Server struct {
	cfg   *config.Config
	q     *queue.Queue
	store *storage.HybridStore
}

func main() {
	cfg := config.Load()
	log.Printf("[EvalPulse API] Initializing on port %s...", cfg.APIPort)

	q := queue.NewQueue(cfg.RedisURL, cfg.RedisPassword)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var redisErr error
	if err := q.Ping(ctx); err != nil {
		log.Printf("[EvalPulse API] Notice: Redis not reachable (%v). Running with hybrid in-memory fallback.", err)
		redisErr = err
	} else {
		log.Printf("[EvalPulse API] Connected to Redis stream at %s", cfg.RedisURL)
		_ = q.EnsureConsumerGroup(context.Background(), cfg.StreamKey, cfg.GroupName)
	}

	store := storage.NewHybridStore(nil)

	// Seed some initial demo benchmark data if store is fresh
	seedInitialMetrics(store)

	srv := &Server{
		cfg:   cfg,
		q:     q,
		store: store,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", srv.handleHealthz(redisErr == nil))
	mux.HandleFunc("POST /api/v1/providers/validate", srv.handleValidateProvider)
	mux.HandleFunc("POST /api/v1/eval/jobs", srv.handleSubmitJob)
	mux.HandleFunc("GET /api/v1/eval/jobs/{id}", srv.handleGetJob)
	mux.HandleFunc("GET /api/v1/eval/jobs", srv.handleListJobs)
	mux.HandleFunc("GET /api/v1/metrics/comparison", srv.handleGetModelComparison)
	mux.HandleFunc("GET /api/v1/metrics/regression-check", srv.handleRegressionCheck)

	handler := enableCORS(mux)

	httpServer := &http.Server{
		Addr:         ":" + cfg.APIPort,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		log.Printf("[EvalPulse API] Server listening on http://localhost:%s", cfg.APIPort)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[EvalPulse API] Fatal error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[EvalPulse API] Shutting down gracefully...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = httpServer.Shutdown(shutdownCtx)
	_ = q.Close()
	log.Println("[EvalPulse API] Server stopped.")
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, x-api-key")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealthz(redisConnected bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := "ok"
		redisStatus := "connected"
		if !redisConnected {
			redisStatus = "standalone_in_memory"
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":    status,
			"redis":     redisStatus,
			"timestamp": time.Now().UTC(),
			"version":   "1.0.0",
		})
	}
}

type ProviderValidationRequest struct {
	Provider    string `json:"provider"`
	ApiKey      string `json:"api_key"`
	EndpointURL string `json:"endpoint_url"`
}

func (s *Server) handleValidateProvider(w http.ResponseWriter, r *http.Request) {
	var req ProviderValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON request payload")
		return
	}

	start := time.Now()
	provider := models.NewProvider(req.Provider)
	opts := models.ModelOptions{
		ApiKey:      req.ApiKey,
		EndpointURL: req.EndpointURL,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	err := provider.ValidateCredentials(ctx, opts)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"provider":   req.Provider,
			"valid":      false,
			"latency_ms": elapsed,
			"message":    fmt.Sprintf("Credential check failed: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"provider":   req.Provider,
		"valid":      true,
		"latency_ms": elapsed,
		"message":    "Credentials verified and quota validated successfully.",
	})
}

type SubmitJobRequest struct {
	SuiteID      string            `json:"suite_id"`
	Prompt       string            `json:"prompt"`
	GroundTruth  string            `json:"ground_truth"`
	TargetModels []string          `json:"target_models"`
	MaxTokens    int               `json:"max_tokens"`
	BudgetCapUSD float64           `json:"budget_cap_usd"`
	ApiKeys      map[string]string `json:"api_keys"`
}

func (s *Server) handleSubmitJob(w http.ResponseWriter, r *http.Request) {
	var req SubmitJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if strings.TrimSpace(req.Prompt) == "" {
		writeError(w, http.StatusBadRequest, "Field 'prompt' is required")
		return
	}

	if len(req.TargetModels) == 0 {
		req.TargetModels = []string{"gemini-1.5-flash", "gpt-4o-mini", "claude-3-5-sonnet", "ollama"}
	}
	if req.MaxTokens <= 0 {
		req.MaxTokens = 512
	}
	if req.BudgetCapUSD <= 0 {
		req.BudgetCapUSD = 1.00
	}

	jobID := uuid.New().String()
	payload := &queue.EvalJobPayload{
		JobID:        jobID,
		SuiteID:      req.SuiteID,
		Prompt:       req.Prompt,
		GroundTruth:  req.GroundTruth,
		TargetModels: req.TargetModels,
		MaxTokens:    req.MaxTokens,
		BudgetCapUSD: req.BudgetCapUSD,
		ApiKeys:      req.ApiKeys,
		CreatedAt:    time.Now().UTC(),
	}

	// Register job as queued in store
	initialJob := &storage.JobRecord{
		JobID:        jobID,
		SuiteID:      req.SuiteID,
		Status:       "queued",
		Prompt:       req.Prompt,
		GroundTruth:  req.GroundTruth,
		TargetModels: req.TargetModels,
		CreatedAt:    payload.CreatedAt,
		Results:      []storage.ModelResultRecord{},
	}
	_ = s.store.SaveJob(r.Context(), initialJob)

	// Attempt enqueue onto Redis stream; if Redis unreachable, process asynchronously via local worker logic
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_, err := s.q.EnqueueJob(ctx, s.cfg.StreamKey, payload)
		if err != nil {
			// Standalone / fallback local execution
			executeLocalJob(payload, s.store)
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"job_id":             jobID,
		"status":             "queued",
		"queued_at":          payload.CreatedAt,
		"target_models":      req.TargetModels,
		"estimated_cost_usd": eval.CalculateCost("gemini-1.5-flash", len(req.Prompt)/4, req.MaxTokens) * float64(len(req.TargetModels)),
	})
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID := r.PathValue("id")
	if jobID == "" {
		writeError(w, http.StatusBadRequest, "Missing job ID")
		return
	}

	job, err := s.store.GetJob(r.Context(), jobID)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("Job %s not found", jobID))
		return
	}

	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	jobs, err := s.store.ListJobs(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list jobs")
		return
	}

	writeJSON(w, http.StatusOK, jobs)
}

func (s *Server) handleGetModelComparison(w http.ResponseWriter, r *http.Request) {
	comparison, err := s.store.GetModelComparison(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to retrieve model comparison")
		return
	}
	writeJSON(w, http.StatusOK, comparison)
}

func (s *Server) handleRegressionCheck(w http.ResponseWriter, r *http.Request) {
	threshold := 5.0
	if th := r.URL.Query().Get("threshold_percent"); th != "" {
		if parsed, err := strconv.ParseFloat(th, 64); err == nil {
			threshold = parsed
		}
	}

	comparison, _ := s.store.GetModelComparison(r.Context())
	var failingModels []string
	var maxDrift float64

	for _, m := range comparison {
		if m.AvgSemanticScore < 0.85 {
			drift := (0.90 - m.AvgSemanticScore) / 0.90 * 100.0
			if drift > maxDrift {
				maxDrift = drift
			}
			if drift > threshold {
				failingModels = append(failingModels, m.Model)
			}
		}
	}

	passed := len(failingModels) == 0
	statusCode := http.StatusOK
	exitCode := 0
	msg := "All models meet baseline standards. No regression detected."

	if !passed {
		statusCode = http.StatusConflict
		exitCode = 1
		msg = fmt.Sprintf("Regression detected in %d model(s) exceeding %0.1f%% threshold.", len(failingModels), threshold)
	}

	writeJSON(w, statusCode, map[string]interface{}{
		"passed":                     passed,
		"exit_code":                  exitCode,
		"threshold_percent":          threshold,
		"max_observed_drift_percent": maxDrift,
		"failing_models":             failingModels,
		"message":                    msg,
		"timestamp":                  time.Now().UTC(),
	})
}

// executeLocalJob executes an evaluation when running without a dedicated worker container.
func executeLocalJob(p *queue.EvalJobPayload, store *storage.HybridStore) {
	time.Sleep(100 * time.Millisecond)
	var results []storage.ModelResultRecord
	var totalCost float64
	var winningModel string
	var bestScore float64 = -1.0

	for _, modelName := range p.TargetModels {
		provider := models.NewProvider(modelName)
		var apiKey string
		if p.ApiKeys != nil {
			apiKey = p.ApiKeys[modelName]
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		resp, err := provider.Generate(ctx, p.Prompt, models.ModelOptions{
			MaxTokens: p.MaxTokens,
			ApiKey:    apiKey,
		})
		cancel()

		if err != nil {
			results = append(results, storage.ModelResultRecord{
				Model:        modelName,
				ErrorMessage: err.Error(),
			})
			continue
		}

		scoreCard := eval.Evaluate(resp.ResponseText, p.GroundTruth, modelName, 0.92, 5.0)
		cost := eval.CalculateCost(modelName, resp.PromptTokens, resp.CompletionTokens)
		totalCost += cost

		if scoreCard.SemanticScore > bestScore {
			bestScore = scoreCard.SemanticScore
			winningModel = modelName
		}

		results = append(results, storage.ModelResultRecord{
			Model:            modelName,
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
		})
	}

	completedJob := &storage.JobRecord{
		JobID:        p.JobID,
		SuiteID:      p.SuiteID,
		Status:       "completed",
		Prompt:       p.Prompt,
		GroundTruth:  p.GroundTruth,
		TargetModels: p.TargetModels,
		WinningModel: winningModel,
		TotalCostUSD: totalCost,
		CreatedAt:    p.CreatedAt,
		CompletedAt:  time.Now().UTC(),
		Results:      results,
	}

	_ = store.SaveJob(context.Background(), completedJob)
}

func seedInitialMetrics(store *storage.HybridStore) {
	// Pre-seed sample benchmarks so dashboard has immediate rich visual data
	modelsList := []string{"gemini-1.5-flash", "gpt-4o-mini", "claude-3-5-sonnet", "ollama:llama3"}
	prompts := []string{
		"Write a concurrent rate-limited worker pool in Go.",
		"Explain the difference between Redis Streams and Kafka.",
		"Implement a JSON schema validator for LLM structured outputs.",
	}

	for i, pr := range prompts {
		jobID := fmt.Sprintf("job-seed-%02d", i+1)
		var results []storage.ModelResultRecord
		var totalCost float64

		for _, m := range modelsList {
			provider := models.NewProvider(m)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			resp, _ := provider.Generate(ctx, pr, models.ModelOptions{MaxTokens: 256})
			cancel()

			sc := eval.Evaluate(resp.ResponseText, "", m, 0.95, 5.0)
			cost := eval.CalculateCost(m, resp.PromptTokens, resp.CompletionTokens)
			totalCost += cost

			results = append(results, storage.ModelResultRecord{
				Model:            m,
				Response:         resp.ResponseText,
				LatencyMs:        resp.LatencyMs,
				TTFTMs:           resp.TTFTMs,
				PromptTokens:     resp.PromptTokens,
				CompletionTokens: resp.CompletionTokens,
				CostUSD:          cost,
				ExactMatch:       sc.ExactMatch,
				SemanticScore:    sc.SemanticScore,
				RegressionAlert:  sc.RegressionAlert,
			})
		}

		_ = store.SaveJob(context.Background(), &storage.JobRecord{
			JobID:        jobID,
			SuiteID:      "core-prompt-suite",
			Status:       "completed",
			Prompt:       pr,
			TargetModels: modelsList,
			WinningModel: "gemini-1.5-flash",
			TotalCostUSD: totalCost,
			CreatedAt:    time.Now().UTC().Add(-time.Duration((3-i)*15) * time.Minute),
			CompletedAt:  time.Now().UTC().Add(-time.Duration((3-i)*15) * time.Minute),
			Results:      results,
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
