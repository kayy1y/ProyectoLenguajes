//go:build gui

package main

import (
	"fmt"
	"slices"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"gamestats/simulator/internal/lab"
)

// rejectionsTab: últimos eventos rechazados, con su código y campo.
func (u *ui) rejectionsTab() fyne.CanvasObject {
	const allCodes = "Todos los códigos"
	var all, rows []lab.RejectionRow
	list := widget.NewList(
		func() int { return len(rows) },
		func() fyne.CanvasObject {
			code := container.NewGridWrap(fyne.NewSize(240, 34), newBadge("", widget.MediumImportance))
			return container.NewBorder(nil, nil, code, nil,
				widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Monospace: true}))
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			r := rows[i]
			c := o.(*fyne.Container)
			at := r.At
			if t, err := time.Parse(time.RFC3339Nano, r.At); err == nil {
				at = t.Local().Format("15:04:05")
			}
			c.Objects[0].(*widget.Label).SetText(fmt.Sprintf("%s  %-18s %s/%s  %s  %s", at, dash(r.Field), dash(r.GameID), dash(r.MatchID), dash(r.EventID), r.Message))
			c.Objects[1].(*fyne.Container).Objects[0].(*badge).Set(" "+r.Code+" ", codeImportance(r.Code))
		})

	filter := widget.NewSelect([]string{allCodes}, nil)
	info := widget.NewLabel("")
	apply := func() {
		rows = rows[:0]
		for _, r := range all {
			if filter.Selected == allCodes || r.Code == filter.Selected {
				rows = append(rows, r)
			}
		}
		info.SetText(fmt.Sprintf("%d de %d rechazos recientes (inválidos, tardíos, conflictos…). Se actualiza cada 3 s.", len(rows), len(all)))
		list.Refresh()
	}
	filter.OnChanged = func(string) { apply() }
	filter.SetSelected(allCodes)

	refresh := func() {
		ctx, cancel := ctx5()
		rs, err := u.lab.Rejections(ctx)
		cancel()
		fyne.Do(func() {
			if err != nil {
				info.SetText("No se pudo consultar: " + err.Error())
				return
			}
			all = rs
			codes := []string{allCodes}
			for _, r := range rs {
				if !slices.Contains(codes, r.Code) {
					codes = append(codes, r.Code)
				}
			}
			filter.Options = codes
			apply()
		})
	}
	go every(3*time.Second, refresh)
	top := container.NewBorder(nil, nil, nil, container.NewGridWrap(fyne.NewSize(240, filter.MinSize().Height), filter), info)
	return container.NewBorder(container.NewPadded(top), nil, nil, nil, list)
}

// codeImportance: los rechazos por momento o duplicado son avisos; el resto, errores.
func codeImportance(code string) widget.Importance {
	switch code {
	case "LATE_EVENT", "MATCH_CLOSED", "DUPLICATE_ID_CONFLICT", "MATCH_ALREADY_STARTED", "MATCH_ALREADY_FINISHED":
		return widget.WarningImportance
	}
	return widget.DangerImportance
}
