package ui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/raulvc/jira-kanban/internal/jira"
)

const minColWidth = 28

var spinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

// boardState holds the mutable navigation state of the board.
type boardState struct {
	data         jira.Board
	colIdx       int
	cardIdx      []int
	scrollOffset []int
	statusMsg    string
	syncing      bool
	syncPhase    string
	syncFetched  int
	syncTotal    int
	spinnerFrame int
	modal        *modalState
	filter       *filterState
	detail       *detailState
	memberFilter string
}

func newBoardState(data jira.Board) *boardState {
	return &boardState{
		data:         data,
		cardIdx:      make([]int, len(data.Columns)),
		scrollOffset: make([]int, len(data.Columns)),
	}
}

func (s *boardState) selectedCard() *jira.Card {
	fd := s.filteredData()
	if s.colIdx >= len(fd.Columns) {
		return nil
	}
	col := fd.Columns[s.colIdx]
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
	fd := s.filteredData()
	if s.cardIdx[next] >= len(fd.Columns[next].Issues) && len(fd.Columns[next].Issues) > 0 {
		s.cardIdx[next] = len(fd.Columns[next].Issues) - 1
	}
	s.statusMsg = ""
}

func (s *boardState) moveCard(delta int) {
	fd := s.filteredData()
	if s.colIdx >= len(fd.Columns) {
		return
	}
	issues := fd.Columns[s.colIdx].Issues
	if len(issues) == 0 {
		return
	}
	next := s.cardIdx[s.colIdx] + delta
	next = max(0, min(next, len(issues)-1))
	s.cardIdx[s.colIdx] = next
	s.statusMsg = ""
}

func (s *boardState) jumpCard(end bool) {
	fd := s.filteredData()
	if s.colIdx >= len(fd.Columns) {
		return
	}
	issues := fd.Columns[s.colIdx].Issues
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

// moveIssueToStatus moves a card from its current column to the column
// matching toStatus. It performs a local, optimistic update so the UI
// reflects the change immediately without waiting for a network round-trip.
func (s *boardState) moveIssueToStatus(issueKey, toStatus string) {
	// Find and remove from the source column.
	var card jira.Card
	srcCol := -1
	srcIdx := -1
	for ci, col := range s.data.Columns {
		for ii, c := range col.Issues {
			if c.Key == issueKey {
				card = c
				srcCol = ci
				srcIdx = ii
				break
			}
		}
		if srcCol >= 0 {
			break
		}
	}
	if srcCol < 0 {
		return
	}

	// Find the destination column by status name (case-insensitive).
	dstCol := -1
	lower := strings.ToLower(toStatus)
	for ci, col := range s.data.Columns {
		if strings.ToLower(col.Name) == lower {
			dstCol = ci
			break
		}
	}
	if dstCol < 0 || dstCol == srcCol {
		return
	}

	// Remove from source.
	s.data.Columns[srcCol].Issues = append(
		s.data.Columns[srcCol].Issues[:srcIdx],
		s.data.Columns[srcCol].Issues[srcIdx+1:]...,
	)
	if s.cardIdx[srcCol] >= len(s.data.Columns[srcCol].Issues) && s.cardIdx[srcCol] > 0 {
		s.cardIdx[srcCol]--
	}

	// Update the card's status and append to the destination column.
	card.Status = toStatus
	s.data.Columns[dstCol].Issues = append(s.data.Columns[dstCol].Issues, card)
}

func (s *boardState) reload(data jira.Board) {
	s.data = data
	if s.colIdx >= len(data.Columns) {
		s.colIdx = 0
	}
	s.cardIdx = make([]int, len(data.Columns))
	s.scrollOffset = make([]int, len(data.Columns))
	s.statusMsg = ""
	s.syncing = false
	s.syncPhase = ""
	s.syncFetched = 0
	s.syncTotal = 0
}

// filteredData returns a copy of the board data with only cards matching
// the current memberFilter. If no filter is set, it returns the original data.
func (s *boardState) filteredData() jira.Board {
	if s.memberFilter == "" {
		return s.data
	}
	result := jira.Board{Name: s.data.Name}
	for _, col := range s.data.Columns {
		fc := jira.Column{Name: col.Name}
		for _, card := range col.Issues {
			if card.Assignee == s.memberFilter {
				fc.Issues = append(fc.Issues, card)
			}
		}
		result.Columns = append(result.Columns, fc)
	}
	return result
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
	fd := s.filteredData()
	if len(fd.Columns) == 0 || width < 5 || height < 3 {
		return
	}
	drawStatusBar(screen, s, boardID, x, y, width)
	drawHelpBar(screen, x, y+height-1, width)
	drawColumns(screen, s, &fd, x, y+2, width, height-3)

	if s.modal != nil {
		drawModal(screen, s.modal, width, height)
	}
	if s.filter != nil {
		drawFilterModal(screen, s.filter, width, height)
	}
	if s.detail != nil {
		drawDetailModal(screen, s.detail, width, height)
	}
}

func drawStatusBar(screen tcell.Screen, s *boardState, boardID, x, y, width int) {
	style := tcell.StyleDefault.Foreground(colFg).Background(colPanel).Bold(true)
	fillRow(screen, x, y, width, style)

	if s.syncing {
		syncStyle := tcell.StyleDefault.Foreground(colCyan).Background(colPanel).Bold(true)
		barStyle := tcell.StyleDefault.Foreground(colCyan).Background(colPanel)
		trackStyle := tcell.StyleDefault.Foreground(colMuted).Background(colPanel)

		frame := spinnerFrames[s.spinnerFrame%len(spinnerFrames)]

		cx := x + 1
		if s.syncTotal > 0 {
			const barW = 20
			pct := s.syncFetched * 100 / s.syncTotal
			filled := min(s.syncFetched*barW/s.syncTotal, barW)

			info := fmt.Sprintf("%s %d/%d ", s.syncPhase, s.syncFetched, s.syncTotal)
			cx += drawText(screen, cx, y, info, trackStyle, width)

			cx += drawText(screen, cx, y, "[", trackStyle, width)
			for i := range barW {
				ch := '░'
				st := trackStyle
				if i < filled {
					ch = '█'
					st = barStyle
				}
				screen.SetContent(cx+i, y, ch, nil, st)
			}
			cx += barW
			cx += drawText(screen, cx, y, "]", trackStyle, width)

			pctText := fmt.Sprintf(" %d%% ", pct)
			cx += drawText(screen, cx, y, pctText, syncStyle, width)
		}

		drawText(screen, cx, y, fmt.Sprintf(" %c Syncing…", frame), syncStyle, width)
		return
	}

	var text string
	if s.statusMsg != "" {
		text = s.statusMsg
	} else {
		fd := s.filteredData()
		n := 0
		ci := 0
		if s.colIdx < len(fd.Columns) {
			n = len(fd.Columns[s.colIdx].Issues)
			if n > 0 {
				ci = s.cardIdx[s.colIdx] + 1
			}
		}
		text = fmt.Sprintf(" %s  board=%d  col %d/%d  card %d/%d",
			fd.Name, boardID, s.colIdx+1, len(fd.Columns), ci, n)
		if s.memberFilter != "" {
			text += fmt.Sprintf("  filter: %s", s.memberFilter)
		}
	}
	drawText(screen, x, y, text, style, width)
}

func drawHelpBar(screen tcell.Screen, x, y, width int) {
	style := tcell.StyleDefault.Foreground(colMuted).Background(colPanel)
	fillRow(screen, x, y, width, style)
	drawText(screen, x, y,
		" ←/→ cols • ↑/↓ cards • f filter • t transition • o browser • r refresh • q quit",
		style, width)
}

func drawColumns(screen tcell.Screen, s *boardState, fd *jira.Board, x, y, width, height int) {
	if height < 1 {
		return
	}
	vc := visibleCols(width, len(fd.Columns))
	start := viewportStart(width, len(fd.Columns), s.colIdx)
	colW := width / vc
	remainder := width - colW*vc
	cx := x

	for vi := range vc {
		ci := start + vi
		if ci >= len(fd.Columns) {
			break
		}
		thisW := colW
		if vi < remainder {
			thisW++
		}
		active := ci == s.colIdx
		drawColumnHeader(screen, fd.Columns[ci], active, cx, y-1, thisW)
		drawColumnCards(screen, fd, s, ci, active, cx, y, thisW, height)

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

func drawColumnCards(screen tcell.Screen, fd *jira.Board, s *boardState, ci int, active bool, x, y, w, h int) {
	col := fd.Columns[ci]
	if len(col.Issues) == 0 {
		drawText(screen, x+1, y, "No issues",
			tcell.StyleDefault.Foreground(colMuted).Background(colBg), w-2)
		return
	}
	curCard := min(s.cardIdx[ci], len(col.Issues)-1)
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
