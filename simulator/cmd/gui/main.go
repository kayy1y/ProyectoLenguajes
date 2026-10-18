//go:build gui

// Command gui es la aplicación de escritorio de GameStats: una ventana nativa para
// ver partidas y estadísticas, generar datos con el simulador y verificar el sistema.
// Es una herramienta de apoyo: solo usa la API pública de ingestión.
//
//	cd simulator && go run -tags gui ./cmd/gui            (o make gui)
//	gamestats-gui --target http://<ip>:8080 --api-key …    contra un servidor remoto
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"gamestats/simulator/internal/lab"
)

type ui struct {
	lab *lab.Lab
	cat *lab.Catalog
	win fyne.Window
}

func main() {
	target := flag.String("target", env("TARGET_URL", "http://localhost:8080"), "URL del servicio de ingestión")
	apiKey := flag.String("api-key", os.Getenv("INGEST_API_KEY"), "X-API-Key, si el servidor la exige")
	contractsDir := flag.String("contracts", os.Getenv("CONTRACTS_DIR"), "carpeta contracts/ (se busca sola si se omite)")
	flag.Parse()

	dir := *contractsDir
	if dir == "" {
		exe, _ := os.Executable()
		var ok bool
		if dir, ok = lab.FindContracts(".", filepath.Dir(exe)); !ok {
			fmt.Fprintln(os.Stderr, "no se encontró la carpeta contracts/: ejecuta la app dentro del repositorio o usa --contracts")
			os.Exit(2)
		}
	}
	l := lab.New(lab.FromContracts(*target, *apiKey, dir))
	cat, err := lab.BuildCatalog(l.Config())
	if err != nil {
		fmt.Fprintln(os.Stderr, "no se pudo leer contracts/:", err)
		os.Exit(2)
	}

	w, _ := build(app.NewWithID("cr.ac.tec.gamestats"), l, cat, *target)
	w.ShowAndRun()
}

// build arma la ventana con sus pestañas; también la usa el test de capturas.
func build(a fyne.App, l *lab.Lab, cat *lab.Catalog, target string) (fyne.Window, *container.AppTabs) {
	a.Settings().SetTheme(gameStatsTheme{})
	w := a.NewWindow("GameStats")
	u := &ui{lab: l, cat: cat, win: w}

	health := widget.NewLabel("ingestión: …")
	header := container.NewBorder(nil, nil,
		widget.NewLabelWithStyle("GameStats", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		health, widget.NewLabel(target))

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Partidas", theme.ListIcon(), u.matchesTab()),
		container.NewTabItemWithIcon("Generar datos", theme.MediaPlayIcon(), u.generateTab()),
		container.NewTabItemWithIcon("Verificación", theme.ConfirmIcon(), u.verifyTab()),
		container.NewTabItemWithIcon("Enviar evento", theme.MailSendIcon(), u.sendTab()),
		container.NewTabItemWithIcon("Rechazos", theme.WarningIcon(), u.rejectionsTab()),
		container.NewTabItemWithIcon("Guía", theme.HelpIcon(), u.guideTab()),
	)
	w.SetContent(container.NewBorder(container.NewPadded(header), nil, nil, nil, tabs))
	w.Resize(fyne.NewSize(1180, 760))

	go every(3*time.Second, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		ok := l.Health(ctx)
		cancel()
		fyne.Do(func() {
			if ok {
				health.SetText("ingestión ✓")
				health.Importance = widget.SuccessImportance
			} else {
				health.SetText("ingestión no responde")
				health.Importance = widget.DangerImportance
			}
			health.Refresh()
		})
	})
	return w, tabs
}

// every ejecuta f ahora y luego cada d, para siempre (la app termina con la ventana).
func every(d time.Duration, f func()) {
	for {
		f()
		time.Sleep(d)
	}
}

func ctx5() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
