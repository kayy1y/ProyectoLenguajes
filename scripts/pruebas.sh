# Funciones para las pruebas manuales. Cargar en la terminal con:
#   source scripts/pruebas.sh
# Cambiar S (sufijo) en cada repetición: las partidas cerradas no se reabren.

S=${S:--1}
G=${G:-arena-combat}
BASE=${BASE:-http://localhost:8080}

ev() { curl -s -w ' [%{http_code}]' -X POST "$BASE/v1/events" -H 'Content-Type: application/json' -d "$1"; echo; }

mk() {
  jq -nc --arg id "$1$S" --arg ts "2026-10-01T$2Z" --arg t "$3" --arg p "$4" --argjson d "$5" \
    --arg m "$M$S" --arg g "$G" \
    '{eventId:$id,timestamp:$ts,gameId:$g,gameVersion:"1.0",matchId:$m,type:$t,data:$d}
     + (if $p=="" then {} else {playerId:$p} end)'
}

res() { curl -s "$BASE/v1/games/$G/matches/$M$S/results"; }

echo "Listo: ev, mk y res cargadas (S=$S, G=$G)"
