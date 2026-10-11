// Package obs reúne los contadores de diagnóstico y el registro de rechazos.
//
// Son contadores de ingestión (cuántos eventos se aceptaron o rechazaron), no
// métricas de juego: esas las calcula exclusivamente Scala.
package obs

import (
	"sync"
	"sync/atomic"
	"time"
)

type Counters struct {
	Accepted     atomic.Int64
	Rejected     atomic.Int64
	Duplicates   atomic.Int64
	Late         atomic.Int64
	Backpressure atomic.Int64
}

func (c *Counters) Snapshot() map[string]int64 {
	return map[string]int64{
		"accepted":     c.Accepted.Load(),
		"rejected":     c.Rejected.Load(),
		"duplicates":   c.Duplicates.Load(),
		"late":         c.Late.Load(),
		"backpressure": c.Backpressure.Load(),
	}
}

type Rejection struct {
	At      time.Time `json:"at"`
	EventID string    `json:"eventId,omitempty"`
	GameID  string    `json:"gameId,omitempty"`
	MatchID string    `json:"matchId,omitempty"`
	Code    string    `json:"code"`
	Field   string    `json:"field,omitempty"`
	Message string    `json:"message,omitempty"`
}

// Rejections guarda los últimos N rechazos en un buffer circular.
type Rejections struct {
	mu   sync.Mutex
	buf  []Rejection
	next int
	full bool
}

func NewRejections(size int) *Rejections { return &Rejections{buf: make([]Rejection, size)} }

func (r *Rejections) Add(x Rejection) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.next] = x
	r.next = (r.next + 1) % len(r.buf)
	if r.next == 0 {
		r.full = true
	}
}

// Last devuelve hasta limit rechazos, del más reciente al más antiguo.
func (r *Rejections) Last(limit int) []Rejection {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := r.next
	if r.full {
		n = len(r.buf)
	}
	if limit <= 0 || limit > n {
		limit = n
	}
	out := make([]Rejection, 0, limit)
	for i := 1; i <= limit; i++ {
		out = append(out, r.buf[(r.next-i+len(r.buf))%len(r.buf)])
	}
	return out
}
