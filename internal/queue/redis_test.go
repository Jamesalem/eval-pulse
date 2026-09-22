package queue

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

const (
	testStream = "eval:jobs"
	testGroup  = "eval-workers"
	testDLQ    = "eval:dlq"
)

func newTestQueue(t *testing.T) (*Queue, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	q := NewQueue(mr.Addr(), "")
	t.Cleanup(func() { _ = q.Close() })
	if err := q.EnsureConsumerGroup(context.Background(), testStream, testGroup); err != nil {
		t.Fatal(err)
	}
	return q, mr
}

func TestEnsureConsumerGroupIsIdempotent(t *testing.T) {
	q, _ := newTestQueue(t)
	if err := q.EnsureConsumerGroup(context.Background(), testStream, testGroup); err != nil {
		t.Fatalf("second call should tolerate BUSYGROUP: %v", err)
	}
}

func TestEnqueueReadAckDelete(t *testing.T) {
	q, _ := newTestQueue(t)
	ctx := context.Background()

	job := &EvalJobPayload{JobID: "job-1", Prompt: "hi", TargetModels: []string{"gpt-4o"}, ApiKeys: map[string]string{"openai": "sk-secret"}}
	if _, err := q.EnqueueJob(ctx, testStream, job); err != nil {
		t.Fatal(err)
	}
	if job.CreatedAt.IsZero() {
		t.Error("EnqueueJob should stamp CreatedAt")
	}

	msgs, err := q.ReadJobs(ctx, testStream, testGroup, "c1", 10, 100*time.Millisecond)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("ReadJobs = %v, %v", msgs, err)
	}
	var decoded EvalJobPayload
	if err := json.Unmarshal([]byte(msgs[0].Values["payload"].(string)), &decoded); err != nil || decoded.JobID != "job-1" {
		t.Fatalf("payload round trip failed: %+v, %v", decoded, err)
	}

	if err := q.AckJob(ctx, testStream, testGroup, msgs[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := q.DeleteMessage(ctx, testStream, msgs[0].ID); err != nil {
		t.Fatal(err)
	}
	if n, _ := q.Client().XLen(ctx, testStream).Result(); n != 0 {
		t.Errorf("stream should be empty after delete (credentials purged), len = %d", n)
	}
}

func TestReadJobsEmptyStream(t *testing.T) {
	q, _ := newTestQueue(t)
	msgs, err := q.ReadJobs(context.Background(), testStream, testGroup, "c1", 10, 50*time.Millisecond)
	if err != nil || len(msgs) != 0 {
		t.Fatalf("expected no messages and no error, got %v, %v", msgs, err)
	}
}

// Regression test: the DLQ used to store fmt.Sprintf("%v", msg.Values), which
// copied users' BYOK API keys into a long-lived stream.
func TestSendToDLQRedactsCredentials(t *testing.T) {
	q, _ := newTestQueue(t)
	ctx := context.Background()

	payload, _ := json.Marshal(EvalJobPayload{JobID: "job-9", Prompt: "p", ApiKeys: map[string]string{"anthropic": "sk-ant-SECRET"}})
	msg := redis.XMessage{ID: "1-0", Values: map[string]interface{}{"job_id": "job-9", "payload": string(payload)}}
	if err := q.SendToDLQ(ctx, testDLQ, msg, "test failure"); err != nil {
		t.Fatal(err)
	}

	entries, err := q.Client().XRange(ctx, testDLQ, "-", "+").Result()
	if err != nil || len(entries) != 1 {
		t.Fatalf("XRange = %v, %v", entries, err)
	}
	for k, v := range entries[0].Values {
		if strings.Contains(v.(string), "SECRET") {
			t.Fatalf("DLQ field %q leaked a credential: %v", k, v)
		}
	}
	if entries[0].Values["job_id"] != "job-9" || entries[0].Values["failure_reason"] != "test failure" {
		t.Errorf("DLQ entry missing diagnostics: %v", entries[0].Values)
	}
}

func TestRedactPayloadDropsUnparseableInput(t *testing.T) {
	if got := RedactPayload("not json sk-SECRET"); strings.Contains(got, "SECRET") {
		t.Fatalf("unparseable payload leaked: %q", got)
	}
	if got := RedactPayload(42); got != "" {
		t.Fatalf("non-string payload = %q, want empty", got)
	}
}
