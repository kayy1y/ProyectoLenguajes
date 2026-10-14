package lab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// MatchSummary es una fila de GET /v1/matches.
type MatchSummary struct {
	GameID       string `json:"gameId"`
	MatchID      string `json:"matchId"`
	Status       string `json:"status"`
	EventCount   int    `json:"eventCount"`
	Completeness string `json:"completeness"`
	ClosedReason string `json:"closedReason"`
	Attempts     int    `json:"attempts"`
	UpdatedAt    string `json:"updatedAt"`
}

// MatchResult es la respuesta de GET …/results. Result es genérico: la app no
// conoce ningún juego, recorre las claves de metrics.
type MatchResult struct {
	MatchSummary
	AnalyzedAt string  `json:"analyzedAt"`
	LastError  string  `json:"lastError"`
	Result     *Result `json:"result"`
}

type Result struct {
	Completeness string `json:"completeness"`
	Input        struct {
		EventsReceived, EventsUsed, EventsIgnored int
	} `json:"input"`
	MatchStats struct {
		DurationSeconds any            `json:"durationSeconds"`
		Participants    any            `json:"participants"`
		TotalEvents     any            `json:"totalEvents"`
		Metrics         map[string]any `json:"metrics"`
	} `json:"matchStats"`
	PlayerStats []struct {
		PlayerID string         `json:"playerId"`
		Metrics  map[string]any `json:"metrics"`
	} `json:"playerStats"`
	Patterns []map[string]any `json:"patterns"`
	Warnings []struct {
		EventID, Code, Message string
	} `json:"warnings"`
}

// Rejection es una fila de GET /v1/rejections.
type RejectionRow struct {
	At      string `json:"at"`
	Code    string `json:"code"`
	Field   string `json:"field"`
	GameID  string `json:"gameId"`
	MatchID string `json:"matchId"`
	EventID string `json:"eventId"`
	Message string `json:"message"`
}

// Health consulta /healthz.
func (js *Lab) Health(ctx context.Context) bool {
	var h struct{ Status string }
	st, err := js.getInto(ctx, "/healthz", &h)
	return err == nil && st == http.StatusOK && h.Status == "ok"
}

// Stats devuelve /v1/stats tal cual.
func (js *Lab) Stats(ctx context.Context) (map[string]any, error) {
	var m map[string]any
	_, err := js.getInto(ctx, "/v1/stats", &m)
	return m, err
}

// Matches lista todas las partidas.
func (js *Lab) Matches(ctx context.Context) ([]MatchSummary, error) {
	var out []MatchSummary
	_, err := js.getInto(ctx, "/v1/matches", &out)
	return out, err
}

// Results trae el estado y, si existe, el resultado de una partida.
func (js *Lab) Results(ctx context.Context, gameID, matchID string) (*MatchResult, int, error) {
	var r MatchResult
	st, err := js.getInto(ctx, "/v1/games/"+url.PathEscape(gameID)+"/matches/"+url.PathEscape(matchID)+"/results", &r)
	return &r, st, err
}

// Rejections trae los últimos rechazos.
func (js *Lab) Rejections(ctx context.Context) ([]RejectionRow, error) {
	var out []RejectionRow
	_, err := js.getInto(ctx, "/v1/rejections?limit=100", &out)
	return out, err
}

// Reanalyze pide reintentar una partida FAILED.
func (js *Lab) Reanalyze(ctx context.Context, gameID, matchID string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		js.cfg.Target+"/v1/games/"+url.PathEscape(gameID)+"/matches/"+url.PathEscape(matchID)+"/reanalyze", nil)
	if err != nil {
		return 0, err
	}
	if js.cfg.APIKey != "" {
		req.Header.Set("X-API-Key", js.cfg.APIKey)
	}
	st, _, err := js.do(req)
	return st, err
}

// SendRaw envía un evento tal cual y devuelve el código HTTP y el cuerpo con sangría.
func (js *Lab) SendRaw(ctx context.Context, body []byte) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, js.cfg.Target+"/v1/events", bytes.NewReader(body))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if js.cfg.APIKey != "" {
		req.Header.Set("X-API-Key", js.cfg.APIKey)
	}
	resp, err := js.client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var pretty bytes.Buffer
	if json.Indent(&pretty, raw, "", "  ") != nil {
		return resp.StatusCode, string(raw), nil
	}
	return resp.StatusCode, pretty.String(), nil
}

func (js *Lab) getInto(ctx context.Context, path string, v any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, js.cfg.Target+path, nil)
	if err != nil {
		return 0, err
	}
	resp, err := js.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil && err != io.EOF {
		return resp.StatusCode, fmt.Errorf("respuesta inválida de %s: %w", path, err)
	}
	return resp.StatusCode, nil
}
