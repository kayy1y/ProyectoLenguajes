// Package match contiene el worker de una partida: una goroutine dueña exclusiva
// del estado de esa partida (eventos, duplicados, ciclo de vida y temporizadores).
//
// Estados: COLLECTING -> (MATCH_FINISHED) FINISHING -> (vence la gracia) envío COMPLETE
//
//	COLLECTING -> (sin eventos durante IdleTimeout) envío INCOMPLETE
//	COLLECTING/FINISHING -> (apagado) envío inmediato
//
// Tras el envío, lo que llegue se rechaza con LATE_EVENT o MATCH_CLOSED.
package match

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"log/slog"
	"time"

	"gamestats/ingestion-go/internal/analysis"
	"gamestats/ingestion-go/internal/event"
	"gamestats/ingestion-go/internal/store"
)

type AckStatus string

const (
	Accepted     AckStatus = "ACCEPTED"
	Duplicate    AckStatus = "DUPLICATE"
	Rejected     AckStatus = "REJECTED"
	Backpressure AckStatus = "BACKPRESSURE"
)

type Ack struct {
	Status AckStatus
	Code   string
}

// Envelope viaja del handler HTTP al worker. Reply debe tener buffer 1 para
// que el worker nunca se bloquee respondiendo.
type Envelope struct {
	Event event.Event
	Reply chan Ack
}

// Closed avisa al dispatcher que la partida se envió a análisis y con qué
// código rechazar lo que llegue después.
type Closed struct {
	Key        event.Key
	RejectCode string
}

type Submitter interface {
	Submit(ctx context.Context, job analysis.Job) error
}

type Config struct {
	GracePeriod             time.Duration
	IdleTimeout             time.Duration
	MaxEvents               int
	AnalyzeIncompleteOnStop bool
}

type Worker struct {
	key   event.Key
	cfg   Config
	in    <-chan Envelope
	pool  Submitter
	done  chan<- Closed
	store *store.Store
	log   *slog.Logger
	now   func() time.Time

	// Estado confinado a la goroutine de Run.
	events     []event.Event
	hashes     map[string][32]byte
	started    bool
	finished   bool
	sent       bool
	rejectCode string
	nextSeq    int64
}

func NewWorker(key event.Key, cfg Config, in <-chan Envelope, pool Submitter, done chan<- Closed, st *store.Store, log *slog.Logger) *Worker {
	return &Worker{
		key: key, cfg: cfg, in: in, pool: pool, done: done, store: st, now: time.Now,
		log:    log.With("component", "match", "gameId", key.GameID, "matchId", key.MatchID),
		hashes: make(map[string][32]byte),
	}
}

// Run procesa eventos hasta que el dispatcher cierra el canal de entrada.
// flushCtx acota cuánto puede esperar el envío de lotes durante el apagado.
func (w *Worker) Run(flushCtx context.Context) {
	idle := time.NewTimer(w.cfg.IdleTimeout)
	defer idle.Stop()
	var grace *time.Timer
	var graceC <-chan time.Time
	idleC := idle.C
	defer func() {
		if grace != nil {
			grace.Stop()
		}
	}()

	for {
		select {
		case env, ok := <-w.in:
			if !ok {
				// Apagado (o cierre normal tras el envío).
				if !w.sent {
					switch {
					case w.finished:
						w.send(flushCtx, "COMPLETE", "MATCH_FINISHED", false)
					case w.cfg.AnalyzeIncompleteOnStop:
						w.send(flushCtx, "INCOMPLETE", "SHUTDOWN", false)
					default:
						w.log.Warn("match discarded on shutdown", "events", len(w.events))
					}
				}
				return
			}
			if w.sent {
				env.Reply <- Ack{Status: Rejected, Code: w.rejectCode}
				continue
			}
			ack := w.accept(env.Event)
			env.Reply <- ack
			if ack.Status == Accepted {
				idle.Reset(w.cfg.IdleTimeout)
				if w.finished && grace == nil {
					grace = time.NewTimer(w.cfg.GracePeriod)
					graceC = grace.C
					idleC = nil // tras MATCH_FINISHED manda el período de gracia
					w.store.SetCollecting(w.key, store.Finishing, len(w.events))
				}
			}
		case <-graceC:
			graceC = nil
			w.send(flushCtx, "COMPLETE", "MATCH_FINISHED", true)
		case <-idleC:
			idleC = nil
			w.send(flushCtx, "INCOMPLETE", "IDLE_TIMEOUT", true)
		}
	}
}

func (w *Worker) accept(e event.Event) Ack {
	h := hash(e)
	if prev, dup := w.hashes[e.EventID]; dup {
		if prev == h {
			return Ack{Status: Duplicate}
		}
		return Ack{Status: Rejected, Code: "DUPLICATE_ID_CONFLICT"}
	}
	if len(w.events) >= w.cfg.MaxEvents {
		return Ack{Status: Rejected, Code: "MATCH_TOO_LARGE"}
	}
	switch e.Type {
	case event.TypeMatchStarted:
		if w.started {
			return Ack{Status: Rejected, Code: "MATCH_ALREADY_STARTED"}
		}
		w.started = true
	case event.TypeMatchFinished:
		if w.finished {
			return Ack{Status: Rejected, Code: "MATCH_ALREADY_FINISHED"}
		}
		w.finished = true
	}
	e.Ingest = &event.Ingest{ReceivedAt: w.now().UTC().Format(event.IngestTimeLayout), Seq: w.nextSeq}
	w.nextSeq++
	w.hashes[e.EventID] = h
	w.events = append(w.events, e)
	status := store.Collecting
	if w.finished {
		status = store.Finishing
	}
	w.store.SetCollecting(w.key, status, len(w.events))
	return Ack{Status: Accepted}
}

// hash identifica el contenido del evento tal como lo envió el productor.
func hash(e event.Event) [32]byte {
	e.Ingest = nil
	b, _ := json.Marshal(e)
	return sha256.Sum256(b)
}

// send ordena y envía el lote. notify=false durante el apagado: el dispatcher ya
// no lee avisos y esperar uno podría bloquear el cierre.
func (w *Worker) send(ctx context.Context, completeness, reason string, notify bool) {
	w.sent = true
	w.rejectCode = "MATCH_CLOSED"
	if w.finished {
		w.rejectCode = "LATE_EVENT"
	}
	events := make([]event.Event, len(w.events))
	copy(events, w.events)
	event.SortTotal(events)
	w.store.SetClosed(w.key, completeness, reason)

	job := analysis.Job{Key: w.key, Request: analysis.Request{
		ContractVersion: "1",
		GameID:          w.key.GameID,
		GameVersion:     events[0].GameVersion,
		MatchID:         w.key.MatchID,
		Completeness:    completeness,
		ClosedReason:    reason,
		Events:          events,
	}}
	w.log.Info("match closed, sending to analysis", "events", len(events), "completeness", completeness, "reason", reason)
	if err := w.pool.Submit(ctx, job); err != nil {
		w.store.SetFailed(w.key, err, job.Request)
		w.log.Error("could not enqueue analysis", "err", err)
	}
	if !notify {
		return
	}
	select {
	case w.done <- Closed{Key: w.key, RejectCode: w.rejectCode}:
	case <-ctx.Done():
	}
}
