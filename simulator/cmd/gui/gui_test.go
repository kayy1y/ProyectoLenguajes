//go:build gui

package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"gamestats/simulator/internal/lab"
)

func newTestWindow(t *testing.T, target string) (fyne.Window, int) {
	t.Helper()
	dir, ok := lab.FindContracts(".")
	if !ok {
		t.Fatal("no se encontró contracts/")
	}
	l := lab.New(lab.FromContracts(target, "", dir))
	cat, err := lab.BuildCatalog(l.Config())
	if err != nil {
		t.Fatal(err)
	}
	w, tabs := build(test.NewTempApp(t), l, cat, target)
	return w, len(tabs.Items)
}

// La ventana se arma aunque la ingestión no responda (no debe entrar en pánico).
func TestWindowBuildsWithoutIngestion(t *testing.T) {
	w, n := newTestWindow(t, "http://127.0.0.1:1")
	if n != 6 {
		t.Fatalf("pestañas = %d, want 6", n)
	}
	w.Close()
}

// Con SNAPSHOT_DIR guarda una captura de cada pestaña contra la ingestión de TARGET_URL.
func TestSnapshots(t *testing.T) {
	out := os.Getenv("SNAPSHOT_DIR")
	if out == "" {
		t.Skip("define SNAPSHOT_DIR para generar capturas")
	}
	target := env("TARGET_URL", "http://localhost:8080")
	w, _ := newTestWindow(t, target)
	at := findTabs(w.Content())
	for i, item := range at.Items {
		fyne.DoAndWait(func() { at.SelectIndex(i) })
		time.Sleep(1500 * time.Millisecond)
		var img = w.Canvas().Capture()
		f, err := os.Create(filepath.Join(out, item.Text+".png"))
		if err != nil {
			t.Fatal(err)
		}
		png.Encode(f, img)
		f.Close()
	}

	// Detalle de una partida de cada juego y la suite de verificación terminada.
	dir, _ := lab.FindContracts(".")
	l := lab.New(lab.FromContracts(target, "", dir))
	u := &ui{lab: l, win: w}
	shot := func(name string, content fyne.CanvasObject) {
		sw := test.NewWindow(content)
		sw.Resize(fyne.NewSize(700, 900))
		f, err := os.Create(filepath.Join(out, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		png.Encode(f, sw.Canvas().Capture())
		f.Close()
	}
	for _, m := range []lab.MatchSummary{{GameID: "arena-combat", MatchID: "match-a1"}, {GameID: "street-racing", MatchID: "race-r1"}} {
		r, st, err := l.Results(t.Context(), m.GameID, m.MatchID)
		box := container.NewVBox()
		fyne.DoAndWait(func() { u.fillDetail(box, m, r, st, err) })
		shot("detalle-"+m.MatchID, container.NewPadded(box))
	}
	j := l.StartVerify()
	for j.View().State == "running" {
		time.Sleep(300 * time.Millisecond)
	}
	summary, box := widget.NewLabel(""), container.NewVBox()
	fyne.DoAndWait(func() { drawChecks(j.View(), summary, box) })
	shot("verificacion-terminada", container.NewVBox(summary, box))
	if v := j.View(); v.Error != "" {
		t.Fatalf("verificación con error: %s", v.Error)
	} else {
		for _, c := range v.Checks {
			if !c.Pass {
				t.Errorf("FAIL %s: %s (esperado %s, obtenido %s)", c.Group, c.Name, c.Expected, c.Actual)
			}
		}
	}
}

func findTabs(o fyne.CanvasObject) *containerTabs {
	switch x := o.(type) {
	case *containerTabs:
		return x
	case *fyne.Container:
		for _, c := range x.Objects {
			if t := findTabs(c); t != nil {
				return t
			}
		}
	}
	return nil
}

type containerTabs = container.AppTabs
