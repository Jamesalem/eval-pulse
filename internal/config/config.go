package config

import (
	"os"
	"strconv"
)

// Config represents runtime configuration parameters for EvalPulse services.
type Config struct {
	RedisURL          string
	RedisPassword     string
	APIPort           string
	WorkerConcurrency int
	StreamKey         string
	DLQStreamKey      string
	GroupName         string
	ConsumerName      string
	BudgetCapUSD      float64
	DefaultTimeoutSec int
}

// Load reads configuration from environment variables with sensible production defaults.
func Load() *Config {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "eval-worker-default"
	}

	return &Config{
		RedisURL:          getEnv("REDIS_URL", "localhost:6379"),
		RedisPassword:     getEnv("REDIS_PASSWORD", ""),
		APIPort:           getEnv("PORT", "8080"),
		WorkerConcurrency: getEnvAsInt("WORKER_CONCURRENCY", 25),
		StreamKey:         getEnv("STREAM_KEY", "eval:jobs"),
		DLQStreamKey:      getEnv("DLQ_STREAM_KEY", "eval:dlq"),
		GroupName:         getEnv("GROUP_NAME", "eval-workers"),
		ConsumerName:      getEnv("CONSUMER_NAME", hostname),
		BudgetCapUSD:      getEnvAsFloat("BUDGET_CAP_USD", 10.0),
		DefaultTimeoutSec: getEnvAsInt("DEFAULT_TIMEOUT_SEC", 30),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvAsFloat(key string, fallback float64) float64 {
	if val := os.Getenv(key); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return fallback
}
