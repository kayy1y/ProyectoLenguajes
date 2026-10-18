//go:build gui

package main

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (u *ui) guideTab() fyne.CanvasObject {
	steps := []string{
		"1. Generar datos → «Escenario demo» → Ejecutar. Se envían partidas de dos juegos en paralelo, con eventos inválidos, un duplicado, un conflicto, un evento tardío y una partida sin cierre.",
		"2. Partidas → cada una pasa por COLLECTING → ANALYZING → ANALYZED. La que no tiene MATCH_FINISHED se cierra por inactividad y queda INCOMPLETE.",
		"3. Selecciona una partida: estadísticas por jugador, por partida, combinadas y patrones secuenciales. Lo no calculable aparece como «—», nunca como 0 inventado.",
		"4. Verificación → Ejecutar suite completa. Compara con los resultados calculados a mano, los 17 eventos inválidos y el ciclo de vida. Guarda el informe como evidencia.",
		"5. Enviar evento → elige un ejemplo inválido y mira el campo y el código del error.",
	}
	box := container.NewVBox(bold("Recorrido sugerido (5 minutos)"))
	for _, s := range steps {
		box.Add(wrapped(s))
	}
	addAll(box, widget.NewSeparator(), bold("Juegos soportados (leídos de contracts/schemas/games)"))
	for _, g := range u.cat.Games {
		box.Add(wrapped(fmt.Sprintf("• %s  v%s:  %s", g.GameID, strings.Join(g.Versions, ", "), strings.Join(g.EventTypes, ", "))))
	}
	addAll(box, widget.NewSeparator(), bold("Cómo está dividido el trabajo"),
		wrapped("• Go (imperativo): recibe, valida con JSON Schema, agrupa por partida (una goroutine por partida), reintenta y expone los resultados. No calcula estadísticas."),
		wrapped("• Scala (funcional): recibe el lote ordenado de una partida y calcula métricas y patrones con funciones puras."),
		wrapped("• Esta aplicación: herramienta de apoyo. Solo usa la API pública de la ingestión; no participa en el análisis."),
		widget.NewSeparator(), bold("Agregar un juego nuevo"),
		wrapped("Un esquema en contracts/schemas/games, un GameAnalyzer en Scala con una línea en el registro, y un fixture con su resultado esperado. Ni la ingestión en Go, ni el núcleo de Scala, ni esta aplicación cambian."))
	return container.NewVScroll(container.NewPadded(box))
}
