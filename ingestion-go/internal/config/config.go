// Package config lee la configuración del servicio desde variables de entorno.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr                  string
	AnalyticsURL              string
	SchemasDir                string
	ResultsDir                string
	APIKey                    string
	LogLevel                  string
	AnalyzerWorkers           int
	IngestQueueSize           int
	MatchQueueSize            int
	MaxEventsPerMatch         int
	AnalyzeMaxRetries         int
	EnqueueTimeout            time.Duration
	MatchGracePeriod          time.Duration
	MatchIdleTimeout          time.Duration
	TombstoneTTL              time.Duration
	AnalyzeTimeout            time.Duration
	AnalyzeBackoffBase        time.Duration
	ShutdownTimeout           time.Duration
	ShutdownAnalyzeIncomplete bool
}

func Load() (Config, error) {
	var errs []error
	c := Config{
		HTTPAddr:        str("HTTP_ADDR", ":8080"),
		AnalyticsURL:    str("ANALYTICS_URL", "http://localhost:8081"),
		SchemasDir:      str("SCHEMAS_DIR", "../contracts/schemas"),
		ResultsDir:      str("RESULTS_DIR", ""),
		APIKey:          str("INGEST_API_KEY", ""),
		LogLevel:        str("LOG_LEVEL", "info"),
		AnalyzerWorkers: num("ANALYZER_WORKERS", 4, &errs),
		IngestQueueSize: num("INGEST_QUEUE_SIZE", 1024, &errs),
		MatchQueueSize:  num("MATCH_QUEUE_SIZE", 256, &errs),

		MaxEventsPerMatch:         num("MAX_EVENTS_PER_MATCH", 10000, &errs),
		AnalyzeMaxRetries:         nonNegative("ANALYZE_MAX_RETRIES", 3, &errs),
		EnqueueTimeout:            dur("INGEST_ENQUEUE_TIMEOUT", 2*time.Second, &errs),
		MatchGracePeriod:          dur("MATCH_GRACE_PERIOD", 2*time.Second, &errs),
		MatchIdleTimeout:          dur("MATCH_IDLE_TIMEOUT", 60*time.Second, &errs),
		TombstoneTTL:              dur("TOMBSTONE_TTL", time.Hour, &errs),
		AnalyzeTimeout:            dur("ANALYZE_TIMEOUT", 5*time.Second, &errs),
		AnalyzeBackoffBase:        dur("ANALYZE_BACKOFF_BASE", 500*time.Millisecond, &errs),
		ShutdownTimeout:           dur("SHUTDOWN_TIMEOUT", 10*time.Second, &errs),
		ShutdownAnalyzeIncomplete: str("SHUTDOWN_ANALYZE_INCOMPLETE", "true") == "true",
	}
	if c.LogLevel != "debug" && c.LogLevel != "info" && c.LogLevel != "warn" && c.LogLevel != "error" {
		errs = append(errs, fmt.Errorf("LOG_LEVEL must be debug, info, warn or error, got %q", c.LogLevel))
	}
	if len(errs) > 0 {
		return Config{}, fmt.Errorf("invalid configuration: %v", errs)
	}
	return c, nil
}

func str(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func num(key string, def int, errs *[]error) int {
	v := str(key, "")
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		*errs = append(*errs, fmt.Errorf("%s must be a positive integer, got %q", key, v))
		return def
	}
	return n
}

func dur(key string, def time.Duration, errs *[]error) time.Duration {
	v := str(key, "")
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		*errs = append(*errs, fmt.Errorf("%s must be a positive duration, got %q", key, v))
		return def
	}
	return d
}

func nonNegative(key string, def int, errs *[]error) int {
	v := str(key, "")
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		*errs = append(*errs, fmt.Errorf("%s must be a non-negative integer, got %q", key, v))
		return def
	}
	return n
}
