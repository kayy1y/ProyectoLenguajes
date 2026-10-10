// Package store guarda el estado consultable de cada partida y su resultado.
//
// Es la única estructura compartida entre goroutines; todo acceso pasa por el RWMutex.
// El resto del estado (mapa de partidas, eventos de cada partida) está confinado
// a una sola goroutine y no necesita locks.
package store

import (
	"encoding/json"
	"sort"
	"sync"
	"time"

	"gamestats/ingestion-go/internal/event"
)

type Status string

const (
	Collecting Status = "COLLECTING"
	Analyzing  Status = "ANALYZING"
	Analyzed   Status = "ANALYZED"
	Failed     Status = "FAILED"
)

type Record struct {
	GameID     string          `json:"gameId"`
	MatchID    string          `json:"matchId"`
	Status     Status          `json:"status"`
	EventCount int             `json:"eventCount"`
	Attempts   int             `json:"attempts"`
	LastError  string          `json:"lastError,omitempty"`
	UpdatedAt  time.Time       `json:"updatedAt"`
	AnalyzedAt *time.Time      `json:"analyzedAt,omitempty"`
	Result     json.RawMessage `json:"result,omitempty"`
}

type Store struct {
	mu      sync.RWMutex
	records map[event.Key]*Record
	now     func() time.Time
}

func New() *Store {
	return &Store{records: make(map[event.Key]*Record), now: time.Now}
}

func (s *Store) update(k event.Key, f func(r *Record)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.records[k]
	if !ok {
		r = &Record{GameID: k.GameID, MatchID: k.MatchID, Status: Collecting}
		s.records[k] = r
	}
	f(r)
	r.UpdatedAt = s.now().UTC()
}

func (s *Store) SetCollecting(k event.Key, eventCount int) {
	s.update(k, func(r *Record) { r.Status = Collecting; r.EventCount = eventCount })
}

func (s *Store) SetAnalyzing(k event.Key) {
	s.update(k, func(r *Record) { r.Status = Analyzing; r.Attempts++ })
}

func (s *Store) SetResult(k event.Key, result json.RawMessage) {
	s.update(k, func(r *Record) {
		at := s.now().UTC()
		r.Status, r.Result, r.AnalyzedAt, r.LastError = Analyzed, result, &at, ""
	})
}

func (s *Store) SetFailed(k event.Key, err error) {
	s.update(k, func(r *Record) { r.Status, r.LastError = Failed, err.Error() })
}

// Get devuelve una copia para que el llamador no comparta memoria con el store.
func (s *Store) Get(k event.Key) (Record, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.records[k]
	if !ok {
		return Record{}, false
	}
	return *r, true
}

// List devuelve las partidas sin el resultado, ordenadas por juego y partida.
func (s *Store) List() []Record {
	s.mu.RLock()
	out := make([]Record, 0, len(s.records))
	for _, r := range s.records {
		c := *r
		c.Result = nil
		out = append(out, c)
	}
	s.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].GameID != out[j].GameID {
			return out[i].GameID < out[j].GameID
		}
		return out[i].MatchID < out[j].MatchID
	})
	return out
}
