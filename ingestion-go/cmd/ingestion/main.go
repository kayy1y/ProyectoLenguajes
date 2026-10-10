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
	"gamestats/ingestion-go/internal/store"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "consulta /healthz y termina (para el HEALTHCHECK de Docker)")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}
	if *healthcheck {
		os.Exit(checkHealth(cfg.HTTPAddr))
	}
	if err := run(cfg, log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st := store.New()
	jobs := make(chan analysis.Job, 64)
	disp := dispatcher.New(cfg.IngestQueueSize, cfg.MatchQueueSize, jobs, st, log)
	pool := analysis.NewPool(analysis.NewClient(cfg.AnalyticsURL, cfg.AnalyzeTimeout), st, cfg.AnalyzerWorkers, log)

	// La ingestión y el análisis tienen contextos separados para apagarlos en orden.
	ingestCtx, stopIngest := context.WithCancel(context.Background())
	defer stopIngest()
	analysisCtx, stopAnalysis := context.WithCancel(context.Background())
	defer stopAnalysis()

	// El dispatcher espera a sus workers antes de terminar; como ellos son los únicos
	// emisores de jobs, al volver Run ya es seguro cerrar el canal.
	dispDone := make(chan struct{})
	go func() { disp.Run(ingestCtx); close(jobs); close(dispDone) }()
	poolDone := make(chan struct{})
	go func() { pool.Run(analysisCtx, jobs); close(poolDone) }()

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.New(disp, st, cfg.EnqueueTimeout, log).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()
	log.Info("ingestion listening", "addr", cfg.HTTPAddr, "analyticsUrl", cfg.AnalyticsURL, "analyzerWorkers", cfg.AnalyzerWorkers)

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		log.Info("shutdown requested")
	}

	// 1) dejar de aceptar peticiones, 2) cerrar workers, 3) dar plazo al análisis en curso.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Warn("http shutdown", "err", err)
	}
	stopIngest()
	<-dispDone
	select {
	case <-poolDone:
	case <-shutdownCtx.Done():
		stopAnalysis()
		<-poolDone
	}
	log.Info("shutdown complete")
	return nil
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
