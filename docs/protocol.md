# Protocolo entre componentes v1

> Estado: **borrador de la fase 1**. Este documento debe coincidir con lo implementado; cualquier cambio de contrato actualiza a la vez los esquemas, los ejemplos, los fixtures y este archivo.

```
simulator / productor ──POST /v1/events──▶ ingestion-go :8080 ──POST /v1/analyze──▶ analytics-scala :8081 (solo red interna)
cliente ──GET /v1/games/{gameId}/matches/{matchId}/results──▶ ingestion-go
```

## 1. API pública de ingestion-go (`:8080`)

| Método y ruta | Uso | Respuestas |
|---|---|---|
| `POST /v1/events` | Recibir un evento ([event-spec.md](event-spec.md)) | 202 / 200 / 400 / 409 / 413 / 503 |
| `GET /v1/matches?status=` | Listar partidas | `200 [{gameId, matchId, status, eventCount, updatedAt}]` |
| `GET /v1/games/{gameId}/matches/{matchId}` | Estado de una partida | 200 / 404 |
| `GET /v1/games/{gameId}/matches/{matchId}/results` | Resultado del análisis | 200 (`ANALYZED`) · 202 (aún no analizada) · 502 (`FAILED`, con `lastError`) · 404 |
| `POST /v1/games/{gameId}/matches/{matchId}/reanalyze` | Reintento manual de una partida `FAILED` | 202 / 409 |
| `GET /v1/rejections?limit=` | Últimos 100 eventos rechazados | 200 |
| `GET /v1/stats` | Contadores de diagnóstico | 200 |
| `GET /healthz` | Liveness | 200 |

Respuesta de `GET …/results` cuando la partida está analizada:
```json
{"gameId":"arena-combat","matchId":"match-a1","status":"ANALYZED","attempts":1,
 "analyzedAt":"2026-10-09T18:00:03.120Z","result":{ "...AnalysisResult v1..." : "" }}
```
`result` es el cuerpo que devuelve Scala, **sin modificar**: Go lo guarda como bytes opacos y no calcula nada.

Estados de una partida: `COLLECTING`, `FINISHING`, `EXPIRED`, `ANALYZING`, `RETRYING`, `ANALYZED`, `FAILED`.

## 2. Go → Scala: `POST /v1/analyze`

- **Cabeceras:** `Content-Type: application/json`, `X-Request-Id: <uuid>`, `X-Contract-Version: 1`.
- **Cuerpo:** [analyze-request.v1.schema.json](../contracts/schemas/analyze-request.v1.schema.json). Ejemplo completo: [analyze-request.F01.json](../contracts/examples/protocol/analyze-request.F01.json).
- `events` contiene todos los eventos aceptados de **una** partida, ya ordenados. Cada evento lleva `ingest.receivedAt` e `ingest.seq` (orden de llegada). Scala vuelve a ordenar por la misma clave, por seguridad (es idempotente).
- `completeness`: `COMPLETE` si llegó `MATCH_FINISHED`; `INCOMPLETE` en cualquier otro caso.
- `closedReason`: `MATCH_FINISHED` | `IDLE_TIMEOUT` | `SHUTDOWN`.
- `batchId`: `gameId:matchId:intento`.

**Respuestas de Scala:**

| HTTP | Cuerpo | Go hace |
|---|---|---|
| 200 | [analysis-result.v1](../contracts/schemas/analysis-result.v1.schema.json) | Pasa a `ANALYZED` |
| 400 | `{"code":"INVALID_BATCH"\|"EMPTY_BATCH","message"}` | Pasa a `FAILED` sin reintentar (es un bug de contrato) |
| 422 | `{"code":"UNSUPPORTED_GAME"\|"UNSUPPORTED_VERSION","message"}` | Pasa a `FAILED` sin reintentar |
| 5xx, timeout o conexión rechazada | — | `RETRYING`: hasta 3 reintentos con backoff de 0.5 s, 1 s y 2 s más jitter; luego `FAILED` |

El timeout por intento es de 5 s (`ANALYZE_TIMEOUT`). Una partida `FAILED` conserva su lote en memoria para `POST …/reanalyze`.

`GET /health` en Scala → `200 {"status":"ok"}`; lo usa el healthcheck de Compose.

## 3. Resultado del análisis

Especificación: [analysis-result.v1.schema.json](../contracts/schemas/analysis-result.v1.schema.json). Ejemplos reales: [contracts/expected/](../contracts/expected/).

- Es **determinista**: no incluye la hora del análisis (`analyzedAt` lo agrega Go en el envoltorio), así que la misma entrada produce exactamente el mismo JSON.
- Las métricas que no se pueden calcular valen `null`, nunca `0` ni `NaN`.
- `playerStats` va ordenado por `playerId`; `patterns`, por `from` y luego por `playerId`; `warnings`, en el orden del análisis.
- Los decimales se redondean a 4 cifras.

## 4. Qué se pierde al reiniciar

Todo vive en la memoria de Go. Al reiniciar se pierden las partidas activas, los lotes `FAILED` y los resultados. Con `RESULTS_DIR` (un volumen), los resultados `ANALYZED` se guardan en disco y se recargan al arrancar. Scala no tiene estado.
