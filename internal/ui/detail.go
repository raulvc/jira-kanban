package ui

import (
	"github.com/gdamore/tcell/v2"

	"github.com/raulvc/jira-kanban/internal/jira"
)

// detailState tracks the issue detail overlay.
// A nil detailState means no detail modal is open.
type detailState struct {
	card    jira.Card
	loading bool
	err     string
	scroll  int
}

// drawDetailModal renders a centered issue detail overlay over the board.
func drawDetailModal(screen tcell.Screen, d *detailState, screenW, screenH int) {
	const padding = 2
	contentW := min(60, screenW-6)

	// Calculate content height
	contentH := 0
	contentH++ // key + summary line
	contentH++ // blank
	contentH++ // status
	contentH++ // assignee
	if len(d.card.Labels) > 0 {
		contentH++ // labels
	}
	contentH++ // blank separator
	descLines := 1
	if d.card.Description != "" {
		descLines = wrappedLineCount(d.card.Description, contentW-2)
	}
	if d.loading {
		descLines = 1
	}
	if d.err != "" {
		descLines = 1
	}
	contentH += descLines

	// Minimum height
	if contentH < 6 {
		contentH = 6
	}

	boxW := contentW + padding*2
	boxH := contentH + padding + 2 // +2 for top/bottom border area
	if boxW > screenW-4 {
		boxW = screenW - 4
		contentW = boxW - padding*2
	}
	if boxH > screenH-4 {
		boxH = screenH - 4
	}

	ox := (screenW - boxW) / 2
	oy := (screenH - boxH) / 2

	// Styles
	bgStyle := tcell.StyleDefault.Foreground(colFg).Background(colPanel)
	borderStyle := tcell.StyleDefault.Foreground(colMuted).Background(colPanel)
	titleStyle := tcell.StyleDefault.Foreground(colBlue).Background(colPanel).Bold(true)
	keyStyle := tcell.StyleDefault.Foreground(colMuted).Background(colPanel)
	valueStyle := tcell.StyleDefault.Foreground(colFg).Background(colPanel)
	descStyle := tcell.StyleDefault.Foreground(colFg).Background(colPanel)
	loadingStyle := tcell.StyleDefault.Foreground(colCyan).Background(colPanel)
	errStyle := tcell.StyleDefault.Foreground(colRed).Background(colPanel).Bold(true)

	// Fill background
	for row := oy; row < oy+boxH; row++ {
		fillRow(screen, ox, row, boxW, bgStyle)
	}

	// Border
	drawBorder(screen, ox, oy, boxW, boxH, borderStyle)

	// Close hint at bottom
	closeY := oy + boxH - 2
	closeStyle := tcell.StyleDefault.Foreground(colMuted).Background(colPanel)
	closeText := " Esc to close "
	drawText(screen, ox+(boxW-len([]rune(closeText)))/2, closeY, closeText, closeStyle, boxW)

	// Content
	cy := oy + 1
	maxCW := contentW

	// Title: Key   Summary
	titleText := d.card.Key + "  "
	drawText(screen, ox+padding, cy, titleText, titleStyle, maxCW)
	remaining := maxCW - len([]rune(titleText))
	if remaining > 0 {
		drawText(screen, ox+padding+len([]rune(titleText)), cy, truncStr(d.card.Summary, remaining), valueStyle, remaining)
	}
	cy++

	// Blank
	cy++

	// Status
	drawText(screen, ox+padding, cy, "Status: ", keyStyle, maxCW)
	drawText(screen, ox+padding+8, cy, d.card.Status, statusColor(d.card.Status), maxCW-8)
	cy++

	// Assignee
	assigneeText := d.card.Assignee
	if assigneeText == "" {
		assigneeText = "Unassigned"
	}
	drawText(screen, ox+padding, cy, "Assignee: ", keyStyle, maxCW)
	assigneeStyle := valueStyle
	if d.card.Assignee != "" {
		assigneeStyle = tcell.StyleDefault.Foreground(assigneeColor(d.card.Assignee)).Background(colPanel)
	}
	drawText(screen, ox+padding+10, cy, assigneeText, assigneeStyle, maxCW-10)
	cy++

	// Labels
	if len(d.card.Labels) > 0 {
		drawText(screen, ox+padding, cy, "Labels: ", keyStyle, maxCW)
		lx := ox + padding + 8
		for _, label := range d.card.Labels {
			lc := labelColor(label)
			ls := tcell.StyleDefault.Foreground(lc).Background(colPanel)
			text := " " + label + " "
			drawn := drawText(screen, lx, cy, text, ls, maxCW-(lx-ox-padding))
			lx += drawn + 1
			if lx-ox-padding >= maxCW {
				break
			}
		}
		cy++
	}

	// Blank separator
	cy++

	// Description or loading/error
	descAreaTop := cy
	descAreaBot := oy + boxH - 3 // leave room for close hint + padding

	if d.loading {
		drawText(screen, ox+padding, cy, "Loading description…", loadingStyle, maxCW)
	} else if d.err != "" {
		drawText(screen, ox+padding, cy, truncStr(d.err, maxCW), errStyle, maxCW)
	} else if d.card.Description != "" {
		drawWrappedText(screen, d.card.Description, ox+padding, descAreaTop, maxCW, descAreaBot, d.scroll, descStyle)
	} else {
		drawText(screen, ox+padding, cy, "No description", tcell.StyleDefault.Foreground(colMuted).Background(colPanel), maxCW)
	}
}

// drawWrappedText renders a multi-line string with word wrapping and vertical scrolling.
func drawWrappedText(screen tcell.Screen, text string, x, y, width, clipBot, scroll int, style tcell.Style) {
	lines := wrapText(text, width)
	if scroll >= len(lines) {
		return
	}
	cy := y
	for i := scroll; i < len(lines); i++ {
		if cy >= clipBot {
			break
		}
		drawText(screen, x, cy, lines[i], style, width)
		cy++
	}
}

// wrapText splits text into lines at width columns, respecting existing newlines.
func wrapText(text string, width int) []string {
	if width <= 0 {
		width = 1
	}
	var result []string
	// First split by existing newlines
	for _, para := range splitLines(text) {
		if para == "" {
			result = append(result, "")
			continue
		}
		// Then hard-wrap each paragraph
		runes := []rune(para)
		for len(runes) > 0 {
			cut := min(len(runes), width)
			result = append(result, string(runes[:cut]))
			runes = runes[cut:]
		}
	}
	return result
}

// splitLines splits text by \n.
func splitLines(text string) []string {
	var result []string
	start := 0
	for i, r := range text {
		if r == '\n' {
			result = append(result, text[start:i])
			start = i + 1
		}
	}
	result = append(result, text[start:])
	return result
}

// statusColor returns an appropriate color for a status name.
func statusColor(status string) tcell.Style {
	switch status {
	case "Done", "Closed":
		return tcell.StyleDefault.Foreground(colGreen).Background(colPanel).Bold(true)
	case "In Progress", "In Review":
		return tcell.StyleDefault.Foreground(colCyan).Background(colPanel).Bold(true)
	case "To Do", "Open":
		return tcell.StyleDefault.Foreground(colYellow).Background(colPanel).Bold(true)
	default:
		return tcell.StyleDefault.Foreground(colFg).Background(colPanel).Bold(true)
	}
}