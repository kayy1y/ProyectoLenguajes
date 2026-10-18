//go:build gui

package main

import (
	"cmp"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"gamestats/simulator/internal/lab"
)

func (u *ui) verifyTab() fyne.CanvasObject {
	intro := wrapped("Ejecuta la batería de pruebas contra el sistema en marcha:\n" +
		"• Casos de prueba: cada fixture pasa por Go y Scala y el resultado debe ser idéntico al calculado a mano (contracts/expected).\n" +
		"• Eventos inválidos: cada ejemplo debe rechazarse con 400 y el primer error debe tener el código y el campo documentados.\n" +
		"• Comportamiento: idempotencia, conflictos, ciclo de vida de la partida, período de gracia y partidas inexistentes.")
	summary := widget.NewLabel("")
	summary.TextStyle = fyne.TextStyle{Bold: true}
	progress := widget.NewProgressBarInfinite()
	progress.Hide()
	results := container.NewVBox()
	var last *lab.Job

	save := widget.NewButtonWithIcon("Guardar informe…", theme.DocumentSaveIcon(), func() {
		if last == nil {
			return
		}
		d := dialog.NewFileSave(func(w fyne.URIWriteCloser, err error) {
			if err != nil || w == nil {
				return
			}
			defer w.Close()
			if _, err := w.Write([]byte(lab.Report(last))); err != nil {
				dialog.ShowError(err, u.win)
			}
		}, u.win)
		d.SetFileName("verificacion-" + time.Now().Format("20060102-150405") + ".md")
		d.SetFilter(storage.NewExtensionFileFilter([]string{".md"}))
		d.Show()
	})
	save.Disable()

	var run *widget.Button
	run = widget.NewButtonWithIcon("Ejecutar suite completa", theme.MediaPlayIcon(), func() {
		run.Disable()
		save.Disable()
		progress.Show()
		progress.Start()
		j := u.lab.StartVerify()
		go u.follow(j, 500*time.Millisecond, func(v *lab.Job) {
			drawChecks(v, summary, results)
			if v.State != "running" {
				progress.Stop()
				progress.Hide()
				run.Enable()
				save.Enable()
				last = v
			}
		})
	})
	run.Importance = widget.HighImportance

	top := container.NewVBox(intro, container.NewHBox(run, save), progress, summary)
	return container.NewBorder(container.NewPadded(top), nil, nil, nil, container.NewVScroll(container.NewPadded(results)))
}

func drawChecks(j *lab.Job, summary *widget.Label, box *fyne.Container) {
	ok := 0
	for _, c := range j.Checks {
		if c.Pass {
			ok++
		}
	}
	switch {
	case j.State == "running":
		summary.SetText(fmt.Sprintf("En curso: %s… (%d comprobaciones hasta ahora, %d correctas)", j.Step, len(j.Checks), ok))
		summary.Importance = widget.MediumImportance
	case j.Error != "":
		summary.SetText("Error: " + j.Error)
		summary.Importance = widget.DangerImportance
	case ok == len(j.Checks):
		summary.SetText(fmt.Sprintf("✓ %d de %d comprobaciones pasaron", ok, len(j.Checks)))
		summary.Importance = widget.SuccessImportance
	default:
		summary.SetText(fmt.Sprintf("✗ %d de %d comprobaciones pasaron", ok, len(j.Checks)))
		summary.Importance = widget.DangerImportance
	}
	summary.Refresh()

	box.RemoveAll()
	box.Add(groupSummary(j))
	group := ""
	for _, c := range j.Checks {
		if c.Group != group {
			group = c.Group
			box.Add(widget.NewSeparator())
			box.Add(bold(lab.GroupTitle(group)))
		}
		verdict := "FAIL"
		if c.Pass {
			verdict = "PASS"
		}
		v := container.NewVBox(newBadge(" "+verdict+" ", statusImportance(verdict)))
		text := c.Name
		if c.Detail != "" {
			text += " — " + c.Detail
		}
		line := fmt.Sprintf("%s\n   esperado: %s   ·   obtenido: %s", text, c.Expected, c.Actual)
		if len(c.Expected) > 70 {
			line = text + "\n   el resultado completo difiere del calculado a mano (ver abajo)"
			if c.Pass {
				line = text + "\n   resultado idéntico al calculado a mano"
			}
		}
		row := container.NewBorder(nil, nil, v, nil, wrapped(line))
		box.Add(row)
		if !c.Pass && len(c.Expected) > 70 {
			diff := widget.NewMultiLineEntry()
			diff.SetText("ESPERADO\n" + c.Expected + "\n\nOBTENIDO\n" + c.Actual)
			diff.Wrapping = fyne.TextWrapBreak
			diff.SetMinRowsVisible(6)
			box.Add(diff)
		}
	}
	box.Refresh()
}

// groupSummary resume cada grupo de comprobaciones en un badge: «Casos de prueba 8/8».
func groupSummary(j *lab.Job) fyne.CanvasObject {
	var groups []string
	pass, total := map[string]int{}, map[string]int{}
	for _, c := range j.Checks {
		if total[c.Group] == 0 {
			groups = append(groups, c.Group)
		}
		total[c.Group]++
		if c.Pass {
			pass[c.Group]++
		}
	}
	names := map[string]string{"fixtures": "Casos de prueba", "inválidos": "Eventos inválidos", "comportamiento": "Comportamiento"}
	row := container.NewHBox()
	for _, g := range groups {
		imp := widget.SuccessImportance
		if pass[g] < total[g] {
			imp = widget.DangerImportance
		}
		row.Add(newBadge(fmt.Sprintf(" %s %d/%d ", cmp.Or(names[g], g), pass[g], total[g]), imp))
	}
	return row
}
