# contracts/ — fuente de verdad compartida

Go, Scala, el simulador, las pruebas y la documentación se apoyan en estos archivos. Un cambio de contrato modifica **en el mismo commit** el esquema, los ejemplos, los fixtures, los resultados esperados y `docs/`.

| Ruta | Contenido | Quién lo usa |
|---|---|---|
| `schemas/event.v1.schema.json` | Sobre común de todos los eventos | Go (validación), docs |
| `schemas/games/<gameId>.v1.schema.json` | Catálogo y carga por tipo de cada juego | Go (validación) |
| `schemas/analyze-request.v1.schema.json` | Cuerpo de Go → Scala | Pruebas de contrato Go y Scala |
| `schemas/analysis-result.v1.schema.json` | Resultado de Scala | Pruebas de contrato y e2e |
| `examples/valid/` | Un evento válido por tipo | Test de validación en Go |
| `examples/invalid/` + `expected.json` | Eventos inválidos y el código/campo de error esperado | Test de validación en Go |
| `examples/protocol/analyze-request.F01.json` | Petición Go → Scala completa para F01 | Test golden de Go (genera) y de Scala (consume) |
| `fixtures/*.jsonl` + `index.json` | Partidas deterministas, un evento por línea **en orden de envío** | Simulador (`--fixture`), tests de Scala, e2e |
| `expected/*.result.json` | Resultado esperado de cada fixture, calculado a mano | Tests golden de Scala, e2e |

Convenciones:
- Los esquemas no declaran `$id`; las referencias son relativas al archivo (`../event.v1.schema.json`). El validador debe cargarlos desde disco con su ruta real. **Cuidado:** la ruta del proyecto tiene espacios, que en una URI `file://` aparecen como `%20`.
- Para prohibir un campo se usa `{"not": {}}` (`$defs/Forbidden`) en lugar de `false`, porque así los validadores indican la ruta del error.
- Las fechas de los fixtures son fijas (2026-10-01T15:00:00Z como base) y cada fixture usa un `matchId` distinto, para poder enviarlos todos al mismo servidor.
- Las métricas no calculables valen `null`. El resultado de Scala no lleva timestamps de análisis, así que la comparación con `expected/` es exacta.
