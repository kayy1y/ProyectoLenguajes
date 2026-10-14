package lab

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gamestats/simulator/internal/verify"
)

func TestHumanizeAndValue(t *testing.T) {
	for in, want := range map[string]string{"killEfficiency": "Kill efficiency", "totalDamage": "Total damage", "dnf": "Dnf", "best_lap": "Best lap"} {
		if got := Humanize(in); got != want {
			t.Errorf("Humanize(%q) = %q, want %q", in, got, want)
		}
	}
	if Value(nil) != "—" || Value(0.6667) != "0.6667" || Value(float64(3)) != "3" || Value("p1") != "p1" {
		t.Fatal("Value formatea mal")
	}
}

func TestColumnsComeFromDataForAnyGame(t *testing.T) {
	var r Result
	json.Unmarshal([]byte(`{"playerStats":[{"playerId":"a","metrics":{"zeta":1,"alpha":null}},{"playerId":"b","metrics":{"beta":2}}],
		"matchStats":{"durationSeconds":null,"participants":2,"totalEvents":9,"metrics":{"winner":"a"}}}`), &r)
	if got := strings.Join(MetricColumns(&r), ","); got != "alpha,zeta,beta" {
		t.Fatalf("columnas = %s", got)
	}
	m := MatchMetrics(&r)
	if m[0][1] != "—" || m[3] != [2]string{"Winner", "a"} {
		t.Fatalf("métricas de partida = %v", m)
	}
}

func TestPatternExplainAndReport(t *testing.T) {
	p := Pattern(map[string]any{"type": "KILL_STREAK", "playerId": "p1", "length": float64(3)})
	if p != "KILL_STREAK — length: 3 · player id: p1" {
		t.Fatalf("Pattern = %q", p)
	}
	if Explain("409 REJECTED LATE_EVENT") == "" || Explain("400 REJECTED INVALID_FORMAT /timestamp") == "" {
		t.Fatal("Explain sin texto")
	}
	j := &Job{StartedAt: time.Now(), Checks: []verify.Check{{Group: "fixtures", Name: "F01", Pass: true}, {Group: "inválidos", Name: "x", Expected: "400 A /b", Actual: "400 A /c"}}}
	rep := Report(j)
	if !strings.Contains(rep, "**1/2**") || !strings.Contains(rep, "| FAIL | x |") {
		t.Fatalf("informe:\n%s", rep)
	}
}
