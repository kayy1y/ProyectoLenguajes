// Package store guarda el estado consultable de cada partida y su resultado.
//
// Es la única estructura compartida entre goroutines; todo acceso pasa por el RWMutex.
// El resto del estado (mapa de partidas, eventos de cada partida) está confinado
// a una sola goroutine y no necesita locks.
//
// Con ResultsDir, los resultados ANALYZED se copian a disco y se recargan al
// arrancar; todo lo demás (partidas activas, lotes FAILED) vive solo en memoria.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"gamestats/ingestion-go/internal/event"
)

type Status string

const (
	Collecting Status = "COLLECTING"
	Finishing  Status = "FINISHING"
	Analyzing  Status = "ANALYZING"
	Retrying   Status = "RETRYING"
	Analyzed   Status = "ANALYZED"
	Failed     Status = "FAILED"
)

type Record struct {
	GameID       string          `json:"gameId"`
	MatchID      string          `json:"matchId"`
	Status       Status          `json:"status"`
	EventCount   int             `json:"eventCount"`
	Completeness string          `json:"completeness,omitempty"`
	ClosedReason string          `json:"closedReason,omitempty"`
	Attempts     int             `json:"attempts"`
	LastError    string          `json:"lastError,omitempty"`
	UpdatedAt    time.Time       `json:"updatedAt"`
	AnalyzedAt   *time.Time      `json:"analyzedAt,omitempty"`
	Result       json.RawMessage `json:"result,omitempty"`

	// Lote pendiente de un análisis FAILED, para re-analizar sin el productor.
	pending any
}

type Store struct {
	mu      sync.RWMutex
	records map[event.Key]*Record
	dir     string
	now     func() time.Time
}

// New crea el store y, si dir no está vacío, recarga los resultados guardados.
func New(dir string) (*Store, error) {
	s := &Store{records: make(map[event.Key]*Record), dir: dir, now: time.Now}
	if dir == "" {
		return s, nil
	}
	files, err := filepath.Glob(filepath.Join(dir, "*", "*.json"))
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var r Record
		if err := json.Unmarshal(b, &r); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		s.records[event.Key{GameID: r.GameID, MatchID: r.MatchID}] = &r
	}
	return s, nil
}

func (s *Store) update(k event.Key, f func(r *Record)) Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.records[k]
	if !ok {
		r = &Record{GameID: k.GameID, MatchID: k.MatchID, Status: Collecting}
		s.records[k] = r
	}
	f(r)
	r.UpdatedAt = s.now().UTC()
	return *r
}

func (s *Store) SetCollecting(k event.Key, status Status, eventCount int) {
	s.update(k, func(r *Record) { r.Status = status; r.EventCount = eventCount })
}

func (s *Store) SetClosed(k event.Key, completeness, reason string) {
	s.update(k, func(r *Record) { r.Status, r.Completeness, r.ClosedReason = Analyzing, completeness, reason })
}

// StartAttempt registra un intento nuevo y devuelve su número (1, 2, ...).
func (s *Store) StartAttempt(k event.Key, retry bool) int {
	rec := s.update(k, func(r *Record) {
		r.Attempts++
		r.Status = Analyzing
		if retry {
			r.Status = Retrying
		}
	})
	return rec.Attempts
}

func (s *Store) SetResult(k event.Key, result json.RawMessage) error {
	rec := s.update(k, func(r *Record) {
		at := s.now().UTC()
		r.Status, r.Result, r.AnalyzedAt, r.LastError, r.pending = Analyzed, result, &at, "", nil
	})
	return s.persist(rec)
}

func (s *Store) SetFailed(k event.Key, err error, pending any) {
	s.update(k, func(r *Record) { r.Status, r.LastError, r.pending = Failed, err.Error(), pending })
}

// TakeFailed devuelve el lote de una partida FAILED y la marca ANALYZING de forma
// atómica, para que dos reintentos simultáneos no la envíen dos veces.
func (s *Store) TakeFailed(k event.Key) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.records[k]
	if !ok || r.Status != Failed || r.pending == nil {
		return nil, false
	}
	p := r.pending
	r.pending, r.Status, r.UpdatedAt = nil, Analyzing, s.now().UTC()
	return p, true
}

// Get devuelve una copia para que el llamador no comparta memoria con el store.
func (s *Store) Get(k event.Key) (Record, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.records[k]
	if !ok {
		return Record{}, false
	}
	c := *r
	c.pending = nil
	return c, true
}

// List devuelve las partidas sin el resultado, ordenadas por juego y partida.
func (s *Store) List() []Record {
	s.mu.RLock()
	out := make([]Record, 0, len(s.records))
	for _, r := range s.records {
		c := *r
		c.Result, c.pending = nil, nil
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

// CountByStatus sirve para /v1/stats.
func (s *Store) CountByStatus() map[Status]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[Status]int{}
	for _, r := range s.records {
		out[r.Status]++
	}
	return out
}

// persist escribe el registro fuera del lock con write-temp + rename (atómico).
func (s *Store) persist(r Record) error {
	if s.dir == "" {
		return nil
	}
	dir := filepath.Join(s.dir, safe(r.GameID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	r.pending = nil
	b, err := json.Marshal(r) // sin indentar: el resultado de Scala se guarda byte a byte
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(dir, safe(r.MatchID)+".json"))
}

// safe evita que un id con separadores escape del directorio (los ids ya están
// restringidos por el esquema; esto es defensa adicional).
func safe(id string) string { return filepath.Base(filepath.Clean("/" + id)) }
