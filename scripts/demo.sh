#!/usr/bin/env bash
# Demo guiada (docs/demo.md). ENTER avanza. Uso: ./scripts/demo.sh [URL]   (por defecto http://localhost:8080)
set -uo pipefail
cd "$(dirname "$0")/.."
API=${1:-http://localhost:8080}
KEY=${INGEST_API_KEY:-}
post() { # post archivo.json  -> envía un evento y muestra la respuesta
  if [[ -n "$KEY" ]]; then curl -s -X POST "$API/v1/events" -H 'Content-Type: application/json' -H "X-API-Key: $KEY" -d @"$1"
  else curl -s -X POST "$API/v1/events" -H 'Content-Type: application/json' -d @"$1"; fi | jq -c .
}
RES="$API/v1/games/arena-combat/matches/match-a1/results"

pause() { printf '\n\033[1m▶ %s\033[0m  [ENTER]' "$*"; read -r _; }
run() { printf '\033[2m$ %s\033[0m\n' "$*"; eval "$@"; }

if [[ "$API" == http://localhost* ]]; then
  pause "1. Levantar los dos servicios"
  run "docker compose up -d --build --wait && docker compose ps"
fi
pause "2. Evento válido"
run "post contracts/examples/valid/arena-attack-hit.json"
pause "3. Evento inválido (y registro de rechazos)"
run "post contracts/examples/invalid/bad-timestamp.json"
run "curl -s '$API/v1/rejections?limit=3' | jq -c '.[]'"
pause "4. Partidas simultáneas de dos juegos con casos especiales (simulador)"
if [[ "$API" == http://localhost* ]]; then
  run "docker compose --profile demo run --rm --no-deps simulator --scenario demo --seed 42 --suffix \"\${SUFFIX:-}\""
else
  run "(cd simulator && go run ./cmd/simulator --target $API --api-key '$KEY' --fixtures-dir ../contracts/fixtures --scenario demo --seed 42 --suffix \"\${SUFFIX:-}\")"
fi
pause "5. Estado de cada partida"
run "curl -s $API/v1/matches | jq -c '.[] | {matchId,status,completeness,closedReason,eventCount}'"
pause "6. Estadística por jugador (match-a1)"
run "curl -s $RES | jq -c '.result.playerStats[]'"
pause "7. Estadística por partida y combinada"
run "curl -s $RES | jq -c '.result.matchStats, [.result.playerStats[] | {playerId, killEfficiency: .metrics.killEfficiency}]'"
pause "8. Patrón secuencial (KILL_STREAK) y segundo juego (COMEBACK)"
run "curl -s $RES | jq -c '.result.patterns'"
run "curl -s $API/v1/games/street-racing/matches/race-r1/results | jq -c '.result.patterns'"
pause "9. Diagnóstico"
run "curl -s $API/v1/stats | jq -c ."
echo; echo "Fin de la demo."
