# GameStats

Plataforma multilenguaje para procesamiento y análisis de eventos de videojuegos (curso Lenguajes de Programación).

- **ingestion-go/** — Go imperativo: recibe, valida, agrupa por partida y coordina de forma concurrente.
- **analytics-scala/** — Scala funcional: calcula estadísticas y detecta patrones secuenciales.
- **simulator/** — Productor externo de eventos deterministas.
- **contracts/** — Fuente de verdad: esquemas JSON, ejemplos, fixtures y resultados esperados.

```
simulator ──POST /v1/events──▶ ingestion-go ──POST /v1/analyze──▶ analytics-scala
                                   │
              GET /v1/games/{gameId}/matches/{matchId}/results
```

> Estado: **Fase 2 (vertical slice)**. El recorrido completo funciona, pero Scala solo calcula `eliminations` por jugador. El plan completo está en [docs/PLAN.md](docs/PLAN.md) y las decisiones en [docs/decisions.md](docs/decisions.md).

## Requisitos de entorno

| Herramienta | Versión verificada localmente |
|---|---|
| Go | 1.26.5 |
| JDK | 24 (se recomienda una LTS: 21 o 25) |
| sbt | pendiente de instalar (`brew install sbt`) |
| Docker / Compose | Compose v5.1.0 |

## Inicio rápido

```bash
make up        # docker compose up -d --build --wait  (ingestion :8080, analytics solo en la red interna)
make demo      # el simulador envía el fixture F01 a ingestion
make results   # GET /v1/games/arena-combat/matches/match-a1/results | jq
make test      # pruebas Go (-race) y Scala
make down
```

Sin `make`:
```bash
docker compose up -d --build --wait
docker compose --profile demo run --rm simulator --fixture F01-arena-normal
curl -s localhost:8080/v1/games/arena-combat/matches/match-a1/results | jq '.result.playerStats'
```
