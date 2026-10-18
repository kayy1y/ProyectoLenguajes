package lab

import (
	"cmp"
	"slices"
	"strings"
	"time"
)

// SortMatches ordena las partidas por una columna: "game", "match", "status",
// "completeness" o "events". Con col vacía ordena por la última actualización.
// Los empates se resuelven por juego y partida, para que el orden sea estable.
func SortMatches(ms []MatchSummary, col string, desc bool) {
	compare := func(a, b MatchSummary) int {
		switch col {
		case "game":
			return strings.Compare(a.GameID, b.GameID)
		case "match":
			return strings.Compare(a.MatchID, b.MatchID)
		case "status":
			return strings.Compare(a.Status, b.Status)
		case "completeness":
			return strings.Compare(a.Completeness, b.Completeness)
		case "events":
			return cmp.Compare(a.EventCount, b.EventCount)
		}
		return parseTime(a.UpdatedAt).Compare(parseTime(b.UpdatedAt))
	}
	slices.SortStableFunc(ms, func(a, b MatchSummary) int {
		c := compare(a, b)
		if desc {
			c = -c
		}
		if c == 0 {
			c = cmp.Or(strings.Compare(a.GameID, b.GameID), strings.Compare(a.MatchID, b.MatchID))
		}
		return c
	})
}

// parseTime lee una fecha RFC 3339; si no se puede, devuelve el instante cero.
func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	return t
}
