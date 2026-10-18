//go:build gui

package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
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

// statTile es una tarjeta con un número grande y su descripción debajo.
type statTile struct {
	widget.BaseWidget
	bg      *canvas.Rectangle
	value   *widget.Label
	caption *widget.Label
}

func newStatTile(caption string, imp widget.Importance) *statTile {
	t := &statTile{bg: canvas.NewRectangle(color.Transparent), caption: widget.NewLabel(caption),
		value: widget.NewLabelWithStyle("—", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})}
	t.value.SizeName = theme.SizeNameHeadingText
	t.value.Importance = imp
	t.caption.SizeName = theme.SizeNameCaptionText
	t.bg.CornerRadius = 10
	t.ExtendBaseWidget(t)
	return t
}

// SetValue cambia el número de la tarjeta.
func (t *statTile) SetValue(v string) { t.value.SetText(v) }

func (t *statTile) CreateRenderer() fyne.WidgetRenderer {
	t.bg.FillColor = theme.Color(theme.ColorNameHeaderBackground)
	body := container.New(layout.NewCustomPaddedVBoxLayout(-12), t.value, t.caption)
	return widget.NewSimpleRenderer(container.NewStack(t.bg, container.NewPadded(body)))
}

func (t *statTile) Refresh() {
	t.bg.FillColor = theme.Color(theme.ColorNameHeaderBackground)
	t.BaseWidget.Refresh()
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
	case "COLLECTING", "FINISHING", "ANALYZING":
		return widget.HighImportance
	}
	return widget.MediumImportance
}

// statusIcon es el icono, ya con su color, de un estado de partida o de completitud.
func statusIcon(s string) fyne.Resource {
	switch s {
	case "ANALYZED", "COMPLETE", "PASS":
		return theme.NewSuccessThemedResource(theme.ConfirmIcon())
	case "FAILED", "FAIL", "TIMEOUT":
		return theme.NewErrorThemedResource(theme.ErrorIcon())
	case "RETRYING":
		return theme.NewWarningThemedResource(theme.MediaReplayIcon())
	case "INCOMPLETE":
		return theme.NewWarningThemedResource(theme.WarningIcon())
	case "COLLECTING":
		return theme.NewPrimaryThemedResource(theme.DownloadIcon())
	case "FINISHING":
		return theme.NewPrimaryThemedResource(theme.HistoryIcon())
	case "ANALYZING":
		return theme.NewPrimaryThemedResource(theme.SearchIcon())
	}
	return nil
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
