// Package match contiene el worker de una partida: una goroutine dueña exclusiva
// del estado de esa partida (eventos, duplicados, ciclo de vida).
package match

import (
	"context"
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

type Worker struct {
	key   event.Key
	in    <-chan Envelope
	jobs  chan<- analysis.Job
	done  chan<- event.Key
	store *store.Store
	log   *slog.Logger
	now   func() time.Time

	// Estado confinado a la goroutine de Run.
	events   []event.Event
	seen     map[string]struct{}
	started  bool
	finished bool
	nextSeq  int64
}

func NewWorker(key event.Key, in <-chan Envelope, jobs chan<- analysis.Job, done chan<- event.Key, st *store.Store, log *slog.Logger) *Worker {
	return &Worker{
		key: key, in: in, jobs: jobs, done: done, store: st, now: time.Now,
		log:  log.With("component", "match", "gameId", key.GameID, "matchId", key.MatchID),
		seen: make(map[string]struct{}),
	}
}

// Run procesa eventos hasta que el dispatcher cierra el canal de entrada.
// Tras enviar el lote a análisis sigue drenando el canal y rechaza lo que llegue,
// para que ningún handler se quede esperando respuesta.
func (w *Worker) Run(ctx context.Context) {
	for env := range w.in {
		if w.finished {
			env.Reply <- Ack{Status: Rejected, Code: "LATE_EVENT"}
			continue
		}
		env.Reply <- w.accept(env.Event)
		if w.finished {
			w.dispatch(ctx)
		}
	}
}

func (w *Worker) accept(e event.Event) Ack {
	if _, dup := w.seen[e.EventID]; dup {
		return Ack{Status: Duplicate}
	}
	switch e.Type {
	case event.TypeMatchStarted:
		if w.started {
			return Ack{Status: Rejected, Code: "MATCH_ALREADY_STARTED"}
		}
		w.started = true
	case event.TypeMatchFinished:
		w.finished = true
	}
	e.Ingest = &event.Ingest{ReceivedAt: w.now().UTC().Format(event.IngestTimeLayout), Seq: w.nextSeq}
	w.nextSeq++
	w.seen[e.EventID] = struct{}{}
	w.events = append(w.events, e)
	w.store.SetCollecting(w.key, len(w.events))
	return Ack{Status: Accepted}
}

func (w *Worker) dispatch(ctx context.Context) {
	events := make([]event.Event, len(w.events))
	copy(events, w.events)
	event.SortTotal(events)
	first := events[0]
	job := analysis.Job{Key: w.key, Request: analysis.Request{
		ContractVersion: "1",
		BatchID:         w.key.String() + ":1",
		GameID:          w.key.GameID,
		GameVersion:     first.GameVersion,
		MatchID:         w.key.MatchID,
		Completeness:    "COMPLETE",
		ClosedReason:    "MATCH_FINISHED",
		Events:          events,
	}}
	w.log.Info("match closed, sending to analysis", "events", len(events))
	select {
	case w.jobs <- job:
	case <-ctx.Done():
		return
	}
	select {
	case w.done <- w.key:
	case <-ctx.Done():
	}
}
