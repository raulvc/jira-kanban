package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/raulvc/jira-kanban/internal/jira"
)

const minColWidth = 28

// boardState holds the mutable navigation state of the board.
type boardState struct {
	data         jira.Board
	colIdx       int
	cardIdx      []int
	scrollOffset []int
	statusMsg    string
	modal        *modalState
}

func newBoardState(data jira.Board) *boardState {
	return &boardState{
		data:         data,
		cardIdx:      make([]int, len(data.Columns)),
		scrollOffset: make([]int, len(data.Columns)),
	}
}

func (s *boardState) selectedCard() *jira.Card {
	if s.colIdx >= len(s.data.Columns) {
		return nil
	}
	col := s.data.Columns[s.colIdx]
	if len(col.Issues) == 0 {
		return nil
	}
	ci := s.cardIdx[s.colIdx]
	if ci < 0 || ci >= len(col.Issues) {
		return nil
	}
	return &col.Issues[ci]
}

func (s *boardState) moveColumn(delta int) {
	next := s.colIdx + delta
	if next < 0 || next >= len(s.data.Columns) {
		return
	}
	s.colIdx = next
	s.statusMsg = ""
}

func (s *boardState) moveCard(delta int) {
	if s.colIdx >= len(s.data.Columns) {
		return
	}
	issues := s.data.Columns[s.colIdx].Issues
	if len(issues) == 0 {
		return
	}
	next := s.cardIdx[s.colIdx] + delta
	next = max(0, min(next, len(issues)-1))
	s.cardIdx[s.colIdx] = next
	s.statusMsg = ""
}

func (s *boardState) jumpCard(end bool) {
	if s.colIdx >= len(s.data.Columns) {
		return
	}
	issues := s.data.Columns[s.colIdx].Issues
	if len(issues) == 0 {
		return
	}
	if end {
		s.cardIdx[s.colIdx] = len(issues) - 1
	} else {
		s.cardIdx[s.colIdx] = 0
	}
	s.statusMsg = ""
}

func (s *boardState) reload(data jira.Board) {
	s.data = data
	if s.colIdx >= len(data.Columns) {
		s.colIdx = 0
	}
	s.cardIdx = make([]int, len(data.Columns))
	s.scrollOffset = make([]int, len(data.Columns))
	s.statusMsg = ""
}

// ── viewport helpers ────────────────────────────────────────────────────────

func visibleCols(width, totalCols int) int {
	if totalCols == 0 {
		return 0
	}
	v := width / minColWidth
	return max(1, min(v, totalCols))
}

func viewportStart(width, totalCols, activeCol int) int {
	vc := visibleCols(width, totalCols)
	if vc == 0 {
		return 0
	}
	start := activeCol - vc + 1
	start = max(start, 0)
	end := start + vc
	if end > totalCols {
		start = max(totalCols-vc, 0)
	}
	return start
}

// ── drawing ─────────────────────────────────────────────────────────────────

// drawBoard renders the complete board into the given screen region.
func drawBoard(screen tcell.Screen, s *boardState, boardID, x, y, width, height int) {
	if len(s.data.Columns) == 0 || width < 5 || height < 3 {
		return
	}
	drawStatusBar(screen, s, boardID, x, y, width)
	drawHelpBar(screen, x, y+height-1, width)
	drawColumns(screen, s, x, y+2, width, height-3)

	if s.modal != nil {
		drawModal(screen, s.modal, width, height)
	}
}

func drawStatusBar(screen tcell.Screen, s *boardState, boardID, x, y, width int) {
	style := tcell.StyleDefault.Foreground(colFg).Background(colPanel).Bold(true)
	fillRow(screen, x, y, width, style)

	var text string
	if s.statusMsg != "" {
		text = s.statusMsg
	} else {
		n := 0
		ci := 0
		if s.colIdx < len(s.data.Columns) {
			n = len(s.data.Columns[s.colIdx].Issues)
			if n > 0 {
				ci = s.cardIdx[s.colIdx] + 1
			}
		}
		text = fmt.Sprintf(" %s  board=%d  col %d/%d  card %d/%d",
			s.data.Name, boardID, s.colIdx+1, len(s.data.Columns), ci, n)
	}
	drawText(screen, x, y, text, style, width)
}

func drawHelpBar(screen tcell.Screen, x, y, width int) {
	style := tcell.StyleDefault.Foreground(colMuted).Background(colPanel)
	fillRow(screen, x, y, width, style)
	drawText(screen, x, y,
		" ←/→ cols • ↑/↓ cards • enter view • t transition • o browser • r refresh • q quit",
		style, width)
}

func drawColumns(screen tcell.Screen, s *boardState, x, y, width, height int) {
	if height < 1 {
		return
	}
	vc := visibleCols(width, len(s.data.Columns))
	start := viewportStart(width, len(s.data.Columns), s.colIdx)
	colW := width / vc
	remainder := width - colW*vc
	cx := x

	for vi := range vc {
		ci := start + vi
		if ci >= len(s.data.Columns) {
			break
		}
		thisW := colW
		if vi < remainder {
			thisW++
		}
		active := ci == s.colIdx
		drawColumnHeader(screen, s.data.Columns[ci], active, cx, y-1, thisW)
		drawColumnCards(screen, s, ci, active, cx, y, thisW, height)

		if vi < vc-1 {
			sepStyle := tcell.StyleDefault.Foreground(colPanel).Background(colBg)
			for row := y; row < y+height; row++ {
				screen.SetContent(cx+thisW-1, row, '│', nil, sepStyle)
			}
		}
		cx += thisW
	}
}

func drawColumnHeader(screen tcell.Screen, col jira.Column, active bool, x, y, w int) {
	var style tcell.Style
	if active {
		style = tcell.StyleDefault.Foreground(colBlue).Background(colCardSel).Bold(true)
	} else {
		style = tcell.StyleDefault.Foreground(colMuted).Background(colPanel)
	}
	fillRow(screen, x, y, w, style)
	name := truncStr(strings.ToUpper(col.Name), w-6)
	drawText(screen, x, y, fmt.Sprintf(" %s  %d", name, len(col.Issues)), style, w)
}

func drawColumnCards(screen tcell.Screen, s *boardState, ci int, active bool, x, y, w, h int) {
	col := s.data.Columns[ci]
	if len(col.Issues) == 0 {
		drawText(screen, x+1, y, "No issues",
			tcell.StyleDefault.Foreground(colMuted).Background(colBg), w-2)
		return
	}
	curCard := s.cardIdx[ci]
	scroll := s.scrollOffset[ci]

	if active {
		scroll = ensureVisible(col.Issues, curCard, scroll, w, h)
		s.scrollOffset[ci] = scroll
	}

	drawY := y - scroll
	for j, card := range col.Issues {
		ch := cardHeight(card, w)
		if drawY+ch <= y {
			drawY += ch
			continue
		}
		if drawY >= y+h {
			break
		}
		drawCard(screen, card, active && j == curCard, x, drawY, w, y, y+h)
		drawY += ch
	}
}

// ── card rendering ──────────────────────────────────────────────────────────

func cardHeight(card jira.Card, colWidth int) int {
	h := wrappedLineCount(card.Summary, colWidth-3)
	if len(card.Labels) > 0 {
		h++
	}
	h += 2 // footer + padding
	return h
}

func ensureVisible(issues []jira.Card, idx, scroll, colW, viewH int) int {
	top := 0
	for j := range idx {
		top += cardHeight(issues[j], colW)
	}
	bot := top + cardHeight(issues[idx], colW)
	if top < scroll {
		scroll = top
	}
	if bot > scroll+viewH {
		scroll = bot - viewH
	}
	return max(scroll, 0)
}

func drawCard(screen tcell.Screen, card jira.Card, selected bool, x, drawY, w, clipTop, clipBot int) {
	style := tcell.StyleDefault.Foreground(colFg).Background(colCardBg)
	if selected {
		style = style.Background(colCardSel)
	}

	ch := cardHeight(card, w)
	for row := drawY; row < drawY+ch; row++ {
		if row >= clipTop && row < clipBot {
			fillRow(screen, x, row, w-1, style)
		}
	}

	lineY := drawY
	lineY = drawCardSummary(screen, card, style, x, lineY, w, clipTop, clipBot)
	lineY = drawCardLabels(screen, card, x, lineY, w, clipTop, clipBot)
	drawCardFooter(screen, card, style, x, lineY, w, clipTop, clipBot)
}

func drawCardSummary(screen tcell.Screen, card jira.Card, style tcell.Style, x, lineY, w, clipTop, clipBot int) int {
	runes := []rune(card.Summary)
	textW := max(w-3, 1)
	if len(runes) == 0 {
		return lineY + 1
	}
	for i := 0; i < len(runes); i += textW {
		if lineY >= clipTop && lineY < clipBot {
			end := min(i+textW, len(runes))
			drawText(screen, x+1, lineY, string(runes[i:end]), style, w-2)
		}
		lineY++
	}
	return lineY
}

func drawCardLabels(screen tcell.Screen, card jira.Card, x, lineY, w, clipTop, clipBot int) int {
	if len(card.Labels) == 0 || lineY < clipTop || lineY >= clipBot {
		if len(card.Labels) > 0 {
			return lineY + 1
		}
		return lineY
	}
	lx := x + 1
	for li, lbl := range card.Labels {
		if li > 2 {
			break
		}
		lc := labelColor(lbl)
		ls := tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(lc).Bold(true)
		short := truncStr(strings.ToUpper(lbl), 18)
		txt := " " + short + " "
		drawn := drawText(screen, lx, lineY, txt, ls, w-2-(lx-x))
		lx += drawn + 1
	}
	return lineY + 1
}

func drawCardFooter(screen tcell.Screen, card jira.Card, style tcell.Style, x, lineY, w, clipTop, clipBot int) {
	if lineY < clipTop || lineY >= clipBot {
		return
	}
	drawText(screen, x+1, lineY, "☐ ", style.Foreground(colCyan), 3)
	kw := drawText(screen, x+3, lineY, card.Key, style.Foreground(colMuted), w-4)

	assignee := card.Assignee
	if assignee == "Unassigned" {
		assignee = "—"
	} else {
		assignee = truncStr(assignee, 14)
	}
	if remaining := w - 4 - kw - 2; remaining > 0 {
		drawText(screen, x+3+kw+2, lineY, assignee, style.Foreground(assigneeColor(card.Assignee)), remaining)
	}
}
