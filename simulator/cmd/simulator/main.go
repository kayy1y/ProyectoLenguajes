// Command simulator es el productor externo de eventos de GameStats.
//
//	simulator --fixture F01-arena-normal            reproduce un fixture
//	simulator --fixture all --expect                todos los fixtures y compara con contracts/expected
//	simulator --scenario demo --seed 42             escenario de la demo (dos juegos en paralelo y casos especiales)
//	simulator --scenario stress --matches 50        carga concurrente
//	simulator --scenario demo --dry-run             imprime los eventos (JSONL) sin enviarlos
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"net/http"

	"gamestats/simulator/internal/scenario"
	"gamestats/simulator/internal/sender"
	"gamestats/simulator/internal/verify"
)

func main() {
	target := flag.String("target", env("TARGET_URL", "http://localhost:8080"), "URL base del servicio de ingestión")
	apiKey := flag.String("api-key", env("INGEST_API_KEY", ""), "valor de X-API-Key, si el servidor lo exige")
	fixturesDir := flag.String("fixtures-dir", env("FIXTURES_DIR", "../contracts/fixtures"), "directorio de fixtures")
	fixture := flag.String("fixture", "", "fixture(s) separados por coma, o 'all'")
	expect := flag.Bool("expect", false, "con --fixture: compara cada resultado con contracts/expected")
	name := flag.String("scenario", env("SCENARIO", ""), "escenario: demo | stress")
	seed := flag.Uint64("seed", 42, "semilla de los generadores")
	matches := flag.Int("matches", 20, "partidas del escenario stress")
	speed := flag.Float64("speed", 50, "compresión del tiempo de juego")
	maxGap := flag.Duration("max-gap", 150*time.Millisecond, "espera máxima entre dos eventos de una partida")
	suffix := flag.String("suffix", "", "sufijo para matchId/eventId (repetir una demo sin reiniciar el servidor)")
	dryRun := flag.Bool("dry-run", false, "imprime los eventos en JSONL y no envía nada")
	wait := flag.Duration("wait", 90*time.Second, "tiempo máximo esperando resultados (0 = no esperar)")
	verbose := flag.Bool("verbose", false, "imprime la respuesta de cada evento")
	flag.Parse()

	var ms []scenario.Match
	var err error
	switch {
	case *fixture != "":
		ms, err = loadFixtures(*fixturesDir, *fixture, *suffix)
	case *name == "demo":
		ms, err = scenario.Demo(*seed, *fixturesDir, *suffix)
	case *name == "stress":
		ms = scenario.Stress(*seed, *matches)
	default:
		fmt.Fprintln(os.Stderr, "uso: simulator --fixture F01-arena-normal | --fixture all --expect | --scenario demo|stress [--seed N]")
		os.Exit(2)
	}
	if err != nil {
		fail(err)
	}
	if *dryRun {
		for _, m := range ms {
			for _, l := range m.Lines {
				fmt.Println(string(l))
			}
		}
		return
	}

	cfg := sender.Config{Target: strings.TrimRight(*target, "/"), APIKey: *apiKey, Speed: *speed, MaxGap: *maxGap,
		Verbose: *verbose, Out: os.Stdout, Client: &http.Client{Timeout: 10 * time.Second}, PollWait: 250 * time.Millisecond}
	ctx := context.Background()
	total := 0
	for _, m := range ms {
		total += len(m.Lines)
	}
	fmt.Printf("enviando %d eventos de %d partidas en paralelo a %s\n", total, len(ms), cfg.Target)
	start := time.Now()
	rep, err := sender.Send(ctx, cfg, ms)
	if err != nil {
		fail(err)
	}
	took := time.Since(start)
	fmt.Printf("enviados en %s (%.0f eventos/s)\n", took.Round(time.Millisecond), float64(total)/took.Seconds())
	if *wait == 0 {
		printCounts(rep)
		return
	}

	waitCtx, cancel := context.WithTimeout(ctx, *wait)
	defer cancel()
	results := sender.WaitResults(waitCtx, cfg, ms)
	if err := sender.SendLate(ctx, cfg, ms, rep); err != nil {
		fail(err)
	}
	printCounts(rep)
	ok := printResults(ms, results)
	if *expect {
		ok = compareExpected(*fixturesDir, *suffix, ms, results) && ok
	}
	if !ok {
		os.Exit(1)
	}
}

func loadFixtures(dir, list, suffix string) ([]scenario.Match, error) {
	names := strings.Split(list, ",")
	if list == "all" {
		var idx map[string]any
		b, err := os.ReadFile(filepath.Join(dir, "index.json"))
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &idx); err != nil {
			return nil, err
		}
		names = sender.SortedKeys(idx)
	}
	var out []scenario.Match
	for _, n := range names {
		m, err := scenario.Fixture(dir, strings.TrimSpace(n), suffix)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func printCounts(rep *sender.Report) {
	counts := rep.Counts()
	fmt.Println("--- respuestas de ingestión")
	for _, k := range sender.SortedKeys(counts) {
		fmt.Printf("%5d  %s\n", counts[k], strings.TrimSpace(k))
	}
}

func printResults(ms []scenario.Match, results map[string]sender.MatchRecord) bool {
	fmt.Println("--- resultados")
	ok := true
	for _, m := range ms {
		r := results[m.MatchID]
		var res struct {
			MatchStats struct {
				Metrics map[string]any
			}
			Patterns []struct{ Type, PlayerID string }
			Warnings []any
		}
		json.Unmarshal(r.Result, &res)
		var pats []string
		for _, p := range res.Patterns {
			pats = append(pats, p.Type+":"+p.PlayerID)
		}
		metrics, _ := json.Marshal(res.MatchStats.Metrics)
		fmt.Printf("%-14s %-24s %-9s %-10s eventos=%-4d avisos=%d patrones=%v\n               %s\n",
			m.GameID, m.MatchID, r.Status, r.Completeness, r.EventCount, len(res.Warnings), pats, metrics)
		if r.Status != "ANALYZED" {
			ok = false
			fmt.Printf("               ERROR: %s %s\n", r.Status, r.LastError)
		}
	}
	return ok
}

// compareExpected verifica que el resultado de cada fixture sea idéntico al calculado a mano.
func compareExpected(fixturesDir, suffix string, ms []scenario.Match, results map[string]sender.MatchRecord) bool {
	fmt.Println("--- comparación con contracts/expected")
	ok := true
	for _, m := range ms {
		c := verify.Expected(fixturesDir, suffix, m.GameID, m.MatchID, results[m.MatchID].Result)
		if c.Pass {
			fmt.Printf("ok   %s\n", c.Name)
			continue
		}
		ok = false
		if c.Detail != "" {
			fmt.Printf("FAIL %s: %s\n", c.Name, c.Detail)
		} else {
			fmt.Printf("FAIL %s\n  esperado: %s\n  obtenido: %s\n", c.Name, c.Expected, c.Actual)
		}
	}
	return ok
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
