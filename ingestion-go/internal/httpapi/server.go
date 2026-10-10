// Package httpapi expone la API pública de ingestión y consulta (docs/protocol.md).
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"gamestats/ingestion-go/internal/event"
	"gamestats/ingestion-go/internal/match"
	"gamestats/ingestion-go/internal/store"
	"gamestats/ingestion-go/internal/validation"
)

const maxEventBytes = 64 << 10

type Submitter interface {
	Submit(ctx context.Context, env match.Envelope) error
}

type Server struct {
	dispatcher     Submitter
	store          *store.Store
	enqueueTimeout time.Duration
	log            *slog.Logger
}

func New(d Submitter, st *store.Store, enqueueTimeout time.Duration, log *slog.Logger) *Server {
	return &Server{dispatcher: d, store: st, enqueueTimeout: enqueueTimeout, log: log.With("component", "http")}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /v1/events", s.postEvent)
	mux.HandleFunc("GET /v1/matches", s.listMatches)
	mux.HandleFunc("GET /v1/games/{gameId}/matches/{matchId}", s.getMatch)
	mux.HandleFunc("GET /v1/games/{gameId}/matches/{matchId}/results", s.getResults)
	return mux
}

type ackResponse struct {
	Status  string                  `json:"status"`
	Code    string                  `json:"code,omitempty"`
	EventID string                  `json:"eventId,omitempty"`
	Errors  []validation.FieldError `json:"errors,omitempty"`
}

func (s *Server) postEvent(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxEventBytes))
	if err != nil {
		writeJSON(w, http.StatusRequestEntityTooLarge, ackResponse{Status: "REJECTED", Code: "EVENT_TOO_LARGE"})
		return
	}
	ev, errs := validation.Parse(body)
	if len(errs) > 0 {
		s.log.Warn("event rejected", "eventId", ev.EventID, "matchId", ev.MatchID, "errors", errs)
		writeJSON(w, http.StatusBadRequest, ackResponse{Status: "REJECTED", EventID: ev.EventID, Errors: errs})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.enqueueTimeout)
	defer cancel()
	reply := make(chan match.Ack, 1)
	if err := s.dispatcher.Submit(ctx, match.Envelope{Event: ev, Reply: reply}); err != nil {
		s.backpressure(w, ev.EventID)
		return
	}
	select {
	case ack := <-reply:
		writeJSON(w, ackHTTPStatus(ack), ackResponse{Status: string(ack.Status), Code: ack.Code, EventID: ev.EventID})
	case <-ctx.Done():
		// Reintentar es seguro: el worker deduplica por eventId.
		s.backpressure(w, ev.EventID)
	}
}

func (s *Server) backpressure(w http.ResponseWriter, eventID string) {
	w.Header().Set("Retry-After", "1")
	writeJSON(w, http.StatusServiceUnavailable, ackResponse{Status: "REJECTED", Code: "BACKPRESSURE", EventID: eventID})
}

func ackHTTPStatus(ack match.Ack) int {
	switch ack.Status {
	case match.Accepted:
		return http.StatusAccepted
	case match.Duplicate:
		return http.StatusOK
	case match.Backpressure:
		return http.StatusServiceUnavailable
	default:
		return http.StatusConflict
	}
}

func (s *Server) listMatches(w http.ResponseWriter, r *http.Request) {
	records := s.store.List()
	if status := r.URL.Query().Get("status"); status != "" {
		filtered := records[:0]
		for _, rec := range records {
			if string(rec.Status) == status {
				filtered = append(filtered, rec)
			}
		}
		records = filtered
	}
	writeJSON(w, http.StatusOK, records)
}

func (s *Server) getMatch(w http.ResponseWriter, r *http.Request) {
	rec, ok := s.store.Get(matchKey(r))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "MATCH_NOT_FOUND"})
		return
	}
	rec.Result = nil
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) getResults(w http.ResponseWriter, r *http.Request) {
	rec, ok := s.store.Get(matchKey(r))
	switch {
	case !ok:
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "MATCH_NOT_FOUND"})
	case rec.Status == store.Analyzed:
		writeJSON(w, http.StatusOK, rec)
	case rec.Status == store.Failed:
		writeJSON(w, http.StatusBadGateway, rec)
	default:
		writeJSON(w, http.StatusAccepted, rec)
	}
}

func matchKey(r *http.Request) event.Key {
	return event.Key{GameID: r.PathValue("gameId"), MatchID: r.PathValue("matchId")}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil && !errors.Is(err, http.ErrHandlerTimeout) {
		slog.Default().Debug("write response", "err", err)
	}
}
