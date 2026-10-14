//go:build gui

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"gamestats/simulator/internal/lab"
)

// sendTab: editor de un evento con plantillas del contrato (válidas e inválidas).
func (u *ui) sendTab() fyne.CanvasObject {
	editor := widget.NewMultiLineEntry()
	editor.TextStyle = fyne.TextStyle{Monospace: true}
	editor.SetText(`{
  "eventId": "mi-evento-1",
  "timestamp": "2026-10-01T15:00:00Z",
  "gameId": "arena-combat",
  "gameVersion": "1.0",
  "matchId": "mi-partida",
  "type": "MATCH_STARTED",
  "data": { "map": "dust" }
}`)
	note := wrapped("")
	response := widget.NewMultiLineEntry()
	response.TextStyle = fyne.TextStyle{Monospace: true}
	response.SetMinRowsVisible(8)
	code := widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	var opts []string
	kindOf := map[string]string{}
	for _, f := range u.cat.Valid {
		o := "válido: " + f
		opts, kindOf[o] = append(opts, o), "valid/"+f
	}
	expectOf := map[string]string{}
	for _, e := range u.cat.Invalid {
		o := "inválido: " + e.File
		opts, kindOf[o] = append(opts, o), "invalid/"+e.File
		expectOf[o] = fmt.Sprintf("Este ejemplo es inválido a propósito. Se espera HTTP 400 con código %s en el campo %s.", e.Code, dash(e.Field))
	}
	tpl := widget.NewSelect(opts, func(o string) {
		kind, file, _ := strings.Cut(kindOf[o], "/")
		b, err := u.lab.Example(kind, file)
		if err != nil {
			note.SetText(err.Error())
			return
		}
		editor.SetText(string(b))
		note.SetText(expectOf[o])
	})
	tpl.PlaceHolder = "Elegir una plantilla del contrato…"

	fresh := widget.NewButtonWithIcon("Renovar ids", theme.ViewRefreshIcon(), func() {
		var e map[string]any
		if json.Unmarshal([]byte(editor.Text), &e) != nil {
			note.SetText("El JSON no es válido: no se pueden renovar los ids.")
			return
		}
		s := fmt.Sprintf("-%04d", rand.IntN(10000))
		for _, k := range []string{"eventId", "matchId"} {
			if v, ok := e[k].(string); ok {
				if i := strings.LastIndex(v, "-"); i > 0 && len(v)-i == 5 {
					v = v[:i]
				}
				e[k] = v + s
			}
		}
		b, _ := json.MarshalIndent(e, "", "  ")
		editor.SetText(string(b))
	})
	send := widget.NewButtonWithIcon("Enviar", theme.MailSendIcon(), func() {
		body := []byte(editor.Text)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			st, resp, err := u.lab.SendRaw(ctx, body)
			cancel()
			fyne.Do(func() {
				if err != nil {
					code.SetText("Sin respuesta: " + err.Error())
					code.Importance = widget.DangerImportance
				} else {
					code.SetText(fmt.Sprintf("HTTP %d", st))
					code.Importance = widget.WarningImportance
					if st == 200 || st == 202 {
						code.Importance = widget.SuccessImportance
					}
				}
				code.Refresh()
				response.SetText(resp)
			})
		}()
	})
	send.Importance = widget.HighImportance

	top := container.NewVBox(wrapped("Construye un evento a mano, o parte de un ejemplo del contrato, y mira cómo responde la ingestión."),
		container.NewBorder(nil, nil, nil, container.NewHBox(fresh, send), tpl), note)
	bottom := container.NewVBox(code, response)
	return container.NewPadded(container.NewBorder(top, bottom, nil, nil, editor))
}

// rejectionsTab: últimos eventos rechazados, con su código y campo.
func (u *ui) rejectionsTab() fyne.CanvasObject {
	var rows []lab.RejectionRow
	list := widget.NewList(
		func() int { return len(rows) },
		func() fyne.CanvasObject {
			return widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			r := rows[i]
			t, err := time.Parse(time.RFC3339Nano, r.At)
			at := r.At
			if err == nil {
				at = t.Local().Format("15:04:05")
			}
			o.(*widget.Label).SetText(fmt.Sprintf("%s  %-24s %-18s %s/%s  %s  %s", at, r.Code, dash(r.Field), dash(r.GameID), dash(r.MatchID), dash(r.EventID), r.Message))
		})
	info := widget.NewLabel("")
	refresh := func() {
		ctx, cancel := ctx5()
		rs, err := u.lab.Rejections(ctx)
		cancel()
		fyne.Do(func() {
			if err != nil {
				info.SetText("No se pudo consultar: " + err.Error())
				return
			}
			rows = rs
			info.SetText(fmt.Sprintf("%d rechazos recientes (inválidos, tardíos, conflictos…). Se actualiza cada 3 s.", len(rs)))
			list.Refresh()
		})
	}
	go every(3*time.Second, refresh)
	return container.NewBorder(container.NewPadded(info), nil, nil, nil, list)
}

func (u *ui) guideTab() fyne.CanvasObject {
	steps := []string{
		"1. Generar datos → «Escenario demo» → Ejecutar. Se envían partidas de dos juegos en paralelo, con eventos inválidos, un duplicado, un conflicto, un evento tardío y una partida sin cierre.",
		"2. Partidas → cada una pasa por COLLECTING → ANALYZING → ANALYZED. La que no tiene MATCH_FINISHED se cierra por inactividad y queda INCOMPLETE.",
		"3. Selecciona una partida: estadísticas por jugador, por partida, combinadas y patrones secuenciales. Lo no calculable aparece como «—», nunca como 0 inventado.",
		"4. Verificación → Ejecutar suite completa. Compara con los resultados calculados a mano, los 17 eventos inválidos y el ciclo de vida. Guarda el informe como evidencia.",
		"5. Enviar evento → elige un ejemplo inválido y mira el campo y el código del error.",
	}
	box := container.NewVBox(bold("Recorrido sugerido (5 minutos)"))
	for _, s := range steps {
		box.Add(wrapped(s))
	}
	addAll(box, widget.NewSeparator(), bold("Juegos soportados (leídos de contracts/schemas/games)"))
	for _, g := range u.cat.Games {
		box.Add(wrapped(fmt.Sprintf("• %s  v%s:  %s", g.GameID, strings.Join(g.Versions, ", "), strings.Join(g.EventTypes, ", "))))
	}
	addAll(box, widget.NewSeparator(), bold("Cómo está dividido el trabajo"),
		wrapped("• Go (imperativo): recibe, valida con JSON Schema, agrupa por partida (una goroutine por partida), reintenta y expone los resultados. No calcula estadísticas."),
		wrapped("• Scala (funcional): recibe el lote ordenado de una partida y calcula métricas y patrones con funciones puras."),
		wrapped("• Esta aplicación: herramienta de apoyo. Solo usa la API pública de la ingestión; no participa en el análisis."),
		widget.NewSeparator(), bold("Agregar un juego nuevo"),
		wrapped("Un esquema en contracts/schemas/games, un GameAnalyzer en Scala con una línea en el registro, y un fixture con su resultado esperado. Ni la ingestión en Go, ni el núcleo de Scala, ni esta aplicación cambian."))
	return container.NewVScroll(container.NewPadded(box))
}
