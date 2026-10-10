# Estadísticas y patrones — derivación

Cada métrica sigue la misma cadena: **eventos requeridos → campos → fórmula → casos límite → ejemplo → prueba**. Los valores de ejemplo están calculados a mano y guardados en [contracts/expected/](../contracts/expected/). Scala debe reproducirlos exactamente.

## 1. Preparación (común a todos los juegos)

1. **Orden total:** `sortBy((timestamp, seq si falta = +∞, ingest.seq))`.
2. **Reglas semánticas.** Se recorren los eventos en orden y se registran los jugadores que hicieron `PLAYER_JOINED`. Un evento que rompe una regla genera un **warning** y queda **excluido** de todas las métricas. Las reglas se evalúan en este orden y se aplica la primera que corresponda:

| Código | Regla | Ámbito |
|---|---|---|
| `UNKNOWN_PLAYER` | El evento tiene `playerId` de alguien que no hizo `PLAYER_JOINED` antes | común |
| `SELF_ELIMINATION` | `ELIMINATION` con `victimPlayerId == playerId` | arena |
| `UNKNOWN_VICTIM` | `ELIMINATION` cuya víctima no se unió | arena |
| `SELF_TARGET` | `ATTACK` con `targetPlayerId == playerId` | arena |
| `UNKNOWN_TARGET` | `ATTACK` cuyo objetivo no se unió | arena |

La coherencia de un evento por sí solo (por ejemplo, que `hit=false` implique `damage=0`) la valida el esquema en Go. Estas reglas, en cambio, necesitan la partida completa, y por eso pertenecen al análisis.

3. Las métricas usan solo los eventos conservados. `input.eventsUsed` = conservados e `input.eventsIgnored` = recibidos − conservados.

Utilidad: `safeRatio(n, d) = if d == 0 then None else Some(n / d)`, que se serializa como `null`. Los decimales se redondean a 4 cifras al serializar.

## 2. Arena — por jugador (AC-07)

Hay un registro por cada jugador que hizo `PLAYER_JOINED`.

| Métrica | Eventos · campos | Fórmula | Límite |
|---|---|---|---|
| `attacks` | `ATTACK.playerId` | conteo | 0 si nunca atacó |
| `hits` | `ATTACK.data.hit` | conteo de los ataques con `hit` | — |
| `accuracy` | `ATTACK` | `safeRatio(hits, attacks)` | `null` si `attacks = 0` |
| `damageDealt` | `ATTACK.data.damage` | suma | 0 |
| `eliminations` | `ELIMINATION.playerId` | conteo | — |
| `deaths` | `ELIMINATION.data.victimPlayerId` | conteo | — |
| `objectivesCaptured` | `OBJECTIVE_CAPTURED.playerId` | conteo | — |

## 3. Arena — por partida (AC-08)

| Métrica | Eventos · campos | Fórmula | Límite |
|---|---|---|---|
| `durationSeconds` | `MATCH_STARTED.timestamp`, `MATCH_FINISHED.timestamp` | diferencia en segundos | `null` si falta alguno de los dos |
| `participants` | `PLAYER_JOINED` | jugadores distintos | — |
| `totalEvents` | — | eventos usados | — |
| `totalEliminations` | jugadores | Σ `eliminations` | — |
| `totalDamage` | jugadores | Σ `damageDealt` | — |
| `topPlayerId` | jugadores | máximo por (`eliminations` desc, `damageDealt` desc, `playerId` asc) | `null` si no hay jugadores |

## 4. Arena — combinadas (AC-09)

| Métrica | Relaciona | Fórmula | Límite |
|---|---|---|---|
| `killEfficiency` (por jugador) | `ELIMINATION` × `ATTACK` | `safeRatio(eliminations, attacks)` | `null` si `attacks = 0` (F10) |
| `eliminationsPerMinute` (por partida) | `ELIMINATION` × `MATCH_STARTED`/`MATCH_FINISHED` | `totalEliminations / (durationSeconds / 60)` | `null` si la duración es `null` o `0` |

## 5. Arena — patrón secuencial `KILL_STREAK` (AC-10)

- **Regla:** un jugador elimina a **3 o más** rivales seguidos sin ser eliminado ni salir de la partida entre medio, según el orden total.
- **Cálculo:** `foldLeft` sobre los eventos ordenados con un estado `Map[PlayerId, racha abierta]`:
  - `ELIMINATION(k → v)`: se añade el evento a la racha de `k` y se **cierra** la de `v`.
  - `PLAYER_LEFT(p)`: se cierra la racha de `p`.
  - Al terminar los eventos se cierran las rachas abiertas.
  - Una racha cerrada con longitud ≥ 3 se emite como patrón **maximal**: una racha de 4 es un solo patrón de `length: 4`, no dos de 3.
- **Salida:** `{type, playerId, fromEventId, toEventId, from, to, length}`.
- **No es un conteo:** en F12, p1 tiene 3 eliminaciones igual que en F01, pero muere entre la 2ª y la 3ª, así que no hay patrón.

## 6. Ejemplo calculado a mano — F01 (`match-a1`, T = 2026-10-01T15:00:00Z)

| eventId | t | evento |
|---|---|---|
| a1-e01 | T+0 | MATCH_STARTED |
| a1-e02..e04 | T+1, T+1, T+2 | PLAYER_JOINED p1, p2, p3 |
| a1-e05 | T+10 | ATTACK p1→p2, acierto, 40 |
| a1-e06 | T+12 | ATTACK p1→p2, acierto, 60 |
| a1-e07 | T+12 | ELIMINATION p1 elimina a p2 |
| a1-e08 | T+20 | ATTACK p2→p1, fallo, 0 |
| a1-e09 | T+25 | ATTACK p1→p3, acierto, 100 |
| a1-e10 | T+25 | ELIMINATION p1 elimina a p3 |
| a1-e11 | T+30 | ATTACK p3→p1, acierto, 30 |
| a1-e12 | T+40 | ATTACK p1→p2, acierto, 100 |
| a1-e13 | T+40 | ELIMINATION p1 elimina a p2 → racha de 3 |
| a1-e14 | T+50 | OBJECTIVE_CAPTURED p2 (flag-A) |
| a1-e15 | T+55 | ATTACK p2→p1, acierto, 100 |
| a1-e16 | T+55 | ELIMINATION p2 elimina a p1 → se cierra la racha de p1 |
| a1-e17 | T+60 | ATTACK p1→p3, fallo, 0 |
| a1-e18 | T+120 | MATCH_FINISHED |

| Jugador | attacks | hits | accuracy | damage | elim. | deaths | obj. | killEfficiency |
|---|---|---|---|---|---|---|---|---|
| p1 | 5 (e05, e06, e09, e12, e17) | 4 | 4/5 = 0.8 | 40+60+100+100 = 300 | 3 | 1 | 0 | 3/5 = 0.6 |
| p2 | 2 (e08, e15) | 1 | 0.5 | 100 | 1 | 2 | 1 | 0.5 |
| p3 | 1 (e11) | 1 | 1.0 | 30 | 0 | 1 | 0 | 0/1 = 0.0 |

- **Partida:** `durationSeconds` = 120 · `participants` = 3 · `totalEvents` = 18 · `totalEliminations` = 4 · `totalDamage` = 430 · `eliminationsPerMinute` = 4 / 2 = 2.0 · `topPlayerId` = p1.
- **Patrón:** `KILL_STREAK` de p1, de a1-e07 a a1-e13, `length` 3.

## 7. Catálogo de fixtures

| Fixture | Qué demuestra | Resultado clave |
|---|---|---|
| F01 arena-normal | Caso normal | tablas anteriores |
| F06 arena-out-of-order | Llegada desordenada, incluido un evento tras `MATCH_FINISHED` | idéntico a F01 (la racha termina en `f06-e13`, no en `e10`) |
| F10 arena-zero-denominator | Denominador cero | p2: `accuracy` = `null` y `killEfficiency` = `null` |
| F11 arena-streak-positive | Racha maximal y dos rachas | p1 `length` 4 (e07→e13); p3 `length` 3 (e15→e19); `eliminationsPerMinute` = 7/(80/60) = 5.25 |
| F12 arena-streak-negative | Una muerte interrumpe la racha | `patterns: []` aunque p1 tenga 3 eliminaciones |
| F13 arena-contradictions | Reglas semánticas | 4 warnings; se usan 6 de 10 eventos |

Las métricas de `street-racing` se documentarán en la fase 8.
