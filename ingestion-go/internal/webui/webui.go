// Package webui sirve la consola web de GameStats: una página estática que corre
// en el navegador del evaluador y usa solo la API pública (/v1/..., /healthz),
// igual que la aplicación de escritorio. No participa en la ingestión ni en el
// análisis: Go sigue ingiriendo y Scala sigue calculando.
package webui

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

//go:embed static
var static embed.FS

// Mount devuelve un handler que atiende la consola en "/" y "/ui/" y delega todo
// lo demás en api. contractsDir es la carpeta contracts/ (fixtures, expected y
// examples); si falta, la consola funciona igual pero sin casos de prueba.
func Mount(api http.Handler, contractsDir string) http.Handler {
	ui := Handler(contractsDir)
	root := http.NewServeMux()
	root.Handle("/", api)
	root.Handle("GET /{$}", ui)
	root.Handle("GET /ui/", ui)
	return root
}

// Handler sirve la página, sus archivos y el paquete de contratos.
func Handler(contractsDir string) http.Handler {
	files, _ := fs.Sub(static, "static")
	bundle := sync.OnceValues(func() ([]byte, error) { return Bundle(contractsDir) })

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		noCache(w)
		http.ServeFileFS(w, r, files, "index.html")
	})
	mux.Handle("GET /ui/", http.StripPrefix("/ui/", noCacheHandler(http.FileServerFS(files))))
	mux.HandleFunc("GET /ui/contracts.json", func(w http.ResponseWriter, r *http.Request) {
		b, err := bundle()
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"code": "CONTRACTS_UNAVAILABLE", "message": err.Error()})
			return
		}
		noCache(w)
		w.Write(b)
	})
	return mux
}

func noCache(w http.ResponseWriter) { w.Header().Set("Cache-Control", "no-cache") }

func noCacheHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { noCache(w); h.ServeHTTP(w, r) })
}

// Fixture es un caso de prueba listo para enviar desde el navegador.
type Fixture struct {
	Name        string            `json:"name"`
	GameID      string            `json:"gameId"`
	MatchID     string            `json:"matchId"`
	Description string            `json:"description"`
	Events      []json.RawMessage `json:"events"`
	Expected    json.RawMessage   `json:"expected,omitempty"`
}

// Example es un ejemplo del contrato. Body va como texto porque algunos
// inválidos ni siquiera son JSON (not-json.txt).
type Example struct {
	Name  string `json:"name"`
	Body  string `json:"body"`
	Code  string `json:"code,omitempty"`
	Field string `json:"field,omitempty"`
}

type contracts struct {
	Fixtures []Fixture `json:"fixtures"`
	Valid    []Example `json:"valid"`
	Invalid  []Example `json:"invalid"`
}

// Bundle reúne en un solo JSON los fixtures (en su orden de envío), sus resultados
// esperados y los ejemplos válidos e inválidos con el error documentado.
func Bundle(contractsDir string) ([]byte, error) {
	fixturesDir := filepath.Join(contractsDir, "fixtures")
	var index map[string]struct {
		File, GameID, MatchID, Expected, Description string
	}
	if err := readJSON(filepath.Join(fixturesDir, "index.json"), &index); err != nil {
		return nil, err
	}
	var c contracts
	for _, name := range sortedKeys(index) {
		info := index[name]
		f := Fixture{Name: name, GameID: info.GameID, MatchID: info.MatchID, Description: info.Description}
		raw, err := os.ReadFile(filepath.Join(fixturesDir, info.File))
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				f.Events = append(f.Events, json.RawMessage(line))
			}
		}
		if info.Expected != "" {
			if f.Expected, err = os.ReadFile(filepath.Join(fixturesDir, info.Expected)); err != nil {
				return nil, err
			}
		}
		c.Fixtures = append(c.Fixtures, f)
	}

	examples := filepath.Join(contractsDir, "examples")
	valid, err := filepath.Glob(filepath.Join(examples, "valid", "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(valid)
	for _, p := range valid {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		c.Valid = append(c.Valid, Example{Name: filepath.Base(p), Body: string(b)})
	}

	var want map[string]struct{ Code, Field string }
	if err := readJSON(filepath.Join(examples, "invalid", "expected.json"), &want); err != nil {
		return nil, err
	}
	for _, name := range sortedKeys(want) {
		b, err := os.ReadFile(filepath.Join(examples, "invalid", name))
		if err != nil {
			return nil, err
		}
		c.Invalid = append(c.Invalid, Example{Name: name, Body: string(b), Code: want[name].Code, Field: want[name].Field})
	}
	return json.Marshal(c)
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
