package lab

import (
	"os"
	"path/filepath"
)

// FindContracts busca la carpeta contracts/ subiendo desde cada punto de partida
// (directorio actual y carpeta del ejecutable), para que la app funcione tanto con
// `go run` como con el binario copiado a bin/.
func FindContracts(starts ...string) (string, bool) {
	for _, s := range starts {
		dir, err := filepath.Abs(s)
		if err != nil {
			continue
		}
		for {
			c := filepath.Join(dir, "contracts")
			if _, err := os.Stat(filepath.Join(c, "fixtures", "index.json")); err == nil {
				return c, true
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return "", false
}

// FromContracts arma la configuración a partir de la carpeta contracts/.
func FromContracts(target, apiKey, contracts string) Config {
	return Config{Target: target, APIKey: apiKey,
		FixturesDir: filepath.Join(contracts, "fixtures"),
		ExamplesDir: filepath.Join(contracts, "examples"),
		SchemasDir:  filepath.Join(contracts, "schemas")}
}
