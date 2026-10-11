#!/usr/bin/env bash
# Prueba de punta a punta sobre Docker Compose (AC-15).
# Requiere: docker compose, curl, jq. Uso: ./tests/e2e/e2e.sh  (o make e2e)
set -euo pipefail
cd "$(dirname "$0")/../.."

export MATCH_IDLE_TIMEOUT=${MATCH_IDLE_TIMEOUT:-5s}
export MATCH_GRACE_PERIOD=${MATCH_GRACE_PERIOD:-1s}
export INGEST_PORT=${INGEST_PORT:-8080}
API="http://localhost:${INGEST_PORT}"
# --no-deps: el simulador no debe arrancar servicios que la prueba detuvo a propósito.
SIM=(docker compose --profile demo run --rm --no-deps -T simulator)
FAILED=0

step() { printf '\n=== %s\n' "$*"; }
pass() { printf 'PASS  %s\n' "$*"; }
fail() { printf 'FAIL  %s\n' "$*"; FAILED=1; }
check() { # check "descripción" valor_obtenido valor_esperado
  if [[ "$2" == "$3" ]]; then pass "$1"; else fail "$1 (obtenido: $2, esperado: $3)"; fi
}
status_of() { curl -s "$API/v1/games/$1/matches/$2" | jq -r '.status // "NONE"'; }
wait_status() { # wait_status juego partida estado segundos
  for _ in $(seq 1 $(($4 * 4))); do
    [[ "$(status_of "$1" "$2")" == "$3" ]] && return 0
    sleep 0.25
  done
  return 1
}
post() { curl -s -o /tmp/gamestats-e2e.json -w '%{http_code}' -X POST "$API/v1/events" -H 'Content-Type: application/json' -d "$1"; }

if [[ "${KEEP_UP:-0}" != "1" ]]; then
  trap 'if [[ $FAILED == 1 ]]; then docker compose logs --no-color > e2e-logs.txt 2>&1 || true; fi; docker compose down -v >/dev/null 2>&1 || true' EXIT
fi

step "1. Arranque limpio (docker compose up --build --wait)"
docker compose down -v >/dev/null 2>&1 || true
docker compose --profile demo build --quiet
docker compose up -d --wait
check "healthz" "$(curl -s "$API/healthz" | jq -r .status)" "ok"

step "2. Todos los fixtures en paralelo == resultados calculados a mano"
if "${SIM[@]}" --fixture all --expect --wait 60s; then pass "fixtures == contracts/expected"; else fail "fixtures != contracts/expected"; fi

step "3. Evento inválido: 400 con campo y código"
code=$(post "$(cat contracts/examples/invalid/bad-timestamp.json)")
check "HTTP del inválido" "$code" "400"
check "código del inválido" "$(jq -r '.errors[0].code + " " + .errors[0].field' /tmp/gamestats-e2e.json)" "INVALID_FORMAT /timestamp"
check "el inválido queda en /v1/rejections" "$(curl -s "$API/v1/rejections?limit=1" | jq -r '.[0].code')" "INVALID_FORMAT"

step "4. Evento para una partida ya analizada: 409"
code=$(post "$(head -1 contracts/fixtures/F01-arena-normal.jsonl)")
check "HTTP del tardío" "$code" "409"
check "código del tardío" "$(jq -r .code /tmp/gamestats-e2e.json)" "LATE_EVENT"

step "5. Escenario demo: dos juegos en paralelo, casos especiales y partida sin cierre"
out=$("${SIM[@]}" --scenario demo --seed 42 --suffix -demo --wait 60s) || fail "simulador demo"
echo "$out" | sed -n '/--- respuestas/,$p'
echo "$out" | grep -q '400 REJECTED INVALID_FORMAT /timestamp' && pass "demo: timestamp inválido rechazado" || fail "demo: timestamp inválido"
echo "$out" | grep -q '400 REJECTED UNKNOWN_EVENT_TYPE /type' && pass "demo: tipo desconocido rechazado" || fail "demo: tipo desconocido"
echo "$out" | grep -q '400 REJECTED REQUIRED_FIELD /playerId' && pass "demo: playerId faltante rechazado" || fail "demo: playerId faltante"
echo "$out" | grep -q '200 DUPLICATE' && pass "demo: duplicado idempotente" || fail "demo: duplicado"
echo "$out" | grep -q '409 REJECTED DUPLICATE_ID_CONFLICT' && pass "demo: duplicado en conflicto" || fail "demo: conflicto"
echo "$out" | grep -q '409 REJECTED LATE_EVENT' && pass "demo: evento tardío" || fail "demo: tardío"
check "demo: partida sin MATCH_FINISHED analizada como INCOMPLETE" \
  "$(curl -s "$API/v1/games/arena-combat/matches/demo-open-demo" | jq -r '.status + " " + .completeness + " " + .closedReason')" \
  "ANALYZED INCOMPLETE IDLE_TIMEOUT"

step "6. Scala caído: reintentos, FAILED y re-análisis manual"
docker compose stop analytics >/dev/null 2>&1
"${SIM[@]}" --fixture F10-arena-zero-denominator --suffix -down --wait 0s >/dev/null
if wait_status arena-combat match-f10-down FAILED 30; then pass "partida FAILED tras agotar reintentos"; else fail "no llegó a FAILED: $(status_of arena-combat match-f10-down)"; fi
check "results de partida FAILED" "$(curl -s -o /dev/null -w '%{http_code}' "$API/v1/games/arena-combat/matches/match-f10-down/results")" "502"
docker compose start analytics >/dev/null 2>&1
docker compose up -d --wait analytics >/dev/null 2>&1
check "POST reanalyze" "$(curl -s -o /dev/null -w '%{http_code}' -X POST "$API/v1/games/arena-combat/matches/match-f10-down/reanalyze")" "202"
if wait_status arena-combat match-f10-down ANALYZED 30; then pass "partida ANALYZED tras re-análisis"; else fail "re-análisis no terminó"; fi
check "killEfficiency null para quien no atacó" \
  "$(curl -s "$API/v1/games/arena-combat/matches/match-f10-down/results" | jq -c '[.result.playerStats[].metrics.killEfficiency]')" "[0.5,null]"

step "7. Reinicio de Go: los resultados persisten en el volumen y la partida no se reabre"
docker compose restart ingestion >/dev/null 2>&1
docker compose up -d --wait ingestion >/dev/null 2>&1
check "resultado de F01 tras reinicio" \
  "$(curl -s "$API/v1/games/arena-combat/matches/match-a1/results" | jq -c '.status, [.result.playerStats[].metrics.eliminations]' | tr -d '\n')" '"ANALYZED"[3,1,0]'
code=$(post "$(sed -n 2p contracts/fixtures/F01-arena-normal.jsonl)")
check "reenvío tras reinicio rechazado" "$code $(jq -r .code /tmp/gamestats-e2e.json)" "409 MATCH_CLOSED"

step "8. Carga concurrente: 30 partidas de ambos juegos"
out=$("${SIM[@]}" --scenario stress --matches 30 --seed 7 --speed 1000 --max-gap 2ms --wait 90s) || fail "stress"
echo "$out" | sed -n '1,2p;/--- respuestas/,/--- resultados/p'
check "stress: partidas analizadas" "$(echo "$out" | grep -cE '^(arena-combat|street-racing) .* ANALYZED ')" "30"

step "9. Estadísticas de diagnóstico"
curl -s "$API/v1/stats" | jq -c .
check "sin partidas FAILED al final" "$(curl -s "$API/v1/stats" | jq -r '.matches.FAILED // 0')" "0"

echo
if [[ $FAILED == 0 ]]; then echo "E2E OK"; else echo "E2E CON FALLOS (logs en e2e-logs.txt)"; fi
exit $FAILED
