// Package dispatcher enruta cada evento al worker de su partida.
//
// Una única goroutine (Run) es dueña del mapa de partidas activas y de las
// partidas cerradas: nadie más lo toca, por eso no lleva mutex.
package dispatcher

import (
	"context"
	"log/slog"
	"sync"

	"gamestats/ingestion-go/internal/analysis"
	"gamestats/ingestion-go/internal/event"
	"gamestats/ingestion-go/internal/match"
	"gamestats/ingestion-go/internal/store"
)

type Dispatcher struct {
	in             chan match.Envelope
	done           chan event.Key
	jobs           chan<- analysis.Job
	matchQueueSize int
	store          *store.Store
	log            *slog.Logger

	// Confinados a la goroutine de Run.
	workers map[event.Key]chan match.Envelope
	closed  map[event.Key]struct{}
}

func New(queueSize, matchQueueSize int, jobs chan<- analysis.Job, st *store.Store, log *slog.Logger) *Dispatcher {
	return &Dispatcher{
		in:             make(chan match.Envelope, queueSize),
		done:           make(chan event.Key, 64),
		jobs:           jobs,
		matchQueueSize: matchQueueSize,
		store:          st,
		log:            log.With("component", "dispatcher"),
		workers:        make(map[event.Key]chan match.Envelope),
		closed:         make(map[event.Key]struct{}),
	}
}

// Submit entrega un evento al dispatcher o falla si ctx vence antes (backpressure).
func (d *Dispatcher) Submit(ctx context.Context, env match.Envelope) error {
	select {
	case d.in <- env:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Run bloquea hasta que ctx se cancela y todos los workers terminaron.
func (d *Dispatcher) Run(ctx context.Context) {
	var wg sync.WaitGroup
	defer func() {
		for _, ch := range d.workers {
			close(ch)
		}
		wg.Wait()
	}()
	for {
		select {
		case env := <-d.in:
			d.route(ctx, env, &wg)
		case k := <-d.done:
			// El worker ya envió su lote: se cierra su canal (solo el emisor cierra)
			// y la partida queda marcada para rechazar eventos posteriores.
			if ch, ok := d.workers[k]; ok {
				close(ch)
				delete(d.workers, k)
			}
			d.closed[k] = struct{}{}
		case <-ctx.Done():
			return
		}
	}
}

func (d *Dispatcher) route(ctx context.Context, env match.Envelope, wg *sync.WaitGroup) {
	k := env.Event.Key()
	if _, ok := d.closed[k]; ok {
		env.Reply <- match.Ack{Status: match.Rejected, Code: "MATCH_CLOSED"}
		return
	}
	ch, ok := d.workers[k]
	if !ok {
		ch = make(chan match.Envelope, d.matchQueueSize)
		d.workers[k] = ch
		w := match.NewWorker(k, ch, d.jobs, d.done, d.store, d.log)
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.Run(ctx)
		}()
		d.log.Info("match worker started", "gameId", k.GameID, "matchId", k.MatchID, "activeMatches", len(d.workers))
	}
	// Envío no bloqueante: una partida saturada no frena a las demás.
	select {
	case ch <- env:
	default:
		env.Reply <- match.Ack{Status: match.Backpressure, Code: "BACKPRESSURE"}
	}
}
