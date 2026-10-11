# Defensa: argumentos y dónde mostrarlos en el código

## Imperativo (Go) vs funcional (Scala)

| Tema | Go: imperativo | Scala: funcional |
|---|---|---|
| Estado | Mutable pero **confinado**: cada partida tiene una sola goroutine dueña (`internal/match/worker.go`) | Inmutable: `case class` + `copy`, sin `var` (`games/arena/ArenaAnalyzer.scala`, `Tally`) |
| Flujo | Bucle `for { select { … } }` con canal, temporizador de gracia y de inactividad | Pipeline `ordered andThen decode andThen semantics` (`core/Analysis.scala`) |
| Errores | Códigos HTTP, reintentos con backoff y estado `FAILED` (`internal/analysis`) | `Either[AnalysisError, …]` en un for-comprehension; `Option` en vez de `null`/NaN (`core/Fn.safeRatio`) |
| Por qué ese paradigma | La ingestión es I/O, tiempo y coordinación: controlar exactamente quién toca qué y cuándo | El análisis es una función pura de la partida: misma entrada, mismo resultado. Es fácil de probar y de reintentar |

## Técnicas funcionales (y dónde están)

| Técnica | Uso |
|---|---|
| `collect` | Filtrar y proyectar a la vez sobre un ADT (`Analysis.context`, `RacingAnalyzer.laps`) |
| `foldLeft` | Contadores por jugador en una pasada (`ArenaAnalyzer.tally`); estado de rachas sin variables (`KillStreak.detect`); reglas semánticas (`Analysis.semantics`) |
| `groupMapReduce`, `groupMap` | Conteo por clave (`Fn.countBy`); vueltas por piloto (`RacingAnalyzer`) |
| `partition`, `partitionMap` | Vueltas válidas e inválidas; eventos decodificados frente a avisos |
| `sortBy` con tupla | Orden total `(timestamp, seq, ingest.seq)` |
| `Option`, `minOption`, `maxOption` | Métricas no calculables → `null` |
| Composición con `andThen` | Etapas del pipeline |
| Transparencia referencial | `PropertySpec`: 50 permutaciones de llegada por fixture dan el mismo resultado |

## Concurrencia genuina (no decorativa)

- **Topología:** un dispatcher (dueño del mapa), un worker por partida y un pool de análisis. Ver la tabla en `docs/architecture.md` §2.
- **Pruebas:**
  - `TestConcurrentMatchesKeepTheirOwnEvents`: 4 partidas × 4 productores concurrentes, cada lote recibe exactamente sus eventos.
  - `TestSlowAnalysisDoesNotBlockOtherMatches`: un análisis lento de una partida no frena la ingestión de otra.
  - `go test -race` en CI.
- **Carga:** el e2e envía 30 partidas (2479 eventos) en paralelo, a unos 2000 eventos/s en local.
- **Backpressure:** el envío no bloqueante al worker devuelve 503 solo a la partida saturada.

## La regla secuencial no es un conteo

- `KILL_STREAK`: F01 y F12 tienen el mismo número de eliminaciones de p1 (3). En F12 hay una muerte en medio, así que no hay racha.
- `COMEBACK`: terminar 1º no basta. Hay que haber estado 3º o peor **antes** de llegar (`RacingSpec`).

## Contratos

- JSON Schema en `contracts/`. Go lo usa para validar, y los mismos ejemplos son pruebas en Go, Scala y e2e.
- Errores con código y campo (`docs/event-spec.md` §6). La prueba exige que el primer error sea el motivo real.
- Versionado: `/v1` en la ruta y `gameVersion` por juego.

## Extensibilidad

- `street-racing` se agregó sin tocar Go ni el núcleo de Scala, salvo una línea del registro.
- Se ve con `git show --stat <commit feat(racing)>`.

## DevOps

- `ci.yml` (gofmt, vet, build, test con `-race`, sbt test, build de imágenes y e2e) → `release.yml` (imágenes multi-arch a GHCR) → `deploy.yml` o pasos manuales.
- Dockerfiles multi-stage con compilación cruzada; imágenes finales distroless (Go, unos 15 MB) y JRE alpine (Scala).
- Healthchecks, `depends_on: service_healthy`, volumen para resultados y API key en producción.

## Preguntas probables

| Pregunta | Respuesta corta |
|---|---|
| ¿Por qué no Kafka? | REST cumple el requisito con menos infraestructura. Los reintentos y la idempotencia cubren las fallas de la demo (ADR-01) |
| ¿Qué pasa si `MATCH_FINISHED` llega antes que otros eventos? | Hay un período de gracia (2 s). Lo que llega después recibe `409 LATE_EVENT` y queda visible en `/v1/rejections` |
| ¿Y si llega dos veces el mismo `eventId`? | Si el contenido es idéntico, `200 DUPLICATE` (idempotente). Si cambia, `409 DUPLICATE_ID_CONFLICT` |
| ¿Go calcula algo? | Solo contadores de ingestión para diagnóstico. Las métricas de juego son 100 % de Scala; Go guarda el resultado como bytes opacos |
| ¿Qué se pierde si se reinicia? | `docs/architecture.md` §6 |
| ¿Cómo se agrega un juego? | Un esquema, un `GameAnalyzer` y una línea en el registro |
