package ui

import (
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"

	"github.com/raulvc/jira-kanban/internal/jira"
)

type createField int

const (
	cfType createField = iota
	cfSummary
	cfDescription
	cfEpic
	cfFieldCount
)

// createIssueState tracks the create-issue form modal.
type createIssueState struct {
	projectKey string
	parentKey  string
	types      []jira.IssueType
	typeIdx    int
	summary    string
	sumCur     int
	desc       string
	descScroll int
	descCur    int
	epicQuery  string
	epicCur    int
	epicKey    string
	epicName   string
	epics      []jira.EpicItem
	epicSel    int
	field      createField
	errMsg     string
	creating   bool
	debounce   *time.Timer
	epicLoaded bool
	epicFreq   map[string]int
}

var typeOrder = map[string]int{"Task": 0, "Bug": 1, "Epic": 2}

// typeStyle returns the icon and badge style for an issue type name.
func typeStyle(name string) (icon string, bg tcell.Color) {
	switch name {
	case "Task":
		return "\u25B8", colCyan // ▸
	case "Bug":
		return "\u25CF", colRed // ●
	case "Epic":
		return "\u2696", colViolet // ⚖
	default:
		return "\u25CB", colBlue // ○
	}
}

func newCreateIssueState(projectKey string) *createIssueState {
	return &createIssueState{
		projectKey: projectKey,
		types: []jira.IssueType{
			{ID: "", Name: "Task"},
			{ID: "", Name: "Bug"},
			{ID: "", Name: "Epic"},
		},
		typeIdx: 0,
	}
}

func (c *createIssueState) isSubtask() bool { return c.parentKey != "" }

func (c *createIssueState) fieldCount() createField {
	if c.isSubtask() {
		return cfDescription + 1
	}
	return cfFieldCount
}

func (c *createIssueState) nextField() {
	c.field = (c.field + 1) % c.fieldCount()
}

func (c *createIssueState) prevField() {
	c.field = (c.field - 1 + c.fieldCount()) % c.fieldCount()
}

func (c *createIssueState) currentTypeName() string {
	if c.typeIdx >= 0 && c.typeIdx < len(c.types) {
		return c.types[c.typeIdx].Name
	}
	return "Task"
}

func (c *createIssueState) currentTypeID() string {
	if c.typeIdx >= 0 && c.typeIdx < len(c.types) {
		return c.types[c.typeIdx].ID
	}
	return ""
}

func (c *createIssueState) cycleType(delta int) {
	c.typeIdx = (c.typeIdx + delta + len(c.types)) % len(c.types)
}

func (c *createIssueState) stopDebounce() {
	if c.debounce != nil {
		c.debounce.Stop()
		c.debounce = nil
	}
}

// clampCur ensures the cursor of the current field is within bounds.
func (c *createIssueState) clampCur() {
	switch c.field {
	case cfSummary:
		c.sumCur = clamp(c.sumCur, 0, len([]rune(c.summary)))
	case cfDescription:
		c.descCur = clamp(c.descCur, 0, len([]rune(c.desc)))
		c.descAutoScroll(descWrapW)
	case cfEpic:
		c.epicCur = clamp(c.epicCur, 0, len([]rune(c.epicQuery)))
		items := c.filteredEpics()
		if c.epicSel >= len(items) {
			c.epicSel = max(0, len(items)-1)
		}
	}
}

const epicMaxVis = 5

func (c *createIssueState) filteredEpics() []jira.EpicItem {
	var out []jira.EpicItem
	if c.epicQuery == "" {
		n := min(epicMaxVis, len(c.epics))
		out = make([]jira.EpicItem, n)
		copy(out, c.epics[:n])
	} else {
		q := foldLower(c.epicQuery)
		for _, e := range c.epics {
			if containsFold(e.Summary, q) || containsFold(e.Key, q) {
				out = append(out, e)
				if len(out) >= epicMaxVis {
					break
				}
			}
		}
	}
	return out
}

func (c *createIssueState) typeRune(r rune) {
	switch c.field {
	case cfSummary:
		runes := []rune(c.summary)
		c.summary = string(runes[:c.sumCur]) + string(r) + string(runes[c.sumCur:])
		c.sumCur++
	case cfDescription:
		runes := []rune(c.desc)
		c.desc = string(runes[:c.descCur]) + string(r) + string(runes[c.descCur:])
		c.descCur++
		c.descAutoScroll(descWrapW)
	case cfEpic:
		runes := []rune(c.epicQuery)
		c.epicQuery = string(runes[:c.epicCur]) + string(r) + string(runes[c.epicCur:])
		c.epicCur++
		c.epicSel = 0
	}
}

func (c *createIssueState) backspace() {
	switch c.field {
	case cfSummary:
		if c.sumCur > 0 {
			runes := []rune(c.summary)
			c.summary = string(runes[:c.sumCur-1]) + string(runes[c.sumCur:])
			c.sumCur--
		}
	case cfDescription:
		if c.descCur > 0 {
			runes := []rune(c.desc)
			c.desc = string(runes[:c.descCur-1]) + string(runes[c.descCur:])
			c.descCur--
			c.descAutoScroll(descWrapW)
		}
	case cfEpic:
		if c.epicCur > 0 {
			runes := []rune(c.epicQuery)
			c.epicQuery = string(runes[:c.epicCur-1]) + string(runes[c.epicCur:])
			c.epicCur--
			items := c.filteredEpics()
			if c.epicSel >= len(items) && len(items) > 0 {
				c.epicSel = len(items) - 1
			}
		}
	}
}

func (c *createIssueState) handleEnter() {
	switch c.field {
	case cfEpic:
		items := c.filteredEpics()
		if c.epicSel >= 0 && c.epicSel < len(items) {
			c.epicKey = items[c.epicSel].Key
			c.epicName = items[c.epicSel].Summary
			c.field = cfSummary
			c.clampCur()
		}
	}
}

func (c *createIssueState) deleteForward() {
	switch c.field {
	case cfSummary:
		runes := []rune(c.summary)
		if c.sumCur < len(runes) {
			c.summary = string(runes[:c.sumCur]) + string(runes[c.sumCur+1:])
		}
	case cfDescription:
		runes := []rune(c.desc)
		if c.descCur < len(runes) {
			c.desc = string(runes[:c.descCur]) + string(runes[c.descCur+1:])
			c.descAutoScroll(descWrapW)
		}
	case cfEpic:
		runes := []rune(c.epicQuery)
		if c.epicCur < len(runes) {
			c.epicQuery = string(runes[:c.epicCur]) + string(runes[c.epicCur+1:])
			items := c.filteredEpics()
			if c.epicSel >= len(items) && len(items) > 0 {
				c.epicSel = len(items) - 1
			}
		}
	}
}

func (c *createIssueState) handleNewline() {
	if c.field == cfDescription {
		runes := []rune(c.desc)
		c.desc = string(runes[:c.descCur]) + "\n" + string(runes[c.descCur:])
		c.descCur++
		c.descAutoScroll(descWrapW)
	}
}

// descAutoScroll adjusts descScroll so the cursor line is always visible.
// w is the content width used for line wrapping.
func (c *createIssueState) descAutoScroll(w int) {
	lines := descLines(c.desc, w)
	curLine := c.descCurLine(lines)
	maxScroll := max(0, len(lines)-descVisH)
	c.descScroll = clamp(c.descScroll, 0, maxScroll)
	if curLine < c.descScroll {
		c.descScroll = curLine
	} else if curLine >= c.descScroll+descVisH {
		c.descScroll = curLine - descVisH + 1
	}
}

// descCurLine returns the display line index that contains the descCur rune offset.
func (c *createIssueState) descCurLine(lines []string) int {
	if len(lines) == 0 {
		return 0
	}
	for i, off := range descLineOffsets(c.desc, lines) {
		if c.descCur <= off+len([]rune(lines[i])) {
			return i
		}
	}
	return len(lines) - 1
}

// descLineOffsets returns the rune offset within c.desc where each display line starts.
func descLineOffsets(desc string, lines []string) []int {
	offsets := make([]int, len(lines))
	runes := []rune(desc)
	offset := 0
	for i, line := range lines {
		offsets[i] = offset
		offset += len([]rune(line))
		if offset < len(runes) && runes[offset] == '\n' {
			offset++
		}
	}
	return offsets
}

// descMoveUp moves descCur to the previous display line, preserving column position.
func (c *createIssueState) descMoveUp(lines []string) {
	curLine := c.descCurLine(lines)
	if curLine == 0 {
		c.descCur = 0
		return
	}
	offsets := descLineOffsets(c.desc, lines)
	curCol := c.descCur - offsets[curLine]
	prevLineRunes := len([]rune(lines[curLine-1]))
	newCol := min(curCol, prevLineRunes)
	c.descCur = offsets[curLine-1] + newCol
}

// descMoveDown moves descCur to the next display line, preserving column position.
func (c *createIssueState) descMoveDown(lines []string) {
	curLine := c.descCurLine(lines)
	if curLine >= len(lines)-1 {
		c.descCur = len([]rune(c.desc))
		return
	}
	offsets := descLineOffsets(c.desc, lines)
	curCol := c.descCur - offsets[curLine]
	nextLineRunes := len([]rune(lines[curLine+1]))
	newCol := min(curCol, nextLineRunes)
	c.descCur = offsets[curLine+1] + newCol
}

func clamp(v, lo, hi int) int { return max(lo, min(v, hi)) }

// descLines splits the description into display lines, wrapping long lines at width.
func descLines(desc string, w int) []string {
	if desc == "" {
		return nil
	}
	var lines []string
	for _, para := range strings.Split(desc, "\n") {
		if para == "" {
			lines = append(lines, "")
			continue
		}
		for len(para) > 0 {
			cut := min(len(para), w)
			lines = append(lines, para[:cut])
			para = para[cut:]
		}
	}
	return lines
}
func sortTypesByPriority(types []jira.IssueType) {
	sort.SliceStable(types, func(i, j int) bool {
		pi, ok1 := typeOrder[types[i].Name]
		pj, ok2 := typeOrder[types[j].Name]
		if ok1 && ok2 {
			return pi < pj
		}
		if ok1 {
			return true
		}
		if ok2 {
			return false
		}
		return types[i].Name < types[j].Name
	})
}

// sortEpicsByFreq sorts epics so those used most on the current board come first.
// freq maps epic summary names to occurrence counts (from board card data).
func sortEpicsByFreq(epics []jira.EpicItem, freq map[string]int) {
	sort.SliceStable(epics, func(i, j int) bool {
		return freq[epics[i].Summary] > freq[epics[j].Summary]
	})
}

// accentFold maps common accented runes to their ASCII base.
var accentFold = map[rune]rune{
	'à': 'a', 'á': 'a', 'â': 'a', 'ã': 'a', 'ä': 'a', 'å': 'a',
	'À': 'a', 'Á': 'a', 'Â': 'a', 'Ã': 'a', 'Ä': 'a', 'Å': 'a',
	'è': 'e', 'é': 'e', 'ê': 'e', 'ë': 'e',
	'È': 'e', 'É': 'e', 'Ê': 'e', 'Ë': 'e',
	'ì': 'i', 'í': 'i', 'î': 'i', 'ï': 'i',
	'Ì': 'i', 'Í': 'i', 'Î': 'i', 'Ï': 'i',
	'ò': 'o', 'ó': 'o', 'ô': 'o', 'õ': 'o', 'ö': 'o',
	'Ò': 'o', 'Ó': 'o', 'Ô': 'o', 'Õ': 'o', 'Ö': 'o',
	'ù': 'u', 'ú': 'u', 'û': 'u', 'ü': 'u',
	'Ù': 'u', 'Ú': 'u', 'Û': 'u', 'Ü': 'u',
	'ñ': 'n', 'Ñ': 'n',
	'ç': 'c', 'Ç': 'c',
	'ÿ': 'y', 'Ÿ': 'y',
	'ß': 's',
}

// foldLower returns a case-insensitive, accent-insensitive lowercase string for matching.
func foldLower(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if base, ok := accentFold[r]; ok {
			b.WriteRune(base)
		} else {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// containsFold reports whether q is found in s after both are fold-lowered.
func containsFold(s, q string) bool {
	return strings.Contains(foldLower(s), q)
}

// ── drawing ────────────────────────────────────────────────────────────────

const descVisH = 3

// descWrapW is the content width used for description line wrapping
// (matches contentW-2 from drawCreateIssue where contentW=54).
const descWrapW = 52

func drawCreateIssue(screen tcell.Screen, c *createIssueState, screenW, screenH int, _ string) {
	const padding = 2
	contentW := 54

	descExtra := descVisH - 1 // description field uses descVisH rows instead of 1

	epicListH := 0
	epicSepRow := 0
	if !c.isSubtask() && c.field == cfEpic && c.epicKey == "" {
		epicListH = epicMaxVis
		epicSepRow = 1
	}
	contentH := 15 + descExtra + epicListH + epicSepRow
	if c.errMsg != "" {
		contentH++
	}

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
	labelStyle := tcell.StyleDefault.Foreground(colMuted).Background(colPanel)
	inputStyle := tcell.StyleDefault.Foreground(colFg).Background(colBg)
	inputPlaceholder := tcell.StyleDefault.Foreground(colMuted).Background(colBg)
	activeLabelStyle := tcell.StyleDefault.Foreground(colCyan).Background(colPanel).Bold(true)
	activeInputStyle := tcell.StyleDefault.Foreground(colFg).Background(colCardSel)
	activeBorder := tcell.StyleDefault.Foreground(colCyan).Background(colBg)
	errStyle := tcell.StyleDefault.Foreground(colRed).Background(colPanel).Bold(true)
	sepStyle := tcell.StyleDefault.Foreground(colMuted).Background(colPanel).Dim(true)

	for row := oy; row < oy+boxH; row++ {
		fillRow(screen, ox, row, boxW, bgStyle)
	}
	drawBorder(screen, ox, oy, boxW, boxH, borderStyle)

	cy := oy + 1
	if c.isSubtask() {
		drawText(screen, ox+padding, cy, " Create Subtask — "+c.parentKey+" ", titleStyle, contentW)
	} else {
		drawText(screen, ox+padding, cy, " Create Issue — "+c.projectKey+" ", titleStyle, contentW)
	}
	cy++
	drawSep(screen, ox+padding, cy, contentW, sepStyle)
	cy++

	// ── Type ──────────────────────────────────────────────────────────────
	lStyle := labelStyle
	if c.field == cfType {
		lStyle = activeLabelStyle
	}
	drawText(screen, ox+padding, cy, " Type", lStyle, contentW)
	cy++
	icon, typeBg := typeStyle(c.currentTypeName())
	bStyle := tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(typeBg).Bold(true)
	badge := " " + icon + " " + c.currentTypeName() + " "
	badgeRowBg := tcell.StyleDefault.Foreground(colFg).Background(colBg)
	badgeRowBorderSt := tcell.StyleDefault.Foreground(colMuted).Background(colBg)
	if c.field == cfType {
		badgeRowBg = tcell.StyleDefault.Foreground(colFg).Background(colCardSel)
		badgeRowBorderSt = activeBorder
	}
	screen.SetContent(ox+padding, cy, '│', nil, badgeRowBorderSt)
	fillRow(screen, ox+padding+1, cy, contentW-2, badgeRowBg)
	drawText(screen, ox+padding+1, cy, badge, bStyle, contentW-2)
	if c.field == cfType {
		drawText(screen, ox+padding+1+len([]rune(badge))+1, cy, "\u2190/\u2192", tcell.StyleDefault.Foreground(colMuted).Background(colCardSel), contentW-2)
		screen.SetContent(ox+padding+contentW-1, cy, '│', nil, badgeRowBorderSt)
	}
	cy++
	drawSep(screen, ox+padding, cy, contentW, sepStyle)
	cy++

	// ── Summary ───────────────────────────────────────────────────────────
	lStyle = labelStyle
	if c.field == cfSummary {
		lStyle = activeLabelStyle
	}
	drawText(screen, ox+padding, cy, " Summary", lStyle, contentW)
	cy++
	drawInputField(screen, ox+padding, cy, contentW, c.summary, "Issue title…", c.field == cfSummary, c.sumCur, inputStyle, activeInputStyle, inputPlaceholder, activeBorder)
	cy++
	drawSep(screen, ox+padding, cy, contentW, sepStyle)
	cy++

	// ── Description ────────────────────────────────────────────────────────
	lStyle = labelStyle
	if c.field == cfDescription {
		lStyle = activeLabelStyle
	}
	drawText(screen, ox+padding, cy, " Description", lStyle, contentW)
	cy++

	// Multiline description box with scroll
	descBoxY := cy
	descBoxH := descVisH
	for row := 0; row < descBoxH; row++ {
		screen.SetContent(ox+padding, cy, '│', nil, tcell.StyleDefault.Foreground(colMuted).Background(colBg))
		fillRow(screen, ox+padding+1, cy, contentW-1, inputStyle)
		cy++
	}

	lines := descLines(c.desc, descWrapW)
	maxScroll := max(0, len(lines)-descBoxH)
	c.descScroll = clamp(c.descScroll, 0, maxScroll)

	for i := 0; i < descBoxH; i++ {
		lineIdx := c.descScroll + i
		row := descBoxY + i
		screen.SetContent(ox+padding, row, '│', nil, tcell.StyleDefault.Foreground(colMuted).Background(colBg))
		fillRow(screen, ox+padding+1, row, contentW-1, inputStyle)
		if c.field == cfDescription {
			screen.SetContent(ox+padding, row, '│', nil, activeBorder)
			fillRow(screen, ox+padding+1, row, contentW-1, activeInputStyle)
		}

		if lineIdx < len(lines) {
			st := inputStyle
			if c.field == cfDescription {
				st = activeInputStyle
			}
			drawText(screen, ox+padding+1, row, lines[lineIdx], st, contentW-2)
		} else if lineIdx == len(lines) && c.field == cfDescription && c.desc == "" {
			drawText(screen, ox+padding+1, row, "Optional, Enter for newline…", inputPlaceholder, contentW-2)
		}
	}

	// Cursor position
	if c.field == cfDescription && c.desc != "" {
		cursorSt := tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(colCyan).Bold(true)
		curLine := c.descCurLine(lines)
		offsets := descLineOffsets(c.desc, lines)
		curCol := c.descCur - offsets[curLine]
		lineRunes := len([]rune(lines[curLine]))
		if curCol > lineRunes {
			curCol = lineRunes
		}
		if curCol < 0 {
			curCol = 0
		}
		curRow := descBoxY + curLine - c.descScroll
		if curRow >= descBoxY && curRow < descBoxY+descBoxH {
			col := ox + padding + 1 + curCol
			if curCol < lineRunes {
				ch := []rune(lines[curLine])[curCol]
				screen.SetContent(col, curRow, ch, nil, cursorSt)
			} else {
				screen.SetContent(col, curRow, '▏', nil, activeInputStyle)
			}
		}
	}

	// Scroll indicator
	if c.field == cfDescription && len(lines) > descBoxH {
		totalScroll := max(1, len(lines)-descBoxH)
		barH := max(1, descBoxH*descBoxH/len(lines))
		barTop := descBoxY + c.descScroll*(descBoxH-barH)/totalScroll
		for i := 0; i < barH && barTop+i < descBoxY+descBoxH; i++ {
			screen.SetContent(ox+padding+contentW-1, barTop+i, '▐', nil, tcell.StyleDefault.Foreground(colMuted).Background(colBg))
		}
	}

	cy = descBoxY + descBoxH
	drawSep(screen, ox+padding, cy, contentW, sepStyle)
	cy++

	// ── Epic ──────────────────────────────────────────────────────────────
	if !c.isSubtask() {
		lStyle = labelStyle
		if c.field == cfEpic {
			lStyle = activeLabelStyle
		}
		epicLabel := " Epic"
		if c.epicKey != "" && c.field != cfEpic {
			epicLabel = " Epic  ✓ " + c.epicKey + " — " + truncStr(c.epicName, contentW-12)
		}
		drawText(screen, ox+padding, cy, epicLabel, lStyle, contentW)
		cy++

		if c.field == cfEpic && c.epicKey == "" {
			drawInputField(screen, ox+padding, cy, contentW, c.epicQuery, "Search epic…", true, c.epicCur, inputStyle, activeInputStyle, inputPlaceholder, activeBorder)
			cy++
			drawSep(screen, ox+padding, cy, contentW, sepStyle)
			cy++

			items := c.filteredEpics()
			epicBtnY := oy + boxH - 2
			for i := range items {
				style := tcell.StyleDefault.Foreground(colFg).Background(colPanel)
				prefix := "  "
				if i == c.epicSel {
					style = tcell.StyleDefault.Foreground(colFg).Background(colCardSel).Bold(true)
					prefix = "▸ "
				}
				text := items[i].Key + "  " + truncStr(items[i].Summary, contentW-10)
				fillRow(screen, ox+1, cy, boxW-2, style)
				drawText(screen, ox+padding, cy, prefix+text, style, contentW)
				cy++
			}
			for cy < epicBtnY {
				fillRow(screen, ox+1, cy, boxW-2, bgStyle)
				cy++
			}
			if len(items) == 0 && c.epicQuery != "" {
				if !c.epicLoaded {
					drawText(screen, ox+padding, epicBtnY-2, "  Loading epics…", tcell.StyleDefault.Foreground(colCyan).Background(colPanel), contentW)
				} else {
					drawText(screen, ox+padding, epicBtnY-2, "  No epics found", inputPlaceholder, contentW)
				}
			}
		} else if c.epicKey != "" {
			epicIcon, epicBg := typeStyle("Epic")
			epicBadgeStyle := tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(epicBg).Bold(true)
			badge := " " + epicIcon + " " + truncStr(c.epicKey+" "+c.epicName, contentW-4) + " "
			screen.SetContent(ox+padding, cy, '│', nil, tcell.StyleDefault.Foreground(colMuted).Background(colBg))
			fillRow(screen, ox+padding+1, cy, contentW-2, tcell.StyleDefault.Foreground(colFg).Background(colBg))
			if c.field == cfEpic {
				screen.SetContent(ox+padding, cy, '│', nil, activeBorder)
				fillRow(screen, ox+padding+1, cy, contentW-2, activeInputStyle)
				screen.SetContent(ox+padding+contentW-1, cy, '│', nil, activeBorder)
			}
			drawText(screen, ox+padding+1, cy, badge, epicBadgeStyle, contentW-2)
			if c.field == cfEpic {
				drawText(screen, ox+padding+1+len([]rune(badge))+1, cy, "Ctrl+U clear", tcell.StyleDefault.Foreground(colMuted).Background(colCardSel), contentW-2)
			}
			cy++
		} else {
			fillRow(screen, ox+padding, cy, contentW, tcell.StyleDefault.Foreground(colMuted).Background(colBg))
			drawText(screen, ox+padding, cy, "  None", inputPlaceholder, contentW)
			cy++
		}
	}

	// Error
	if c.errMsg != "" {
		cy++
		drawText(screen, ox+padding, cy, " ✗ "+truncStr(c.errMsg, contentW-3), errStyle, contentW)
	}

	// Buttons at bottom
	btnY := oy + boxH - 2
	// Separator row between content and buttons
	drawSep(screen, ox+padding, btnY-1, contentW, sepStyle)
	fillRow(screen, ox+1, btnY, boxW-2, bgStyle)
	createStyle := tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(colGreen).Bold(true)
	cancelStyle := tcell.StyleDefault.Foreground(colFg).Background(colMuted)
	createText := " Create (Enter) "
	cancelText := " Cancel (Esc) "
	gap := 3
	totalBtnW := len([]rune(createText)) + gap + len([]rune(cancelText))
	btnX := ox + (boxW-totalBtnW)/2
	drawText(screen, btnX, btnY, createText, createStyle, contentW)
	btnX += len([]rune(createText)) + gap
	drawText(screen, btnX, btnY, cancelText, cancelStyle, contentW)

	// Creating overlay
	if c.creating {
		for row := oy; row < oy+boxH; row++ {
			fillRow(screen, ox, row, boxW, tcell.StyleDefault.Foreground(colFg).Background(colPanel).Dim(true))
		}
		drawBorder(screen, ox, oy, boxW, boxH, borderStyle.Dim(true))
		creatingStyle := tcell.StyleDefault.Foreground(colCyan).Background(colPanel).Bold(true)
		drawText(screen, ox+(boxW-len([]rune(" Creating…")))/2, oy+boxH/2, " Creating…", creatingStyle, boxW)
	}
}

// drawInputField draws a labeled input row with a left border indicator.
// curPos is the rune offset of the cursor within value (-1 means end of text).
// When cursor is within text, the character at cursor position is highlighted.
// When cursor is at end, a ▏ bar is drawn.
func drawInputField(screen tcell.Screen, x, y, w int, value, placeholder string, active bool, curPos int, style, activeStyle, placeholderStyle, activeBorder tcell.Style) {
	borderCh := '│'
	borderSt := tcell.StyleDefault.Foreground(colMuted).Background(colBg)
	inputSt := style
	if active {
		borderSt = activeBorder
		inputSt = activeStyle
	}
	screen.SetContent(x, y, borderCh, nil, borderSt)
	fillRow(screen, x+1, y, w-1, inputSt)
	if value == "" {
		drawText(screen, x+1, y, placeholder, placeholderStyle, w-1)
		return
	}
	runes := []rune(value)
	if curPos < 0 || curPos > len(runes) {
		curPos = len(runes)
	}
	visW := w - 2
	// Horizontal scroll: keep cursor visible
	scrollStart := 0
	if curPos >= visW {
		scrollStart = curPos - visW + 1
	}
	// Draw visible text
	visible := string(runes[scrollStart:min(scrollStart+visW, len(runes))])
	drawText(screen, x+1, y, visible, inputSt, visW)
	if !active {
		return
	}
	cursorCol := curPos - scrollStart
	if cursorCol < 0 || cursorCol >= visW {
		return
	}
	if curPos < len(runes) {
		// Cursor within text: highlight the character at cursor position
		ch := runes[curPos]
		cursorSt := tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(colCyan).Bold(true)
		screen.SetContent(x+1+cursorCol, y, ch, nil, cursorSt)
	} else {
		// Cursor at end of text: draw bar
		screen.SetContent(x+1+cursorCol, y, '▏', nil, inputSt)
	}
}

// drawSep draws a horizontal separator line.
func drawSep(screen tcell.Screen, x, y, w int, style tcell.Style) {
	line := strings.Repeat("─", w)
	drawText(screen, x, y, line, style, w)
}