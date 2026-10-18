//go:build gui

package main

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed icon.svg
var iconSVG []byte

// appIcon es el icono de la ventana y de la barra de tareas.
var appIcon = fyne.NewStaticResource("gamestats.svg", iconSVG)
