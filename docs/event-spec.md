# Especificación de eventos v1 (para productores)

Este documento basta para construir un evento válido sin leer el código. La especificación formal está en [contracts/schemas/](../contracts/schemas/) (JSON Schema draft 2020-12), y hay ejemplos en [contracts/examples/valid/](../contracts/examples/valid/).

## 1. Envío

```
POST /v1/events
Content-Type: application/json
X-API-Key: <clave>        # solo si el servidor tiene INGEST_API_KEY configurada
```
Un evento por petición, de 64 KB como máximo.

## 2. Sobre común (todos los juegos)

| Campo | Tipo | Requerido | Regla |
|---|---|---|---|
| `eventId` | string | sí | `^[A-Za-z0-9._:-]{1,64}$`. Único dentro de la partida (se recomienda único global). Reenviar el mismo evento es seguro. |
| `timestamp` | string | sí | Momento en que ocurrió el evento (event-time), en **UTC con `Z`**: `2026-10-01T15:00:12Z` o `2026-10-01T15:00:12.250Z` (máx. ms). Se rechazan desplazamientos como `-06:00` y fechas imposibles. |
| `gameId` | string | sí | Juego registrado: `arena-combat` (y `street-racing` cuando se agregue). |
| `gameVersion` | string | sí | `MAYOR.MENOR`. Debe estar soportada por el juego (`arena-combat`: `1.0`). |
| `matchId` | string | sí | `^[A-Za-z0-9._-]{1,64}$`. La partida se identifica por **(`gameId`, `matchId`)**. |
| `type` | string | sí | Uno del catálogo del juego. |
| `playerId` | string | condicional | `^[A-Za-z0-9._-]{1,64}$`. **Obligatorio** en eventos de jugador y **prohibido** en `MATCH_STARTED` y `MATCH_FINISHED`. |
| `seq` | entero ≥ 0 | no (recomendado) | Secuencia del productor dentro de la partida; desempata eventos con el mismo `timestamp`. |
| `data` | objeto | sí | Carga del tipo de evento. **No admite campos extra.** |

Cualquier otro campo de primer nivel se rechaza, en particular `ingest`, que lo agrega el servidor.

## 3. Catálogo `arena-combat` v1.0

Deathmatch todos contra todos, con reaparición y sin equipos.

| `type` | `playerId` | `data` |
|---|---|---|
| `MATCH_STARTED` | prohibido | `{"map": string(1..64)}` |
| `PLAYER_JOINED` | jugador que entra | `{}` |
| `ATTACK` | atacante | `{"targetPlayerId": id, "weapon": string(1..32), "hit": boolean, "damage": int 0..1000}`. Si `hit` es `false`, `damage` debe ser `0`. |
| `ELIMINATION` | eliminador | `{"victimPlayerId": id, "weapon": string(1..32)}` |
| `OBJECTIVE_CAPTURED` | quien captura | `{"objectiveId": string(1..64)}` |
| `PLAYER_LEFT` | jugador que sale | `{"reason": "DISCONNECT" \| "QUIT"}` |
| `MATCH_FINISHED` | prohibido | `{"reason": "COMPLETED" \| "ABORTED"}` |

Los eventos describen **hechos**, nunca estadísticas: no se aceptan campos como `kills` o `accuracy`, que se calculan en el análisis.

## 4. Ejemplo mínimo

```json
{"eventId":"a1-e05","timestamp":"2026-10-01T15:00:10Z","gameId":"arena-combat","gameVersion":"1.0",
 "matchId":"match-a1","playerId":"p1","type":"ATTACK","seq":5,
 "data":{"targetPlayerId":"p2","weapon":"rifle","hit":true,"damage":40}}
```

## 5. Respuestas

| HTTP | `status` / `code` | Significado | ¿Reintentar? |
|---|---|---|---|
| 202 | `ACCEPTED` | Evento aceptado | — |
| 200 | `DUPLICATE` | Ese `eventId` ya llegó con el mismo contenido; se ignora | No hace falta |
| 400 | `REJECTED` + `errors[]` | Evento inválido (tabla 6) | No, primero hay que corregirlo |
| 409 | `DUPLICATE_ID_CONFLICT` | El mismo `eventId` llegó antes con otro contenido | No |
| 409 | `MATCH_ALREADY_STARTED` / `MATCH_ALREADY_FINISHED` | Segundo inicio o segundo fin de la partida | No |
| 409 | `LATE_EVENT` / `MATCH_CLOSED` | La partida ya se envió a análisis | No: hay que usar un `matchId` nuevo |
| 413 | `MATCH_TOO_LARGE` | La partida superó el límite de eventos | No |
| 503 | `BACKPRESSURE` | Servidor saturado | Sí, tras `Retry-After` (es idempotente) |

Cuerpo de 400:
```json
{"status":"REJECTED","eventId":"a1-e05","errors":[{"field":"/data/damage","code":"OUT_OF_RANGE","message":"must be >= 0"}]}
```

## 6. Códigos de error de validación

| Código | Cuándo | Ejemplo |
|---|---|---|
| `MALFORMED_JSON` | El cuerpo no es JSON | `not-json.txt` |
| `REQUIRED_FIELD` | Falta un campo obligatorio | `missing-event-id.json`, `player-event-without-player.json`, `missing-data-field.json` |
| `FORBIDDEN_FIELD` | Hay un campo que no corresponde a ese tipo | `global-event-with-player.json` |
| `UNKNOWN_FIELD` | Hay un campo fuera del contrato | `extra-data-field.json`, `producer-ingest-field.json` |
| `INVALID_TYPE` | El tipo JSON es incorrecto | `bad-data-type.json`, `data-not-object.json` |
| `INVALID_FORMAT` | El patrón o el formato no coinciden | `bad-timestamp.json`, `non-utc-timestamp.json`, `impossible-date.json` |
| `OUT_OF_RANGE` | El número está fuera de rango | `negative-damage.json` |
| `INVALID_VALUE` | El valor no es admisible en ese contexto | `miss-with-damage.json` |
| `UNKNOWN_GAME` | `gameId` no está registrado | `unknown-game.json` |
| `UNSUPPORTED_GAME_VERSION` | La versión no está soportada | `unsupported-version.json` |
| `UNKNOWN_EVENT_TYPE` | `type` no está en el catálogo | `unknown-type.json` |

El mapa completo de ejemplo → código → campo está en [contracts/examples/invalid/expected.json](../contracts/examples/invalid/expected.json).

## 7. Semántica temporal y de orden

- `timestamp` es el **event-time** y es el que se usa para las métricas. El servidor registra aparte `ingest.receivedAt` (ingestion-time), solo para diagnóstico.
- No se rechazan timestamps antiguos.
- Orden total dentro de una partida: `(timestamp, seq — si falta cuenta como +∞, orden de llegada)`. Dos eventos con el mismo `timestamp` **no implican causalidad**; este orden es una convención. Para que el orden sea el que pretendes, envía `seq`.
- Los eventos pueden llegar desordenados: se ordenan antes de analizarlos.
- La partida se analiza cuando llega `MATCH_FINISHED` más un período de gracia (2 s por defecto). Lo que llegue después recibe `LATE_EVENT`. Si una partida pasa un tiempo sin eventos (60 s por defecto), se analiza como `INCOMPLETE`.
- Para reiniciar una partida se usa un `matchId` nuevo; una partida cerrada no se reabre.

## 8. Versionado

- La versión del contrato está en la ruta (`/v1`) y en el nombre del esquema (`*.v1.schema.json`). Los cambios incompatibles crean `v2`.
- `gameVersion` versiona el **juego**. Una versión nueva de un juego se agrega al `enum` de su esquema.
- Agregar un juego nuevo consiste en agregar `contracts/schemas/games/<gameId>.v1.schema.json` (y su analizador en Scala). No hace falta tocar este sobre común.
