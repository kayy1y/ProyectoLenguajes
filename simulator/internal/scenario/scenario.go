// Package scenario genera partidas deterministas: la misma semilla produce
// exactamente los mismos eventos. Los eventos describen hechos del juego; nunca
// llevan estadísticas precalculadas (eso lo calcula Scala).
package scenario

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Match es una partida lista para enviar.
type Match struct {
	GameID  string
	MatchID string
	// Lines son los eventos en orden de envío; At es su event-time (para el ritmo de envío).
	Lines [][]byte
	At    []time.Time
	// AfterAnalyzed se envía cuando la partida ya fue analizada (eventos tardíos).
	AfterAnalyzed [][]byte
	// Expect describe el resultado que se espera de la partida, para el resumen.
	Expect string
	// Open indica que la partida no envía MATCH_FINISHED (se cerrará por inactividad).
	Open bool
}

type event struct {
	EventID     string         `json:"eventId"`
	Timestamp   string         `json:"timestamp"`
	GameID      string         `json:"gameId"`
	GameVersion string         `json:"gameVersion"`
	MatchID     string         `json:"matchId"`
	PlayerID    string         `json:"playerId,omitempty"`
	Type        string         `json:"type"`
	Seq         int            `json:"seq"`
	Data        map[string]any `json:"data"`
	at          time.Time
}

type builder struct {
	game, match string
	events      []event
}

func (b *builder) add(at time.Time, typ, player string, data map[string]any) {
	b.events = append(b.events, event{GameID: b.game, GameVersion: "1.0", MatchID: b.match, PlayerID: player, Type: typ, Data: data, at: at})
}

// build ordena por event-time (estable) y asigna eventId y seq en ese orden.
func (b *builder) build() Match {
	sort.SliceStable(b.events, func(i, j int) bool { return b.events[i].at.Before(b.events[j].at) })
	m := Match{GameID: b.game, MatchID: b.match}
	for i := range b.events {
		e := &b.events[i]
		e.Seq = i + 1
		e.EventID = fmt.Sprintf("%s-e%03d", b.match, i+1)
		e.Timestamp = e.at.UTC().Format("2006-01-02T15:04:05.000Z")
		line, _ := json.Marshal(e)
		m.Lines = append(m.Lines, line)
		m.At = append(m.At, e.at)
	}
	return m
}

// Arena genera un deathmatch con reaparición: el daño acumulado sobre un jugador
// provoca su eliminación y vuelve con 100 de vida.
func Arena(rng *rand.Rand, matchID string, base time.Time, players, attacks int) Match {
	b := &builder{game: "arena-combat", match: matchID}
	ids := make([]string, players)
	health := map[string]int{}
	b.add(base, "MATCH_STARTED", "", map[string]any{"map": []string{"dust", "harbor", "temple"}[rng.IntN(3)]})
	for i := range ids {
		ids[i] = fmt.Sprintf("p%d", i+1)
		health[ids[i]] = 100
		b.add(base.Add(time.Second), "PLAYER_JOINED", ids[i], map[string]any{})
	}
	t := base.Add(5 * time.Second)
	for i := 0; i < attacks; i++ {
		t = t.Add(time.Duration(500+rng.IntN(2500)) * time.Millisecond)
		a := ids[rng.IntN(players)]
		target := ids[rng.IntN(players)]
		for target == a {
			target = ids[rng.IntN(players)]
		}
		hit := rng.Float64() < 0.6
		damage := 0
		if hit {
			damage = 20 + rng.IntN(41)
		}
		b.add(t, "ATTACK", a, map[string]any{"targetPlayerId": target, "weapon": []string{"rifle", "shotgun", "pistol"}[rng.IntN(3)], "hit": hit, "damage": damage})
		if hit {
			health[target] -= damage
			if health[target] <= 0 {
				b.add(t, "ELIMINATION", a, map[string]any{"victimPlayerId": target, "weapon": "rifle"})
				health[target] = 100
			}
		}
		if rng.Float64() < 0.04 {
			b.add(t, "OBJECTIVE_CAPTURED", ids[rng.IntN(players)], map[string]any{"objectiveId": fmt.Sprintf("flag-%c", 'A'+rng.IntN(3))})
		}
	}
	b.add(t.Add(5*time.Second), "MATCH_FINISHED", "", map[string]any{"reason": "COMPLETED"})
	m := b.build()
	m.Expect = "ANALYZED"
	return m
}

// Racing genera una carrera: cada piloto tiene un ritmo propio, las vueltas
// pueden ser inválidas, las posiciones cambian según el tiempo acumulado y
// algunos pilotos abandonan.
func Racing(rng *rand.Rand, matchID string, base time.Time, drivers, laps int) Match {
	b := &builder{game: "street-racing", match: matchID}
	b.add(base, "MATCH_STARTED", "", map[string]any{"track": []string{"monza", "imola", "suzuka"}[rng.IntN(3)], "totalLaps": laps})
	ids := make([]string, drivers)
	pace := map[string]int{}
	cum := map[string]int{}
	pos := map[string]int{}
	out := map[string]bool{}
	for i := range ids {
		ids[i] = fmt.Sprintf("d%d", i+1)
		pace[ids[i]] = 60000 + rng.IntN(3000)
		pos[ids[i]] = i + 1
		b.add(base.Add(time.Second), "PLAYER_JOINED", ids[i], map[string]any{"gridPosition": i + 1})
	}
	start := base.Add(2 * time.Second)
	at := func(ms int) time.Time { return start.Add(time.Duration(ms) * time.Millisecond) }
	for lap := 1; lap <= laps; lap++ {
		for _, d := range ids {
			if out[d] {
				continue
			}
			lapTime := pace[d] + rng.IntN(2500) - 1000
			cum[d] += lapTime
			b.add(at(cum[d]), "LAP_COMPLETED", d, map[string]any{"lapNumber": lap, "lapTimeMs": lapTime, "valid": rng.Float64() > 0.15})
			if lap < laps && rng.Float64() < 0.03 {
				out[d] = true
				b.add(at(cum[d]+500), "PLAYER_LEFT", d, map[string]any{"reason": "DISCONNECT"})
			}
		}
		for i, d := range ranking(ids, cum, out) {
			if pos[d] != i+1 {
				pos[d] = i + 1
				b.add(at(cum[d]), "POSITION_CHANGED", d, map[string]any{"position": i + 1, "lapNumber": lap})
			}
		}
	}
	last := 0
	for i, d := range ranking(ids, cum, out) {
		b.add(at(cum[d]), "PLAYER_FINISHED", d, map[string]any{"finalPosition": i + 1, "totalTimeMs": cum[d]})
		last = max(last, cum[d])
	}
	b.add(at(last+5000), "MATCH_FINISHED", "", map[string]any{"reason": "COMPLETED"})
	m := b.build()
	m.Expect = "ANALYZED"
	return m
}

func ranking(ids []string, cum map[string]int, out map[string]bool) []string {
	var active []string
	for _, d := range ids {
		if !out[d] {
			active = append(active, d)
		}
	}
	sort.SliceStable(active, func(i, j int) bool { return cum[active[i]] < cum[active[j]] })
	return active
}

// Fixture carga un fixture de contracts/fixtures tal cual (en su orden de envío).
func Fixture(dir, name, suffix string) (Match, error) {
	path := name
	if !strings.HasSuffix(path, ".jsonl") {
		path = filepath.Join(dir, name+".jsonl")
	}
	f, err := os.Open(path)
	if err != nil {
		return Match{}, err
	}
	defer f.Close()
	var m Match
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 64<<10)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var e event
		if err := json.Unmarshal(line, &e); err != nil {
			return Match{}, fmt.Errorf("%s: %w", path, err)
		}
		if suffix != "" {
			// Permite repetir una demo sin chocar con partidas ya cerradas.
			var raw map[string]any
			json.Unmarshal(line, &raw)
			raw["matchId"] = e.MatchID + suffix
			raw["eventId"] = e.EventID + suffix
			line, _ = json.Marshal(raw)
			e.MatchID += suffix
		}
		t, _ := time.Parse(time.RFC3339, e.Timestamp)
		m.GameID, m.MatchID = e.GameID, e.MatchID
		m.Lines = append(m.Lines, append([]byte(nil), line...))
		m.At = append(m.At, t)
	}
	m.Expect = "ANALYZED"
	return m, sc.Err()
}

// Demo arma el escenario de la defensa: partidas simultáneas de los dos juegos,
// eventos inválidos, duplicados, un evento tardío y una partida sin cierre.
func Demo(seed uint64, fixturesDir, suffix string) ([]Match, error) {
	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	base := time.Date(2026, 10, 1, 18, 0, 0, 0, time.UTC)
	f01, err := Fixture(fixturesDir, "F01-arena-normal", suffix)
	if err != nil {
		return nil, err
	}
	f17, err := Fixture(fixturesDir, "F17-racing-comeback", suffix)
	if err != nil {
		return nil, err
	}
	arena := Arena(rng, "demo-arena"+suffix, base, 4, 60)
	race := Racing(rng, "demo-race"+suffix, base, 5, 4)

	open := Arena(rng, "demo-open"+suffix, base, 3, 8)
	open.Lines, open.At = open.Lines[:len(open.Lines)-1], open.At[:len(open.At)-1] // sin MATCH_FINISHED
	open.Open, open.Expect = true, "ANALYZED (INCOMPLETE por inactividad)"

	// Casos especiales intercalados en la partida generada de arena, a partir de un ATTACK.
	mid := len(arena.Lines) / 2
	for !bytes.Contains(arena.Lines[mid], []byte(`"type":"ATTACK"`)) {
		mid++
	}
	first := arena.Lines[mid]
	inject := [][]byte{
		mutate(first, func(e map[string]any) { e["eventId"] = "bad-ts" + suffix; e["timestamp"] = "2026-10-01 18:00" }),
		mutate(first, func(e map[string]any) { e["eventId"] = "bad-type" + suffix; e["type"] = "DANCE" }),
		mutate(first, func(e map[string]any) { e["eventId"] = "no-player" + suffix; delete(e, "playerId") }),
		first, // duplicado exacto
		mutate(first, func(e map[string]any) { e["data"].(map[string]any)["weapon"] = "bazooka" }), // mismo id, otro contenido
	}
	mid++
	at := arena.At[mid]
	for i, line := range inject {
		arena.Lines = append(arena.Lines[:mid+i], append([][]byte{line}, arena.Lines[mid+i:]...)...)
		arena.At = append(arena.At[:mid+i], append([]time.Time{at}, arena.At[mid+i:]...)...)
	}
	f01.AfterAnalyzed = [][]byte{mutate(f01.Lines[4], func(e map[string]any) { e["eventId"] = e["eventId"].(string) + "-late" })}
	return []Match{f01, f17, arena, race, open}, nil
}

// Stress genera n partidas mezclando ambos juegos.
func Stress(seed uint64, n int) []Match {
	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	base := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	out := make([]Match, 0, n)
	for i := 0; i < n; i++ {
		if i%2 == 0 {
			out = append(out, Arena(rng, fmt.Sprintf("stress-arena-%d-%d", seed, i), base, 2+rng.IntN(5), 40+rng.IntN(80)))
		} else {
			out = append(out, Racing(rng, fmt.Sprintf("stress-race-%d-%d", seed, i), base, 3+rng.IntN(6), 2+rng.IntN(5)))
		}
	}
	return out
}

// WithSuffix devuelve la partida con matchId y eventId terminados en suffix, para
// poder repetir un escenario sin chocar con partidas ya cerradas.
func WithSuffix(m Match, suffix string) Match {
	if suffix == "" {
		return m
	}
	rewrite := func(line []byte) []byte {
		return mutate(line, func(e map[string]any) {
			e["matchId"] = e["matchId"].(string) + suffix
			e["eventId"] = e["eventId"].(string) + suffix
		})
	}
	out := m
	out.MatchID = m.MatchID + suffix
	out.Lines = nil
	for _, l := range m.Lines {
		out.Lines = append(out.Lines, rewrite(l))
	}
	out.AfterAnalyzed = nil
	for _, l := range m.AfterAnalyzed {
		out.AfterAnalyzed = append(out.AfterAnalyzed, rewrite(l))
	}
	return out
}

func mutate(line []byte, f func(map[string]any)) []byte {
	var e map[string]any
	json.Unmarshal(line, &e)
	f(e)
	out, _ := json.Marshal(e)
	return out
}
