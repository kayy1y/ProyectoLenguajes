// Command ingestion es el servicio Go de GameStats: recibe eventos, los agrupa por
// partida de forma concurrente y envía cada partida cerrada al servicio Scala.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gamestats/ingestion-go/internal/analysis"
	"gamestats/ingestion-go/internal/config"
	"gamestats/ingestion-go/internal/dispatcher"
	"gamestats/ingestion-go/internal/httpapi"
	"gamestats/ingestion-go/internal/match"
	"gamestats/ingestion-go/internal/obs"
	"gamestats/ingestion-go/internal/store"
	"gamestats/ingestion-go/internal/validation"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "consulta /healthz y termina (para el HEALTHCHECK de Docker)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *healthcheck {
		os.Exit(checkHealth(cfg.HTTPAddr))
	}
	var level slog.Level
	_ = level.UnmarshalText([]byte(cfg.LogLevel))
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	if err := run(cfg, log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	validator, err := validation.Load(cfg.SchemasDir)
	if err != nil {
		return err
	}
	st, err := store.New(cfg.ResultsDir)
	if err != nil {
		return fmt.Errorf("load results: %w", err)
	}
	pool := analysis.NewPool(analysis.PoolConfig{
		Workers: cfg.AnalyzerWorkers, QueueSize: 64, MaxRetries: cfg.AnalyzeMaxRetries, BackoffBase: cfg.AnalyzeBackoffBase,
	}, analysis.NewClient(cfg.AnalyticsURL, cfg.AnalyzeTimeout), st, log)
	disp := dispatcher.New(dispatcher.Config{
		QueueSize: cfg.IngestQueueSize, MatchQueueSize: cfg.MatchQueueSize, TombstoneTTL: cfg.TombstoneTTL,
		Match: match.Config{
			GracePeriod: cfg.MatchGracePeriod, IdleTimeout: cfg.MatchIdleTimeout,
			MaxEvents: cfg.MaxEventsPerMatch, AnalyzeIncompleteOnStop: cfg.ShutdownAnalyzeIncomplete,
		},
	}, pool, st, log)

	// Contextos separados para apagar en orden: ingestión primero, análisis al final.
	ingestCtx, stopIngest := context.WithCancel(context.Background())
	defer stopIngest()
	analysisCtx, stopAnalysis := context.WithCancel(context.Background())
	defer stopAnalysis()

	dispDone := make(chan struct{})
	go func() { disp.Run(ingestCtx, analysisCtx); close(dispDone) }()
	poolDone := make(chan struct{})
	go func() { pool.Run(analysisCtx); close(poolDone) }()

	srv := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.New(httpapi.Deps{
			Validator: validator, Dispatcher: disp, Pool: pool, Store: st,
			Counters: &obs.Counters{}, Rejections: obs.NewRejections(100),
			EnqueueTimeout: cfg.EnqueueTimeout, APIKey: cfg.APIKey, Log: log,
		}).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()
	log.Info("ingestion listening", "addr", cfg.HTTPAddr, "analyticsUrl", cfg.AnalyticsURL,
		"games", validator.Games(), "analyzerWorkers", cfg.AnalyzerWorkers,
		"gracePeriod", cfg.MatchGracePeriod.String(), "idleTimeout", cfg.MatchIdleTimeout.String(),
		"resultsDir", cfg.ResultsDir, "apiKey", cfg.APIKey != "")

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("shutdown requested")
	}

	// 1) dejar de aceptar peticiones, 2) cerrar partidas (los workers envían sus
	// lotes), 3) vaciar la cola de análisis dentro del plazo.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Warn("http shutdown", "err", err)
	}
	stopIngest()
	waitOrCancel(dispDone, shutdownCtx, stopAnalysis)
	pool.Drain()
	waitOrCancel(poolDone, shutdownCtx, stopAnalysis)
	log.Info("shutdown complete")
	return nil
}

// waitOrCancel espera done; si vence el plazo, cancela el análisis y vuelve a esperar.
func waitOrCancel(done <-chan struct{}, deadline context.Context, cancel context.CancelFunc) {
	select {
	case <-done:
	case <-deadline.Done():
		cancel()
		<-done
	}
}

func checkHealth(addr string) int {
	port := addr[strings.LastIndex(addr, ":")+1:]
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/healthz", port))
	if err != nil {
		return 1
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
