package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/evalpulse/eval-pulse/internal/models"
	"github.com/evalpulse/eval-pulse/internal/queue"
	"github.com/evalpulse/eval-pulse/internal/storage"
)

type stubProvider struct {
	name    string
	text    string
	err     error
	panics  bool
	latency int64
}

func (s *stubProvider) Name() string { return s.name }

func (s *stubProvider) ValidateCredentials(context.Context, models.ModelOptions) error { return nil }

func (s *stubProvider) Generate(ctx context.Context, prompt string, opts models.ModelOptions) (*models.ModelResponse, error) {
	if s.panics {
		panic("boom")
	}
	if s.err != nil {
		return nil, s.err
	}
	return &models.ModelResponse{Model: s.name, ResponseText: s.text, PromptTokens: 10, CompletionTokens: 10, LatencyMs: s.latency}, nil
}

func newTestRunner(stubs map[string]*stubProvider) *Runner {
	return &Runner{
		PerModelTimeout: time.Second,
		NewProvider:     func(id string) models.Provider { return stubs[id] },
	}
}

func TestRunPicksBestSuccessfulModelAndPreservesOrder(t *testing.T) {
	r := newTestRunner(map[string]*stubProvider{
		"a": {name: "a", text: "go worker pool", latency: 50},
		"b": {name: "b", err: errors.New("upstream 500")},
		"c": {name: "c", text: "totally unrelated", latency: 10},
	})
	job := &queue.EvalJobPayload{JobID: "j", Prompt: "p", GroundTruth: "go worker pool", TargetModels: []string{"a", "b", "c"}, CreatedAt: time.Now()}

	rec := r.Run(context.Background(), job)

	if rec.Status != storage.StatusCompleted {
		t.Errorf("status = %s, want completed", rec.Status)
	}
	if rec.WinningModel != "a" {
		t.Errorf("winner = %q, want a", rec.WinningModel)
	}
	for i, want := range []string{"a", "b", "c"} {
		if rec.Results[i].Model != want {
			t.Errorf("results[%d] = %s, want %s", i, rec.Results[i].Model, want)
		}
	}
	if rec.Results[1].ErrorMessage == "" {
		t.Error("expected error recorded for model b")
	}
	if rec.CompletedAt == nil {
		t.Error("CompletedAt not set")
	}
}

// Regression test: a failed model scored 0, which beat the initial best score
// of -1, so an errored model could be declared the winner.
func TestRunAllFailedMarksJobFailedWithNoWinner(t *testing.T) {
	r := newTestRunner(map[string]*stubProvider{
		"a": {name: "a", err: errors.New("x")},
		"b": {name: "b", panics: true},
	})
	rec := r.Run(context.Background(), &queue.EvalJobPayload{JobID: "j", TargetModels: []string{"a", "b"}})

	if rec.Status != storage.StatusFailed {
		t.Errorf("status = %s, want failed", rec.Status)
	}
	if rec.WinningModel != "" {
		t.Errorf("winner = %q, want none", rec.WinningModel)
	}
	if rec.Results[1].ErrorMessage == "" {
		t.Error("a provider panic should be captured as an error result")
	}
}

func TestSimulatedResultsAreNeverGraded(t *testing.T) {
	r := &Runner{PerModelTimeout: time.Second, NewProvider: models.NewMockProvider}
	rec := r.Run(context.Background(), &queue.EvalJobPayload{
		JobID: "j", Prompt: "p", GroundTruth: "an unrelated reference answer", TargetModels: []string{"mock"},
	})
	res := rec.Results[0]
	if !res.Simulated || res.Graded || res.RegressionAlert {
		t.Fatalf("simulated output must not be graded or alert: %+v", res)
	}
}

func TestBuildRecordBreaksTiesByLatency(t *testing.T) {
	rec := BuildRecord(&queue.EvalJobPayload{JobID: "j"}, []storage.ModelResultRecord{
		{Model: "slow", SemanticScore: 0.9, LatencyMs: 500},
		{Model: "fast", SemanticScore: 0.9, LatencyMs: 100},
	}, time.Now())
	if rec.WinningModel != "fast" {
		t.Errorf("winner = %s, want fast", rec.WinningModel)
	}
}

func TestEstimateCostUSD(t *testing.T) {
	cheap := EstimateCostUSD("hello", []string{"gpt-4o-mini"}, 100)
	pricey := EstimateCostUSD("hello", []string{"gpt-4o-mini", "claude-3-5-sonnet"}, 100)
	if cheap <= 0 || pricey <= cheap {
		t.Errorf("unexpected estimates cheap=%f pricey=%f", cheap, pricey)
	}
	if free := EstimateCostUSD("hello", []string{"ollama:llama3"}, 1000); free != 0 {
		t.Errorf("local model estimate = %f, want 0", free)
	}
}
