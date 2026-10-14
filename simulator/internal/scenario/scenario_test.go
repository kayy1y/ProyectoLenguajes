package scenario

import (
	"bytes"
	"encoding/json"
	"math/rand/v2"
	"strings"
	"testing"
	"time"
)

func TestSameSeedSameEvents(t *testing.T) {
	a, _ := Demo(42, "../../../contracts/fixtures", "")
	b, _ := Demo(42, "../../../contracts/fixtures", "")
	c, _ := Demo(7, "../../../contracts/fixtures", "")
	if !equal(a, b) {
		t.Fatal("same seed must produce identical events")
	}
	if equal(a, c) {
		t.Fatal("different seeds should produce different generated matches")
	}
}

func TestGeneratedMatchesAreWellFormed(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for _, m := range []Match{Arena(rng, "a", base, 4, 100), Racing(rng, "r", base, 6, 5)} {
		ids := map[string]bool{}
		var prev time.Time
		for i, line := range m.Lines {
			var e struct {
				EventID, Type, Timestamp, PlayerID string
				Seq                                int
			}
			if err := json.Unmarshal(line, &e); err != nil {
				t.Fatal(err)
			}
			ts, err := time.Parse(time.RFC3339, e.Timestamp)
			if err != nil || ts.Before(prev) {
				t.Fatalf("%s: event %d out of order or bad timestamp %q", m.MatchID, i, e.Timestamp)
			}
			prev = ts
			if ids[e.EventID] || e.Seq != i+1 {
				t.Fatalf("%s: duplicate id or wrong seq at %d", m.MatchID, i)
			}
			ids[e.EventID] = true
			global := e.Type == "MATCH_STARTED" || e.Type == "MATCH_FINISHED"
			if global == (e.PlayerID != "") {
				t.Fatalf("%s: playerId rule broken for %s", m.MatchID, e.Type)
			}
		}
		if !bytes.Contains(m.Lines[0], []byte("MATCH_STARTED")) || !bytes.Contains(m.Lines[len(m.Lines)-1], []byte("MATCH_FINISHED")) {
			t.Fatalf("%s: must start with MATCH_STARTED and end with MATCH_FINISHED", m.MatchID)
		}
	}
}

func TestDemoContainsSpecialCases(t *testing.T) {
	ms, err := Demo(42, "../../../contracts/fixtures", "")
	if err != nil {
		t.Fatal(err)
	}
	games := map[string]bool{}
	var open, late int
	for _, m := range ms {
		games[m.GameID] = true
		if m.Open {
			open++
		}
		late += len(m.AfterAnalyzed)
	}
	all := bytes.Join(ms[2].Lines, nil)
	for _, want := range []string{`"bad-ts"`, `"DANCE"`, `"no-player"`, `"bazooka"`} {
		if !bytes.Contains(all, []byte(want)) {
			t.Errorf("demo arena match lacks special case %s", want)
		}
	}
	if len(games) != 2 || open != 1 || late != 1 {
		t.Fatalf("games=%v open=%d late=%d", games, open, late)
	}
}

func equal(a, b []Match) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !bytes.Equal(bytes.Join(a[i].Lines, nil), bytes.Join(b[i].Lines, nil)) {
			return false
		}
	}
	return true
}

func TestWithSuffixRewritesIDs(t *testing.T) {
	ms := Stress(7, 2)
	got := WithSuffix(ms[0], "-x1")
	if got.MatchID != ms[0].MatchID+"-x1" || len(got.Lines) != len(ms[0].Lines) {
		t.Fatalf("matchId o cantidad de eventos incorrectos: %s", got.MatchID)
	}
	var first map[string]any
	if err := json.Unmarshal(got.Lines[0], &first); err != nil {
		t.Fatal(err)
	}
	if first["matchId"] != got.MatchID || !strings.HasSuffix(first["eventId"].(string), "-x1") {
		t.Fatalf("el evento no lleva el sufijo: %v", first)
	}
	if WithSuffix(ms[0], "").MatchID != ms[0].MatchID {
		t.Fatal("sufijo vacío no debe cambiar nada")
	}
}
