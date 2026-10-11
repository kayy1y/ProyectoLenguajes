package store

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"gamestats/ingestion-go/internal/event"
)

func TestResultsSurviveRestartWithResultsDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "carpeta con espacios")
	s, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	k := event.Key{GameID: "arena-combat", MatchID: "match-a1"}
	s.SetCollecting(k, Collecting, 18)
	s.SetClosed(k, "COMPLETE", "MATCH_FINISHED")
	s.StartAttempt(k, false)
	if err := s.SetResult(k, json.RawMessage(`{"resultVersion":"1"}`)); err != nil {
		t.Fatal(err)
	}
	failed := event.Key{GameID: "arena-combat", MatchID: "match-x"}
	s.SetFailed(failed, errors.New("boom"), "pending batch")

	reloaded, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	rec, ok := reloaded.Get(k)
	if !ok || rec.Status != Analyzed || string(rec.Result) != `{"resultVersion":"1"}` || rec.EventCount != 18 {
		t.Fatalf("analyzed result not restored: %+v", rec)
	}
	if _, ok := reloaded.Get(failed); ok {
		t.Fatal("FAILED matches live only in memory and must not be restored")
	}
}

func TestTakeFailedIsAtomic(t *testing.T) {
	s, _ := New("")
	k := event.Key{GameID: "g", MatchID: "m"}
	s.SetFailed(k, errors.New("boom"), "batch")
	if p, ok := s.TakeFailed(k); !ok || p != "batch" {
		t.Fatalf("first take: %v %v", p, ok)
	}
	if _, ok := s.TakeFailed(k); ok {
		t.Fatal("second take must fail: the batch was already handed out")
	}
}
