# Guion de demo (10–12 min)

`scripts/demo.sh` ejecuta estos pasos con pausas (`ENTER` para avanzar). Necesita Docker, curl y jq.

| # | Qué se muestra | Comando | Requisito |
|---|---|---|---|
| 1 | Dos servicios separados y sanos | `docker compose up -d --build --wait && docker compose ps` | AC-14, AC-15 |
| 2 | El contrato | `docs/event-spec.md` y `contracts/examples/valid/arena-attack-hit.json` | AC-01 |
| 3 | Un evento válido | `curl -i -X POST localhost:8080/v1/events -H 'Content-Type: application/json' -d @contracts/examples/valid/arena-attack-hit.json` → 202 | AC-02 |
| 4 | Un evento inválido, con su motivo | `… -d @contracts/examples/invalid/bad-timestamp.json` → 400 `INVALID_FORMAT /timestamp`; luego `curl localhost:8080/v1/rejections \| jq` | AC-03 |
| 5 | Partidas simultáneas de dos juegos y casos especiales | En otra terminal: `docker compose logs -f ingestion \| jq -c '{msg,matchId,component}'`. Luego `make demo` | AC-04, AC-12 |
| 6 | Estado por partida | `curl localhost:8080/v1/matches \| jq -c '.[] \| {matchId,status,completeness,closedReason}'` | AC-05 |
| 7 | Estadística por jugador | `curl -s localhost:8080/v1/games/arena-combat/matches/match-a1/results \| jq '.result.playerStats'` | AC-07, AC-11 |
| 8 | Estadística por partida | `… \| jq '.result.matchStats'` | AC-08 |
| 9 | Estadística combinada | `… \| jq '[.result.playerStats[] \| {playerId, killEfficiency: .metrics.killEfficiency}]'` | AC-09 |
| 10 | Patrón secuencial | `… \| jq '.result.patterns'` → `KILL_STREAK` de p1. Compararlo con F12, donde hay 3 eliminaciones pero no hay racha | AC-10 |
| 11 | Segundo juego | `curl -s localhost:8080/v1/games/street-racing/matches/race-r1/results \| jq '.result.patterns'` → `COMEBACK` | AC-13 |
| 12 | Scala se cae | `docker compose stop analytics`, enviar F10, ver `FAILED`, `docker compose start analytics`, `POST …/reanalyze` | robustez |
| 13 | Pruebas y CI | `make test`, página de Actions | AC-16 |
| 14 | Deployment | pasos 3–7 contra la IP pública (`--target http://<ip>:8080 --api-key …`) | AC-17 |
| 15 | Historial | `git log --oneline` | AC-18 |

Para repetir la demo sin reiniciar, usar `--suffix -2` en el simulador, porque las partidas cerradas no se reabren. Otra opción es `docker compose down -v`, que borra los resultados.

## Qué decir en cada paso (resumen)

- **Paso 5.** "Cada partida es una goroutine, y por eso los logs de `match-a1`, `race-r1` y `demo-arena` aparecen intercalados. El simulador envía inválidos, un duplicado idéntico (200), uno en conflicto (409) y un evento tardío (409)."
- **Paso 6.** "`demo-open` nunca envió `MATCH_FINISHED`; se cerró por inactividad y se analizó como `INCOMPLETE`, con la duración en `null` en vez de inventarla."
- **Paso 10.** "La racha depende del orden: F12 tiene las mismas 3 eliminaciones que F01, pero una muerte en medio rompe el patrón."
