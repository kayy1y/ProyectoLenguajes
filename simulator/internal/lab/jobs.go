package lab

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gamestats/simulator/internal/scenario"
	"gamestats/simulator/internal/sender"
	"gamestats/simulator/internal/verify"
)

// RunRequest pide generar datos: fixtures (Names, o "all"), demo o stress.
type RunRequest struct {
	Kind    string   `json:"kind"` // fixtures | demo | stress
	Names   []string `json:"names"`
	Seed    uint64   `json:"seed"`
	Matches int      `json:"matches"`
	Speed   float64  `json:"speed"`
}

// MatchView resume una partida enviada por un trabajo.
type MatchView struct {
	GameID       string        `json:"gameId"`
	MatchID      string        `json:"matchId"`
	Status       string        `json:"status"`
	Completeness string        `json:"completeness,omitempty"`
	EventCount   int           `json:"eventCount"`
	Attempts     int           `json:"attempts"`
	Patterns     []string      `json:"patterns"`
	Warnings     int           `json:"warnings"`
	LastError    string        `json:"lastError,omitempty"`
	Check        *verify.Check `json:"check,omitempty"`
}

// Job es una ejecución en segundo plano (generar datos o verificar).
type Job struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	State      string         `json:"state"` // running | done | error
	Step       string         `json:"step"`
	Suffix     string         `json:"suffix"`
	StartedAt  time.Time      `json:"startedAt"`
	FinishedAt *time.Time     `json:"finishedAt,omitempty"`
	Total      int            `json:"total"`
	Sent       int            `json:"sent"`
	Counts     map[string]int `json:"counts"`
	Matches    []MatchView    `json:"matches"`
	Checks     []verify.Check `json:"checks"`
	Log        []string       `json:"log"`
	Error      string         `json:"error,omitempty"`
	mu         sync.Mutex
}

// View devuelve una copia consistente del trabajo (se puede leer desde otra goroutine).
func (j *Job) View() *Job {
	j.mu.Lock()
	defer j.mu.Unlock()
	c := &Job{ID: j.ID, Kind: j.Kind, State: j.State, Step: j.Step, Suffix: j.Suffix, StartedAt: j.StartedAt,
		FinishedAt: j.FinishedAt, Total: j.Total, Sent: j.Sent, Error: j.Error,
		Counts:  make(map[string]int, len(j.Counts)),
		Matches: append([]MatchView{}, j.Matches...),
		Checks:  append([]verify.Check{}, j.Checks...),
		Log:     append([]string{}, j.Log...)}
	for k, v := range j.Counts {
		c.Counts[k] = v
	}
	return c
}

func (j *Job) update(f func(*Job)) { j.mu.Lock(); f(j); j.mu.Unlock() }

func (j *Job) logf(format string, args ...any) {
	j.update(func(j *Job) { j.Log = append(j.Log, time.Now().Format("15:04:05")+"  "+fmt.Sprintf(format, args...)) })
}

func (j *Job) ack(a sender.Ack) {
	j.update(func(j *Job) {
		j.Sent++
		j.Counts[strings.TrimSpace(fmt.Sprintf("%d %s %s", a.HTTP, a.Status, a.Code))]++
	})
}

func (j *Job) finish(err error) {
	j.update(func(j *Job) {
		now := time.Now()
		j.FinishedAt = &now
		j.Step = ""
		if err != nil {
			j.State, j.Error = "error", err.Error()
		} else {
			j.State = "done"
		}
	})
}

// Config indica dónde está la ingestión y dónde están los contratos.
type Config struct {
	Target, APIKey                       string
	FixturesDir, ExamplesDir, SchemasDir string
}

// Lab ejecuta trabajos en segundo plano contra la ingestión.
type Lab struct {
	cfg    Config
	client *http.Client
	mu     sync.Mutex
	byID   map[string]*Job
	order  []string
	seq    int
}

const maxJobs = 20

// New crea un Lab con un cliente HTTP propio.
func New(cfg Config) *Lab {
	cfg.Target = strings.TrimRight(cfg.Target, "/")
	return &Lab{cfg: cfg, client: &http.Client{Timeout: 10 * time.Second}, byID: map[string]*Job{}}
}

// Config devuelve la configuración en uso.
func (js *Lab) Config() Config { return js.cfg }

// Get devuelve un trabajo por id (nil si no existe).
func (js *Lab) Get(id string) *Job {
	js.mu.Lock()
	defer js.mu.Unlock()
	return js.byID[id]
}

// create registra un trabajo nuevo con un sufijo único: las partidas cerradas no
// se reabren, así que cada ejecución usa ids propios.
func (js *Lab) create(kind string) *Job {
	js.mu.Lock()
	defer js.mu.Unlock()
	js.seq++
	id := strconv.FormatInt(time.Now().Unix(), 36) + strconv.Itoa(js.seq)
	j := &Job{ID: id, Kind: kind, State: "running", Suffix: "-" + id, StartedAt: time.Now(),
		Counts: map[string]int{}, Matches: []MatchView{}, Checks: []verify.Check{}, Log: []string{}}
	js.byID[id] = j
	js.order = append(js.order, id)
	if len(js.order) > maxJobs {
		delete(js.byID, js.order[0])
		js.order = js.order[1:]
	}
	return j
}

func (js *Lab) senderConfig(j *Job, speed float64) sender.Config {
	if speed <= 0 {
		speed = 50
	}
	return sender.Config{Target: js.cfg.Target, APIKey: js.cfg.APIKey, Speed: min(speed, 5000), MaxGap: 150 * time.Millisecond,
		Out: io.Discard, Client: js.client, PollWait: 250 * time.Millisecond, OnAck: j.ack}
}

func (js *Lab) buildMatches(req RunRequest, suffix string) ([]scenario.Match, error) {
	switch req.Kind {
	case "demo":
		return scenario.Demo(req.Seed, js.cfg.FixturesDir, suffix)
	case "stress":
		n := req.Matches
		if n < 1 || n > 200 {
			return nil, fmt.Errorf("matches debe estar entre 1 y 200")
		}
		ms := scenario.Stress(req.Seed, n)
		for i := range ms {
			ms[i] = scenario.WithSuffix(ms[i], suffix)
		}
		return ms, nil
	case "fixtures":
		names := req.Names
		if len(names) == 0 {
			return nil, fmt.Errorf("indica al menos un fixture en names (o \"all\")")
		}
		if len(names) == 1 && names[0] == "all" {
			idx, err := readIndex(js.cfg.FixturesDir)
			if err != nil {
				return nil, err
			}
			names = names[:0]
			for n := range idx {
				names = append(names, n)
			}
			sort.Strings(names)
		}
		var ms []scenario.Match
		for _, n := range names {
			if strings.ContainsAny(n, `/\.`) {
				return nil, fmt.Errorf("fixture inválido: %q", n)
			}
			m, err := scenario.Fixture(js.cfg.FixturesDir, n, suffix)
			if err != nil {
				return nil, fmt.Errorf("fixture %q: %w", n, err)
			}
			ms = append(ms, m)
		}
		return ms, nil
	}
	return nil, fmt.Errorf("kind debe ser fixtures, demo o stress")
}

// StartRun valida la solicitud y lanza el envío en una goroutine.
func (js *Lab) StartRun(req RunRequest) (*Job, error) {
	j := js.create("run:" + req.Kind)
	ms, err := js.buildMatches(req, j.Suffix)
	if err != nil {
		js.mu.Lock()
		delete(js.byID, j.ID)
		js.order = js.order[:len(js.order)-1]
		js.mu.Unlock()
		return nil, err
	}
	total := 0
	for _, m := range ms {
		total += len(m.Lines) + len(m.AfterAnalyzed)
	}
	j.update(func(j *Job) { j.Total = total })
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		defer cancel()
		j.finish(js.execute(ctx, j, ms, js.senderConfig(j, req.Speed), req.Kind != "stress"))
	}()
	return j, nil
}

// execute envía las partidas, espera sus resultados y los resume en el trabajo.
func (js *Lab) execute(ctx context.Context, j *Job, ms []scenario.Match, cfg sender.Config, compare bool) error {
	j.update(func(j *Job) { j.Step = "enviando eventos" })
	j.logf("enviando %d partidas en paralelo", len(ms))
	rep, err := sender.Send(ctx, cfg, ms)
	if err != nil {
		return err
	}
	j.update(func(j *Job) { j.Step = "esperando análisis" })
	j.logf("eventos enviados; esperando el análisis de Scala")
	waitCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	results := sender.WaitResults(waitCtx, cfg, ms)
	if err := sender.SendLate(ctx, cfg, ms, rep); err != nil {
		return err
	}
	var views []MatchView
	for _, m := range ms {
		views = append(views, js.matchView(j.Suffix, m, results[m.MatchID], compare))
	}
	j.update(func(j *Job) { j.Matches = views })
	for _, v := range views {
		j.logf("%s/%s → %s", v.GameID, v.MatchID, v.Status)
	}
	return nil
}

func (js *Lab) matchView(suffix string, m scenario.Match, r sender.MatchRecord, compare bool) MatchView {
	v := MatchView{GameID: m.GameID, MatchID: m.MatchID, Status: r.Status, Completeness: r.Completeness,
		EventCount: r.EventCount, Attempts: r.Attempts, LastError: r.LastError, Patterns: []string{}}
	var res struct {
		Patterns []struct{ Type, PlayerID string }
		Warnings []any
	}
	json.Unmarshal(r.Result, &res)
	for _, p := range res.Patterns {
		v.Patterns = append(v.Patterns, p.Type+" · "+p.PlayerID)
	}
	v.Warnings = len(res.Warnings)
	if compare && verify.Has(js.cfg.FixturesDir, m.GameID, strings.TrimSuffix(m.MatchID, suffix)) {
		c := verify.Expected(js.cfg.FixturesDir, suffix, m.GameID, m.MatchID, r.Result)
		v.Check = &c
	}
	return v
}
