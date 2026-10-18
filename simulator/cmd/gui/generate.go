//go:build gui

package main

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"gamestats/simulator/internal/lab"
)

const (
	modeFixtures = "Casos de prueba (fixtures)"
	modeDemo     = "Escenario demo"
	modeStress   = "Carga (stress)"
)

var speeds = map[string]float64{"Rápido": 200, "Normal": 50, "Lento (se ve el intercalado)": 8}

func (u *ui) generateTab() fyne.CanvasObject {
	var names, labels []string
	byLabel := map[string]string{}
	for _, f := range u.cat.Fixtures {
		l := fmt.Sprintf("%s  [%s]  —  %s", f.Name, f.GameID, short(f.Description, 95))
		labels = append(labels, l)
		byLabel[l] = f.Name
		names = append(names, f.Name)
	}
	fixtures := widget.NewCheckGroup(labels, nil)
	if len(labels) > 0 {
		fixtures.SetSelected(labels[:1])
	}
	allBtn := widget.NewButton("Todos", func() { fixtures.SetSelected(labels) })
	noneBtn := widget.NewButton("Ninguno", func() { fixtures.SetSelected(nil) })

	seed := widget.NewEntry()
	seed.SetText("42")
	matches := widget.NewEntry()
	matches.SetText("20")
	seed.Validator = func(s string) error {
		if _, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64); err != nil {
			return errors.New("la semilla debe ser un número entero positivo, por ejemplo 42")
		}
		return nil
	}
	matches.Validator = func(s string) error {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err != nil || n < 1 || n > 200 {
			return errors.New("las partidas deben ser un número entre 1 y 200")
		}
		return nil
	}
	speed := widget.NewSelect([]string{"Rápido", "Normal", "Lento (se ve el intercalado)"}, nil)
	speed.SetSelected("Rápido")

	desc := func(n string) string {
		for _, s := range u.cat.Scenarios {
			if s.Name == n {
				return s.Description
			}
		}
		return ""
	}
	fixturesBox := container.NewVBox(container.NewHBox(allBtn, noneBtn), fixtures)
	demoBox := container.NewVBox(wrapped(desc("demo")), widget.NewForm(widget.NewFormItem("Semilla", seed)),
		widget.NewLabel("La misma semilla genera exactamente los mismos eventos."))
	stressBox := container.NewVBox(wrapped(desc("stress")))
	stressForm := widget.NewForm(widget.NewFormItem("Partidas", matches), widget.NewFormItem("Semilla", seed))
	options := container.NewStack()

	mode := widget.NewRadioGroup([]string{modeFixtures, modeDemo, modeStress}, func(s string) {
		switch s {
		case modeDemo:
			options.Objects = []fyne.CanvasObject{demoBox}
		case modeStress:
			options.Objects = []fyne.CanvasObject{container.NewVBox(stressBox, stressForm)}
		default:
			options.Objects = []fyne.CanvasObject{fixturesBox}
		}
		options.Refresh()
	})
	mode.Horizontal = true
	mode.SetSelected(modeFixtures)

	progress := widget.NewProgressBar()
	state := widget.NewLabel("Elige qué enviar y pulsa Ejecutar. Cada ejecución usa identificadores propios: se puede repetir sin reiniciar nada.")
	state.Wrapping = fyne.TextWrapWord
	out := container.NewVBox()

	var run *widget.Button
	run = widget.NewButtonWithIcon("Ejecutar", theme.MediaPlayIcon(), func() {
		req := lab.RunRequest{Speed: speeds[speed.Selected]}
		// Antes una semilla inválida se convertía en 0 sin avisar.
		if mode.Selected != modeFixtures {
			if err := seed.Validate(); err != nil {
				dialog.ShowError(err, u.win)
				return
			}
		}
		if mode.Selected == modeStress {
			if err := matches.Validate(); err != nil {
				dialog.ShowError(err, u.win)
				return
			}
		}
		req.Seed, _ = strconv.ParseUint(strings.TrimSpace(seed.Text), 10, 64)
		switch mode.Selected {
		case modeDemo:
			req.Kind = "demo"
		case modeStress:
			req.Kind = "stress"
			req.Matches, _ = strconv.Atoi(strings.TrimSpace(matches.Text))
		default:
			req.Kind = "fixtures"
			for _, l := range fixtures.Selected {
				req.Names = append(req.Names, byLabel[l])
			}
			sort.Strings(req.Names)
		}
		j, err := u.lab.StartRun(req)
		if err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		run.Disable()
		go u.follow(j, 400*time.Millisecond, func(v *lab.Job) {
			drawRun(v, progress, state, out)
			if v.State != "running" {
				run.Enable()
			}
		})
	})
	run.Importance = widget.HighImportance

	form := container.NewVBox(mode, options, widget.NewSeparator(),
		container.NewHBox(widget.NewLabel("Velocidad"), speed, run))
	return container.NewVScroll(container.NewPadded(container.NewVBox(form, widget.NewSeparator(), state, progress, out)))
}

// follow consulta el trabajo hasta que termina y entrega cada vista en el hilo de la UI.
func (u *ui) follow(j *lab.Job, every time.Duration, draw func(*lab.Job)) {
	for {
		v := j.View()
		fyne.Do(func() { draw(v) })
		if v.State != "running" {
			return
		}
		time.Sleep(every)
	}
}

func drawRun(j *lab.Job, progress *widget.ProgressBar, state *widget.Label, out *fyne.Container) {
	if j.Total > 0 {
		progress.SetValue(float64(j.Sent) / float64(j.Total))
	}
	switch j.State {
	case "running":
		state.SetText(fmt.Sprintf("En curso: %s · %d/%d eventos · sufijo %s", j.Step, j.Sent, j.Total, j.Suffix))
	case "done":
		state.SetText(fmt.Sprintf("Terminado: %d eventos enviados (sufijo %s). Las partidas ya están en la pestaña Partidas.", j.Sent, j.Suffix))
	default:
		state.SetText("Error: " + j.Error)
	}
	out.RemoveAll()
	out.Add(bold("Respuestas de la ingestión"))
	keys := make([]string, 0, len(j.Counts))
	for k := range j.Counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		row := container.NewHBox(container.NewCenter(newBadge(fmt.Sprintf(" %d ", j.Counts[k]), httpImportance(k))),
			widget.NewLabelWithStyle(k, fyne.TextAlignLeading, fyne.TextStyle{Monospace: true}))
		if e := lab.Explain(k); e != "" {
			row.Add(widget.NewLabel("— " + e))
		}
		out.Add(row)
	}
	if len(j.Matches) > 0 {
		addAll(out, widget.NewSeparator(), bold("Partidas"))
		for _, m := range j.Matches {
			st := container.NewCenter(newBadge(" "+m.Status+" ", statusImportance(m.Status)))
			row := container.NewHBox(widget.NewLabel(fmt.Sprintf("%s / %s", m.GameID, m.MatchID)), st,
				widget.NewLabel(fmt.Sprintf("%d eventos", m.EventCount)))
			if len(m.Patterns) > 0 {
				row.Add(widget.NewLabel("patrones: " + strings.Join(m.Patterns, ", ")))
			}
			if m.Check != nil {
				verdict := "FAIL"
				if m.Check.Pass {
					verdict = "PASS"
				}
				row.Add(container.NewCenter(newBadge(" calculado a mano: "+verdict+" ", statusImportance(verdict))))
			}
			out.Add(row)
		}
	}
	out.Refresh()
}

// httpImportance colorea una respuesta de la ingestión según su código HTTP
// ("202 ACCEPTED", "409 REJECTED LATE_EVENT", …).
func httpImportance(key string) widget.Importance {
	switch {
	case strings.HasPrefix(key, "2"):
		return widget.SuccessImportance
	case strings.HasPrefix(key, "400"), strings.HasPrefix(key, "5"):
		return widget.DangerImportance
	}
	return widget.WarningImportance
}
