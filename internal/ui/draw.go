package ui

import "github.com/gdamore/tcell/v2"

// drawText writes text at (x,y) using style, clipping to maxWidth. It returns
// the number of columns consumed.
func drawText(screen tcell.Screen, x, y int, text string, style tcell.Style, maxWidth int) int {
	col := 0
	for _, r := range text {
		if col >= maxWidth {
			break
		}
		screen.SetContent(x+col, y, r, nil, style)
		col++
	}
	return col
}

// fillRow fills a single row of width w at (x,y) with spaces in the given style.
func fillRow(screen tcell.Screen, x, y, w int, style tcell.Style) {
	for col := x; col < x+w; col++ {
		screen.SetContent(col, y, ' ', nil, style)
	}
}

// truncStr shortens s to at most maxLen runes, appending "…" when truncated.
func truncStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-1]) + "…"
}

// wrappedLineCount returns how many terminal rows text occupies when word-wrapped
// at width columns, respecting existing newlines.
func wrappedLineCount(text string, width int) int {
	if width <= 0 {
		width = 1
	}
	return len(wrapText(text, width))
}
