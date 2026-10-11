// Package dispatcher enruta cada evento al worker de su partida.
//
// Una única goroutine (Run) es dueña del mapa de partidas activas y de las
// partidas cerradas: nadie más lo toca, por eso no lleva mutex.
package dispatcher

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"gamestats/ingestion-go/internal/event"
	"gamestats/ingestion-go/internal/match"
	"gamestats/ingestion-go/internal/store"
)

type Config struct {
	QueueSize      int
	MatchQueueSize int
	TombstoneTTL   time.Duration
	Match          match.Config
}

type tombstone struct {
	code string
	at   time.Time
}

type Dispatcher struct {
	cfg   Config
	in    chan match.Envelope
	done  chan match.Closed
	pool  match.Submitter
	store *store.Store
	log   *slog.Logger

	// Confinados a la goroutine de Run.
	workers map[event.Key]chan match.Envelope
	closed  map[event.Key]tombstone
}

func New(cfg Config, pool match.Submitter, st *store.Store, log *slog.Logger) *Dispatcher {
	return &Dispatcher{
		cfg:     cfg,
		in:      make(chan match.Envelope, cfg.QueueSize),
		done:    make(chan match.Closed, 64),
		pool:    pool,
		store:   st,
		log:     log.With("component", "dispatcher"),
		workers: make(map[event.Key]chan match.Envelope),
		closed:  make(map[event.Key]tombstone),
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
// flushCtx se entrega a los workers para acotar el envío de lotes al apagar.
func (d *Dispatcher) Run(ctx, flushCtx context.Context) {
	var wg sync.WaitGroup
	sweep := time.NewTicker(d.cfg.TombstoneTTL / 4)
	defer sweep.Stop()
	defer func() {
		// Cerrar el canal es la señal de apagado; cada worker vacía su partida y sale.
		for _, ch := range d.workers {
			close(ch)
		}
		wg.Wait()
	}()
	for {
		select {
		case env := <-d.in:
			d.route(flushCtx, env, &wg)
		case c := <-d.done:
			// El worker ya envió su lote: se cierra su canal (solo el emisor cierra)
			// y la partida queda marcada para rechazar eventos posteriores.
			if ch, ok := d.workers[c.Key]; ok {
				close(ch)
				delete(d.workers, c.Key)
			}
			d.closed[c.Key] = tombstone{code: c.RejectCode, at: time.Now()}
		case now := <-sweep.C:
			for k, t := range d.closed {
				if now.Sub(t.at) > d.cfg.TombstoneTTL {
					delete(d.closed, k)
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (d *Dispatcher) route(flushCtx context.Context, env match.Envelope, wg *sync.WaitGroup) {
	k := env.Event.Key()
	if t, ok := d.closed[k]; ok {
		env.Reply <- match.Ack{Status: match.Rejected, Code: t.code}
		return
	}
	ch, ok := d.workers[k]
	if !ok {
		// Una partida ya cerrada antes de un reinicio (resultados recargados de
		// RESULTS_DIR) no se reabre: se rechaza igual que con un tombstone.
		if rec, exists := d.store.Get(k); exists && rec.Status != store.Collecting && rec.Status != store.Finishing {
			d.closed[k] = tombstone{code: "MATCH_CLOSED", at: time.Now()}
			env.Reply <- match.Ack{Status: match.Rejected, Code: "MATCH_CLOSED"}
			return
		}
		ch = make(chan match.Envelope, d.cfg.MatchQueueSize)
		d.workers[k] = ch
		w := match.NewWorker(k, d.cfg.Match, ch, d.pool, d.done, d.store, d.log)
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.Run(flushCtx)
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
