package lab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"gamestats/simulator/internal/verify"
)

// StartVerify lanza la suite completa: fixtures, ejemplos inválidos y comportamiento.
func (js *Lab) StartVerify() *Job {
	j := js.create("verify")
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
		defer cancel()
		j.finish(js.runVerify(ctx, j))
	}()
	return j
}

func (js *Lab) addCheck(j *Job, c verify.Check) {
	j.update(func(j *Job) { j.Checks = append(j.Checks, c) })
}

func (js *Lab) runVerify(ctx context.Context, j *Job) error {
	// 1. Todos los fixtures en paralelo contra los resultados calculados a mano.
	j.update(func(j *Job) { j.Step = "fixtures" })
	j.logf("grupo 1: fixtures contra contracts/expected")
	ms, err := js.buildMatches(RunRequest{Kind: "fixtures", Names: []string{"all"}}, j.Suffix)
	if err != nil {
		return err
	}
	total := 0
	for _, m := range ms {
		total += len(m.Lines)
	}
	j.update(func(j *Job) { j.Total = total })
	if err := js.execute(ctx, j, ms, js.senderConfig(j, 200), true); err != nil {
		return err
	}
	for _, v := range j.View().Matches {
		if v.Check != nil {
			js.addCheck(j, *v.Check)
		} else {
			js.addCheck(j, verify.Check{Group: "fixtures", Name: v.MatchID, Detail: "sin resultado esperado"})
		}
	}

	// 2. Ejemplos inválidos: el primer error debe ser el documentado.
	j.update(func(j *Job) { j.Step = "ejemplos inválidos" })
	j.logf("grupo 2: ejemplos inválidos contra expected.json")
	checks, err := verify.Invalid(js.cfg.ExamplesDir, func(body []byte) (verify.Rejection, error) {
		st, m, err := js.postEvent(ctx, body)
		if err != nil {
			return verify.Rejection{}, err
		}
		r := verify.Rejection{HTTP: st}
		if errs, ok := m["errors"].([]any); ok && len(errs) > 0 {
			if e, ok := errs[0].(map[string]any); ok {
				r.Code, _ = e["code"].(string)
				r.Field, _ = e["field"].(string)
			}
		} else {
			r.Code, _ = m["code"].(string)
		}
		return r, nil
	})
	if err != nil {
		return err
	}
	for _, c := range checks {
		js.addCheck(j, c)
	}

	// 3. Comportamiento del ciclo de vida de una partida.
	j.update(func(j *Job) { j.Step = "comportamiento" })
	j.logf("grupo 3: ciclo de vida, duplicados y eventos tardíos")
	js.behaviors(ctx, j)
	return nil
}

func (js *Lab) behaviors(ctx context.Context, j *Job) {
	match := "verif" + j.Suffix
	mk := func(id, typ, player string, data map[string]any) []byte {
		e := map[string]any{"eventId": id + j.Suffix, "timestamp": "2026-10-01T15:00:00Z", "gameId": "arena-combat",
			"gameVersion": "1.0", "matchId": match, "type": typ, "data": data}
		if player != "" {
			e["playerId"] = player
		}
		b, _ := json.Marshal(e)
		return b
	}
	post := func(name, want string, body []byte) {
		st, m, err := js.postEvent(ctx, body)
		js.addCheck(j, labelCheck(name, want, st, m, err))
	}
	get := func(name, want, path string) {
		st, m, err := js.getJSON(ctx, path)
		js.addCheck(j, labelCheck(name, want, st, m, err))
	}
	game := "/v1/games/arena-combat/matches/"
	start := mk("v1", "MATCH_STARTED", "", map[string]any{"map": "dust"})
	post("Evento válido aceptado", "202 ACCEPTED", start)
	post("Mismo evento repetido es idempotente", "200 DUPLICATE", start)
	post("Mismo eventId con otro contenido es conflicto", "409 DUPLICATE_ID_CONFLICT", mk("v1", "MATCH_STARTED", "", map[string]any{"map": "otro"}))
	post("Segundo MATCH_STARTED se rechaza", "409 MATCH_ALREADY_STARTED", mk("v2", "MATCH_STARTED", "", map[string]any{"map": "dust"}))
	post("Jugador se une", "202 ACCEPTED", mk("v3", "PLAYER_JOINED", "p1", map[string]any{}))
	get("Resultado de una partida abierta aún no existe", "202 COLLECTING", game+match+"/results")
	post("Fin de partida aceptado", "202 ACCEPTED", mk("v4", "MATCH_FINISHED", "", map[string]any{"reason": "COMPLETED"}))
	post("Segundo MATCH_FINISHED se rechaza", "409 MATCH_ALREADY_FINISHED", mk("v5", "MATCH_FINISHED", "", map[string]any{"reason": "COMPLETED"}))

	status := ""
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline) && ctx.Err() == nil; time.Sleep(300 * time.Millisecond) {
		if _, m, err := js.getJSON(ctx, game+match); err == nil {
			status, _ = m["status"].(string)
			if status == "ANALYZED" || status == "FAILED" {
				break
			}
		}
	}
	c := verify.Check{Group: "comportamiento", Name: "La partida se analiza tras el período de gracia", Expected: "ANALYZED", Actual: status}
	c.Pass = status == "ANALYZED"
	js.addCheck(j, c)

	post("Evento posterior al análisis se rechaza", "409 LATE_EVENT", mk("v6", "PLAYER_JOINED", "p2", map[string]any{}))
	get("Partida inexistente responde 404", "404 MATCH_NOT_FOUND", game+"no-existe"+j.Suffix)
	get("Servicio de ingestión saludable", "200 ok", "/healthz")
}

func labelCheck(name, want string, status int, m map[string]any, err error) verify.Check {
	c := verify.Check{Group: "comportamiento", Name: name, Expected: want}
	if err != nil {
		c.Detail = err.Error()
		return c
	}
	label, _ := m["code"].(string)
	if label == "" {
		label, _ = m["status"].(string)
	}
	c.Actual = fmt.Sprintf("%d %s", status, label)
	c.Pass = c.Actual == want
	return c
}

func (js *Lab) postEvent(ctx context.Context, body []byte) (int, map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, js.cfg.Target+"/v1/events", bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if js.cfg.APIKey != "" {
		req.Header.Set("X-API-Key", js.cfg.APIKey)
	}
	return js.do(req)
}

func (js *Lab) getJSON(ctx context.Context, path string) (int, map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, js.cfg.Target+path, nil)
	if err != nil {
		return 0, nil, err
	}
	return js.do(req)
}

func (js *Lab) do(req *http.Request) (int, map[string]any, error) {
	resp, err := js.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	var m map[string]any
	json.NewDecoder(resp.Body).Decode(&m)
	if m == nil {
		m = map[string]any{}
	}
	return resp.StatusCode, m, nil
}
