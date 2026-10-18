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
	sortKeys := []string{"game", "match", "status", "completeness", "events"}
	widths := []float32{110, 120, 135, 140, 75}
	sortCol, sortDesc := -1, true // -1: las actualizadas más recientemente primero

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
	var apply func()
	// Encabezados clicables: un clic ordena por esa columna, otro invierte el orden.
	table.ShowHeaderRow = true
	table.CreateHeader = func() fyne.CanvasObject {
		b := widget.NewButton("", nil)
		b.Importance = widget.LowImportance
		b.Alignment = widget.ButtonAlignLeading
		b.IconPlacement = widget.ButtonIconTrailingText
		return b
	}
	table.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		if id.Col < 0 {
			return
		}
		b, col := o.(*widget.Button), id.Col
		b.SetText(headers[col])
		switch {
		case col != sortCol:
			b.SetIcon(nil)
		case sortDesc:
			b.SetIcon(theme.MoveDownIcon())
		default:
			b.SetIcon(theme.MoveUpIcon())
		}
		b.OnTapped = func() {
			if sortCol == col {
				sortDesc = !sortDesc
			} else {
				sortCol, sortDesc = col, false
			}
			table.UnselectAll()
			apply()
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

	const allGames, allStates = "Todos los juegos", "Todos los estados"
	games := []string{allGames}
	for _, g := range u.cat.Games {
		games = append(games, g.GameID)
	}
	gameSel := widget.NewSelect(games, nil)
	stateSel := widget.NewSelect([]string{allStates, "COLLECTING", "FINISHING", "ANALYZING", "RETRYING", "ANALYZED", "FAILED"}, nil)
	count := widget.NewLabel("")
	count.SizeName = theme.SizeNameCaptionText
	empty := wrapped("")
	empty.Alignment = fyne.TextAlignCenter

	apply = func() {
		rows = rows[:0]
		for _, m := range all {
			switch {
			case gameSel.Selected != allGames && m.GameID != gameSel.Selected,
				stateSel.Selected != allStates && m.Status != stateSel.Selected,
				filter != "" && !strings.Contains(strings.ToLower(m.MatchID+" "+m.GameID+" "+m.Status), filter):
				continue
			}
			rows = append(rows, m)
		}
		key := ""
		if sortCol >= 0 {
			key = sortKeys[sortCol]
		}
		lab.SortMatches(rows, key, sortDesc)
		count.SetText(fmt.Sprintf("Mostrando %d de %d partidas", len(rows), len(all)))
		switch {
		case len(all) == 0:
			empty.SetText("Todavía no hay partidas.\nVe a «Generar datos» y ejecuta el escenario demo.")
			empty.Show()
		case len(rows) == 0:
			empty.SetText("Ninguna partida coincide con los filtros.")
			empty.Show()
		default:
			empty.Hide()
		}
		table.Refresh()
	}
	gameSel.OnChanged = func(string) { apply() }
	stateSel.OnChanged = func(string) { apply() }
	gameSel.SetSelected(allGames)
	stateSel.SetSelected(allStates)
	search := widget.NewEntry()
	search.SetPlaceHolder("Buscar partida…")
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

	top := container.NewVBox(kpis, problem,
		container.NewBorder(nil, nil, nil, refreshBtn, search),
		container.NewGridWithColumns(2, gameSel, stateSel))
	left := container.NewBorder(top, count, nil, nil, container.NewStack(table, container.NewCenter(empty)))
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
	box.Add(container.NewHBox(container.NewCenter(newBadge("  "+r.Status+"  ", statusImportance(r.Status))),
		widget.NewLabel(fmt.Sprintf("%s · %d eventos · %d intento(s) · cierre: %s", m.GameID, r.EventCount, r.Attempts, dash(r.ClosedReason)))))

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

	box.Add(section(theme.InfoIcon(), "Estadísticas de la partida"))
	grid := container.NewGridWithColumns(3)
	for _, kv := range lab.MatchMetrics(res) {
		t := newStatTile(kv[0], widget.MediumImportance)
		t.SetValue(kv[1])
		grid.Add(t)
	}
	box.Add(grid)

	box.Add(section(theme.AccountIcon(), fmt.Sprintf("Estadísticas por jugador (%d)", len(res.PlayerStats))))
	cols := lab.MetricColumns(res)
	pt := widget.NewTable(
		func() (int, int) { return len(res.PlayerStats), len(cols) + 1 },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, o fyne.CanvasObject) {
			l, p := o.(*widget.Label), res.PlayerStats[id.Row]
			if id.Col == 0 {
				l.TextStyle.Bold = true
				l.SetText(p.PlayerID)
				return
			}
			l.TextStyle.Bold = false
			l.SetText(lab.Value(p.Metrics[cols[id.Col-1]]))
		})
	pt.StickyColumnCount = 1 // el jugador queda fijo al desplazarse a la derecha
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
	note := widget.NewLabel("«—» = no calculable con los datos recibidos (el sistema no inventa valores).")
	note.SizeName = theme.SizeNameCaptionText
	box.Add(note)

	box.Add(section(theme.MediaFastForwardIcon(), fmt.Sprintf("Patrones secuenciales (%d)", len(res.Patterns))))
	if len(res.Patterns) == 0 {
		box.Add(widget.NewLabel("Ningún patrón en esta partida."))
	}
	for _, p := range res.Patterns {
		icon := widget.NewIcon(theme.NewSuccessThemedResource(theme.MediaFastForwardIcon()))
		box.Add(container.NewBorder(nil, nil, icon, nil, wrapped(lab.Pattern(p))))
	}

	box.Add(section(theme.WarningIcon(), fmt.Sprintf("Calidad de los datos (%d avisos)", len(res.Warnings))))
	box.Add(widget.NewLabel(fmt.Sprintf("Eventos recibidos %d · usados %d · ignorados %d",
		res.Input.EventsReceived, res.Input.EventsUsed, res.Input.EventsIgnored)))
	if len(res.Warnings) == 0 {
		ok := widget.NewLabel("Sin avisos: todos los eventos eran coherentes.")
		ok.Importance = widget.SuccessImportance
		box.Add(ok)
	}
	for _, w := range res.Warnings {
		l := wrapped(fmt.Sprintf("%s  %s: %s", w.EventID, w.Code, w.Message))
		l.Importance = widget.WarningImportance
		icon := widget.NewIcon(theme.NewWarningThemedResource(theme.WarningIcon()))
		box.Add(container.NewBorder(nil, nil, icon, nil, l))
	}
	box.Refresh()
}

// section es el título de una sección del detalle, con separador e icono.
func section(icon fyne.Resource, title string) fyne.CanvasObject {
	return container.NewVBox(widget.NewSeparator(), container.NewHBox(widget.NewIcon(icon), bold(title)))
}
