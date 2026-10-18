//go:build gui

package main

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
)

// mainMenu pone el menú de la ventana y los atajos Ctrl+1…6 para cambiar de pestaña.
func (u *ui) mainMenu(tabs *container.AppTabs) {
	var goTo []*fyne.MenuItem
	for i, t := range tabs.Items {
		sc := &desktop.CustomShortcut{KeyName: fyne.KeyName(strconv.Itoa(i + 1)), Modifier: fyne.KeyModifierShortcutDefault}
		item := fyne.NewMenuItem(t.Text, func() { tabs.SelectIndex(i) })
		item.Shortcut = sc
		goTo = append(goTo, item)
		u.win.Canvas().AddShortcut(sc, func(fyne.Shortcut) { tabs.SelectIndex(i) })
	}
	about := fyne.NewMenuItem("Acerca de GameStats", func() {
		dialog.ShowInformation("GameStats",
			"Plataforma para procesar y analizar eventos de videojuegos.\n\n"+
				"Ingestión y concurrencia en Go · análisis funcional en Scala.\n"+
				"Proyecto del curso de Lenguajes de Programación.", u.win)
	})
	u.win.SetMainMenu(fyne.NewMainMenu(
		fyne.NewMenu("Archivo"), // Fyne agrega aquí la opción para salir
		fyne.NewMenu("Ir a", goTo...),
		fyne.NewMenu("Ayuda", about),
	))
}
