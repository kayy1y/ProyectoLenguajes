// Package analysis envía lotes de partidas cerradas al servicio Scala.
//
// Go no interpreta la respuesta: la guarda como bytes opacos en el store.
package analysis

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
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

// StatusError es una respuesta no-200 de Scala.
type StatusError struct {
	Code int
	Body string
}

func (e *StatusError) Error() string { return fmt.Sprintf("analytics returned %d: %s", e.Code, e.Body) }

// Retryable: errores de red, timeouts y 5xx/429. Un 4xx es un error de contrato
// y reintentar no lo arregla.
func Retryable(err error) bool {
	var se *StatusError
	if errors.As(err, &se) {
		return se.Code >= 500 || se.Code == http.StatusTooManyRequests
	}
	return true
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
		return nil, &StatusError{Code: 0, Body: "encode request: " + err.Error()}
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
		return nil, &StatusError{Code: resp.StatusCode, Body: string(bytes.TrimSpace(respBody))}
	}
	if !json.Valid(respBody) {
		return nil, &StatusError{Code: http.StatusBadGateway, Body: "analytics returned invalid JSON"}
	}
	return respBody, nil
}

type Analyzer interface {
	Analyze(ctx context.Context, req Request) (json.RawMessage, error)
}

type PoolConfig struct {
	Workers     int
	QueueSize   int
	MaxRetries  int
	BackoffBase time.Duration
}

// Pool consume trabajos con un número fijo de goroutines, separado de la ingestión:
// una llamada lenta a Scala no frena la recepción de eventos de otras partidas.
type Pool struct {
	cfg      PoolConfig
	client   Analyzer
	store    *store.Store
	log      *slog.Logger
	queue    chan Job
	draining chan struct{}
	once     sync.Once
}

var ErrDraining = errors.New("analysis pool is shutting down")

func NewPool(cfg PoolConfig, client Analyzer, st *store.Store, log *slog.Logger) *Pool {
	return &Pool{
		cfg: cfg, client: client, store: st, log: log.With("component", "analysis"),
		queue: make(chan Job, cfg.QueueSize), draining: make(chan struct{}),
	}
}

// Submit encola un trabajo; bloquea si la cola está llena hasta que ctx venza.
func (p *Pool) Submit(ctx context.Context, job Job) error {
	select {
	case <-p.draining:
		return ErrDraining
	default:
	}
	select {
	case p.queue <- job:
		return nil
	case <-p.draining:
		return ErrDraining
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Reanalyze reenvía el lote de una partida FAILED.
func (p *Pool) Reanalyze(ctx context.Context, k event.Key) error {
	pending, ok := p.store.TakeFailed(k)
	if !ok {
		return errors.New("match is not in FAILED state")
	}
	req := pending.(Request)
	if err := p.Submit(ctx, Job{Key: k, Request: req}); err != nil {
		p.store.SetFailed(k, err, req)
		return err
	}
	return nil
}

// Drain deja de aceptar trabajos; Run termina al vaciar la cola.
func (p *Pool) Drain() { p.once.Do(func() { close(p.draining) }) }

// Run bloquea hasta que la cola se vacía tras Drain o ctx se cancela.
func (p *Pool) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < p.cfg.Workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case job := <-p.queue:
					p.process(ctx, id, job)
				case <-p.draining:
					for {
						select {
						case job := <-p.queue:
							p.process(ctx, id, job)
						default:
							return
						}
					}
				case <-ctx.Done():
					return
				}
			}
		}(i)
	}
	wg.Wait()
}

func (p *Pool) process(ctx context.Context, worker int, job Job) {
	log := p.log.With("worker", worker, "gameId", job.Key.GameID, "matchId", job.Key.MatchID)
	var err error
attempts:
	for retry := 0; retry <= p.cfg.MaxRetries; retry++ {
		attempt := p.store.StartAttempt(job.Key, retry > 0)
		job.Request.BatchID = fmt.Sprintf("%s:%d", job.Key, attempt)
		start := time.Now()
		var result json.RawMessage
		result, err = p.client.Analyze(ctx, job.Request)
		if err == nil {
			if perr := p.store.SetResult(job.Key, result); perr != nil {
				log.Error("persist result", "err", perr)
			}
			log.Info("match analyzed", "attempt", attempt, "events", len(job.Request.Events), "took", time.Since(start).String())
			return
		}
		if !Retryable(err) || retry == p.cfg.MaxRetries {
			break attempts
		}
		wait := p.backoff(retry)
		log.Warn("analysis failed, retrying", "attempt", attempt, "err", err, "wait", wait.String())
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			err = ctx.Err()
			break attempts
		}
	}
	p.store.SetFailed(job.Key, err, job.Request)
	log.Error("analysis failed", "err", err, "retryable", Retryable(err))
}

// backoff: base * 2^n más un jitter de hasta base/2.
func (p *Pool) backoff(n int) time.Duration {
	d := p.cfg.BackoffBase << n
	if half := int64(p.cfg.BackoffBase / 2); half > 0 {
		d += time.Duration(rand.Int64N(half))
	}
	return d
}
