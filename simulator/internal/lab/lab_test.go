package lab

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

const contracts = "../../../contracts"

// fakeIngestion imita la API de ingestión: acepta eventos y devuelve, para la
// partida match-a1 (con cualquier sufijo), el resultado esperado de F01.
func fakeIngestion(t *testing.T) (*httptest.Server, *[]http.Header) {
	t.Helper()
	var mu sync.Mutex
	var seen []http.Header
	ids := regexp.MustCompile(`"((?:match-a1|a1-e\d+))"`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Clone())
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/events":
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `{"status":"ACCEPTED","eventId":"x"}`)
		case strings.HasSuffix(r.URL.Path, "/results") && strings.Contains(r.URL.Path, "/match-a1"):
			suffix := strings.TrimPrefix(strings.Split(r.URL.Path, "/")[5], "match-a1")
			b, err := os.ReadFile(contracts + "/expected/F01-arena-normal.result.json")
			if err != nil {
				t.Error(err)
			}
			res := ids.ReplaceAll(b, []byte(`"$1`+suffix+`"`))
			io.WriteString(w, `{"status":"ANALYZED","completeness":"COMPLETE","eventCount":18,"attempts":1,"result":`+string(res)+`}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `{"code":"MATCH_NOT_FOUND"}`)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func newLab(target, key string) *Lab { return New(FromContracts(target, key, contracts)) }

func waitJob(t *testing.T, j *Job) *Job {
	t.Helper()
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if v := j.View(); v.State != "running" {
			return v
		}
	}
	t.Fatal("el trabajo no terminó")
	return nil
}

func TestCatalogIsDiscoveredFromContracts(t *testing.T) {
	c, err := BuildCatalog(FromContracts("", "", contracts))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Fixtures) < 8 || len(c.Invalid) != 17 || len(c.Valid) == 0 {
		t.Fatalf("catálogo incompleto: %d fixtures, %d inválidos, %d válidos", len(c.Fixtures), len(c.Invalid), len(c.Valid))
	}
	games := map[string]GameInfo{}
	for _, g := range c.Games {
		games[g.GameID] = g
	}
	if len(games["arena-combat"].EventTypes) < 5 || len(games["street-racing"].EventTypes) < 5 {
		t.Fatalf("juegos mal descubiertos: %+v", c.Games)
	}
}

func TestFindContractsWalksUp(t *testing.T) {
	dir, ok := FindContracts("does-not-exist-xyz", ".")
	if !ok || !strings.HasSuffix(dir, "contracts") {
		t.Fatalf("no encontró contracts/ subiendo desde el paquete: %q %v", dir, ok)
	}
}

func TestRunFixtureJobComparesAgainstExpectedWithSuffix(t *testing.T) {
	up, seen := fakeIngestion(t)
	l := newLab(up.URL, "secreto")
	for i := 0; i < 2; i++ { // dos ejecuciones seguidas: cada una usa su propio sufijo
		j, err := l.StartRun(RunRequest{Kind: "fixtures", Names: []string{"F01-arena-normal"}, Speed: 5000})
		if err != nil {
			t.Fatal(err)
		}
		v := waitJob(t, j)
		if v.State != "done" || len(v.Matches) != 1 {
			t.Fatalf("trabajo: %+v", v)
		}
		m := v.Matches[0]
		if m.Status != "ANALYZED" || m.Check == nil || !m.Check.Pass {
			t.Fatalf("la comparación debía pasar tras quitar el sufijo: %+v / %+v", m, m.Check)
		}
		if v.Sent != 18 || v.Counts["202 ACCEPTED"] != 18 {
			t.Fatalf("progreso incorrecto: sent=%d counts=%v", v.Sent, v.Counts)
		}
	}
	if got := (*seen)[0].Get("X-API-Key"); got != "secreto" {
		t.Fatalf("la clave no llegó a la ingestión: %q", got)
	}
}

func TestRunRejectsBadRequests(t *testing.T) {
	up, _ := fakeIngestion(t)
	l := newLab(up.URL, "")
	for _, req := range []RunRequest{{Kind: "nada"}, {Kind: "fixtures"}, {Kind: "fixtures", Names: []string{"../x"}},
		{Kind: "stress", Matches: 0}, {Kind: "stress", Matches: 9999}} {
		if _, err := l.StartRun(req); err == nil {
			t.Fatalf("%+v debía rechazarse", req)
		}
	}
}
