package event

import (
	"testing"
	"time"
)

func TestSortTotal(t *testing.T) {
	base := time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC)
	seq := func(n int64) *int64 { return &n }
	mk := func(id string, sec int, s *int64, arrival int64) Event {
		return Event{EventID: id, Time: base.Add(time.Duration(sec) * time.Second), Seq: s, Ingest: &Ingest{Seq: arrival}}
	}
	// Llegada desordenada; mismo timestamp desempatado por seq y, sin seq, por llegada.
	events := []Event{
		mk("late-no-seq-b", 10, nil, 5),
		mk("t10-seq2", 10, seq(2), 1),
		mk("t0", 0, seq(9), 4),
		mk("late-no-seq-a", 10, nil, 3),
		mk("t10-seq1", 10, seq(1), 2),
	}
	SortTotal(events)
	want := []string{"t0", "t10-seq1", "t10-seq2", "late-no-seq-a", "late-no-seq-b"}
	for i, e := range events {
		if e.EventID != want[i] {
			t.Fatalf("position %d: got %s, want %s", i, e.EventID, want[i])
		}
	}
}
