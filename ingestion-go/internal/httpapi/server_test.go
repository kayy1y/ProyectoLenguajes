package httpapi_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gamestats/ingestion-go/internal/analysis"
	"gamestats/ingestion-go/internal/dispatcher"
	"gamestats/ingestion-go/internal/event"
	"gamestats/ingestion-go/internal/httpapi"
	"gamestats/ingestion-go/internal/match"
	"gamestats/ingestion-go/internal/obs"
	"gamestats/ingestion-go/internal/store"
	"gamestats/ingestion-go/internal/validation"
)

const contracts = "../../../contracts"

// fakeAnalytics registra las peticiones y responde según handle (200 por defecto).
type fakeAnalytics struct {
	mu       sync.Mutex
	requests []analysis.Request
	handle   func(req analysis.Request, n int) (int, time.Duration)
}

func (f *fakeAnalytics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req analysis.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	f.requests = append(f.requests, req)
	n := len(f.requests)
	handle := f.handle
	f.mu.Unlock()
	status, delay := http.StatusOK, time.Duration(0)
	if handle != nil {
		status, delay = handle(req, n)
	}
	time.Sleep(delay)
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"resultVersion":"1","matchId":%q,"events":%d}`, req.MatchID, len(req.Events))
}

func (f *fakeAnalytics) setHandle(h func(analysis.Request, int) (int, time.Duration)) {
	f.mu.Lock()
	f.handle = h
	f.mu.Unlock()
}

func (f *fakeAnalytics) byMatch(matchID string) []analysis.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []analysis.Request
	for _, r := range f.requests {
		if r.MatchID == matchID {
			out = append(out, r)
		}
	}
	return out
}

type options struct {
	grace, idle time.Duration
	retries     int
	maxEvents   int
	apiKey      string
}

type system struct {
	api      *httptest.Server
	store    *store.Store
	fake     *fakeAnalytics
	shutdown func()
}

func newSystem(t *testing.T, o options) *system {
	t.Helper()
	if o.grace == 0 {
		o.grace = 50 * time.Millisecond
	}
	if o.idle == 0 {
		o.idle = 5 * time.Second
	}
	if o.maxEvents == 0 {
		o.maxEvents = 10000
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	fake := &fakeAnalytics{}
	analytics := httptest.NewServer(fake)
	t.Cleanup(analytics.Close)

	validator, err := validation.Load(filepath.Join(contracts, "schemas"))
	if err != nil {
		t.Fatal(err)
	}
	st, _ := store.New("")
	pool := analysis.NewPool(analysis.PoolConfig{Workers: 2, QueueSize: 16, MaxRetries: o.retries, BackoffBase: 10 * time.Millisecond},
		analysis.NewClient(analytics.URL, 3*time.Second), st, log)
	disp := dispatcher.New(dispatcher.Config{QueueSize: 256, MatchQueueSize: 256, TombstoneTTL: time.Hour,
		Match: match.Config{GracePeriod: o.grace, IdleTimeout: o.idle, MaxEvents: o.maxEvents, AnalyzeIncompleteOnStop: true}},
		pool, st, log)

	ingestCtx, stopIngest := context.WithCancel(context.Background())
	analysisCtx, stopAnalysis := context.WithCancel(context.Background())
	dispDone, poolDone := make(chan struct{}), make(chan struct{})
	go func() { disp.Run(ingestCtx, analysisCtx); close(dispDone) }()
	go func() { pool.Run(analysisCtx); close(poolDone) }()

	api := httptest.NewServer(httpapi.New(httpapi.Deps{
		Validator: validator, Dispatcher: disp, Pool: pool, Store: st,
		Counters: &obs.Counters{}, Rejections: obs.NewRejections(100),
		EnqueueTimeout: time.Second, APIKey: o.apiKey, Log: log,
	}).Handler())

	var once sync.Once
	shutdown := func() {
		once.Do(func() {
			api.Close()
			stopIngest()
			<-dispDone
			pool.Drain()
			<-poolDone
			stopAnalysis()
		})
	}
	t.Cleanup(shutdown)
	return &system{api: api, store: st, fake: fake, shutdown: shutdown}
}

func (s *system) post(t *testing.T, body []byte, header ...string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, s.api.URL+"/v1/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (s *system) get(t *testing.T, path string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Get(s.api.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

// waitStatus espera a que la partida llegue a want (ANALYZED, FAILED, ...).
func (s *system) waitStatus(t *testing.T, game, matchID string, want store.Status) store.Record {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		rec, ok := s.store.Get(eventKey(game, matchID))
		if ok && rec.Status == want {
			return rec
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: want status %s, got %+v", matchID, want, rec)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(contracts, path))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func fixture(t *testing.T, name string) [][]byte {
	t.Helper()
	f, err := os.Open(filepath.Join(contracts, "fixtures", name+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var out [][]byte
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		out = append(out, append([]byte(nil), sc.Bytes()...))
	}
	return out
}

// arena construye un evento válido de arena-combat.
func arena(matchID, id string, sec int, typ, player string, data string) []byte {
	ts := time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC).Add(time.Duration(sec) * time.Second).Format(time.RFC3339)
	p := ""
	if player != "" {
		p = fmt.Sprintf(`"playerId":%q,`, player)
	}
	return []byte(fmt.Sprintf(`{"eventId":%q,"timestamp":%q,"gameId":"arena-combat","gameVersion":"1.0","matchId":%q,%s"type":%q,"data":%s}`,
		id, ts, matchID, p, typ, data))
}

func TestPostEventResponses(t *testing.T) {
	s := newSystem(t, options{})
	valid := readFile(t, "examples/valid/arena-attack-hit.json")

	if code, body := s.post(t, valid); code != http.StatusAccepted || body["status"] != "ACCEPTED" {
		t.Fatalf("valid event: got %d %v", code, body)
	}
	if code, body := s.post(t, valid); code != http.StatusOK || body["status"] != "DUPLICATE" {
		t.Fatalf("duplicate event: got %d %v", code, body)
	}
	conflict := bytes.Replace(valid, []byte(`"damage": 40`), []byte(`"damage": 41`), 1)
	if code, body := s.post(t, conflict); code != http.StatusConflict || body["code"] != "DUPLICATE_ID_CONFLICT" {
		t.Fatalf("conflicting duplicate: got %d %v", code, body)
	}
	started := readFile(t, "examples/valid/arena-match-started.json")
	s.post(t, started)
	restarted := bytes.Replace(started, []byte(`ex1-e01`), []byte(`ex1-e99`), 1)
	if code, body := s.post(t, restarted); code != http.StatusConflict || body["code"] != "MATCH_ALREADY_STARTED" {
		t.Fatalf("second MATCH_STARTED: got %d %v", code, body)
	}

	var expected map[string]struct{ Code, Field string }
	json.Unmarshal(readFile(t, "examples/invalid/expected.json"), &expected)
	for name, want := range expected {
		code, body := s.post(t, readFile(t, "examples/invalid/"+name))
		found := false
		errs, _ := body["errors"].([]any)
		for _, e := range errs {
			m := e.(map[string]any)
			found = found || (m["code"] == want.Code && m["field"] == want.Field)
		}
		if code != http.StatusBadRequest || !found {
			t.Errorf("%s: got %d %v, want 400 %s at %s", name, code, body, want.Code, want.Field)
		}
	}

	resp, _ := http.Get(s.api.URL + "/v1/rejections?limit=5")
	var rej []obs.Rejection
	json.NewDecoder(resp.Body).Decode(&rej)
	resp.Body.Close()
	if len(rej) != 5 {
		t.Errorf("rejections: got %d entries, want 5", len(rej))
	}
	_, stats := s.get(t, "/v1/stats")
	ev := stats["events"].(map[string]any)
	if ev["accepted"].(float64) != 2 || ev["duplicates"].(float64) != 1 || ev["rejected"].(float64) != float64(2+len(expected)) {
		t.Errorf("unexpected counters: %v", ev)
	}
}

func TestF01FlowsToAnalyticsInOrder(t *testing.T) {
	s := newSystem(t, options{})
	for _, line := range fixture(t, "F01-arena-normal") {
		if code, body := s.post(t, line); code != http.StatusAccepted {
			t.Fatalf("got %d %v", code, body)
		}
	}
	rec := s.waitStatus(t, "arena-combat", "match-a1", store.Analyzed)
	if rec.Completeness != "COMPLETE" || rec.ClosedReason != "MATCH_FINISHED" || rec.Attempts != 1 {
		t.Fatalf("unexpected record %+v", rec)
	}
	reqs := s.fake.byMatch("match-a1")
	if len(reqs) != 1 || len(reqs[0].Events) != 18 || reqs[0].BatchID != "arena-combat:match-a1:1" {
		t.Fatalf("unexpected analytics requests: %+v", reqs)
	}
	for i, e := range reqs[0].Events {
		if want := fmt.Sprintf("a1-e%02d", i+1); e.EventID != want || e.Ingest == nil {
			t.Fatalf("event %d: got %s, want %s", i, e.EventID, want)
		}
	}
	code, body := s.get(t, "/v1/games/arena-combat/matches/match-a1/results")
	if code != http.StatusOK || body["result"] == nil {
		t.Fatalf("results: %d %v", code, body)
	}
}

// F06: e10 llega después de e13 y e17 llega después de MATCH_FINISHED (dentro de la gracia).
func TestOutOfOrderAndGracePeriod(t *testing.T) {
	s := newSystem(t, options{grace: 300 * time.Millisecond})
	for _, line := range fixture(t, "F06-arena-out-of-order") {
		if code, body := s.post(t, line); code != http.StatusAccepted {
			t.Fatalf("got %d %v", code, body)
		}
	}
	s.waitStatus(t, "arena-combat", "match-f06", store.Analyzed)
	reqs := s.fake.byMatch("match-f06")
	if len(reqs) != 1 || len(reqs[0].Events) != 18 {
		t.Fatalf("want 1 request with 18 events, got %+v", reqs)
	}
	for i, e := range reqs[0].Events {
		if want := fmt.Sprintf("f06-e%02d", i+1); e.EventID != want {
			t.Fatalf("position %d: got %s, want %s", i, e.EventID, want)
		}
	}
	late := arena("match-f06", "f06-late", 130, "OBJECTIVE_CAPTURED", "p1", `{"objectiveId":"flag-B"}`)
	if code, body := s.post(t, late); code != http.StatusConflict || body["code"] != "LATE_EVENT" {
		t.Fatalf("late event: got %d %v", code, body)
	}
}

func TestIdleTimeoutAnalyzesIncompleteMatch(t *testing.T) {
	s := newSystem(t, options{idle: 150 * time.Millisecond})
	lines := fixture(t, "F01-arena-normal")
	for _, line := range lines[:len(lines)-1] { // sin MATCH_FINISHED
		s.post(t, line)
	}
	rec := s.waitStatus(t, "arena-combat", "match-a1", store.Analyzed)
	if rec.Completeness != "INCOMPLETE" || rec.ClosedReason != "IDLE_TIMEOUT" {
		t.Fatalf("unexpected record %+v", rec)
	}
	if code, body := s.post(t, lines[len(lines)-1]); code != http.StatusConflict || body["code"] != "MATCH_CLOSED" {
		t.Fatalf("event after expiry: got %d %v", code, body)
	}
}

// Varias partidas, cada una alimentada por varios productores concurrentes.
func TestConcurrentMatchesKeepTheirOwnEvents(t *testing.T) {
	s := newSystem(t, options{grace: 100 * time.Millisecond})
	const matches, producers, attacksPerProducer = 4, 4, 60
	var wg sync.WaitGroup
	var failures atomic.Int64
	for m := 0; m < matches; m++ {
		matchID := fmt.Sprintf("conc-%d", m)
		s.post(t, arena(matchID, matchID+"-start", 0, "MATCH_STARTED", "", `{"map":"dust"}`))
		for _, p := range []string{"p1", "p2"} {
			s.post(t, arena(matchID, matchID+"-join-"+p, 1, "PLAYER_JOINED", p, `{}`))
		}
		for g := 0; g < producers; g++ {
			wg.Add(1)
			go func(matchID string, g int) {
				defer wg.Done()
				for i := 0; i < attacksPerProducer; i++ {
					body := arena(matchID, fmt.Sprintf("%s-g%d-%d", matchID, g, i), 2+i, "ATTACK", "p1",
						`{"targetPlayerId":"p2","weapon":"rifle","hit":true,"damage":1}`)
					if code, _ := s.post(t, body); code != http.StatusAccepted {
						failures.Add(1)
					}
				}
			}(matchID, g)
		}
	}
	wg.Wait()
	if failures.Load() > 0 {
		t.Fatalf("%d events were not accepted", failures.Load())
	}
	for m := 0; m < matches; m++ {
		matchID := fmt.Sprintf("conc-%d", m)
		s.post(t, arena(matchID, matchID+"-fin", 500, "MATCH_FINISHED", "", `{"reason":"COMPLETED"}`))
	}
	for m := 0; m < matches; m++ {
		matchID := fmt.Sprintf("conc-%d", m)
		s.waitStatus(t, "arena-combat", matchID, store.Analyzed)
		reqs := s.fake.byMatch(matchID)
		want := 1 + 2 + producers*attacksPerProducer + 1
		if len(reqs) != 1 || len(reqs[0].Events) != want {
			t.Fatalf("%s: want 1 request with %d events, got %d requests", matchID, want, len(reqs))
		}
		for _, e := range reqs[0].Events {
			if e.MatchID != matchID {
				t.Fatalf("%s received event of %s", matchID, e.MatchID)
			}
		}
	}
}

// Un análisis lento de una partida no frena la ingestión ni el análisis de otra.
func TestSlowAnalysisDoesNotBlockOtherMatches(t *testing.T) {
	s := newSystem(t, options{grace: 10 * time.Millisecond})
	s.fake.setHandle(func(req analysis.Request, _ int) (int, time.Duration) {
		if req.MatchID == "slow" {
			return http.StatusOK, 1500 * time.Millisecond
		}
		return http.StatusOK, 0
	})
	s.post(t, arena("slow", "slow-start", 0, "MATCH_STARTED", "", `{"map":"dust"}`))
	s.post(t, arena("slow", "slow-fin", 10, "MATCH_FINISHED", "", `{"reason":"COMPLETED"}`))
	s.waitStatus(t, "arena-combat", "slow", store.Analyzing)

	start := time.Now()
	s.post(t, arena("fast", "fast-start", 0, "MATCH_STARTED", "", `{"map":"dust"}`))
	if code, _ := s.post(t, arena("fast", "fast-fin", 10, "MATCH_FINISHED", "", `{"reason":"COMPLETED"}`)); code != http.StatusAccepted {
		t.Fatalf("fast match rejected: %d", code)
	}
	if took := time.Since(start); took > 300*time.Millisecond {
		t.Fatalf("ingestion of another match took %s while slow analysis was running", took)
	}
	s.waitStatus(t, "arena-combat", "fast", store.Analyzed)
	if rec, _ := s.store.Get(eventKey("arena-combat", "slow")); rec.Status == store.Analyzed {
		t.Fatal("slow match finished first; test did not exercise concurrency")
	}
	s.waitStatus(t, "arena-combat", "slow", store.Analyzed)
}

func TestTransientAnalyticsErrorsAreRetried(t *testing.T) {
	s := newSystem(t, options{retries: 3})
	s.fake.setHandle(func(_ analysis.Request, n int) (int, time.Duration) {
		if n <= 2 {
			return http.StatusServiceUnavailable, 0
		}
		return http.StatusOK, 0
	})
	for _, line := range fixture(t, "F10-arena-zero-denominator") {
		s.post(t, line)
	}
	rec := s.waitStatus(t, "arena-combat", "match-f10", store.Analyzed)
	if rec.Attempts != 3 {
		t.Fatalf("want 3 attempts, got %d", rec.Attempts)
	}
	reqs := s.fake.byMatch("match-f10")
	if reqs[2].BatchID != "arena-combat:match-f10:3" {
		t.Fatalf("batch id should carry the attempt, got %s", reqs[2].BatchID)
	}
}

func TestFailedMatchCanBeReanalyzed(t *testing.T) {
	s := newSystem(t, options{retries: 1})
	s.fake.setHandle(func(analysis.Request, int) (int, time.Duration) { return http.StatusInternalServerError, 0 })
	for _, line := range fixture(t, "F10-arena-zero-denominator") {
		s.post(t, line)
	}
	rec := s.waitStatus(t, "arena-combat", "match-f10", store.Failed)
	if rec.Attempts != 2 || rec.LastError == "" {
		t.Fatalf("unexpected failed record %+v", rec)
	}
	if code, _ := s.get(t, "/v1/games/arena-combat/matches/match-f10/results"); code != http.StatusBadGateway {
		t.Fatalf("results of failed match: got %d, want 502", code)
	}

	s.fake.setHandle(nil)
	resp, err := http.Post(s.api.URL+"/v1/games/arena-combat/matches/match-f10/reanalyze", "application/json", nil)
	if err != nil || resp.StatusCode != http.StatusAccepted {
		t.Fatalf("reanalyze: %v %v", err, resp)
	}
	resp.Body.Close()
	rec = s.waitStatus(t, "arena-combat", "match-f10", store.Analyzed)
	if rec.Attempts != 3 || rec.LastError != "" {
		t.Fatalf("unexpected record after reanalysis %+v", rec)
	}
	resp, _ = http.Post(s.api.URL+"/v1/games/arena-combat/matches/match-f10/reanalyze", "application/json", nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("reanalyze of analyzed match: got %d, want 409", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestContractErrorsAreNotRetried(t *testing.T) {
	s := newSystem(t, options{retries: 3})
	s.fake.setHandle(func(analysis.Request, int) (int, time.Duration) { return http.StatusUnprocessableEntity, 0 })
	for _, line := range fixture(t, "F10-arena-zero-denominator") {
		s.post(t, line)
	}
	if rec := s.waitStatus(t, "arena-combat", "match-f10", store.Failed); rec.Attempts != 1 {
		t.Fatalf("4xx must not be retried, got %d attempts", rec.Attempts)
	}
}

func TestAPIKeyAndMatchSizeLimit(t *testing.T) {
	s := newSystem(t, options{apiKey: "secret", maxEvents: 2})
	start := arena("lim", "lim-1", 0, "MATCH_STARTED", "", `{"map":"dust"}`)
	if code, _ := s.post(t, start); code != http.StatusUnauthorized {
		t.Fatalf("without key: got %d, want 401", code)
	}
	if code, _ := s.post(t, start, "X-API-Key", "wrong"); code != http.StatusUnauthorized {
		t.Fatalf("wrong key: got %d, want 401", code)
	}
	s.post(t, start, "X-API-Key", "secret")
	s.post(t, arena("lim", "lim-2", 1, "PLAYER_JOINED", "p1", `{}`), "X-API-Key", "secret")
	code, body := s.post(t, arena("lim", "lim-3", 2, "PLAYER_JOINED", "p2", `{}`), "X-API-Key", "secret")
	if code != http.StatusRequestEntityTooLarge || body["code"] != "MATCH_TOO_LARGE" {
		t.Fatalf("over limit: got %d %v", code, body)
	}
	if code, _ := s.get(t, "/v1/matches"); code != http.StatusOK {
		t.Fatalf("reads must not require the key, got %d", code)
	}
}

// Al apagar, las partidas abiertas se envían como INCOMPLETE/SHUTDOWN.
func TestShutdownFlushesOpenMatches(t *testing.T) {
	s := newSystem(t, options{})
	lines := fixture(t, "F01-arena-normal")
	for _, line := range lines[:5] {
		s.post(t, line)
	}
	s.shutdown()
	reqs := s.fake.byMatch("match-a1")
	if len(reqs) != 1 || reqs[0].ClosedReason != "SHUTDOWN" || reqs[0].Completeness != "INCOMPLETE" || len(reqs[0].Events) != 5 {
		t.Fatalf("want one INCOMPLETE/SHUTDOWN batch of 5 events, got %+v", reqs)
	}
}

func eventKey(game, matchID string) event.Key { return event.Key{GameID: game, MatchID: matchID} }

// Tras un reinicio con RESULTS_DIR, una partida analizada no se reabre.
func TestRestoredMatchIsNotReopened(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.New(dir)
	k := eventKey("arena-combat", "match-a1")
	st.SetCollecting(k, store.Collecting, 18)
	st.SetResult(k, json.RawMessage(`{"resultVersion":"1"}`))

	restored, _ := store.New(dir)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	validator, _ := validation.Load(filepath.Join(contracts, "schemas"))
	pool := analysis.NewPool(analysis.PoolConfig{Workers: 1, QueueSize: 1}, analysis.NewClient("http://127.0.0.1:1", time.Second), restored, log)
	disp := dispatcher.New(dispatcher.Config{QueueSize: 8, MatchQueueSize: 8, TombstoneTTL: time.Hour,
		Match: match.Config{GracePeriod: time.Second, IdleTimeout: time.Second, MaxEvents: 100}}, pool, restored, log)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { disp.Run(ctx, ctx); close(done) }()
	defer func() { cancel(); <-done }()
	api := httptest.NewServer(httpapi.New(httpapi.Deps{Validator: validator, Dispatcher: disp, Pool: pool, Store: restored,
		Counters: &obs.Counters{}, Rejections: obs.NewRejections(10), EnqueueTimeout: time.Second, Log: log}).Handler())
	defer api.Close()

	s := &system{api: api, store: restored}
	if code, body := s.post(t, fixture(t, "F01-arena-normal")[0]); code != http.StatusConflict || body["code"] != "MATCH_CLOSED" {
		t.Fatalf("got %d %v, want 409 MATCH_CLOSED", code, body)
	}
	if rec, _ := restored.Get(k); rec.Status != store.Analyzed || string(rec.Result) != `{"resultVersion":"1"}` {
		t.Fatalf("restored result was modified: %+v", rec)
	}
}
