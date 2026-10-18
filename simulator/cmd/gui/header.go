//go:build gui

package main

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// header es la barra superior: logo, nombre, servidor y estado de la ingestión.
// El estado se consulta cada 3 s en segundo plano.
func (u *ui) header(target string) fyne.CanvasObject {
	logo := canvas.NewImageFromResource(appIcon)
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(38, 38))

	title := bold("GameStats")
	title.SizeName = theme.SizeNameSubHeadingText
	subtitle := widget.NewLabel("Ingestión en Go · análisis en Scala")
	subtitle.SizeName = theme.SizeNameCaptionText
	names := container.New(layout.NewCustomPaddedVBoxLayout(-14), title, subtitle)

	server := widget.NewLabelWithStyle(target, fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})
	health := newBadge("  conectando…  ", widget.MediumImportance)

	left := container.NewHBox(logo, names)
	right := container.NewHBox(widget.NewIcon(theme.ComputerIcon()), container.NewCenter(server), container.NewCenter(health))
	bg := canvas.NewRectangle(theme.Color(theme.ColorNameHeaderBackground))

	go every(3*time.Second, func() {
		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		ok := u.lab.Health(ctx)
		cancel()
		ms := time.Since(start).Milliseconds()
		fyne.Do(func() {
			if ok {
				health.Set(fmt.Sprintf("  ● ingestión en línea · %d ms  ", ms), widget.SuccessImportance)
			} else {
				health.Set("  ● ingestión sin respuesta  ", widget.DangerImportance)
			}
		})
	})

	bar := container.NewBorder(nil, nil, left, right)
	return container.NewVBox(container.NewStack(bg, container.NewPadded(bar)), widget.NewSeparator())
}
