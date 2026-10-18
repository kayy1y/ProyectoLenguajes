//go:build gui

package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// gameStatsTheme es el tema de la app: parte del tema por defecto de Fyne y solo
// cambia la paleta (morado como color principal) y algunos tamaños.
type gameStatsTheme struct{}

var _ fyne.Theme = gameStatsTheme{}

var darkPalette = map[fyne.ThemeColorName]color.Color{
	theme.ColorNameBackground:          color.NRGBA{18, 20, 28, 255},
	theme.ColorNameHeaderBackground:    color.NRGBA{26, 29, 41, 255},
	theme.ColorNameMenuBackground:      color.NRGBA{26, 29, 41, 255},
	theme.ColorNameOverlayBackground:   color.NRGBA{26, 29, 41, 255},
	theme.ColorNameInputBackground:     color.NRGBA{30, 34, 48, 255},
	theme.ColorNameButton:              color.NRGBA{38, 43, 61, 255},
	theme.ColorNameSeparator:           color.NRGBA{42, 47, 66, 255},
	theme.ColorNamePrimary:             color.NRGBA{124, 108, 255, 255},
	theme.ColorNameHyperlink:           color.NRGBA{150, 138, 255, 255},
	theme.ColorNameFocus:               color.NRGBA{124, 108, 255, 127},
	theme.ColorNameSelection:           color.NRGBA{124, 108, 255, 64},
	theme.ColorNameHover:               color.NRGBA{124, 108, 255, 38},
	theme.ColorNameForegroundOnPrimary: color.White,
	theme.ColorNameSuccess:             color.NRGBA{61, 214, 140, 255},
	theme.ColorNameWarning:             color.NRGBA{245, 184, 61, 255},
	theme.ColorNameError:               color.NRGBA{255, 92, 122, 255},
}

var lightPalette = map[fyne.ThemeColorName]color.Color{
	theme.ColorNameBackground:          color.NRGBA{245, 246, 250, 255},
	theme.ColorNameHeaderBackground:    color.NRGBA{236, 238, 245, 255},
	theme.ColorNameMenuBackground:      color.White,
	theme.ColorNameOverlayBackground:   color.White,
	theme.ColorNameInputBackground:     color.White,
	theme.ColorNameButton:              color.NRGBA{230, 232, 242, 255},
	theme.ColorNameSeparator:           color.NRGBA{218, 221, 232, 255},
	theme.ColorNamePrimary:             color.NRGBA{91, 75, 245, 255},
	theme.ColorNameHyperlink:           color.NRGBA{91, 75, 245, 255},
	theme.ColorNameFocus:               color.NRGBA{91, 75, 245, 127},
	theme.ColorNameSelection:           color.NRGBA{91, 75, 245, 50},
	theme.ColorNameHover:               color.NRGBA{91, 75, 245, 25},
	theme.ColorNameForegroundOnPrimary: color.White,
	theme.ColorNameSuccess:             color.NRGBA{31, 157, 97, 255},
	theme.ColorNameWarning:             color.NRGBA{201, 132, 0, 255},
	theme.ColorNameError:               color.NRGBA{217, 51, 79, 255},
}

func (gameStatsTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	p := darkPalette
	if v == theme.VariantLight {
		p = lightPalette
	}
	if c, ok := p[n]; ok {
		return c
	}
	return theme.DefaultTheme().Color(n, v)
}

func (gameStatsTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNamePadding:
		return 5
	case theme.SizeNameInputRadius, theme.SizeNameButtonRadius:
		return 8
	case theme.SizeNameSelectionRadius:
		return 6
	case theme.SizeNameCardRadius:
		return 10
	case theme.SizeNameScrollBarRadius:
		return 4
	}
	return theme.DefaultTheme().Size(n)
}

func (gameStatsTheme) Font(s fyne.TextStyle) fyne.Resource { return theme.DefaultTheme().Font(s) }

func (gameStatsTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }
