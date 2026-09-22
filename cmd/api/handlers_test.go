package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/evalpulse/eval-pulse/internal/config"
	"github.com/evalpulse/eval-pulse/internal/queue"
	"github.com/evalpulse/eval-pulse/internal/storage"
)

func newTestServer(t *testing.T) (*Server, http.Handler) {
	t.Helper()
	cfg := &config.Config{
		BudgetCapUSD:       10,
		DefaultTimeoutSec:  5,
		CORSAllowedOrigins: []string{"http://allowed.example"},
	}
	store := storage.NewHybridStore(nil)
	srv := newServer(cfg, queue.NewQueue("127.0.0.1:0", ""), store, false)
	return srv, srv.routes()
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestSubmitJobValidation(t *testing.T) {
	_, h := newTestServer(t)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"malformed", `{`, "Invalid JSON"},
		{"empty prompt", `{"prompt":"   "}`, "prompt"},
		{"max tokens too high", `{"prompt":"hi","max_tokens":999999}`, "max_tokens"},
		{"negative budget", `{"prompt":"hi","budget_cap_usd":-1}`, "budget_cap_usd"},
		{"budget over server cap", `{"prompt":"hi","budget_cap_usd":500}`, "server limit"},
		{"worst case exceeds cap", `{"prompt":"hi","target_models":["claude-3-5-sonnet"],"max_tokens":8000,"budget_cap_usd":0.01}`, "exceeds the budget cap"},
		{"bad ollama endpoint", `{"prompt":"hi","api_keys":{"ollama_endpoint":"file:///etc/passwd"}}`, "Ollama endpoint"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, h, http.MethodPost, "/api/v1/eval/jobs", tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body)
			}
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Errorf("body %q does not mention %q", rec.Body.String(), tc.want)
			}
		})
	}
}

func TestSubmitJobRejectsOversizedBody(t *testing.T) {
	_, h := newTestServer(t)
	huge := `{"prompt":"` + strings.Repeat("a", maxBodyBytes+10) + `"}`
	rec := do(t, h, http.MethodPost, "/api/v1/eval/jobs", huge)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestSubmitJobRunsLocallyAndCompletes(t *testing.T) {
	srv, h := newTestServer(t)

	rec := do(t, h, http.MethodPost, "/api/v1/eval/jobs",
		`{"prompt":"hello","target_models":["mock-a","mock-b","mock-a"],"ground_truth":"hello"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d body %s", rec.Code, rec.Body)
	}
	var resp struct {
		JobID         string   `json:"job_id"`
		ExecutionMode string   `json:"execution_mode"`
		TargetModels  []string `json:"target_models"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ExecutionMode != "local" {
		t.Errorf("execution_mode = %s, want local", resp.ExecutionMode)
	}
	if len(resp.TargetModels) != 2 {
		t.Errorf("duplicate target models not removed: %v", resp.TargetModels)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.waitLocalJobs(ctx)

	rec = do(t, h, http.MethodGet, "/api/v1/eval/jobs/"+resp.JobID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get job status = %d", rec.Code)
	}
	var job storage.JobRecord
	_ = json.Unmarshal(rec.Body.Bytes(), &job)
	if job.Status != storage.StatusCompleted || len(job.Results) != 2 || job.WinningModel == "" {
		t.Errorf("unexpected job %+v", job)
	}
}

func TestGetJobNotFound(t *testing.T) {
	_, h := newTestServer(t)
	if rec := do(t, h, http.MethodGet, "/api/v1/eval/jobs/nope", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestListJobsRejectsBadLimit(t *testing.T) {
	_, h := newTestServer(t)
	if rec := do(t, h, http.MethodGet, "/api/v1/eval/jobs?limit=abc", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if rec := do(t, h, http.MethodGet, "/api/v1/eval/jobs?limit=5000", ""); rec.Code != http.StatusOK {
		t.Fatalf("large limit should be clamped, got %d", rec.Code)
	}
}

func TestRegressionCheckOnlyCountsGradedRuns(t *testing.T) {
	srv, h := newTestServer(t)
	ctx := context.Background()

	// An ungraded (no ground truth) low score must not fail the gate.
	_ = srv.store.SaveJob(ctx, &storage.JobRecord{JobID: "u", CreatedAt: time.Now(), Results: []storage.ModelResultRecord{
		{Model: "m1", SemanticScore: 0.3, LatencyMs: 10},
	}})
	rec := do(t, h, http.MethodGet, "/api/v1/metrics/regression-check", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("ungraded data should pass, got %d: %s", rec.Code, rec.Body)
	}

	_ = srv.store.SaveJob(ctx, &storage.JobRecord{JobID: "g", CreatedAt: time.Now(), Results: []storage.ModelResultRecord{
		{Model: "m2", SemanticScore: 0.5, LatencyMs: 10, Graded: true},
	}})
	rec = do(t, h, http.MethodGet, "/api/v1/metrics/regression-check?threshold_percent=5", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("graded regression should fail with 409, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"m2"`) || strings.Contains(rec.Body.String(), `"m1"`) {
		t.Errorf("unexpected failing models: %s", rec.Body)
	}

	if rec := do(t, h, http.MethodGet, "/api/v1/metrics/regression-check?threshold_percent=-3", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("invalid threshold should be 400, got %d", rec.Code)
	}
}

func TestValidateRejectsUnknownProvider(t *testing.T) {
	_, h := newTestServer(t)
	rec := do(t, h, http.MethodPost, "/api/v1/providers/validate", `{"provider":"openrouter","api_key":"x"}`)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Unsupported provider") {
		t.Fatalf("status = %d body %s", rec.Code, rec.Body)
	}
}

func TestCORSAllowList(t *testing.T) {
	_, h := newTestServer(t)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/eval/jobs", nil)
	req.Header.Set("Origin", "http://allowed.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://allowed.example" {
		t.Errorf("allowed origin not echoed")
	}

	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "http://evil.example")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("disallowed origin received CORS header")
	}
}

func TestRecoverPanics(t *testing.T) {
	h := recoverPanics(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
