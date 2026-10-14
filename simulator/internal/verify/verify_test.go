package verify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func fixtures(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "index.json"), []byte(`{"F1":{"gameId":"g","matchId":"m1","expected":"F1.result.json"}}`), 0o644)
	os.WriteFile(filepath.Join(dir, "F1.result.json"), []byte(`{"matchId":"m1","patterns":[{"fromEventId":"e7"}],"n":1.5}`), 0o644)
	return dir
}

func TestExpectedMatchesWithAndWithoutSuffix(t *testing.T) {
	dir := fixtures(t)
	if c := Expected(dir, "", "g", "m1", json.RawMessage(`{"n":1.5,"patterns":[{"fromEventId":"e7"}],"matchId":"m1"}`)); !c.Pass {
		t.Fatalf("sin sufijo debía coincidir: %+v", c)
	}
	got := json.RawMessage(`{"matchId":"m1-r2","patterns":[{"fromEventId":"e7-r2"}],"n":1.5}`)
	if c := Expected(dir, "-r2", "g", "m1-r2", got); !c.Pass {
		t.Fatalf("con sufijo debía coincidir tras normalizar: %+v", c)
	}
}

func TestExpectedDetectsDifferences(t *testing.T) {
	dir := fixtures(t)
	c := Expected(dir, "", "g", "m1", json.RawMessage(`{"matchId":"m1","patterns":[],"n":2}`))
	if c.Pass || c.Expected == "" || c.Actual == "" {
		t.Fatalf("debía fallar y traer esperado y obtenido: %+v", c)
	}
	if c := Expected(dir, "", "g", "desconocida", json.RawMessage(`{}`)); c.Pass || c.Detail == "" {
		t.Fatalf("una partida sin esperado debe fallar con detalle: %+v", c)
	}
	if c := Expected(dir, "", "g", "m1", nil); c.Pass {
		t.Fatalf("sin resultado debe fallar: %+v", c)
	}
}

func TestInvalidComparesCodeAndField(t *testing.T) {
	dir := t.TempDir()
	inv := filepath.Join(dir, "invalid")
	os.MkdirAll(inv, 0o755)
	os.WriteFile(filepath.Join(inv, "expected.json"), []byte(`{"a.json":{"code":"X","field":"/f"},"b.json":{"code":"Y","field":"/g"}}`), 0o644)
	os.WriteFile(filepath.Join(inv, "a.json"), []byte(`{}`), 0o644)
	os.WriteFile(filepath.Join(inv, "b.json"), []byte(`{}`), 0o644)
	n := 0
	checks, err := Invalid(dir, func([]byte) (Rejection, error) {
		n++
		if n == 1 {
			return Rejection{400, "X", "/f"}, nil
		}
		return Rejection{400, "Z", "/g"}, nil
	})
	if err != nil || len(checks) != 2 {
		t.Fatalf("err=%v checks=%v", err, checks)
	}
	if !checks[0].Pass || checks[1].Pass {
		t.Fatalf("a debía pasar y b fallar: %+v", checks)
	}
}
