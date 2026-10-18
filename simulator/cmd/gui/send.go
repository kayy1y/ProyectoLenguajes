//go:build gui

package main

import (
	"bytes"
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
	code := newBadge(" sin enviar ", widget.MediumImportance)
	valid := widget.NewLabel("")
	editor.OnChanged = func(s string) {
		if json.Valid([]byte(s)) {
			valid.SetText("✓ JSON válido")
			valid.Importance = widget.SuccessImportance
		} else {
			valid.SetText("✗ JSON inválido: revisa comas, llaves y comillas")
			valid.Importance = widget.DangerImportance
		}
		valid.Refresh()
	}
	editor.OnChanged(editor.Text)
	format := widget.NewButtonWithIcon("Formatear", theme.DocumentIcon(), func() {
		var out bytes.Buffer
		if json.Indent(&out, []byte(editor.Text), "", "  ") == nil {
			editor.SetText(out.String())
		}
	})

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
					code.Set(" sin respuesta ", widget.DangerImportance)
					response.SetText(err.Error())
					return
				}
				code.Set(fmt.Sprintf(" HTTP %d ", st), httpImportance(fmt.Sprint(st)))
				response.SetText(resp)
			})
		}()
	})
	send.Importance = widget.HighImportance

	top := container.NewVBox(wrapped("Construye un evento a mano, o parte de un ejemplo del contrato, y mira cómo responde la ingestión."),
		container.NewBorder(nil, nil, nil, container.NewHBox(format, fresh, send), tpl), note)
	bottom := container.NewVBox(valid, container.NewHBox(bold("Respuesta"), code), response)
	return container.NewPadded(container.NewBorder(top, bottom, nil, nil, editor))
}
