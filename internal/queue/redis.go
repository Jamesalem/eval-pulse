package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// EvalJobPayload represents a distributed evaluation task placed on the stream.
type EvalJobPayload struct {
	JobID        string            `json:"job_id"`
	SuiteID      string            `json:"suite_id,omitempty"`
	Prompt       string            `json:"prompt"`
	GroundTruth  string            `json:"ground_truth,omitempty"`
	TargetModels []string          `json:"target_models"`
	MaxTokens    int               `json:"max_tokens"`
	BudgetCapUSD float64           `json:"budget_cap_usd"`
	ApiKeys      map[string]string `json:"api_keys,omitempty"` // Cline BYOK ephemeral credentials
	CreatedAt    time.Time         `json:"created_at"`
}

// Queue manages Redis Stream operations for producer and consumer groups.
type Queue struct {
	client *redis.Client
}

// NewQueue instantiates a Redis stream client.
func NewQueue(redisURL, password string) *Queue {
	client := redis.NewClient(&redis.Options{
		Addr:     redisURL,
		Password: password,
		DB:       0,
	})
	return &Queue{client: client}
}

// Ping checks Redis connectivity.
func (q *Queue) Ping(ctx context.Context) error {
	return q.client.Ping(ctx).Err()
}

// Close gracefully closes the Redis client.
func (q *Queue) Close() error {
	return q.client.Close()
}

// EnsureConsumerGroup creates the consumer group if it does not already exist.
func (q *Queue) EnsureConsumerGroup(ctx context.Context, stream, group string) error {
	err := q.client.XGroupCreateMkStream(ctx, stream, group, "$").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return fmt.Errorf("failed to create consumer group %s on %s: %w", group, stream, err)
	}
	return nil
}

// EnqueueJob publishes an evaluation job payload onto the Redis Stream.
func (q *Queue) EnqueueJob(ctx context.Context, stream string, job *EvalJobPayload) (string, error) {
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now().UTC()
	}

	data, err := json.Marshal(job)
	if err != nil {
		return "", fmt.Errorf("failed to marshal job payload: %w", err)
	}

	msgID, err := q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		MaxLen: 50000,
		Approx: true,
		Values: map[string]interface{}{
			"job_id":     job.JobID,
			"payload":    string(data),
			"created_at": job.CreatedAt.Format(time.RFC3339),
		},
	}).Result()
	if err != nil {
		return "", fmt.Errorf("failed to publish to stream %s: %w", stream, err)
	}

	return msgID, nil
}

// ReadJobs pulls new or pending messages from the stream using consumer groups.
func (q *Queue) ReadJobs(ctx context.Context, stream, group, consumer string, count int64, block time.Duration) ([]redis.XMessage, error) {
	streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, ">"},
		Count:    count,
		Block:    block,
	}).Result()

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}

	if len(streams) == 0 {
		return nil, nil
	}

	return streams[0].Messages, nil
}

// AckJob acknowledges processing completion for a stream message.
func (q *Queue) AckJob(ctx context.Context, stream, group string, messageIDs ...string) error {
	return q.client.XAck(ctx, stream, group, messageIDs...).Err()
}

// AutoclaimAbandoned retrieves tasks from dead or crashed workers idle longer than minIdle.
func (q *Queue) AutoclaimAbandoned(ctx context.Context, stream, group, consumer string, minIdle time.Duration) ([]redis.XMessage, error) {
	msgs, _, err := q.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   stream,
		Group:    group,
		Consumer: consumer,
		MinIdle:  minIdle,
		Start:    "0-0",
		Count:    10,
	}).Result()
	return msgs, err
}

// SendToDLQ forwards an unprocessable message to the dead-letter stream.
func (q *Queue) SendToDLQ(ctx context.Context, dlqStream string, msg redis.XMessage, reason string) error {
	return q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: dlqStream,
		MaxLen: 10000,
		Approx: true,
		Values: map[string]interface{}{
			"original_id":    msg.ID,
			"original_values": fmt.Sprintf("%v", msg.Values),
			"failure_reason": reason,
			"failed_at":      time.Now().UTC().Format(time.RFC3339),
		},
	}).Err()
}
