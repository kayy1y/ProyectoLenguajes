// Package analysis envía lotes de partidas cerradas al servicio Scala.
//
// Go no interpreta la respuesta: la guarda como bytes opacos en el store.
package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"gamestats/ingestion-go/internal/event"
	"gamestats/ingestion-go/internal/store"
)

// Request es el contrato analyze-request.v1 (contracts/schemas/analyze-request.v1.schema.json).
type Request struct {
	ContractVersion string        `json:"contractVersion"`
	BatchID         string        `json:"batchId"`
	GameID          string        `json:"gameId"`
	GameVersion     string        `json:"gameVersion"`
	MatchID         string        `json:"matchId"`
	Completeness    string        `json:"completeness"`
	ClosedReason    string        `json:"closedReason"`
	Events          []event.Event `json:"events"`
}

type Job struct {
	Key     event.Key
	Request Request
}

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{baseURL: baseURL, http: &http.Client{Timeout: timeout}}
}

// Analyze devuelve el cuerpo de la respuesta 200 de Scala.
func (c *Client) Analyze(ctx context.Context, req Request) (json.RawMessage, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/analyze", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Contract-Version", "1")
	httpReq.Header.Set("X-Request-Id", req.BatchID)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call analytics: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("read analytics response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("analytics returned %d: %s", resp.StatusCode, bytes.TrimSpace(respBody))
	}
	if !json.Valid(respBody) {
		return nil, fmt.Errorf("analytics returned invalid JSON")
	}
	return respBody, nil
}

// Pool consume trabajos con un número fijo de goroutines, separado de la ingestión:
// una llamada lenta a Scala no frena la recepción de eventos de otras partidas.
type Pool struct {
	client  *Client
	store   *store.Store
	workers int
	log     *slog.Logger
}

func NewPool(client *Client, st *store.Store, workers int, log *slog.Logger) *Pool {
	return &Pool{client: client, store: st, workers: workers, log: log}
}

// Run bloquea hasta que jobs se cierra (y se vacía) o ctx se cancela.
func (p *Pool) Run(ctx context.Context, jobs <-chan Job) {
	var wg sync.WaitGroup
	for i := 0; i < p.workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case job, ok := <-jobs:
					if !ok {
						return
					}
					p.process(ctx, id, job)
				case <-ctx.Done():
					return
				}
			}
		}(i)
	}
	wg.Wait()
}

func (p *Pool) process(ctx context.Context, worker int, job Job) {
	log := p.log.With("component", "analysis", "worker", worker, "gameId", job.Key.GameID, "matchId", job.Key.MatchID)
	p.store.SetAnalyzing(job.Key)
	start := time.Now()
	result, err := p.client.Analyze(ctx, job.Request)
	if err != nil {
		p.store.SetFailed(job.Key, err)
		log.Error("analysis failed", "err", err)
		return
	}
	p.store.SetResult(job.Key, result)
	log.Info("match analyzed", "events", len(job.Request.Events), "took", time.Since(start).String())
}
