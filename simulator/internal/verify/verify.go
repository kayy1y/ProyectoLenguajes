// Package verify compara los resultados del sistema con los calculados a mano
// (contracts/expected) y los rechazos con los códigos documentados
// (contracts/examples/invalid/expected.json). Devuelve resultados estructurados
// para que los use tanto el CLI como la consola web.
package verify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

// Check es el resultado de una comprobación.
type Check struct {
	Group    string `json:"group"`
	Name     string `json:"name"`
	Pass     bool   `json:"pass"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

type fixtureInfo struct {
	GameID   string `json:"gameId"`
	MatchID  string `json:"matchId"`
	Expected string `json:"expected"`
}

// Expected compara el resultado obtenido de una partida con su archivo esperado.
// Si la partida se envió con un sufijo (matchId y eventId terminan en él), se
// quita antes de comparar, de modo que la comprobación sirve en cada repetición.
func Expected(fixturesDir, suffix, gameID, matchID string, got json.RawMessage) Check {
	base := strings.TrimSuffix(matchID, suffix)
	c := Check{Group: "fixtures", Name: base}
	file, ok := findExpected(fixturesDir, gameID, base)
	if !ok {
		c.Detail = "no hay resultado esperado para " + gameID + "/" + base
		return c
	}
	want, err := os.ReadFile(file)
	if err != nil {
		c.Detail = err.Error()
		return c
	}
	if len(got) == 0 {
		c.Detail = "la partida no tiene resultado"
		return c
	}
	c.Name = filepath.Base(file)
	norm := Normalize(got, suffix)
	var w, g any
	if err := json.Unmarshal(want, &w); err != nil {
		c.Detail = err.Error()
		return c
	}
	if err := json.Unmarshal(norm, &g); err != nil {
		c.Detail = err.Error()
		return c
	}
	c.Expected, c.Actual = compact(want), compact(norm)
	c.Pass = reflect.DeepEqual(w, g)
	return c
}

// Normalize quita el sufijo de los identificadores del resultado ("match-a1-x" -> "match-a1").
func Normalize(result json.RawMessage, suffix string) json.RawMessage {
	if suffix == "" {
		return result
	}
	return bytes.ReplaceAll(result, []byte(suffix+`"`), []byte(`"`))
}

// Has indica si existe un resultado esperado para la partida base (sin sufijo).
func Has(fixturesDir, gameID, matchID string) bool {
	_, ok := findExpected(fixturesDir, gameID, matchID)
	return ok
}

func findExpected(fixturesDir, gameID, matchID string) (string, bool) {
	var idx map[string]fixtureInfo
	b, err := os.ReadFile(filepath.Join(fixturesDir, "index.json"))
	if err != nil || json.Unmarshal(b, &idx) != nil {
		return "", false
	}
	for _, f := range idx {
		if f.GameID == gameID && f.MatchID == matchID {
			return filepath.Join(fixturesDir, f.Expected), true
		}
	}
	return "", false
}

func compact(b []byte) string {
	var out bytes.Buffer
	if json.Compact(&out, b) != nil {
		return string(b)
	}
	return out.String()
}

// Rejection es lo que el servicio de ingestión respondió a un evento inválido.
type Rejection struct {
	HTTP        int
	Code, Field string
}

// Invalid envía cada ejemplo de examplesDir/invalid con post y comprueba que el
// primer error sea el código y el campo documentados en expected.json.
func Invalid(examplesDir string, post func(body []byte) (Rejection, error)) ([]Check, error) {
	dir := filepath.Join(examplesDir, "invalid")
	b, err := os.ReadFile(filepath.Join(dir, "expected.json"))
	if err != nil {
		return nil, err
	}
	var want map[string]struct{ Code, Field string }
	if err := json.Unmarshal(b, &want); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(want))
	for n := range want {
		names = append(names, n)
	}
	sort.Strings(names)
	var out []Check
	for _, n := range names {
		w := want[n]
		c := Check{Group: "inválidos", Name: n, Expected: fmt.Sprintf("400 %s %s", w.Code, w.Field)}
		body, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			c.Detail = err.Error()
			out = append(out, c)
			continue
		}
		r, err := post(body)
		if err != nil {
			c.Detail = err.Error()
			out = append(out, c)
			continue
		}
		c.Actual = fmt.Sprintf("%d %s %s", r.HTTP, r.Code, r.Field)
		c.Pass = r.HTTP == 400 && r.Code == w.Code && r.Field == w.Field
		out = append(out, c)
	}
	return out, nil
}
