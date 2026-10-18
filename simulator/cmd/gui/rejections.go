//go:build gui

package main

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"gamestats/simulator/internal/lab"
)

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
