# GameStats

Plataforma multilenguaje para procesar y analizar eventos de videojuegos (curso Lenguajes de Programación).
**Go** (imperativo y concurrente) ingiere y coordina; **Scala** (funcional) calcula estadísticas y detecta patrones.

![CI](../../actions/workflows/ci.yml/badge.svg)

```
simulator ──POST /v1/events──▶ ingestion-go ──POST /v1/analyze──▶ analytics-scala
 (2 juegos, partidas            (valida, agrupa por partida,        (métricas por jugador, por partida,
  en paralelo)                   goroutine por partida, reintentos)  combinadas y patrones secuenciales)
                                       │
                 GET /v1/games/{gameId}/matches/{matchId}/results
```

| Carpeta | Qué contiene |
|---|---|
| [ingestion-go/](ingestion-go/) | Servicio Go: validación con JSON Schema, dispatcher, workers por partida, pool de análisis, store y API |
| [analytics-scala/](analytics-scala/) | Servicio Scala 3: núcleo puro y un analizador por juego (`arena-combat`, `street-racing`) |
| [simulator/](simulator/) | Productor externo: fixtures, escenarios `demo` y `stress` con semilla |
| [contracts/](contracts/) | Fuente de verdad: esquemas, ejemplos, fixtures y resultados esperados calculados a mano |
| [tests/e2e/](tests/e2e/) | Prueba de punta a punta sobre Docker Compose |
| [deploy/](deploy/) | Compose y `.env` de producción |
| [docs/](docs/) | Documentación técnica |

## Inicio rápido

Requisitos: Docker con Compose v2, `curl` y `jq`.

```bash
make up        # docker compose up -d --build --wait
make demo      # escenario demo: 5 partidas de 2 juegos en paralelo y casos especiales
make results   # resultado de match-a1
./scripts/demo.sh   # demo guiada paso a paso
make e2e       # prueba de punta a punta (23 comprobaciones)
make down
```

Sin `make`:
```bash
docker compose up -d --build --wait
docker compose --profile demo run --rm simulator --scenario demo --seed 42
curl -s localhost:8080/v1/games/arena-combat/matches/match-a1/results | jq '.result'
```

Desarrollo local: Go 1.26, JDK 21 y sbt 1.x.
```bash
make test      # go test -race (ingestion-go, simulator) + sbt test
```

## Qué calcula

| Juego | Por jugador | Por partida | Combinada | Patrón secuencial |
|---|---|---|---|---|
| `arena-combat` | ataques, aciertos, precisión, daño, eliminaciones, muertes, objetivos | duración, participantes, eliminaciones y daño totales, mejor jugador | `killEfficiency` = ELIMINATION ÷ ATTACK; `eliminationsPerMinute` | `KILL_STREAK`: 3+ eliminaciones sin morir |
| `street-racing` | vueltas válidas e inválidas, mejor vuelta y promedio, posición final | finalistas, abandonos, vuelta rápida, ganador | `positionsGained` = salida − llegada | `COMEBACK`: estar 3º o peor y ganar |

Las métricas que no se pueden calcular valen `null`; nunca se inventa un 0. La derivación de cada una está en [docs/statistics.md](docs/statistics.md).

## API (ingestion-go :8080)

| Ruta | Uso |
|---|---|
| `POST /v1/events` | Recibir un evento ([docs/event-spec.md](docs/event-spec.md)) |
| `GET /v1/matches` | Partidas y su estado |
| `GET /v1/games/{gameId}/matches/{matchId}[/results]` | Estado o resultado de una partida |
| `POST /v1/games/{gameId}/matches/{matchId}/reanalyze` | Reintentar una partida `FAILED` |
| `GET /v1/rejections`, `GET /v1/stats`, `GET /healthz` | Diagnóstico |

El detalle completo está en [docs/protocol.md](docs/protocol.md).

## Documentación

- [Arquitectura y concurrencia](docs/architecture.md)
- [Especificación de eventos](docs/event-spec.md)
- [Protocolo entre componentes](docs/protocol.md)
- [Estadísticas y patrones](docs/statistics.md)
- [Decisiones de arquitectura (ADR)](docs/decisions.md)
- [Pruebas](docs/testing.md)
- [Deployment](docs/deployment.md)
- [Guion de demo](docs/demo.md)
- [Defensa](docs/defense.md)

## CI/CD

| Workflow | Cuándo | Qué hace |
|---|---|---|
| `ci.yml` | push y PR | gofmt, vet, build, `go test -race`, `sbt test assembly`, build de las 3 imágenes, e2e con Compose |
| `release.yml` | push a `main` y tags `v*` | Publica imágenes amd64 y arm64 en GHCR |
| `deploy.yml` | manual | Despliega en la VM la etiqueta elegida y comprueba `/healthz` |
