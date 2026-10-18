//go:build gui

package main

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"gamestats/simulator/internal/lab"
)

// matchesTab: lista de partidas (izquierda) y detalle genérico de la seleccionada (derecha).
func (u *ui) matchesTab() fyne.CanvasObject {
	var all, rows []lab.MatchSummary
	filter := ""
	headers := []string{"Juego", "Partida", "Estado", "Completitud", "Eventos"}
	widths := []float32{110, 120, 135, 140, 60}

	detail := container.NewVBox(wrapped("Selecciona una partida para ver sus estadísticas."))
	var selected *lab.MatchSummary

	table := widget.NewTable(
		func() (int, int) { return len(rows), len(headers) },
		func() fyne.CanvasObject {
			l := widget.NewLabel("")
			l.Truncation = fyne.TextTruncateEllipsis
			return container.NewBorder(nil, nil, widget.NewIcon(nil), nil, l)
		},
		func(id widget.TableCellID, o fyne.CanvasObject) {
			c := o.(*fyne.Container)
			l, icon := c.Objects[0].(*widget.Label), c.Objects[1].(*widget.Icon)
			if id.Row >= len(rows) {
				return
			}
			m := rows[id.Row]
			l.Importance = widget.MediumImportance
			l.TextStyle = fyne.TextStyle{}
			icon.Hide()
			switch id.Col {
			case 0:
				l.SetText(m.GameID)
			case 1:
				l.SetText(m.MatchID)
			case 2, 3:
				s := m.Status
				if id.Col == 3 {
					s = m.Completeness
				}
				l.Importance = statusImportance(s)
				l.TextStyle.Bold = true
				l.SetText(dash(s))
				if r := statusIcon(s); r != nil {
					icon.SetResource(r)
					icon.Show()
				}
			case 4:
				l.SetText(fmt.Sprint(m.EventCount))
			}
			c.Refresh() // vuelve a acomodar la celda si el icono se mostró u ocultó
		})
	table.ShowHeaderRow = true
	table.CreateHeader = func() fyne.CanvasObject { return bold("") }
	table.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		if id.Col >= 0 {
			o.(*widget.Label).SetText(headers[id.Col])
		}
	}
	for i, w := range widths {
		table.SetColumnWidth(i, w)
	}

	showDetail := func(m lab.MatchSummary) {
		selected = &m
		go func() {
			ctx, cancel := ctx5()
			r, st, err := u.lab.Results(ctx, m.GameID, m.MatchID)
			cancel()
			fyne.Do(func() { u.fillDetail(detail, m, r, st, err) })
		}()
	}
	table.OnSelected = func(id widget.TableCellID) {
		if id.Row >= 0 && id.Row < len(rows) {
			showDetail(rows[id.Row])
		}
	}

	kpis, updateKPIs := summaryTiles()
	problem := widget.NewLabel("")
	problem.Truncation = fyne.TextTruncateEllipsis
	problem.Importance = widget.DangerImportance
	problem.Hide()
	apply := func() {
		rows = rows[:0]
		for _, m := range all {
			if filter == "" || strings.Contains(strings.ToLower(m.MatchID+" "+m.GameID+" "+m.Status), filter) {
				rows = append(rows, m)
			}
		}
		table.Refresh()
	}
	search := widget.NewEntry()
	search.SetPlaceHolder("Filtrar por partida, juego o estado…")
	search.OnChanged = func(s string) { filter = strings.ToLower(strings.TrimSpace(s)); apply() }

	refresh := func() {
		ctx, cancel := ctx5()
		ms, err := u.lab.Matches(ctx)
		st, _ := u.lab.Stats(ctx)
		cancel()
		fyne.Do(func() {
			if err != nil {
				problem.SetText("No se pudo consultar la ingestión: " + err.Error())
				problem.Show()
				return
			}
			problem.Hide()
			all = ms
			apply()
			updateKPIs(st, len(ms))
		})
	}
	refreshBtn := widget.NewButtonWithIcon("Actualizar", theme.ViewRefreshIcon(), func() {
		go refresh()
		if selected != nil {
			showDetail(*selected)
		}
	})
	go every(3*time.Second, refresh)

	top := container.NewVBox(kpis, problem, container.NewBorder(nil, nil, nil, refreshBtn, search))
	left := container.NewBorder(top, nil, nil, nil, table)
	split := container.NewHSplit(left, container.NewVScroll(container.NewPadded(detail)))
	split.Offset = 0.52
	return split
}

// summaryTiles son las tarjetas con los totales de /v1/stats; la función que
// devuelve las actualiza.
func summaryTiles() (fyne.CanvasObject, func(st map[string]any, matches int)) {
	keys := []string{"accepted", "rejected", "duplicates", "late"}
	matches := newStatTile("Partidas", widget.HighImportance)
	tiles := []*statTile{
		newStatTile("Aceptados", widget.SuccessImportance),
		newStatTile("Rechazados", widget.DangerImportance),
		newStatTile("Duplicados", widget.WarningImportance),
		newStatTile("Tardíos", widget.WarningImportance),
	}
	grid := container.NewGridWithColumns(len(tiles)+1, matches)
	for _, t := range tiles {
		grid.Add(t)
	}
	return grid, func(st map[string]any, n int) {
		matches.SetValue(fmt.Sprint(n))
		ev, _ := st["events"].(map[string]any)
		for i, k := range keys {
			v := "—"
			if ev != nil {
				v = lab.Value(ev[k])
			}
			tiles[i].SetValue(v)
		}
	}
}

// fillDetail dibuja el resultado sin conocer el juego: las columnas salen de metrics.
func (u *ui) fillDetail(box *fyne.Container, m lab.MatchSummary, r *lab.MatchResult, status int, err error) {
	box.RemoveAll()
	title := widget.NewLabelWithStyle(m.MatchID, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.SizeName = theme.SizeNameSubHeadingText
	box.Add(title)
	if err != nil {
		box.Add(wrapped("Error: " + err.Error()))
		return
	}
	if status == 404 {
		box.Add(wrapped("La partida no existe."))
		return
	}
	st := widget.NewLabel(r.Status)
	st.Importance = statusImportance(r.Status)
	box.Add(container.NewHBox(widget.NewLabel(m.GameID+" ·"), st,
		widget.NewLabel(fmt.Sprintf("· %d eventos · %d intento(s) · cierre: %s", r.EventCount, r.Attempts, dash(r.ClosedReason)))))

	if r.Status == "FAILED" {
		box.Add(wrapped("El análisis falló: " + r.LastError))
		box.Add(widget.NewButtonWithIcon("Reanalizar", theme.ViewRefreshIcon(), func() {
			go func() {
				ctx, cancel := ctx5()
				code, err := u.lab.Reanalyze(ctx, m.GameID, m.MatchID)
				cancel()
				fyne.Do(func() {
					if err != nil || code != 202 {
						dialog.ShowInformation("Reanalizar", fmt.Sprintf("No se pudo (HTTP %d, %v)", code, err), u.win)
					} else {
						dialog.ShowInformation("Reanalizar", "Reanálisis solicitado. Pulsa Actualizar en unos segundos.", u.win)
					}
				})
			}()
		}))
		return
	}
	res := r.Result
	if res == nil {
		box.Add(wrapped("La partida sigue en curso o en análisis. Pulsa Actualizar en unos segundos."))
		return
	}

	addAll(box, widget.NewSeparator(), bold("Estadísticas de la partida"))
	grid := container.NewGridWithColumns(4)
	for _, kv := range lab.MatchMetrics(res) {
		v := widget.NewLabelWithStyle(kv[1], fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		grid.Add(container.NewVBox(v, widget.NewLabel(kv[0])))
	}
	box.Add(grid)

	addAll(box, widget.NewSeparator(), bold("Estadísticas por jugador"))
	cols := lab.MetricColumns(res)
	pt := widget.NewTable(
		func() (int, int) { return len(res.PlayerStats), len(cols) + 1 },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, o fyne.CanvasObject) {
			p := res.PlayerStats[id.Row]
			if id.Col == 0 {
				o.(*widget.Label).SetText(p.PlayerID)
				return
			}
			o.(*widget.Label).SetText(lab.Value(p.Metrics[cols[id.Col-1]]))
		})
	pt.ShowHeaderRow = true
	pt.CreateHeader = func() fyne.CanvasObject { return bold("") }
	pt.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		if id.Col == 0 {
			o.(*widget.Label).SetText("Jugador")
		} else if id.Col > 0 {
			o.(*widget.Label).SetText(lab.Humanize(cols[id.Col-1]))
		}
	}
	pt.SetColumnWidth(0, 80)
	for i, c := range cols {
		pt.SetColumnWidth(i+1, float32(max(80, 9*len(lab.Humanize(c)))))
	}
	rowsH := float32(len(res.PlayerStats)+1) * 38
	box.Add(container.NewGridWrap(fyne.NewSize(600, min(rowsH, 300)), pt))
	box.Add(widget.NewLabel("«—» = no calculable con los datos recibidos (el sistema no inventa valores)."))

	addAll(box, widget.NewSeparator(), bold("Patrones secuenciales"))
	if len(res.Patterns) == 0 {
		box.Add(widget.NewLabel("Ningún patrón en esta partida."))
	}
	for _, p := range res.Patterns {
		box.Add(wrapped("• " + lab.Pattern(p)))
	}

	addAll(box, widget.NewSeparator(), bold("Calidad de los datos"))
	box.Add(widget.NewLabel(fmt.Sprintf("Eventos recibidos %d · usados %d · ignorados %d",
		res.Input.EventsReceived, res.Input.EventsUsed, res.Input.EventsIgnored)))
	if len(res.Warnings) == 0 {
		box.Add(widget.NewLabel("Sin avisos: todos los eventos eran coherentes."))
	}
	for _, w := range res.Warnings {
		l := wrapped(fmt.Sprintf("⚠ %s  %s: %s", w.EventID, w.Code, w.Message))
		l.Importance = widget.WarningImportance
		box.Add(l)
	}
	box.Refresh()
}
