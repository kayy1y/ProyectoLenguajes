// Package sender envía partidas al servicio de ingestión: una goroutine por
// partida, de modo que los eventos de partidas distintas llegan intercalados.
package sender

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"sort"
	"sync"
	"time"

	"gamestats/simulator/internal/scenario"
)

type Config struct {
	Target   string
	APIKey   string
	Speed    float64       // compresión del tiempo de juego (event-time / Speed)
	MaxGap   time.Duration // tope de espera entre dos eventos de una partida
	Verbose  bool
	Out      io.Writer
	Client   *http.Client
	PollWait time.Duration
}

type Ack struct {
	Match, EventID string
	HTTP           int
	Status, Code   string
}

type Report struct {
	mu   sync.Mutex
	Acks []Ack
}

func (r *Report) add(a Ack) { r.mu.Lock(); r.Acks = append(r.Acks, a); r.mu.Unlock() }

// Counts agrupa las respuestas por "HTTP STATUS CODE".
func (r *Report) Counts() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := map[string]int{}
	for _, a := range r.Acks {
		out[fmt.Sprintf("%d %s %s", a.HTTP, a.Status, a.Code)]++
	}
	return out
}

// Send envía todas las partidas en paralelo y espera a que terminen.
func Send(ctx context.Context, cfg Config, matches []scenario.Match) (*Report, error) {
	rep := &Report{}
	var wg sync.WaitGroup
	errs := make(chan error, len(matches))
	for i, m := range matches {
		wg.Add(1)
		go func(i int, m scenario.Match) {
			defer wg.Done()
			// Arranque escalonado y jitter: las partidas se intercalan de verdad.
			jitter := rand.New(rand.NewPCG(uint64(i), 7))
			time.Sleep(time.Duration(i*20) * time.Millisecond)
			for j, line := range m.Lines {
				if j > 0 {
					gap := time.Duration(float64(m.At[j].Sub(m.At[j-1])) / cfg.Speed)
					gap = min(gap, cfg.MaxGap) + time.Duration(jitter.IntN(5))*time.Millisecond
					select {
					case <-time.After(gap):
					case <-ctx.Done():
						errs <- ctx.Err()
						return
					}
				}
				if err := post(ctx, cfg, m.MatchID, line, rep); err != nil {
					errs <- err
					return
				}
			}
		}(i, m)
	}
	wg.Wait()
	close(errs)
	if err := <-errs; err != nil {
		return rep, err
	}
	return rep, nil
}

func post(ctx context.Context, cfg Config, match string, line []byte, rep *Report) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.Target+"/v1/events", bytes.NewReader(line))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("X-API-Key", cfg.APIKey)
	}
	resp, err := cfg.Client.Do(req)
	if err != nil {
		return fmt.Errorf("send event of %s: %w", match, err)
	}
	defer resp.Body.Close()
	var body struct {
		Status, Code, EventID string
		Errors                []struct{ Field, Code string }
	}
	json.NewDecoder(resp.Body).Decode(&body)
	code := body.Code
	if len(body.Errors) > 0 {
		code = body.Errors[0].Code + " " + body.Errors[0].Field
	}
	rep.add(Ack{Match: match, EventID: body.EventID, HTTP: resp.StatusCode, Status: body.Status, Code: code})
	if cfg.Verbose {
		fmt.Fprintf(cfg.Out, "%-22s %-24s %d %s %s\n", match, body.EventID, resp.StatusCode, body.Status, code)
	}
	return nil
}

type MatchRecord struct {
	GameID, MatchID, Status, Completeness, ClosedReason, LastError string
	EventCount, Attempts                                           int
	Result                                                         json.RawMessage
}

// WaitResults consulta cada partida hasta que deja de estar en curso o vence ctx.
func WaitResults(ctx context.Context, cfg Config, matches []scenario.Match) map[string]MatchRecord {
	out := map[string]MatchRecord{}
	pending := map[string]scenario.Match{}
	for _, m := range matches {
		pending[m.MatchID] = m
	}
	for len(pending) > 0 {
		for id, m := range pending {
			resp, err := cfg.Client.Get(fmt.Sprintf("%s/v1/games/%s/matches/%s/results", cfg.Target, m.GameID, m.MatchID))
			if err != nil {
				continue
			}
			var rec MatchRecord
			json.NewDecoder(resp.Body).Decode(&rec)
			resp.Body.Close()
			if rec.Status == "ANALYZED" || rec.Status == "FAILED" {
				out[id] = rec
				delete(pending, id)
			}
		}
		select {
		case <-ctx.Done():
			for id, m := range pending {
				out[id] = MatchRecord{GameID: m.GameID, MatchID: id, Status: "TIMEOUT"}
			}
			return out
		case <-time.After(cfg.PollWait):
		}
	}
	return out
}

// SendLate envía los eventos tardíos de las partidas ya analizadas.
func SendLate(ctx context.Context, cfg Config, matches []scenario.Match, rep *Report) error {
	for _, m := range matches {
		for _, line := range m.AfterAnalyzed {
			if err := post(ctx, cfg, m.MatchID, line, rep); err != nil {
				return err
			}
		}
	}
	return nil
}

// SortedKeys devuelve las claves de un mapa ordenadas (para salidas estables).
func SortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
