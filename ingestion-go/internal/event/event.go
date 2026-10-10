// Package event define el evento recibido y el orden total dentro de una partida.
package event

import (
	"encoding/json"
	"math"
	"sort"
	"time"
)

// IngestTimeLayout es UTC con precisión de milisegundos, como exige el contrato.
const IngestTimeLayout = "2006-01-02T15:04:05.000Z"

const (
	TypeMatchStarted  = "MATCH_STARTED"
	TypeMatchFinished = "MATCH_FINISHED"
)

// Key identifica una partida de forma inequívoca.
type Key struct {
	GameID  string
	MatchID string
}

func (k Key) String() string { return k.GameID + ":" + k.MatchID }

// Event conserva los campos del productor tal como llegaron para reenviarlos a Scala.
// Go no interpreta Data: su contenido pertenece al análisis.
type Event struct {
	EventID     string          `json:"eventId"`
	Timestamp   string          `json:"timestamp"`
	GameID      string          `json:"gameId"`
	GameVersion string          `json:"gameVersion"`
	MatchID     string          `json:"matchId"`
	PlayerID    *string         `json:"playerId,omitempty"`
	Type        string          `json:"type"`
	Seq         *int64          `json:"seq,omitempty"`
	Data        json.RawMessage `json:"data"`
	Ingest      *Ingest         `json:"ingest,omitempty"`

	// Time es Timestamp ya parseado; se usa solo para ordenar.
	Time time.Time `json:"-"`
}

// Ingest lo agrega Go al aceptar el evento.
type Ingest struct {
	ReceivedAt string `json:"receivedAt"`
	Seq        int64  `json:"seq"`
}

func (e Event) Key() Key { return Key{GameID: e.GameID, MatchID: e.MatchID} }

// SortTotal ordena por (timestamp, seq ausente = +inf, orden de llegada).
func SortTotal(events []Event) {
	seq := func(e Event) int64 {
		if e.Seq == nil {
			return math.MaxInt64
		}
		return *e.Seq
	}
	sort.SliceStable(events, func(i, j int) bool {
		a, b := events[i], events[j]
		if !a.Time.Equal(b.Time) {
			return a.Time.Before(b.Time)
		}
		if seq(a) != seq(b) {
			return seq(a) < seq(b)
		}
		return a.Ingest.Seq < b.Ingest.Seq
	})
}
