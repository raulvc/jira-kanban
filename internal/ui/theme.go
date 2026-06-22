package ui

import (
	"github.com/gdamore/tcell/v2"

	"github.com/raulvc/jira-kanban/internal/config"
)

// Theme holds all named colors used by the TUI.
type Theme struct {
	Name string

	Bg      tcell.Color
	Fg      tcell.Color
	Muted   tcell.Color
	Blue    tcell.Color
	Cyan    tcell.Color
	Green   tcell.Color
	Orange  tcell.Color
	Red     tcell.Color
	Panel   tcell.Color
	CardBg  tcell.Color
	CardSel tcell.Color
	Yellow  tcell.Color
	Violet  tcell.Color
	Pink    tcell.Color
	Teal    tcell.Color
	Sky     tcell.Color
	Lime    tcell.Color
	Gold    tcell.Color
	Coral   tcell.Color
	Mauve   tcell.Color
	Aqua    tcell.Color

	BadgeFg tcell.Color
}

var themes = []Theme{
	{
		Name: "Kanagawa Dark",
		Bg: tcell.NewRGBColor(31, 31, 40), Fg: tcell.NewRGBColor(220, 215, 186),
		Muted: tcell.NewRGBColor(114, 113, 105), Blue: tcell.NewRGBColor(126, 156, 216),
		Cyan: tcell.NewRGBColor(127, 180, 202), Green: tcell.NewRGBColor(152, 187, 108),
		Orange: tcell.NewRGBColor(255, 160, 102), Red: tcell.NewRGBColor(228, 104, 118),
		Panel: tcell.NewRGBColor(42, 42, 55), CardBg: tcell.NewRGBColor(38, 38, 50),
		CardSel: tcell.NewRGBColor(34, 50, 73), Yellow: tcell.NewRGBColor(230, 195, 132),
		Violet: tcell.NewRGBColor(149, 127, 184), Pink: tcell.NewRGBColor(200, 130, 170),
		Teal: tcell.NewRGBColor(90, 195, 170), Sky: tcell.NewRGBColor(100, 175, 230),
		Lime: tcell.NewRGBColor(170, 210, 90), Gold: tcell.NewRGBColor(220, 175, 80),
		Coral: tcell.NewRGBColor(240, 130, 100), Mauve: tcell.NewRGBColor(170, 120, 190),
		Aqua: tcell.NewRGBColor(80, 190, 210), BadgeFg: tcell.ColorBlack,
	},
	{
		Name: "Kanagawa Light",
		Bg: tcell.NewRGBColor(220, 215, 186), Fg: tcell.NewRGBColor(84, 84, 109),
		Muted: tcell.NewRGBColor(114, 113, 105), Blue: tcell.NewRGBColor(45, 79, 143),
		Cyan: tcell.NewRGBColor(70, 115, 120), Green: tcell.NewRGBColor(80, 120, 40),
		Orange: tcell.NewRGBColor(204, 109, 0), Red: tcell.NewRGBColor(195, 64, 67),
		Panel: tcell.NewRGBColor(200, 192, 147), CardBg: tcell.NewRGBColor(229, 224, 196),
		CardSel: tcell.NewRGBColor(184, 178, 150), Yellow: tcell.NewRGBColor(160, 130, 40),
		Violet: tcell.NewRGBColor(122, 122, 160), Pink: tcell.NewRGBColor(160, 100, 100),
		Teal: tcell.NewRGBColor(70, 130, 130), Sky: tcell.NewRGBColor(60, 110, 155),
		Lime: tcell.NewRGBColor(80, 130, 50), Gold: tcell.NewRGBColor(140, 115, 50),
		Coral: tcell.NewRGBColor(170, 65, 50), Mauve: tcell.NewRGBColor(120, 80, 140),
		Aqua: tcell.NewRGBColor(60, 130, 140), BadgeFg: tcell.NewRGBColor(255, 255, 255),
	},
	{
		Name: "Darcula",
		Bg: tcell.NewRGBColor(43, 43, 43), Fg: tcell.NewRGBColor(169, 183, 198),
		Muted: tcell.NewRGBColor(128, 128, 128), Blue: tcell.NewRGBColor(104, 151, 187),
		Cyan: tcell.NewRGBColor(74, 158, 142), Green: tcell.NewRGBColor(106, 135, 89),
		Orange: tcell.NewRGBColor(204, 120, 50), Red: tcell.NewRGBColor(199, 84, 80),
		Panel: tcell.NewRGBColor(49, 51, 53), CardBg: tcell.NewRGBColor(60, 63, 65),
		CardSel: tcell.NewRGBColor(33, 66, 131), Yellow: tcell.NewRGBColor(187, 181, 41),
		Violet: tcell.NewRGBColor(152, 118, 170), Pink: tcell.NewRGBColor(176, 86, 143),
		Teal: tcell.NewRGBColor(74, 158, 142), Sky: tcell.NewRGBColor(126, 170, 208),
		Lime: tcell.NewRGBColor(141, 182, 112), Gold: tcell.NewRGBColor(255, 198, 109),
		Coral: tcell.NewRGBColor(232, 87, 76), Mauve: tcell.NewRGBColor(168, 140, 192),
		Aqua: tcell.NewRGBColor(91, 168, 176), BadgeFg: tcell.ColorBlack,
	},
	{
		Name: "Darcula Light",
		Bg: tcell.NewRGBColor(251, 251, 251), Fg: tcell.NewRGBColor(51, 51, 51),
		Muted: tcell.NewRGBColor(128, 128, 128), Blue: tcell.NewRGBColor(0, 51, 176),
		Cyan: tcell.NewRGBColor(0, 110, 123), Green: tcell.NewRGBColor(6, 125, 23),
		Orange: tcell.NewRGBColor(204, 120, 50), Red: tcell.NewRGBColor(199, 84, 80),
		Panel: tcell.NewRGBColor(231, 231, 231), CardBg: tcell.NewRGBColor(250, 250, 250),
		CardSel: tcell.NewRGBColor(200, 215, 240), Yellow: tcell.NewRGBColor(148, 138, 28),
		Violet: tcell.NewRGBColor(112, 80, 148), Pink: tcell.NewRGBColor(176, 86, 143),
		Teal: tcell.NewRGBColor(0, 120, 130), Sky: tcell.NewRGBColor(40, 100, 180),
		Lime: tcell.NewRGBColor(141, 182, 112), Gold: tcell.NewRGBColor(255, 198, 109),
		Coral: tcell.NewRGBColor(232, 87, 76), Mauve: tcell.NewRGBColor(168, 140, 192),
		Aqua: tcell.NewRGBColor(91, 168, 176), BadgeFg: tcell.NewRGBColor(255, 255, 255),
	},
}

// currentTheme is the index into themes for the active theme.
var currentTheme int

// T returns the currently active theme.
func T() *Theme { return &themes[currentTheme] }

// SetThemeByName sets the active theme by name.
// If name is empty or not found, it falls back to the first theme.
func SetThemeByName(name string) {
	for i, th := range themes {
		if th.Name == name {
			currentTheme = i
			return
		}
	}
	currentTheme = 0
}

// ThemeName returns the name of the currently active theme.
func ThemeName() string { return T().Name }

// cycleTheme advances to the next theme and returns the new theme name.
func cycleTheme() string {
	currentTheme = (currentTheme + 1) % len(themes)
	return T().Name
}

func labelColor(label string) tcell.Color {
	t := T()
	palette := []tcell.Color{t.Red, t.Orange, t.Cyan, t.Green, t.Yellow, t.Violet, t.Blue}
	return palette[strhash(label)%len(palette)]
}

func assigneeColor(name string) tcell.Color {
	t := T()
	if name == "" || name == "Unassigned" {
		return t.Muted
	}
	palette := []tcell.Color{t.Cyan, t.Green, t.Orange, t.Yellow, t.Violet, t.Blue}
	return palette[strhash(name)%len(palette)]
}

func epicColor(name string) tcell.Color {
	t := T()
	if name == "" {
		return t.Muted
	}
	palette := []tcell.Color{t.Violet, t.Blue, t.Cyan, t.Green, t.Yellow, t.Orange, t.Red, t.Pink, t.Teal, t.Sky, t.Lime, t.Gold, t.Coral, t.Mauve, t.Aqua}
	return palette[strhash(name)%len(palette)]
}

func loadThemePrefs() error {
	path, err := config.Path()
	if err != nil {
		return err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	SetThemeByName(cfg.Theme)
	return nil
}

func saveThemePrefs() {
	path, err := config.Path()
	if err != nil {
		return
	}
	cfg, err := config.Load(path)
	if err != nil {
		return
	}
	cfg.Theme = ThemeName()
	_ = config.Save(path, cfg)
}

func strhash(s string) int {
	h := 0
	for _, r := range s {
		h = h*31 + int(r)
	}
	if h < 0 {
		h = -h
	}
	return h
}