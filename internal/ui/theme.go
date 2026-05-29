// Package ui implements the terminal user interface for the Jira kanban board.
package ui

import "github.com/gdamore/tcell/v2"

// Kanagawa-inspired color palette.
var (
	colBg      = tcell.NewRGBColor(31, 31, 40)
	colFg      = tcell.NewRGBColor(220, 215, 186)
	colMuted   = tcell.NewRGBColor(114, 113, 105)
	colBlue    = tcell.NewRGBColor(126, 156, 216)
	colCyan    = tcell.NewRGBColor(127, 180, 202)
	colGreen   = tcell.NewRGBColor(152, 187, 108)
	colOrange  = tcell.NewRGBColor(255, 160, 102)
	colRed     = tcell.NewRGBColor(228, 104, 118)
	colPanel   = tcell.NewRGBColor(42, 42, 55)
	colCardBg  = tcell.NewRGBColor(38, 38, 50)
	colCardSel = tcell.NewRGBColor(34, 50, 73)
	colYellow  = tcell.NewRGBColor(230, 195, 132)
	colViolet  = tcell.NewRGBColor(149, 127, 184)
)

var labelPalette = []tcell.Color{colRed, colOrange, colCyan, colGreen, colYellow, colViolet, colBlue}
var assigneePalette = []tcell.Color{colCyan, colGreen, colOrange, colYellow, colViolet, colBlue}

// labelColor returns a deterministic color for a label string.
func labelColor(label string) tcell.Color {
	return labelPalette[strhash(label)%len(labelPalette)]
}

// assigneeColor returns a deterministic color for an assignee name.
// Unassigned issues get the muted color.
func assigneeColor(name string) tcell.Color {
	if name == "" || name == "Unassigned" {
		return colMuted
	}
	return assigneePalette[strhash(name)%len(assigneePalette)]
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
