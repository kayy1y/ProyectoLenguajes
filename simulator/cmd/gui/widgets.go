//go:build gui

package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// Piezas pequeñas de la interfaz que usan varias pestañas.

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
