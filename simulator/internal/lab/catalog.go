// Package lab reúne lo que usa la aplicación de escritorio: catálogo de
// contracts/, generación de datos con el simulador y la suite de verificación.
package lab

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type FixtureInfo struct {
	Name        string `json:"name"`
	GameID      string `json:"gameId"`
	MatchID     string `json:"matchId"`
	Events      int    `json:"events"`
	Description string `json:"description"`
}

type InvalidExample struct {
	File  string `json:"file"`
	Code  string `json:"code"`
	Field string `json:"field"`
}

type GameInfo struct {
	GameID     string   `json:"gameId"`
	Versions   []string `json:"versions"`
	EventTypes []string `json:"eventTypes"`
}

type ScenarioInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Catalog describe lo que hay en contracts/: fixtures, escenarios, ejemplos y juegos.
type Catalog struct {
	Fixtures  []FixtureInfo    `json:"fixtures"`
	Scenarios []ScenarioInfo   `json:"scenarios"`
	Valid     []string         `json:"validExamples"`
	Invalid   []InvalidExample `json:"invalidExamples"`
	Games     []GameInfo       `json:"games"`
}

// readIndex lee contracts/fixtures/index.json (nombre -> datos).
func readIndex(dir string) (map[string]struct {
	GameID      string `json:"gameId"`
	MatchID     string `json:"matchId"`
	Events      int    `json:"events"`
	Description string `json:"description"`
}, error) {
	var idx map[string]struct {
		GameID      string `json:"gameId"`
		MatchID     string `json:"matchId"`
		Events      int    `json:"events"`
		Description string `json:"description"`
	}
	b, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil {
		return nil, err
	}
	return idx, json.Unmarshal(b, &idx)
}

// BuildCatalog lee contracts/. Agregar un juego (esquema + fixture) no requiere cambiar esta app.
func BuildCatalog(cfg Config) (*Catalog, error) {
	idx, err := readIndex(cfg.FixturesDir)
	if err != nil {
		return nil, err
	}
	c := &Catalog{
		Scenarios: []ScenarioInfo{
			{"demo", "Cinco partidas de dos juegos en paralelo, eventos inválidos, un duplicado, un conflicto, un evento tardío y una partida sin cierre."},
			{"stress", "N partidas generadas con semilla, mezclando ambos juegos, enviadas en paralelo."},
		},
	}
	names := make([]string, 0, len(idx))
	for n := range idx {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		f := idx[n]
		c.Fixtures = append(c.Fixtures, FixtureInfo{n, f.GameID, f.MatchID, f.Events, f.Description})
	}
	c.Valid = listFiles(filepath.Join(cfg.ExamplesDir, "valid"))
	if b, err := os.ReadFile(filepath.Join(cfg.ExamplesDir, "invalid", "expected.json")); err == nil {
		var want map[string]struct{ Code, Field string }
		if json.Unmarshal(b, &want) == nil {
			keys := make([]string, 0, len(want))
			for k := range want {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				c.Invalid = append(c.Invalid, InvalidExample{k, want[k].Code, want[k].Field})
			}
		}
	}
	c.Games = readGames(filepath.Join(cfg.SchemasDir, "games"))
	return c, nil
}

func listFiles(dir string) []string {
	entries, _ := os.ReadDir(dir)
	var out []string
	for _, e := range entries {
		if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, e.Name())
		}
	}
	return out
}

// readGames descubre los juegos soportados leyendo contracts/schemas/games/*.schema.json:
// agregar un juego es agregar su esquema, la consola no cambia.
func readGames(dir string) []GameInfo {
	files, _ := filepath.Glob(filepath.Join(dir, "*.schema.json"))
	sort.Strings(files)
	var out []GameInfo
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var s struct {
			AllOf []struct {
				Properties struct {
					GameID      struct{ Const string }  `json:"gameId"`
					GameVersion struct{ Enum []string } `json:"gameVersion"`
					Type        struct{ Enum []string } `json:"type"`
				} `json:"properties"`
			} `json:"allOf"`
		}
		if json.Unmarshal(b, &s) != nil {
			continue
		}
		for _, a := range s.AllOf {
			if a.Properties.GameID.Const != "" {
				out = append(out, GameInfo{a.Properties.GameID.Const, a.Properties.GameVersion.Enum, a.Properties.Type.Enum})
			}
		}
	}
	return out
}
