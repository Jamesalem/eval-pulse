package config

import (
	"log"
	"os"
	"strconv"
	"strings"
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
	// ReclaimIdleSec is how long a delivered-but-unacknowledged message may sit
	// before another worker reclaims it (crashed consumer recovery).
	ReclaimIdleSec int
	// CORSAllowedOrigins lists origins allowed to call the API; "*" allows any.
	CORSAllowedOrigins []string
	// SeedDemoData populates an empty store with sample benchmarks at startup.
	SeedDemoData bool
}

// Load reads configuration from environment variables with sensible production defaults.
func Load() *Config {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "eval-worker-default"
	}

	cfg := &Config{
		RedisURL:           getEnv("REDIS_URL", "localhost:6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		APIPort:            getEnv("PORT", "8080"),
		WorkerConcurrency:  getEnvAsInt("WORKER_CONCURRENCY", 25),
		StreamKey:          getEnv("STREAM_KEY", "eval:jobs"),
		DLQStreamKey:       getEnv("DLQ_STREAM_KEY", "eval:dlq"),
		GroupName:          getEnv("GROUP_NAME", "eval-workers"),
		ConsumerName:       getEnv("CONSUMER_NAME", hostname),
		BudgetCapUSD:       getEnvAsFloat("BUDGET_CAP_USD", 10.0),
		DefaultTimeoutSec:  getEnvAsInt("DEFAULT_TIMEOUT_SEC", 30),
		ReclaimIdleSec:     getEnvAsInt("RECLAIM_IDLE_SEC", 300),
		CORSAllowedOrigins: getEnvAsList("CORS_ALLOWED_ORIGINS", []string{"*"}),
		SeedDemoData:       getEnvAsBool("SEED_DEMO_DATA", true),
	}
	cfg.normalize()
	return cfg
}

// normalize clamps values that would otherwise deadlock or misbehave at runtime.
func (c *Config) normalize() {
	if c.WorkerConcurrency < 1 {
		log.Printf("[config] WORKER_CONCURRENCY=%d is invalid; using 1", c.WorkerConcurrency)
		c.WorkerConcurrency = 1
	}
	if c.DefaultTimeoutSec < 1 {
		c.DefaultTimeoutSec = 30
	}
	if c.BudgetCapUSD <= 0 {
		c.BudgetCapUSD = 10.0
	}
	// A reclaim window shorter than a job's worst-case runtime would steal
	// jobs that are still being processed.
	if min := c.DefaultTimeoutSec * 2; c.ReclaimIdleSec < min {
		c.ReclaimIdleSec = min
	}
}

func getEnv(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
		log.Printf("[config] ignoring invalid integer %s=%q", key, val)
	}
	return fallback
}

func getEnvAsFloat(key string, fallback float64) float64 {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
		log.Printf("[config] ignoring invalid number %s=%q", key, val)
	}
	return fallback
}

func getEnvAsBool(key string, fallback bool) bool {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
		log.Printf("[config] ignoring invalid boolean %s=%q", key, val)
	}
	return fallback
}

func getEnvAsList(key string, fallback []string) []string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	var out []string
	for _, part := range strings.Split(val, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}
