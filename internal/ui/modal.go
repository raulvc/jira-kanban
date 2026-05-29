package ui

import (
	"github.com/gdamore/tcell/v2"

	"github.com/raulvc/jira-kanban/internal/jira"
)

// modalState tracks a transition picker modal.
// A nil modalState means no modal is open.
type modalState struct {
	issueKey    string
	transitions []jira.Transition
	selected    int
	errMsg      string
}

func (m *modalState) moveSelection(delta int) {
	if len(m.transitions) == 0 {
		return
	}
	m.selected += delta
	m.selected = max(0, min(m.selected, len(m.transitions)-1))
}

func (m *modalState) selectedTransition() *jira.Transition {
	if m.selected < 0 || m.selected >= len(m.transitions) {
		return nil
	}
	return &m.transitions[m.selected]
}

// drawModal renders a centered transition picker over the board.
func drawModal(screen tcell.Screen, m *modalState, screenW, screenH int) {
	// Modal dimensions
	const padding = 2
	contentW := 40
	itemCount := len(m.transitions)
	// title(1) + blank(1) + items + blank(1) + buttons(1) + optional error(1)
	contentH := 4 + itemCount
	if m.errMsg != "" {
		contentH++
	}

	// Clamp to screen
	boxW := contentW + padding*2
	boxH := contentH + padding
	if boxW > screenW-4 {
		boxW = screenW - 4
		contentW = boxW - padding*2
	}
	if boxH > screenH-4 {
		boxH = screenH - 4
	}

	// Center
	ox := (screenW - boxW) / 2
	oy := (screenH - boxH) / 2

	// Styles
	bgStyle := tcell.StyleDefault.Foreground(colFg).Background(colPanel)
	borderStyle := tcell.StyleDefault.Foreground(colMuted).Background(colPanel)
	titleStyle := tcell.StyleDefault.Foreground(colBlue).Background(colPanel).Bold(true)
	itemStyle := tcell.StyleDefault.Foreground(colFg).Background(colPanel)
	selStyle := tcell.StyleDefault.Foreground(colFg).Background(colCardSel).Bold(true)
	btnOKStyle := tcell.StyleDefault.Foreground(colBg).Background(colGreen).Bold(true)
	btnCancelStyle := tcell.StyleDefault.Foreground(colFg).Background(colMuted)
	errStyle := tcell.StyleDefault.Foreground(colRed).Background(colPanel).Bold(true)

	// Fill background
	for row := oy; row < oy+boxH; row++ {
		fillRow(screen, ox, row, boxW, bgStyle)
	}

	// Border
	drawBorder(screen, ox, oy, boxW, boxH, borderStyle)

	// Title
	cy := oy + 1
	title := " Transition: " + m.issueKey + " "
	drawText(screen, ox+padding, cy, title, titleStyle, contentW)
	cy += 2

	// Transition items
	maxVisible := boxH - 5
	if m.errMsg != "" {
		maxVisible--
	}
	if maxVisible < 1 {
		maxVisible = 1
	}

	// Scroll window
	scrollStart := 0
	if m.selected >= scrollStart+maxVisible {
		scrollStart = m.selected - maxVisible + 1
	}
	if m.selected < scrollStart {
		scrollStart = m.selected
	}

	for i := scrollStart; i < len(m.transitions) && i < scrollStart+maxVisible; i++ {
		t := m.transitions[i]
		style := itemStyle
		prefix := "  "
		if i == m.selected {
			style = selStyle
			prefix = "▸ "
		}
		fillRow(screen, ox+1, cy, boxW-2, style)
		text := truncStr(t.Name, contentW-3)
		drawText(screen, ox+padding, cy, prefix+text, style, contentW)
		cy++
	}

	// Error message
	if m.errMsg != "" {
		cy++
		drawText(screen, ox+padding, cy, truncStr(m.errMsg, contentW), errStyle, contentW)
	}

	// Buttons at bottom
	btnY := oy + boxH - 2
	fillRow(screen, ox+1, btnY, boxW-2, bgStyle)
	okText := " OK (Enter) "
	cancelText := " Cancel (Esc) "
	gap := 2
	totalBtnW := len([]rune(okText)) + gap + len([]rune(cancelText))
	btnX := ox + (boxW-totalBtnW)/2
	drawText(screen, btnX, btnY, okText, btnOKStyle, contentW)
	btnX += len([]rune(okText)) + gap
	drawText(screen, btnX, btnY, cancelText, btnCancelStyle, contentW)
}

func drawBorder(screen tcell.Screen, x, y, w, h int, style tcell.Style) {
	// Corners
	screen.SetContent(x, y, '┌', nil, style)
	screen.SetContent(x+w-1, y, '┐', nil, style)
	screen.SetContent(x, y+h-1, '└', nil, style)
	screen.SetContent(x+w-1, y+h-1, '┘', nil, style)
	// Top/bottom edges
	for col := x + 1; col < x+w-1; col++ {
		screen.SetContent(col, y, '─', nil, style)
		screen.SetContent(col, y+h-1, '─', nil, style)
	}
	// Left/right edges
	for row := y + 1; row < y+h-1; row++ {
		screen.SetContent(x, row, '│', nil, style)
		screen.SetContent(x+w-1, row, '│', nil, style)
	}
}
