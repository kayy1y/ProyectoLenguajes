# Pruebas

| Nivel | Cantidad | Comando | Dónde |
|---|---|---|---|
| Go `ingestion-go` (unitarias y de sistema, con `-race`) | 19 | `cd ingestion-go && go test -race ./...` | `internal/*/*_test.go` |
| Go `simulator` | 3 | `cd simulator && go test -race ./...` | `internal/scenario` |
| Scala (golden, propiedades, unitarias, HTTP) | 42 | `cd analytics-scala && sbt test` | `src/test/scala/gamestats` |
| End-to-end sobre Docker Compose | 27 comprobaciones | `make e2e` o `./tests/e2e/e2e.sh` | `tests/e2e/e2e.sh` |

`make test` corre las suites de Go y Scala. CI (`.github/workflows/ci.yml`) corre todo lo anterior en cada push y PR.

## Matriz de casos

| Caso | Fixture o dato | Prueba | Esperado |
|---|---|---|---|
| Partida normal | F01 | Scala `GoldenSpec`, e2e paso 2 | Tabla de `docs/statistics.md` |
| Dos partidas del mismo juego en paralelo | generadas | Go `TestConcurrentMatchesKeepTheirOwnEvents`, e2e pasos 5 y 8 | Cada lote solo contiene sus eventos |
| Dos juegos en paralelo | F01 + F17 + generadas | e2e pasos 2 y 5 | Ambos analizados correctamente |
| Inválidos (sin `eventId`, timestamp, tipo, `data`…) | `examples/invalid` (17) | Go `TestInvalidExamplesMatchExpectedCodes`, `TestPostEventResponses`, e2e paso 3 | 400; el **primer** error tiene el código y el campo esperados |
| Duplicado idéntico y en conflicto | ejemplo válido, demo | Go `TestPostEventResponses`, e2e paso 5 | `200 DUPLICATE` y `409 DUPLICATE_ID_CONFLICT` |
| Fuera de orden y rezagado dentro de la gracia | F06 | Go `TestOutOfOrderAndGracePeriod`, Scala `GoldenSpec` | Resultado idéntico a F01 |
| Evento global sin `playerId` | ejemplos | Go `TestValidExamplesAreAccepted` | Aceptado; un `ATTACK` sin `playerId` da 400 |
| Partida sin `MATCH_FINISHED` | F01 recortado, demo-open | Go `TestIdleTimeoutAnalyzesIncompleteMatch`, Scala `EdgeCaseSpec`, e2e paso 5 | `INCOMPLETE`, `durationSeconds` y `eliminationsPerMinute` = `null` |
| Scala caído | F10 | Go `TestFailedMatchCanBeReanalyzed`, e2e paso 6 | `RETRYING` → `FAILED` (502) → `reanalyze` → `ANALYZED` |
| Error transitorio de Scala | F10 | Go `TestTransientAnalyticsErrorsAreRetried` | `ANALYZED` con `attempts` = 3 |
| Error de contrato (4xx) | F10 | Go `TestContractErrorsAreNotRetried` | `FAILED` con `attempts` = 1 |
| Reinicio de Go | F01 | Go `TestResultsSurviveRestartWithResultsDir`, `TestRestoredMatchIsNotReopened`, e2e paso 7 | El resultado persiste; un reenvío da 409 |
| Apagado ordenado | F01 parcial | Go `TestShutdownFlushesOpenMatches` | Lote `INCOMPLETE/SHUTDOWN` |
| Carreras de datos | — | todo Go con `-race` (suite repetida 5 veces sin fallos) | Sin reportes |
| Análisis lento aislado | — | Go `TestSlowAnalysisDoesNotBlockOtherMatches` | La otra partida responde en < 300 ms |
| Lote vacío o juego desconocido | — | Scala `EdgeCaseSpec` | 400 `EMPTY_BATCH`, 422 `UNSUPPORTED_GAME` |
| Métrica combinada con denominador cero | F10 | Scala `GoldenSpec`, e2e paso 6 | `killEfficiency` = `null` |
| Racha positiva y maximal | F11 | Scala `GoldenSpec`, `KillStreakSpec` | p1 de longitud 4 y p3 de longitud 3 |
| Racha negativa (muerte intermedia) | F12 | Scala `GoldenSpec`, `KillStreakSpec` | Sin patrón, aunque haya 3 eliminaciones |
| Eventos contradictorios | F13 | Scala `GoldenSpec` | 4 avisos; se excluyen 4 de 10 eventos |
| Remontada (segundo juego) | F17 / F18 | Scala `GoldenSpec`, `RacingSpec` | `COMEBACK` de d3; ninguna en F18 |
| Orden de llegada irrelevante | todos | Scala `PropertySpec` (50 permutaciones por fixture) | Mismo resultado |
| API key y límite de tamaño | — | Go `TestAPIKeyAndMatchSizeLimit` | 401 y `413 MATCH_TOO_LARGE` |
| Carga | 30 partidas, 2479 eventos | e2e paso 8 | Las 30 `ANALYZED` (unos 2000 eventos/s en local) |

## Datos deterministas

Los fixtures (`contracts/fixtures`) tienen fechas fijas y resultados calculados **a mano** (`contracts/expected`). Los escenarios generados usan semilla: `simulator --scenario demo --seed 42 --dry-run | shasum` da siempre el mismo hash.
