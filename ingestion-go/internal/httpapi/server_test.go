package httpapi_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"gamestats/ingestion-go/internal/analysis"
	"gamestats/ingestion-go/internal/dispatcher"
	"gamestats/ingestion-go/internal/httpapi"
	"gamestats/ingestion-go/internal/store"
)

const contracts = "../../../contracts"

type fakeAnalytics struct {
	mu       sync.Mutex
	requests []analysis.Request
}

func (f *fakeAnalytics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req analysis.Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f.mu.Lock()
	f.requests = append(f.requests, req)
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, `{"resultVersion":"1","matchId":"`+req.MatchID+`"}`)
}

func newSystem(t *testing.T) (*httptest.Server, *store.Store, *fakeAnalytics) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	fake := &fakeAnalytics{}
	analytics := httptest.NewServer(fake)
	t.Cleanup(analytics.Close)

	st := store.New()
	jobs := make(chan analysis.Job, 8)
	disp := dispatcher.New(64, 64, jobs, st, log)
	pool := analysis.NewPool(analysis.NewClient(analytics.URL, time.Second), st, 2, log)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); disp.Run(ctx) }()
	go func() { defer wg.Done(); pool.Run(ctx, jobs) }()
	t.Cleanup(func() { cancel(); wg.Wait() })

	api := httptest.NewServer(httpapi.New(disp, st, time.Second, log).Handler())
	t.Cleanup(api.Close)
	return api, st, fake
}

func post(t *testing.T, url string, body []byte) (int, map[string]any) {
	t.Helper()
	resp, err := http.Post(url+"/v1/events", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(contracts, path))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestPostEventResponses(t *testing.T) {
	api, _, _ := newSystem(t)
	valid := readFile(t, "examples/valid/arena-attack-hit.json")

	if code, body := post(t, api.URL, valid); code != http.StatusAccepted || body["status"] != "ACCEPTED" {
		t.Fatalf("valid event: got %d %v", code, body)
	}
	if code, body := post(t, api.URL, valid); code != http.StatusOK || body["status"] != "DUPLICATE" {
		t.Fatalf("duplicate event: got %d %v", code, body)
	}

	cases := []struct{ file, code string }{
		{"examples/invalid/bad-timestamp.json", "INVALID_FORMAT"},
		{"examples/invalid/impossible-date.json", "INVALID_FORMAT"},
		{"examples/invalid/global-event-with-player.json", "FORBIDDEN_FIELD"},
		{"examples/invalid/player-event-without-player.json", "REQUIRED_FIELD"},
		{"examples/invalid/missing-event-id.json", "REQUIRED_FIELD"},
		{"examples/invalid/producer-ingest-field.json", "UNKNOWN_FIELD"},
		{"examples/invalid/not-json.txt", "MALFORMED_JSON"},
	}
	for _, c := range cases {
		code, body := post(t, api.URL, readFile(t, c.file))
		errs, _ := body["errors"].([]any)
		if code != http.StatusBadRequest || len(errs) == 0 || errs[0].(map[string]any)["code"] != c.code {
			t.Errorf("%s: got %d %v, want 400 %s", c.file, code, body, c.code)
		}
	}
}

func TestF01FlowsToAnalyticsInOrder(t *testing.T) {
	api, st, fake := newSystem(t)
	f, err := os.Open(filepath.Join(contracts, "fixtures/F01-arena-normal.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if code, body := post(t, api.URL, sc.Bytes()); code != http.StatusAccepted {
			t.Fatalf("got %d %v", code, body)
		}
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		resp, err := http.Get(api.URL + "/v1/games/arena-combat/matches/match-a1/results")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			break
		}
		if time.Now().After(deadline) {
			rec := st.List()
			t.Fatalf("match not analyzed in time, last status %d, store %+v", resp.StatusCode, rec)
		}
		time.Sleep(20 * time.Millisecond)
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.requests) != 1 {
		t.Fatalf("analytics received %d requests, want 1", len(fake.requests))
	}
	req := fake.requests[0]
	if len(req.Events) != 18 || req.Completeness != "COMPLETE" || req.BatchID != "arena-combat:match-a1:1" {
		t.Fatalf("unexpected request header fields: events=%d completeness=%s batch=%s", len(req.Events), req.Completeness, req.BatchID)
	}
	for i, e := range req.Events {
		want := "a1-e" + twoDigits(i+1)
		if e.EventID != want || e.Ingest == nil {
			t.Fatalf("event %d: got %s (ingest %v), want %s", i, e.EventID, e.Ingest, want)
		}
	}
	if late, body := post(t, api.URL, readFile(t, "examples/valid/arena-attack-hit.json")); late != http.StatusAccepted {
		t.Fatalf("event of another match must still be accepted, got %d %v", late, body)
	}
}

func twoDigits(n int) string { return string([]byte{byte('0' + n/10), byte('0' + n%10)}) }
