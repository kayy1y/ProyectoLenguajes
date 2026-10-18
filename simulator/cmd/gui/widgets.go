//go:build gui

package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Piezas pequeñas de la interfaz que usan varias pestañas.

// badge es una etiqueta con fondo redondeado del color de su importancia
// (verde = bien, rojo = error, …). Se usa para estados.
type badge struct {
	widget.BaseWidget
	bg    *canvas.Rectangle
	label *widget.Label
	imp   widget.Importance
}

func newBadge(text string, imp widget.Importance) *badge {
	b := &badge{bg: canvas.NewRectangle(color.Transparent), imp: imp,
		label: widget.NewLabelWithStyle(text, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})}
	b.bg.CornerRadius = 12
	b.ExtendBaseWidget(b)
	return b
}

// Set cambia el texto y el color del badge.
func (b *badge) Set(text string, imp widget.Importance) {
	b.label.Text = text
	b.imp = imp
	b.Refresh()
}

func (b *badge) CreateRenderer() fyne.WidgetRenderer {
	b.applyColors()
	return widget.NewSimpleRenderer(container.NewStack(b.bg, b.label))
}

func (b *badge) Refresh() {
	b.applyColors()
	b.BaseWidget.Refresh()
}

func (b *badge) applyColors() {
	r, g, bl, _ := importanceColor(b.imp).RGBA()
	b.bg.FillColor = color.NRGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), 45}
	b.label.Importance = b.imp
	b.bg.Refresh()
	b.label.Refresh()
}

// importanceColor es el color del tema que corresponde a una importancia.
func importanceColor(imp widget.Importance) color.Color {
	switch imp {
	case widget.SuccessImportance:
		return theme.Color(theme.ColorNameSuccess)
	case widget.WarningImportance:
		return theme.Color(theme.ColorNameWarning)
	case widget.DangerImportance:
		return theme.Color(theme.ColorNameError)
	case widget.HighImportance:
		return theme.Color(theme.ColorNamePrimary)
	}
	return theme.Color(theme.ColorNameDisabled)
}

func bold(s string) *widget.Label {
	return widget.NewLabelWithStyle(s, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
}

func wrapped(s string) *widget.Label {
	l := widget.NewLabel(s)
	l.Wrapping = fyne.TextWrapWord
	return l
}

func statusImportance(s string) widget.Importance {
	switch s {
	case "ANALYZED", "COMPLETE", "PASS":
		return widget.SuccessImportance
	case "FAILED", "FAIL", "TIMEOUT":
		return widget.DangerImportance
	case "RETRYING", "INCOMPLETE":
		return widget.WarningImportance
	}
	return widget.MediumImportance
}

func addAll(c *fyne.Container, objs ...fyne.CanvasObject) {
	for _, o := range objs {
		c.Add(o)
	}
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// short recorta un texto largo para que quepa en una línea.
func short(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
