// Package validation valida eventos contra el JSON Schema de contracts/schemas.
//
// Proceso: JSON bien formado -> sobre común (event.v1) -> juego registrado ->
// esquema del juego (games/<gameId>.v1). Los errores del validador se traducen a
// los códigos documentados en docs/event-spec.md.
package validation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"gamestats/ingestion-go/internal/event"
)

type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Los esquemas se registran bajo una URL ficticia para que las referencias relativas
// (../event.v1.schema.json) se resuelvan sin depender de la ruta real en disco.
const baseURL = "https://gamestats.local/schemas/"

var printer = message.NewPrinter(language.English)

type Registry struct {
	envelope *jsonschema.Schema
	games    map[string]*jsonschema.Schema
}

// Load compila el sobre común y todos los esquemas de juego de dir.
func Load(dir string) (*Registry, error) {
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	var gameFiles []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".schema.json") {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		rel = filepath.ToSlash(rel)
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		doc, err := jsonschema.UnmarshalJSON(f)
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		if strings.HasPrefix(rel, "games/") {
			gameFiles = append(gameFiles, rel)
		}
		return c.AddResource(baseURL+rel, doc)
	})
	if err != nil {
		return nil, fmt.Errorf("load schemas from %s: %w", dir, err)
	}
	r := &Registry{games: make(map[string]*jsonschema.Schema)}
	if r.envelope, err = c.Compile(baseURL + "event.v1.schema.json"); err != nil {
		return nil, err
	}
	for _, rel := range gameFiles {
		gameID := strings.TrimSuffix(strings.TrimPrefix(rel, "games/"), ".v1.schema.json")
		if r.games[gameID], err = c.Compile(baseURL + rel); err != nil {
			return nil, err
		}
	}
	if len(r.games) == 0 {
		return nil, fmt.Errorf("no game schemas found in %s/games", dir)
	}
	return r, nil
}

// Games devuelve los juegos registrados, ordenados.
func (r *Registry) Games() []string {
	out := make([]string, 0, len(r.games))
	for g := range r.games {
		out = append(out, g)
	}
	sort.Strings(out)
	return out
}

// Parse valida y decodifica un evento. Si hay errores, el evento devuelto solo
// sirve para registrar el rechazo (eventId, matchId si se pudieron leer).
func (r *Registry) Parse(body []byte) (event.Event, []FieldError) {
	var e event.Event
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(body))
	if err != nil {
		return e, []FieldError{{Code: "MALFORMED_JSON", Message: err.Error()}}
	}
	_ = json.Unmarshal(body, &e) // mejor esfuerzo, para identificar el rechazo

	if err := r.envelope.Validate(inst); err != nil {
		return e, translate(err)
	}
	game, ok := r.games[e.GameID]
	if !ok {
		return e, []FieldError{{Field: "/gameId", Code: "UNKNOWN_GAME", Message: fmt.Sprintf("game %q is not registered; known games: %s", e.GameID, strings.Join(r.Games(), ", "))}}
	}
	if err := game.Validate(inst); err != nil {
		return e, translate(err)
	}

	// El esquema ya garantizó formato, tipos y fechas válidas.
	e.Time, _ = time.Parse(time.RFC3339, e.Timestamp)
	var compact bytes.Buffer
	if err := json.Compact(&compact, e.Data); err == nil {
		e.Data = compact.Bytes()
	}
	return e, nil
}

func translate(err error) []FieldError {
	verr, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return []FieldError{{Code: "INVALID_EVENT", Message: err.Error()}}
	}
	var out []FieldError
	seen := map[string]bool{}
	var walk func(v *jsonschema.ValidationError)
	walk = func(v *jsonschema.ValidationError) {
		if len(v.Causes) > 0 {
			for _, c := range v.Causes {
				walk(c)
			}
			return
		}
		for _, fe := range leaf(v) {
			if k := fe.Field + "|" + fe.Code; !seen[k] {
				seen[k] = true
				out = append(out, fe)
			}
		}
	}
	walk(verr)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Field < out[j].Field })
	return out
}

func leaf(v *jsonschema.ValidationError) []FieldError {
	loc := "/" + strings.Join(v.InstanceLocation, "/")
	if loc == "/" {
		loc = ""
	}
	msg := v.ErrorKind.LocalizedString(printer)
	one := func(code string) []FieldError { return []FieldError{{Field: loc, Code: code, Message: msg}} }
	switch k := v.ErrorKind.(type) {
	case *kind.Required:
		out := make([]FieldError, 0, len(k.Missing))
		for _, m := range k.Missing {
			out = append(out, FieldError{Field: loc + "/" + m, Code: "REQUIRED_FIELD", Message: "is required"})
		}
		return out
	case *kind.AdditionalProperties:
		out := make([]FieldError, 0, len(k.Properties))
		for _, p := range k.Properties {
			out = append(out, FieldError{Field: loc + "/" + p, Code: "UNKNOWN_FIELD", Message: "is not part of the contract"})
		}
		return out
	case *kind.FalseSchema:
		// unevaluatedProperties: false aplica un esquema false a cada campo sobrante.
		return []FieldError{{Field: loc, Code: "UNKNOWN_FIELD", Message: "is not part of the contract"}}
	case *kind.Not:
		// Los campos prohibidos se declaran con {"not": {}} ($defs/Forbidden).
		return one("FORBIDDEN_FIELD")
	case *kind.Type:
		return one("INVALID_TYPE")
	case *kind.Format, *kind.Pattern:
		return one("INVALID_FORMAT")
	case *kind.Minimum, *kind.Maximum, *kind.ExclusiveMinimum, *kind.ExclusiveMaximum, *kind.MinLength, *kind.MaxLength:
		return one("OUT_OF_RANGE")
	case *kind.Enum, *kind.Const:
		switch loc {
		case "/type":
			return one("UNKNOWN_EVENT_TYPE")
		case "/gameVersion":
			return one("UNSUPPORTED_GAME_VERSION")
		case "/gameId":
			return one("UNKNOWN_GAME")
		}
		return one("INVALID_VALUE")
	default:
		return one("INVALID_VALUE")
	}
}
