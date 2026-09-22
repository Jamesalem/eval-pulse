package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/evalpulse/eval-pulse/internal/config"
	"github.com/evalpulse/eval-pulse/internal/eval"
	"github.com/evalpulse/eval-pulse/internal/models"
	"github.com/evalpulse/eval-pulse/internal/queue"
	"github.com/evalpulse/eval-pulse/internal/runner"
	"github.com/evalpulse/eval-pulse/internal/storage"
	"github.com/google/uuid"
)

// Request limits. These keep a single request from monopolising memory,
// provider quota, or the local fallback executor.
const (
	maxBodyBytes        = 1 << 20 // 1 MiB
	maxPromptChars      = 32_000
	maxGroundTruthChars = 32_000
	maxSuiteIDChars     = 128
	maxModelIDChars     = 100
	maxTargetModels     = 12
	maxOutputTokens     = 8192
	maxAPIKeyEntries    = 20
	defaultMaxTokens    = 512
	defaultBudgetCapUSD = 1.00
	maxListLimit        = 100
	maxLocalJobs        = 8
)

var defaultTargetModels = []string{"gemini-1.5-flash", "gpt-4o-mini", "claude-3-5-sonnet", "ollama:llama3"}

// Server holds the API's dependencies.
type Server struct {
	cfg          *config.Config
	q            *queue.Queue
	store        *storage.HybridStore
	runner       *runner.Runner
	redisEnabled bool

	// localSlots bounds concurrent in-process executions when Redis is unavailable.
	localSlots chan struct{}
	localWG    sync.WaitGroup
}

func newServer(cfg *config.Config, q *queue.Queue, store *storage.HybridStore, redisEnabled bool) *Server {
	return &Server{
		cfg:          cfg,
		q:            q,
		store:        store,
		runner:       runner.New(time.Duration(cfg.DefaultTimeoutSec) * time.Second),
		redisEnabled: redisEnabled,
		localSlots:   make(chan struct{}, maxLocalJobs),
	}
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("POST /api/v1/providers/validate", s.handleValidateProvider)
	mux.HandleFunc("POST /api/v1/eval/jobs", s.handleSubmitJob)
	mux.HandleFunc("GET /api/v1/eval/jobs/{id}", s.handleGetJob)
	mux.HandleFunc("GET /api/v1/eval/jobs", s.handleListJobs)
	mux.HandleFunc("GET /api/v1/metrics/comparison", s.handleGetModelComparison)
	mux.HandleFunc("GET /api/v1/metrics/regression-check", s.handleRegressionCheck)

	return chain(mux,
		recoverPanics,
		logRequests,
		securityHeaders,
		cors(s.cfg.CORSAllowedOrigins),
	)
}

// waitLocalJobs blocks until in-process fallback jobs finish or ctx expires.
func (s *Server) waitLocalJobs(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		s.localWG.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		log.Println("[EvalPulse API] Timed out waiting for local jobs to finish")
	}
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	status := "ok"
	redisStatus := "standalone_in_memory"
	if s.redisEnabled {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := s.q.Ping(ctx); err != nil {
			status = "degraded"
			redisStatus = "unreachable"
		} else {
			redisStatus = "connected"
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    status,
		"redis":     redisStatus,
		"timestamp": time.Now().UTC(),
		"version":   version,
	})
}

type ProviderValidationRequest struct {
	Provider    string `json:"provider"`
	ApiKey      string `json:"api_key"`
	EndpointURL string `json:"endpoint_url"`
}

func (s *Server) handleValidateProvider(w http.ResponseWriter, r *http.Request) {
	var req ProviderValidationRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	req.Provider = strings.TrimSpace(req.Provider)
	if models.Family(req.Provider) == models.FamilyMock {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Unsupported provider %q", req.Provider))
		return
	}
	req.EndpointURL = strings.TrimSpace(req.EndpointURL)
	if err := models.ValidateEndpointURL(req.EndpointURL); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	start := time.Now()
	err := models.NewProvider(req.Provider).ValidateCredentials(ctx, models.ModelOptions{
		ApiKey:      strings.TrimSpace(req.ApiKey),
		EndpointURL: req.EndpointURL,
	})
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"provider":   req.Provider,
			"valid":      false,
			"latency_ms": elapsed,
			"message":    fmt.Sprintf("Credential check failed: %v", err),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"provider":   req.Provider,
		"valid":      true,
		"latency_ms": elapsed,
		"message":    "Credentials verified successfully.",
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

// normalize applies defaults and validates the request, returning a
// user-facing error message when it is invalid.
func (req *SubmitJobRequest) normalize(serverBudgetCap float64) error {
	req.SuiteID = strings.TrimSpace(req.SuiteID)
	if strings.TrimSpace(req.Prompt) == "" {
		return errors.New("Field 'prompt' is required")
	}
	if len(req.Prompt) > maxPromptChars {
		return fmt.Errorf("Field 'prompt' exceeds %d characters", maxPromptChars)
	}
	if len(req.GroundTruth) > maxGroundTruthChars {
		return fmt.Errorf("Field 'ground_truth' exceeds %d characters", maxGroundTruthChars)
	}
	if len(req.SuiteID) > maxSuiteIDChars {
		return fmt.Errorf("Field 'suite_id' exceeds %d characters", maxSuiteIDChars)
	}

	seen := make(map[string]bool, len(req.TargetModels))
	cleaned := make([]string, 0, len(req.TargetModels))
	for _, m := range req.TargetModels {
		m = strings.TrimSpace(m)
		if m == "" || seen[m] {
			continue
		}
		if len(m) > maxModelIDChars {
			return fmt.Errorf("Model identifier %.20q… is too long", m)
		}
		seen[m] = true
		cleaned = append(cleaned, m)
	}
	if len(cleaned) == 0 {
		cleaned = append(cleaned, defaultTargetModels...)
	}
	if len(cleaned) > maxTargetModels {
		return fmt.Errorf("At most %d target models may be evaluated per job", maxTargetModels)
	}
	req.TargetModels = cleaned

	switch {
	case req.MaxTokens == 0:
		req.MaxTokens = defaultMaxTokens
	case req.MaxTokens < 0 || req.MaxTokens > maxOutputTokens:
		return fmt.Errorf("Field 'max_tokens' must be between 1 and %d", maxOutputTokens)
	}

	switch {
	case req.BudgetCapUSD == 0:
		req.BudgetCapUSD = defaultBudgetCapUSD
	case req.BudgetCapUSD < 0:
		return errors.New("Field 'budget_cap_usd' must be positive")
	case req.BudgetCapUSD > serverBudgetCap:
		return fmt.Errorf("Field 'budget_cap_usd' exceeds the server limit of $%.2f", serverBudgetCap)
	}

	if len(req.ApiKeys) > maxAPIKeyEntries {
		return errors.New("Too many entries in 'api_keys'")
	}
	if err := models.ValidateEndpointURL(strings.TrimSpace(req.ApiKeys[models.OllamaEndpointKey])); err != nil {
		return fmt.Errorf("Invalid Ollama endpoint: %v", err)
	}
	return nil
}

func (s *Server) handleSubmitJob(w http.ResponseWriter, r *http.Request) {
	var req SubmitJobRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := req.normalize(s.cfg.BudgetCapUSD); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Enforce the budget cap before any tokens are spent.
	estimated := runner.EstimateCostUSD(req.Prompt, req.TargetModels, req.MaxTokens)
	if estimated > req.BudgetCapUSD {
		writeError(w, http.StatusBadRequest, fmt.Sprintf(
			"Worst-case cost $%.4f exceeds the budget cap of $%.2f. Reduce max tokens or target models, or raise the cap.",
			estimated, req.BudgetCapUSD))
		return
	}

	payload := &queue.EvalJobPayload{
		JobID:        uuid.NewString(),
		SuiteID:      req.SuiteID,
		Prompt:       req.Prompt,
		GroundTruth:  req.GroundTruth,
		TargetModels: req.TargetModels,
		MaxTokens:    req.MaxTokens,
		BudgetCapUSD: req.BudgetCapUSD,
		ApiKeys:      req.ApiKeys,
		CreatedAt:    time.Now().UTC(),
	}

	queued := &storage.JobRecord{
		JobID:        payload.JobID,
		SuiteID:      payload.SuiteID,
		Status:       storage.StatusQueued,
		Prompt:       payload.Prompt,
		GroundTruth:  payload.GroundTruth,
		TargetModels: payload.TargetModels,
		CreatedAt:    payload.CreatedAt,
		Results:      []storage.ModelResultRecord{},
	}
	if err := s.store.SaveJob(r.Context(), queued); err != nil {
		log.Printf("[EvalPulse API] Warning: could not persist queued job %s: %v", payload.JobID, err)
	}

	mode := "stream"
	if !s.tryEnqueue(r.Context(), payload) {
		if !s.runLocally(payload) {
			queued.Status = storage.StatusFailed
			_ = s.store.SaveJob(context.Background(), queued)
			writeError(w, http.StatusServiceUnavailable, "Evaluation capacity exhausted. Retry shortly.")
			return
		}
		mode = "local"
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"job_id":             payload.JobID,
		"status":             storage.StatusQueued,
		"execution_mode":     mode,
		"queued_at":          payload.CreatedAt,
		"target_models":      payload.TargetModels,
		"estimated_cost_usd": estimated,
	})
}

// tryEnqueue publishes to the Redis stream when Redis is configured.
func (s *Server) tryEnqueue(ctx context.Context, payload *queue.EvalJobPayload) bool {
	if !s.redisEnabled {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := s.q.EnqueueJob(ctx, s.cfg.StreamKey, payload); err != nil {
		log.Printf("[EvalPulse API] Enqueue failed for job %s, falling back to local execution: %v", payload.JobID, err)
		return false
	}
	return true
}

// runLocally executes the job in-process when no worker is reachable. It
// returns false if the bounded local executor is saturated.
func (s *Server) runLocally(payload *queue.EvalJobPayload) bool {
	select {
	case s.localSlots <- struct{}{}:
	default:
		return false
	}

	s.localWG.Add(1)
	go func() {
		defer func() {
			<-s.localSlots
			s.localWG.Done()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.cfg.DefaultTimeoutSec+5)*time.Second)
		defer cancel()

		record := s.runner.Run(ctx, payload)
		if err := s.store.SaveJob(ctx, record); err != nil {
			log.Printf("[EvalPulse API] Failed to save local job %s: %v", payload.JobID, err)
		}
	}()
	return true
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	jobID := strings.TrimSpace(r.PathValue("id"))
	if jobID == "" || len(jobID) > 64 {
		writeError(w, http.StatusBadRequest, "Invalid job ID")
		return
	}

	job, err := s.store.GetJob(r.Context(), jobID)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("Job %s not found", jobID))
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load job")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		parsed, err := strconv.Atoi(l)
		if err != nil || parsed < 1 {
			writeError(w, http.StatusBadRequest, "Query parameter 'limit' must be a positive integer")
			return
		}
		limit = min(parsed, maxListLimit)
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

// handleRegressionCheck is the CI gate. Drift is measured against the same
// baseline used when scoring individual results, and only graded runs (those
// with a ground truth) count — exploratory runs have nothing to regress from.
func (s *Server) handleRegressionCheck(w http.ResponseWriter, r *http.Request) {
	threshold := eval.DefaultDriftThresholdPercent
	if th := r.URL.Query().Get("threshold_percent"); th != "" {
		parsed, err := strconv.ParseFloat(th, 64)
		if err != nil || parsed <= 0 || parsed > 100 {
			writeError(w, http.StatusBadRequest, "Query parameter 'threshold_percent' must be between 0 and 100")
			return
		}
		threshold = parsed
	}

	comparison, err := s.store.GetModelComparison(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to retrieve model comparison")
		return
	}

	failingModels := []string{}
	var maxDrift float64
	gradedModels := 0
	for _, m := range comparison {
		if m.GradedEvals == 0 {
			continue
		}
		gradedModels++
		drift := (eval.DefaultBaselineScore - m.AvgGradedScore) / eval.DefaultBaselineScore * 100.0
		maxDrift = max(maxDrift, drift)
		if drift > threshold {
			failingModels = append(failingModels, m.Model)
		}
	}

	passed := len(failingModels) == 0
	statusCode, exitCode := http.StatusOK, 0
	msg := "All graded models meet baseline standards. No regression detected."
	switch {
	case !passed:
		statusCode, exitCode = http.StatusConflict, 1
		msg = fmt.Sprintf("Regression detected in %d model(s) exceeding the %.1f%% drift threshold.", len(failingModels), threshold)
	case gradedModels == 0:
		msg = "No graded runs yet. Regression gating needs a benchmark with a ground truth run against a real provider (simulated responses are not graded)."
	}

	writeJSON(w, statusCode, map[string]any{
		"passed":                     passed,
		"exit_code":                  exitCode,
		"threshold_percent":          threshold,
		"baseline_score":             eval.DefaultBaselineScore,
		"graded_models":              gradedModels,
		"max_observed_drift_percent": roundTo1(maxDrift),
		"failing_models":             failingModels,
		"message":                    msg,
		"timestamp":                  time.Now().UTC(),
	})
}

func roundTo1(v float64) float64 {
	return float64(int64(v*10+0.5)) / 10
}

// decodeJSON reads a size-limited JSON body into dst.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return fmt.Errorf("Request body exceeds %d bytes", maxBodyBytes)
		}
		return errors.New("Invalid JSON request payload")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[EvalPulse API] Failed to encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
