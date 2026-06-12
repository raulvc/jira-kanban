package ui

import (
	"sort"

	"github.com/gdamore/tcell/v2"

	"github.com/raulvc/jira-kanban/internal/jira"
)

// filterState tracks an assignee-filter picker modal.
// A nil filterState means no filter modal is open.
type filterState struct {
	members []string
	query   string
	selected int
}

func newFilterState(data jira.Board) *filterState {
	seen := make(map[string]bool)
	for _, col := range data.Columns {
		for _, card := range col.Issues {
			if card.Assignee != "" && card.Assignee != "Unassigned" && !seen[card.Assignee] {
				seen[card.Assignee] = true
			}
		}
	}
	members := make([]string, 0, len(seen))
	for m := range seen {
		members = append(members, m)
	}
	sort.Strings(members)
	return &filterState{members: members}
}

func (f *filterState) filtered() []string {
	if f.query == "" {
		return f.members
	}
	q := toLower(f.query)
	var out []string
	for _, m := range f.members {
		if containsLower(m, q) {
			out = append(out, m)
		}
	}
	return out
}

func (f *filterState) moveSelection(delta int) {
	items := f.filtered()
	if len(items) == 0 {
		return
	}
	f.selected += delta
	f.selected = max(0, min(f.selected, len(items)-1))
}

func (f *filterState) typeRune(r rune) {
	f.query += string(r)
	f.selected = 0
}

func (f *filterState) backspace() {
	if len(f.query) > 0 {
		runes := []rune(f.query)
		f.query = string(runes[:len(runes)-1])
		items := f.filtered()
		if f.selected >= len(items) && len(items) > 0 {
			f.selected = len(items) - 1
		}
	}
}

func containsLower(s, q string) bool {
	return len(q) <= len(s) && searchLower(s, q)
}

func searchLower(s, q string) bool {
	ls := toLower(s)
	for i := 0; i <= len(ls)-len(q); i++ {
		if ls[i:i+len(q)] == q {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	runes := []rune(s)
	for i, r := range runes {
		if r >= 'A' && r <= 'Z' {
			runes[i] = r + 32
		}
	}
	return string(runes)
}

// drawFilterModal renders a centered member filter picker over the board.
func drawFilterModal(screen tcell.Screen, f *filterState, screenW, screenH int, currentUser string) {
	const padding = 2
	contentW := 40
	items := f.filtered()
	contentH := 5 + len(items) + 1 // title + search + blank + items + "All / Clear"

	boxW := contentW + padding*2
	boxH := contentH + padding
	if boxW > screenW-4 {
		boxW = screenW - 4
		contentW = boxW - padding*2
	}
	if boxH > screenH-4 {
		boxH = screenH - 4
	}

	ox := (screenW - boxW) / 2
	oy := (screenH - boxH) / 2

	bgStyle := tcell.StyleDefault.Foreground(colFg).Background(colPanel)
	borderStyle := tcell.StyleDefault.Foreground(colMuted).Background(colPanel)
	titleStyle := tcell.StyleDefault.Foreground(colBlue).Background(colPanel).Bold(true)
	searchStyle := tcell.StyleDefault.Foreground(colFg).Background(colBg)
	searchPlaceholder := tcell.StyleDefault.Foreground(colMuted).Background(colBg)
	itemStyle := tcell.StyleDefault.Foreground(colFg).Background(colPanel)
	selStyle := tcell.StyleDefault.Foreground(colFg).Background(colCardSel).Bold(true)
	meStyle := tcell.StyleDefault.Foreground(colGold).Background(colPanel).Bold(true)

	for row := oy; row < oy+boxH; row++ {
		fillRow(screen, ox, row, boxW, bgStyle)
	}
	drawBorder(screen, ox, oy, boxW, boxH, borderStyle)

	cy := oy + 1
	drawText(screen, ox+padding, cy, " Filter by member ", titleStyle, contentW)
	cy++

	fillRow(screen, ox+padding, cy, contentW, searchStyle)
	if f.query == "" {
		drawText(screen, ox+padding, cy, " Search…", searchPlaceholder, contentW)
	} else {
		drawText(screen, ox+padding, cy, " "+f.query+"▏", searchStyle, contentW)
	}
	cy++
	cy++

	maxVisible := max(boxH-6, 1)

	scrollStart := 0
	if f.selected >= scrollStart+maxVisible {
		scrollStart = f.selected - maxVisible + 1
	}
	if f.selected < scrollStart {
		scrollStart = f.selected
	}

	for i := scrollStart; i < len(items) && i < scrollStart+maxVisible; i++ {
		style := itemStyle
		prefix := "  "
		if items[i] == currentUser {
			style = meStyle
			prefix = "★ "
		}
		if i == f.selected {
			style = selStyle
			prefix = "▸ "
			if items[i] == currentUser {
				prefix = "▸★"
			}
		}
		fillRow(screen, ox+1, cy, boxW-2, style)
		text := truncStr(items[i], contentW-3)
		drawText(screen, ox+padding, cy, prefix+text, style, contentW)
		cy++
	}

	if len(items) == 0 {
		drawText(screen, ox+padding, cy, "  No matches", searchPlaceholder, contentW)
	}

	btnY := oy + boxH - 2
	fillRow(screen, ox+1, btnY, boxW-2, bgStyle)
	allStyle := tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(colOrange).Bold(true)
	cancelStyle := tcell.StyleDefault.Foreground(colFg).Background(colMuted)
	allText := " All (Esc) "
	clearText := " Clear "
	gap := 2
	totalBtnW := len([]rune(allText)) + gap + len([]rune(clearText))
	btnX := ox + (boxW-totalBtnW)/2
	drawText(screen, btnX, btnY, allText, allStyle, contentW)
	btnX += len([]rune(allText)) + gap
	drawText(screen, btnX, btnY, clearText, cancelStyle, contentW)
}