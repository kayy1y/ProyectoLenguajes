package validation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const contracts = "../../../contracts"

func load(t *testing.T) *Registry {
	t.Helper()
	r, err := Load(filepath.Join(contracts, "schemas"))
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestValidExamplesAreAccepted(t *testing.T) {
	r := load(t)
	files, _ := filepath.Glob(filepath.Join(contracts, "examples/valid/*.json"))
	if len(files) == 0 {
		t.Fatal("no valid examples found")
	}
	for _, f := range files {
		body, _ := os.ReadFile(f)
		e, errs := r.Parse(body)
		if len(errs) > 0 {
			t.Errorf("%s: unexpected errors %+v", filepath.Base(f), errs)
		}
		if e.Time.IsZero() || len(e.Data) == 0 {
			t.Errorf("%s: event not fully decoded: %+v", filepath.Base(f), e)
		}
	}
}

// Cada ejemplo inválido debe rechazarse con el código y el campo de expected.json.
func TestInvalidExamplesMatchExpectedCodes(t *testing.T) {
	r := load(t)
	raw, err := os.ReadFile(filepath.Join(contracts, "examples/invalid/expected.json"))
	if err != nil {
		t.Fatal(err)
	}
	var expected map[string]struct{ Code, Field string }
	if err := json.Unmarshal(raw, &expected); err != nil {
		t.Fatal(err)
	}
	for name, want := range expected {
		body, _ := os.ReadFile(filepath.Join(contracts, "examples/invalid", name))
		_, errs := r.Parse(body)
		// El primer error es el que ve el productor: tiene que ser el motivo real.
		if len(errs) == 0 || errs[0].Code != want.Code || errs[0].Field != want.Field {
			t.Errorf("%s: want first error %s at %q, got %+v", name, want.Code, want.Field, errs)
		}
		for _, e := range errs {
			if e.Code == "UNKNOWN_FIELD" && envelopeFields[e.Field] {
				t.Errorf("%s: spurious %+v", name, e)
			}
		}
	}
}

func TestFixturesAreValid(t *testing.T) {
	r := load(t)
	files, _ := filepath.Glob(filepath.Join(contracts, "fixtures/*.jsonl"))
	for _, f := range files {
		body, _ := os.ReadFile(f)
		for i, line := range splitLines(body) {
			if _, errs := r.Parse(line); len(errs) > 0 {
				t.Errorf("%s line %d: %+v", filepath.Base(f), i+1, errs)
			}
		}
	}
}

func splitLines(b []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, c := range b {
		if c == '\n' {
			if i > start {
				out = append(out, b[start:i])
			}
			start = i + 1
		}
	}
	return out
}
