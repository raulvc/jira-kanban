package ui

import (
	"github.com/gdamore/tcell/v2"

	"github.com/raulvc/jira-kanban/internal/jira"
)

// detailState tracks the issue detail overlay.
// A nil detailState means no detail modal is open.
type detailState struct {
	card      jira.Card
	loading   bool
	err       string
	scroll    int
	maxScroll int
	viewH     int
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
	if d.card.Epic != "" {
		contentH++ // epic
	}
	if len(d.card.Labels) > 0 {
		contentH++ // labels
	}
	contentH++ // blank separator
	descLines := 1
	if d.card.Description != "" {
		segs := d.card.RichDesc
		if segs == nil {
			segs = []jira.DescSeg{{Text: d.card.Description, Style: jira.DsText}}
		}
		descLines = richDescLineCount(segs, contentW-2)
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
	closeText := " Esc/q to close • a assign "
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

	// Epic
	if d.card.Epic != "" {
		drawText(screen, ox+padding, cy, "Epic: ", keyStyle, maxCW)
		epBadge := " " + truncStr(d.card.Epic, maxCW-8) + " "
		epStyle := tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(epicColor(d.card.Epic)).Bold(true)
		drawText(screen, ox+padding+6, cy, epBadge, epStyle, maxCW-6)
		cy++
	}

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
	descAreaH := descAreaBot - descAreaTop
	d.viewH = descAreaH

	if d.loading {
		drawText(screen, ox+padding, cy, "Loading description…", loadingStyle, maxCW)
		d.maxScroll = 0
	} else if d.err != "" {
		drawText(screen, ox+padding, cy, truncStr(d.err, maxCW), errStyle, maxCW)
		d.maxScroll = 0
	} else if d.card.Description != "" {
		descW := contentW - 2
		segs := d.card.RichDesc
		if segs == nil {
			segs = []jira.DescSeg{{Text: d.card.Description, Style: jira.DsText}}
		}
		totalDescLines := richDescLineCount(segs, descW)
		needsScrollbar := totalDescLines > descAreaH
		if needsScrollbar {
			descW--
		}
		totalDescLines = richDescLineCount(segs, descW)
		d.maxScroll = max(0, totalDescLines-descAreaH)
		if d.scroll > d.maxScroll {
			d.scroll = d.maxScroll
		}
		drawRichWrappedText(screen, segs, ox+padding, descAreaTop, descW, descAreaBot, d.scroll, descStyle)
		if needsScrollbar {
			trackStyle := tcell.StyleDefault.Foreground(colMuted).Background(colPanel)
			thumbStyle := tcell.StyleDefault.Foreground(colBlue).Background(colPanel)
			drawScrollbar(screen, ox+padding+descW+1, descAreaTop, descAreaH, d.scroll, totalDescLines, trackStyle, thumbStyle)
		}
	} else {
		drawText(screen, ox+padding, cy, "No description", tcell.StyleDefault.Foreground(colMuted).Background(colPanel), maxCW)
		d.maxScroll = 0
	}
}

// drawScrollbar draws a vertical scrollbar on the right side of a region.
func drawScrollbar(screen tcell.Screen, x, y, viewH, scroll, totalLines int, trackStyle, thumbStyle tcell.Style) {
	if totalLines <= viewH || viewH <= 0 {
		return
	}
	thumbH := max(1, viewH*viewH/totalLines)
	maxScroll := totalLines - viewH
	thumbTop := y
	if maxScroll > 0 {
		thumbTop = y + scroll*(viewH-thumbH)/maxScroll
	}
	for row := y; row < y+viewH; row++ {
		ch := '│'
		s := trackStyle
		if row >= thumbTop && row < thumbTop+thumbH {
			ch = '█'
			s = thumbStyle
		}
		screen.SetContent(x, row, ch, nil, s)
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

// styledLine is one display line with styled chunks.
type styledLine struct {
	chunks []styledChunk
}

type styledChunk struct {
	text  string
	style jira.DescStyle
}

// richDescLineCount counts display lines for styled segments at a given width.
func richDescLineCount(segs []jira.DescSeg, width int) int {
	return len(richDescWrapLines(segs, width))
}

// richDescWrapLines wraps styled segments into display lines at a given width.
func richDescWrapLines(segs []jira.DescSeg, width int) []styledLine {
	if width <= 0 {
		width = 1
	}
	// First, split segments at \n boundaries, building a flat list of "logical lines"
	// where each logical line is a slice of styled chunks.
	var logicalLines [][]styledChunk
	var curLine []styledChunk

	for _, seg := range segs {
		parts := splitAtNewlines(seg.Text)
		for pi, part := range parts {
			if pi > 0 {
				logicalLines = append(logicalLines, curLine)
				curLine = nil
			}
			if part != "" {
				curLine = append(curLine, styledChunk{text: part, style: seg.Style})
			}
		}
	}
	if len(curLine) > 0 || len(logicalLines) == 0 {
		logicalLines = append(logicalLines, curLine)
	}

	// Then hard-wrap each logical line at width.
	var result []styledLine
	for _, ll := range logicalLines {
		if len(ll) == 0 {
			result = append(result, styledLine{})
			continue
		}
		// Concatenate all text to measure total width, then split.
		wrapped := wrapStyledLine(ll, width)
		result = append(result, wrapped...)
	}
	return result
}

// splitAtNewlines splits s at \n, keeping empty strings between them.
func splitAtNewlines(s string) []string {
	var result []string
	start := 0
	for i, r := range s {
		if r == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

// wrapStyledLine hard-wraps a single logical line (sequence of styled chunks)
// at the given width, producing multiple styledLines.
func wrapStyledLine(chunks []styledChunk, width int) []styledLine {
	// Flatten into rune-position → (rune, style) for precise wrapping.
	type rEntry struct {
		r     rune
		style jira.DescStyle
	}
	var runes []rEntry
	for _, c := range chunks {
		for _, r := range c.text {
			runes = append(runes, rEntry{r, c.style})
		}
	}
	if len(runes) == 0 {
		return []styledLine{{}}
	}

	var lines []styledLine
	pos := 0
	for pos < len(runes) {
		end := min(pos+width, len(runes))
		var sl styledLine
		var cur styledChunk
		for i := pos; i < end; i++ {
			if len(cur.text) > 0 && cur.style != runes[i].style {
				sl.chunks = append(sl.chunks, cur)
				cur = styledChunk{style: runes[i].style}
			}
			cur.text += string(runes[i].r)
			cur.style = runes[i].style
		}
		if len(cur.text) > 0 {
			sl.chunks = append(sl.chunks, cur)
		}
		lines = append(lines, sl)
		pos = end
	}
	return lines
}

// descSegStyle maps a DescStyle to a tcell.Style.
func descSegStyle(ds jira.DescStyle, base tcell.Style) tcell.Style {
	switch ds {
	case jira.DsLink:
		return tcell.StyleDefault.Foreground(colCyan).Background(colPanel).Underline(true)
	case jira.DsCode:
		return tcell.StyleDefault.Foreground(colMuted).Background(colBg).Dim(true)
	case jira.DsHeading:
		return base.Bold(true)
	default:
		return base
	}
}

// drawRichWrappedText renders styled description segments with wrapping and scrolling.
func drawRichWrappedText(screen tcell.Screen, segs []jira.DescSeg, x, y, width, clipBot, scroll int, baseStyle tcell.Style) {
	lines := richDescWrapLines(segs, width)
	if scroll >= len(lines) {
		return
	}
	cy := y
	for i := scroll; i < len(lines); i++ {
		if cy >= clipBot {
			break
		}
		col := 0
		for _, chunk := range lines[i].chunks {
			st := descSegStyle(chunk.style, baseStyle)
			for _, r := range chunk.text {
				if col >= width {
					break
				}
				screen.SetContent(x+col, cy, r, nil, st)
				col++
			}
		}
		cy++
	}
}