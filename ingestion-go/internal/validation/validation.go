// Package validation hace la validación estructural de un evento.
//
// Versión mínima de la vertical slice: campos obligatorios, formato de timestamp y
// regla de playerId. En la fase 4 se reemplaza por el JSON Schema de contracts/.
package validation

import (
	"bytes"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"gamestats/ingestion-go/internal/event"
)

type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

var tsPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]{1,3})?Z$`)

// Parse decodifica y valida un evento. Si hay errores, el evento devuelto no debe usarse.
func Parse(body []byte) (event.Event, []FieldError) {
	var e event.Event
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&e); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			return e, []FieldError{{Field: "/" + typeErr.Field, Code: "INVALID_TYPE", Message: err.Error()}}
		}
		return e, []FieldError{{Field: "", Code: "MALFORMED_JSON", Message: err.Error()}}
	}
	if e.Ingest != nil {
		return e, []FieldError{{Field: "/ingest", Code: "UNKNOWN_FIELD", Message: "ingest is added by the server"}}
	}

	var errs []FieldError
	required := func(field, value string) {
		if value == "" {
			errs = append(errs, FieldError{Field: field, Code: "REQUIRED_FIELD", Message: "is required"})
		}
	}
	required("/eventId", e.EventID)
	required("/timestamp", e.Timestamp)
	required("/gameId", e.GameID)
	required("/gameVersion", e.GameVersion)
	required("/matchId", e.MatchID)
	required("/type", e.Type)

	if e.Timestamp != "" {
		t, err := time.Parse(time.RFC3339, e.Timestamp)
		if !tsPattern.MatchString(e.Timestamp) || err != nil {
			errs = append(errs, FieldError{Field: "/timestamp", Code: "INVALID_FORMAT", Message: "must be UTC ISO 8601 with Z, e.g. 2026-10-01T15:00:00Z"})
		}
		e.Time = t
	}

	if len(bytes.TrimSpace(e.Data)) == 0 || bytes.TrimSpace(e.Data)[0] != '{' {
		errs = append(errs, FieldError{Field: "/data", Code: "INVALID_TYPE", Message: "must be an object"})
	}

	global := e.Type == event.TypeMatchStarted || e.Type == event.TypeMatchFinished
	switch {
	case global && e.PlayerID != nil:
		errs = append(errs, FieldError{Field: "/playerId", Code: "FORBIDDEN_FIELD", Message: "global events must not have playerId"})
	case !global && e.Type != "" && e.PlayerID == nil:
		errs = append(errs, FieldError{Field: "/playerId", Code: "REQUIRED_FIELD", Message: "is required for player events"})
	}
	return e, errs
}
