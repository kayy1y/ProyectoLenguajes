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

> Estado: **Fase 0 (arranque)**. El plan completo está en [docs/PLAN.md](docs/PLAN.md) y las decisiones en [docs/decisions.md](docs/decisions.md).

## Requisitos de entorno

| Herramienta | Versión verificada localmente |
|---|---|
| Go | 1.26.5 |
| JDK | 24 (se recomienda una LTS: 21 o 25) |
| sbt | pendiente de instalar (`brew install sbt`) |
| Docker / Compose | Compose v5.1.0 |

## Inicio rápido

_Se completará en la Fase 2 (vertical slice)._

```bash
make up      # docker compose up -d --build --wait
make demo    # ejecuta el simulador con el escenario de demo
make test    # pruebas Go (-race) y Scala
make down
```
