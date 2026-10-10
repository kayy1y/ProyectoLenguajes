// Package config lee la configuración del servicio desde variables de entorno.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr        string
	AnalyticsURL    string
	AnalyzerWorkers int
	IngestQueueSize int
	MatchQueueSize  int
	EnqueueTimeout  time.Duration
	AnalyzeTimeout  time.Duration
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	var errs []error
	c := Config{
		HTTPAddr:        str("HTTP_ADDR", ":8080"),
		AnalyticsURL:    str("ANALYTICS_URL", "http://localhost:8081"),
		AnalyzerWorkers: num("ANALYZER_WORKERS", 4, &errs),
		IngestQueueSize: num("INGEST_QUEUE_SIZE", 1024, &errs),
		MatchQueueSize:  num("MATCH_QUEUE_SIZE", 256, &errs),
		EnqueueTimeout:  dur("INGEST_ENQUEUE_TIMEOUT", 2*time.Second, &errs),
		AnalyzeTimeout:  dur("ANALYZE_TIMEOUT", 5*time.Second, &errs),
		ShutdownTimeout: dur("SHUTDOWN_TIMEOUT", 10*time.Second, &errs),
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
