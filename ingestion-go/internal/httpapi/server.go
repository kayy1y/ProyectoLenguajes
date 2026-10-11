// Package httpapi expone la API pública de ingestión y consulta (docs/protocol.md).
package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"gamestats/ingestion-go/internal/event"
	"gamestats/ingestion-go/internal/match"
	"gamestats/ingestion-go/internal/obs"
	"gamestats/ingestion-go/internal/store"
	"gamestats/ingestion-go/internal/validation"
)

const maxEventBytes = 64 << 10

type Submitter interface {
	Submit(ctx context.Context, env match.Envelope) error
}

type Reanalyzer interface {
	Reanalyze(ctx context.Context, k event.Key) error
}

type Deps struct {
	Validator      *validation.Registry
	Dispatcher     Submitter
	Pool           Reanalyzer
	Store          *store.Store
	Counters       *obs.Counters
	Rejections     *obs.Rejections
	EnqueueTimeout time.Duration
	APIKey         string
	Log            *slog.Logger
}

type Server struct {
	Deps
	started time.Time
}

func New(d Deps) *Server {
	d.Log = d.Log.With("component", "http")
	return &Server{Deps: d, started: time.Now()}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /v1/events", s.auth(s.postEvent))
	mux.HandleFunc("GET /v1/matches", s.listMatches)
	mux.HandleFunc("GET /v1/games/{gameId}/matches/{matchId}", s.getMatch)
	mux.HandleFunc("GET /v1/games/{gameId}/matches/{matchId}/results", s.getResults)
	mux.HandleFunc("POST /v1/games/{gameId}/matches/{matchId}/reanalyze", s.auth(s.reanalyze))
	mux.HandleFunc("GET /v1/rejections", s.rejections)
	mux.HandleFunc("GET /v1/stats", s.stats)
	return mux
}

// auth exige X-API-Key en las rutas de escritura solo si INGEST_API_KEY está configurada.
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.APIKey != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-API-Key")), []byte(s.APIKey)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"status": "REJECTED", "code": "UNAUTHORIZED"})
			return
		}
		next(w, r)
	}
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
		s.reject(obs.Rejection{Code: "EVENT_TOO_LARGE"})
		writeJSON(w, http.StatusRequestEntityTooLarge, ackResponse{Status: "REJECTED", Code: "EVENT_TOO_LARGE"})
		return
	}
	ev, errs := s.Validator.Parse(body)
	if len(errs) > 0 {
		s.reject(obs.Rejection{EventID: ev.EventID, GameID: ev.GameID, MatchID: ev.MatchID, Code: errs[0].Code, Field: errs[0].Field, Message: errs[0].Message})
		s.Log.Warn("event rejected", "eventId", ev.EventID, "matchId", ev.MatchID, "errors", errs)
		writeJSON(w, http.StatusBadRequest, ackResponse{Status: "REJECTED", EventID: ev.EventID, Errors: errs})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.EnqueueTimeout)
	defer cancel()
	reply := make(chan match.Ack, 1)
	if err := s.Dispatcher.Submit(ctx, match.Envelope{Event: ev, Reply: reply}); err != nil {
		s.backpressure(w, ev.EventID)
		return
	}
	select {
	case ack := <-reply:
		s.count(ev, ack)
		writeJSON(w, ackHTTPStatus(ack), ackResponse{Status: string(ack.Status), Code: ack.Code, EventID: ev.EventID})
	case <-ctx.Done():
		// Reintentar es seguro: el worker deduplica por eventId.
		s.backpressure(w, ev.EventID)
	}
}

func (s *Server) count(ev event.Event, ack match.Ack) {
	switch ack.Status {
	case match.Accepted:
		s.Counters.Accepted.Add(1)
	case match.Duplicate:
		s.Counters.Duplicates.Add(1)
	case match.Backpressure:
		s.Counters.Backpressure.Add(1)
	default:
		if ack.Code == "LATE_EVENT" || ack.Code == "MATCH_CLOSED" {
			s.Counters.Late.Add(1)
		}
		s.reject(obs.Rejection{EventID: ev.EventID, GameID: ev.GameID, MatchID: ev.MatchID, Code: ack.Code})
		s.Log.Warn("event rejected", "eventId", ev.EventID, "matchId", ev.MatchID, "code", ack.Code)
	}
}

func (s *Server) reject(r obs.Rejection) {
	r.At = time.Now().UTC()
	s.Counters.Rejected.Add(1)
	s.Rejections.Add(r)
}

func (s *Server) backpressure(w http.ResponseWriter, eventID string) {
	s.Counters.Backpressure.Add(1)
	w.Header().Set("Retry-After", "1")
	writeJSON(w, http.StatusServiceUnavailable, ackResponse{Status: "REJECTED", Code: "BACKPRESSURE", EventID: eventID})
}

func ackHTTPStatus(ack match.Ack) int {
	switch {
	case ack.Status == match.Accepted:
		return http.StatusAccepted
	case ack.Status == match.Duplicate:
		return http.StatusOK
	case ack.Status == match.Backpressure:
		return http.StatusServiceUnavailable
	case ack.Code == "MATCH_TOO_LARGE":
		return http.StatusRequestEntityTooLarge
	default:
		return http.StatusConflict
	}
}

func (s *Server) listMatches(w http.ResponseWriter, r *http.Request) {
	records := s.Store.List()
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
	rec, ok := s.Store.Get(matchKey(r))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "MATCH_NOT_FOUND"})
		return
	}
	rec.Result = nil
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) getResults(w http.ResponseWriter, r *http.Request) {
	rec, ok := s.Store.Get(matchKey(r))
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

func (s *Server) reanalyze(w http.ResponseWriter, r *http.Request) {
	k := matchKey(r)
	if err := s.Pool.Reanalyze(r.Context(), k); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"code": "NOT_FAILED", "message": err.Error()})
		return
	}
	s.Log.Info("reanalysis requested", "gameId", k.GameID, "matchId", k.MatchID)
	rec, _ := s.Store.Get(k)
	rec.Result = nil
	writeJSON(w, http.StatusAccepted, rec)
}

func (s *Server) rejections(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	writeJSON(w, http.StatusOK, s.Rejections.Last(limit))
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	byStatus := s.Store.CountByStatus()
	writeJSON(w, http.StatusOK, map[string]any{
		"events":        s.Counters.Snapshot(),
		"matches":       byStatus,
		"activeMatches": byStatus[store.Collecting] + byStatus[store.Finishing],
		"uptimeSeconds": int(time.Since(s.started).Seconds()),
	})
}

func matchKey(r *http.Request) event.Key {
	return event.Key{GameID: r.PathValue("gameId"), MatchID: r.PathValue("matchId")}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
