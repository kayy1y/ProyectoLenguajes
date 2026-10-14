package lab

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// Humanize convierte una clave de métrica en un título: "killEfficiency" -> "Kill efficiency".
func Humanize(key string) string {
	var b strings.Builder
	for i, r := range key {
		switch {
		case r == '_' || r == '-':
			b.WriteRune(' ')
		case i > 0 && unicode.IsUpper(r):
			b.WriteRune(' ')
			b.WriteRune(unicode.ToLower(r))
		case i == 0:
			b.WriteRune(unicode.ToUpper(r))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Value formatea un valor de métrica. nil significa "no calculable": se muestra "—",
// nunca un 0 inventado.
func Value(v any) string {
	switch x := v.(type) {
	case nil:
		return "—"
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case string:
		return x
	default:
		return fmt.Sprint(x)
	}
}

// MetricColumns devuelve la unión ordenada (por primera aparición) de las claves
// de metrics de todos los jugadores: las columnas salen de los datos, no del juego.
func MetricColumns(r *Result) []string {
	seen := map[string]bool{}
	var cols []string
	for _, p := range r.PlayerStats {
		keys := make([]string, 0, len(p.Metrics))
		for k := range p.Metrics {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if !seen[k] {
				seen[k] = true
				cols = append(cols, k)
			}
		}
	}
	return cols
}

// MatchMetrics une las métricas comunes y las propias del juego, ordenadas.
func MatchMetrics(r *Result) [][2]string {
	out := [][2]string{
		{"Duración (s)", Value(r.MatchStats.DurationSeconds)},
		{"Participantes", Value(r.MatchStats.Participants)},
		{"Eventos", Value(r.MatchStats.TotalEvents)},
	}
	keys := make([]string, 0, len(r.MatchStats.Metrics))
	for k := range r.MatchStats.Metrics {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, [2]string{Humanize(k), Value(r.MatchStats.Metrics[k])})
	}
	return out
}

// Pattern describe un patrón en una línea: "KILL_STREAK — player id: p1 · length: 3 · …".
func Pattern(p map[string]any) string {
	keys := make([]string, 0, len(p))
	for k := range p {
		if k != "type" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, strings.ToLower(Humanize(k))+": "+Value(p[k]))
	}
	return Value(p["type"]) + " — " + strings.Join(parts, " · ")
}

var explain = map[string]string{
	"202 ACCEPTED":               "Evento aceptado",
	"200 DUPLICATE":              "Duplicado idéntico (idempotente, no se cuenta dos veces)",
	"409 DUPLICATE_ID_CONFLICT":  "Mismo eventId con contenido distinto",
	"409 LATE_EVENT":             "Llegó después de enviar la partida a analizar",
	"409 MATCH_CLOSED":           "La partida ya estaba cerrada",
	"409 MATCH_ALREADY_STARTED":  "Segundo MATCH_STARTED",
	"409 MATCH_ALREADY_FINISHED": "Segundo MATCH_FINISHED",
	"413 MATCH_TOO_LARGE":        "La partida superó el límite de eventos",
	"413 EVENT_TOO_LARGE":        "Evento mayor a 64 KB",
	"503 BACKPRESSURE":           "Cola de la partida llena: reintentar",
}

// Explain da el significado de una respuesta de la ingestión ("409 REJECTED LATE_EVENT").
func Explain(key string) string {
	k := strings.Replace(key, " REJECTED", "", 1)
	if e, ok := explain[k]; ok {
		return e
	}
	if strings.HasPrefix(key, "400 ") {
		return "Evento inválido: rechazado con campo y código"
	}
	return ""
}

// Report genera el informe Markdown de una verificación (evidencia para docs/evidence/).
func Report(j *Job) string {
	ok := 0
	for _, c := range j.Checks {
		if c.Pass {
			ok++
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Informe de verificación de GameStats\n\nFecha: %s\n\nResultado: **%d/%d** comprobaciones pasaron.\n",
		j.StartedAt.Format("2006-01-02 15:04:05"), ok, len(j.Checks))
	group := ""
	for _, c := range j.Checks {
		if c.Group != group {
			group = c.Group
			fmt.Fprintf(&b, "\n## %s\n\n| Estado | Comprobación | Esperado | Obtenido |\n|---|---|---|---|\n", GroupTitle(group))
		}
		state := "FAIL"
		if c.Pass {
			state = "PASS"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", state, c.Name, cell(c.Expected), cell(c.Actual))
	}
	return b.String()
}

func cell(s string) string {
	if len(s) > 60 {
		return "(JSON)"
	}
	return "`" + strings.ReplaceAll(s, "|", `\|`) + "`"
}

// GroupTitle es el título legible de un grupo de comprobaciones.
func GroupTitle(g string) string {
	switch g {
	case "fixtures":
		return "Casos de prueba contra resultados calculados a mano"
	case "inválidos":
		return "Eventos inválidos (código y campo del primer error)"
	case "comportamiento":
		return "Comportamiento del sistema"
	}
	return g
}

// Example lee un ejemplo del contrato (valid/… o invalid/…).
func (js *Lab) Example(kind, file string) ([]byte, error) {
	if (kind != "valid" && kind != "invalid") || file != filepath.Base(file) {
		return nil, fmt.Errorf("ejemplo inválido")
	}
	return os.ReadFile(filepath.Join(js.cfg.ExamplesDir, kind, file))
}
