// Command simulator es el productor externo de eventos de GameStats.
//
// Vertical slice: reproduce un fixture JSONL (contracts/fixtures) evento por evento.
// En la fase 6 se agregan escenarios generados con semilla y envío concurrente.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func main() {
	target := flag.String("target", env("TARGET_URL", "http://localhost:8080"), "URL base del servicio de ingestión")
	fixtures := flag.String("fixtures-dir", env("FIXTURES_DIR", "../contracts/fixtures"), "directorio de fixtures")
	fixture := flag.String("fixture", "", "nombre del fixture (p. ej. F01-arena-normal) o ruta a un .jsonl")
	apiKey := flag.String("api-key", env("INGEST_API_KEY", ""), "valor de X-API-Key, si el servidor lo exige")
	flag.Parse()

	if *fixture == "" {
		fmt.Fprintln(os.Stderr, "uso: simulator --fixture F01-arena-normal [--target URL]")
		os.Exit(2)
	}
	path := *fixture
	if !strings.HasSuffix(path, ".jsonl") {
		path = filepath.Join(*fixtures, path+".jsonl")
	}
	if err := replay(path, *target, *apiKey, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func replay(path, target, apiKey string, out io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	counts := map[string]int{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), 64<<10)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		req, err := http.NewRequest(http.MethodPost, target+"/v1/events", bytes.NewReader(line))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		if apiKey != "" {
			req.Header.Set("X-API-Key", apiKey)
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("send event: %w", err)
		}
		var ack struct {
			Status, Code, EventID string
		}
		json.NewDecoder(resp.Body).Decode(&ack)
		resp.Body.Close()
		label := fmt.Sprintf("%d %s %s", resp.StatusCode, ack.Status, ack.Code)
		counts[strings.TrimSpace(label)]++
		fmt.Fprintf(out, "%-10s %d %s %s\n", ack.EventID, resp.StatusCode, ack.Status, ack.Code)
	}
	if err := sc.Err(); err != nil {
		return err
	}

	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Fprintln(out, "--- resumen")
	for _, k := range keys {
		fmt.Fprintf(out, "%4d  %s\n", counts[k], k)
	}
	return nil
}
