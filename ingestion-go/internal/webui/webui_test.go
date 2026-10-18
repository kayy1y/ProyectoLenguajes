package webui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const contractsDir = "../../../contracts"

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestMountServesConsoleAndKeepsAPI(t *testing.T) {
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-From", "api")
		w.WriteHeader(http.StatusTeapot)
	})
	h := Mount(api, contractsDir)

	if rec := get(t, h, "/"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<title>GameStats</title>") {
		t.Fatalf("GET / = %d, quiero la página de la consola", rec.Code)
	}
	for _, f := range []string{"/ui/app.js", "/ui/style.css", "/ui/icon.svg"} {
		if rec := get(t, h, f); rec.Code != http.StatusOK || rec.Body.Len() == 0 {
			t.Errorf("GET %s = %d", f, rec.Code)
		}
	}
	// Todo lo que no es la consola sigue llegando a la API sin cambios.
	for _, p := range []string{"/healthz", "/v1/matches", "/v1/stats", "/no-existe"} {
		if rec := get(t, h, p); rec.Header().Get("X-From") != "api" {
			t.Errorf("GET %s no llegó a la API", p)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", nil))
	if rec.Header().Get("X-From") != "api" {
		t.Error("POST / debe seguir llegando a la API")
	}
}

func TestContractsBundle(t *testing.T) {
	rec := get(t, Handler(contractsDir), "/ui/contracts.json")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /ui/contracts.json = %d: %s", rec.Code, rec.Body)
	}
	var c contracts
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	if len(c.Fixtures) != 8 || len(c.Invalid) != 17 || len(c.Valid) == 0 {
		t.Fatalf("fixtures=%d invalid=%d valid=%d; quiero 8, 17 y al menos uno", len(c.Fixtures), len(c.Invalid), len(c.Valid))
	}
	for _, f := range c.Fixtures {
		if len(f.Events) == 0 || len(f.Expected) == 0 || f.GameID == "" || f.MatchID == "" {
			t.Errorf("fixture %s incompleto", f.Name)
		}
	}
	for _, e := range c.Invalid {
		if e.Code == "" || e.Body == "" {
			t.Errorf("inválido %s sin código o sin cuerpo", e.Name)
		}
	}
}

func TestContractsMissingIsReported(t *testing.T) {
	rec := get(t, Handler(t.TempDir()), "/ui/contracts.json")
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "CONTRACTS_UNAVAILABLE") {
		t.Fatalf("sin contracts/ quiero 503 CONTRACTS_UNAVAILABLE, obtuve %d %s", rec.Code, rec.Body)
	}
	if rec := get(t, Handler(t.TempDir()), "/"); rec.Code != http.StatusOK {
		t.Fatalf("la página debe cargar aunque falten los contratos: %d", rec.Code)
	}
}
