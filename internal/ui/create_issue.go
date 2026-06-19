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
	cfButtons    // virtual field: OK / Cancel buttons
	cfFieldCount // total navigable positions (not a field itself)
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
	btnIdx     int // 0 = OK, 1 = Cancel
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
		return "\u25B8", T().Cyan // ▸
	case "Bug":
		return "\u25CF", T().Red // ●
	case "Epic":
		return "\u2696", T().Violet // ⚖
	default:
		return "\u25CB", T().Blue // ○
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

// navCount returns the total number of navigable positions (fields + buttons).
func (c *createIssueState) navCount() createField {
	return cfButtons + 1
}

func (c *createIssueState) nextField() {
	for {
		c.field = (c.field + 1) % c.navCount()
		if !c.isSubtask() || c.field != cfEpic {
			break
		}
	}
	c.clampCur()
}

func (c *createIssueState) prevField() {
	for {
		c.field = (c.field - 1 + c.navCount()) % c.navCount()
		if !c.isSubtask() || c.field != cfEpic {
			break
		}
	}
	c.clampCur()
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
		c.sumCur = clamp(c.sumCur, len([]rune(c.summary)))
	case cfDescription:
		c.descCur = clamp(c.descCur, len([]rune(c.desc)))
		c.descAutoScroll()
	case cfEpic:
		c.epicCur = clamp(c.epicCur, len([]rune(c.epicQuery)))
		items := c.filteredEpics()
		if c.epicSel >= len(items) {
			c.epicSel = max(0, len(items)-1)
		}
	case cfButtons:
		c.btnIdx = clamp(c.btnIdx, 1)
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
		c.descAutoScroll()
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
			c.descAutoScroll()
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
			c.field = cfEpic
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
			c.descAutoScroll()
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
		c.descAutoScroll()
	}
}

// descAutoScroll adjusts descScroll so the cursor line is always visible.
// descAutoScroll adjusts descScroll so the cursor line is always visible.
func (c *createIssueState) descAutoScroll() {
	lines := descLines(c.desc)
	curLine := c.descCurLine(lines)
	maxScroll := max(0, len(lines)-descVisH)
	c.descScroll = clamp(c.descScroll, maxScroll)
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

func clamp(v, hi int) int { return max(0, min(v, hi)) }

// descLines splits the description into display lines, wrapping long lines at width.
func descLines(desc string) []string {
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
			cut := min(len(para), descWrapW)
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

// createLayout holds computed layout values for the create-issue modal.
type createLayout struct {
	ox, oy, boxW, boxH, contentW int
}

func drawCreateIssue(screen tcell.Screen, c *createIssueState, screenW, screenH int, _ string) {
	const padding = 2
	contentW := 54

	descExtra := descVisH - 1

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
	lay := createLayout{ox, oy, boxW, boxH, contentW}

	bgStyle := tcell.StyleDefault.Foreground(T().Fg).Background(T().Panel)
	borderStyle := tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel)
	titleStyle := tcell.StyleDefault.Foreground(T().Blue).Background(T().Panel).Bold(true)
	labelStyle := tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel)
	inputStyle := tcell.StyleDefault.Foreground(T().Fg).Background(T().Bg)
	inputPlaceholder := tcell.StyleDefault.Foreground(T().Muted).Background(T().Bg)
	activeLabelStyle := tcell.StyleDefault.Foreground(T().Cyan).Background(T().Panel).Bold(true)
	activeInputStyle := tcell.StyleDefault.Foreground(T().Fg).Background(T().CardSel)
	activeBorder := tcell.StyleDefault.Foreground(T().Cyan).Background(T().Bg)
	errStyle := tcell.StyleDefault.Foreground(T().Red).Background(T().Panel).Bold(true)
	sepStyle := tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel).Dim(true)

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

	cy = drawCreateTypeSection(screen, c, lay, cy, labelStyle, activeLabelStyle, activeBorder, sepStyle)
	cy = drawCreateSummarySection(screen, c, lay, cy, labelStyle, activeLabelStyle, inputStyle, activeInputStyle, inputPlaceholder, activeBorder, sepStyle)
	cy = drawCreateDescSection(screen, c, lay, cy, labelStyle, activeLabelStyle, inputStyle, activeInputStyle, inputPlaceholder, activeBorder)
	cy = drawCreateEpicSection(screen, c, lay, cy, labelStyle, activeLabelStyle, inputStyle, activeInputStyle, inputPlaceholder, activeBorder, sepStyle, bgStyle)

	if c.errMsg != "" {
		cy++
		drawText(screen, ox+padding, cy, " ✗ "+truncStr(c.errMsg, contentW-3), errStyle, contentW)
	}

	drawCreateButtons(screen, c, lay, bgStyle, sepStyle, borderStyle)
}

func drawCreateTypeSection(screen tcell.Screen, c *createIssueState, lay createLayout, cy int, labelStyle, activeLabelStyle, activeBorder, sepStyle tcell.Style) int {
	lStyle := labelStyle
	if c.field == cfType {
		lStyle = activeLabelStyle
	}
	drawText(screen, lay.ox+2, cy, " Type", lStyle, lay.contentW)
	cy++
	icon, typeBg := typeStyle(c.currentTypeName())
	bStyle := tcell.StyleDefault.Foreground(T().BadgeFg).Background(typeBg).Bold(true)
	badge := " " + icon + " " + c.currentTypeName() + " "
	badgeRowBg := tcell.StyleDefault.Foreground(T().Fg).Background(T().Bg)
	badgeRowBorderSt := tcell.StyleDefault.Foreground(T().Muted).Background(T().Bg)
	if c.field == cfType {
		badgeRowBg = tcell.StyleDefault.Foreground(T().Fg).Background(T().CardSel)
		badgeRowBorderSt = activeBorder
	}
	screen.SetContent(lay.ox+2, cy, '│', nil, badgeRowBorderSt)
	fillRow(screen, lay.ox+3, cy, lay.contentW-2, badgeRowBg)
	drawText(screen, lay.ox+3, cy, badge, bStyle, lay.contentW-2)
	if c.field == cfType {
		drawText(screen, lay.ox+3+len([]rune(badge))+1, cy, "\u2190/\u2192", tcell.StyleDefault.Foreground(T().Muted).Background(T().CardSel), lay.contentW-2)
		screen.SetContent(lay.ox+2+lay.contentW-1, cy, '│', nil, badgeRowBorderSt)
	}
	cy++
	drawSep(screen, lay.ox+2, cy, lay.contentW, sepStyle)
	cy++
	return cy
}

func drawCreateSummarySection(screen tcell.Screen, c *createIssueState, lay createLayout, cy int, labelStyle, activeLabelStyle, inputStyle, activeInputStyle, inputPlaceholder, activeBorder, sepStyle tcell.Style) int {
	lStyle := labelStyle
	if c.field == cfSummary {
		lStyle = activeLabelStyle
	}
	drawText(screen, lay.ox+2, cy, " Summary", lStyle, lay.contentW)
	cy++
	drawInputField(screen, lay.ox+2, cy, lay.contentW, c.summary, "Issue title…", c.field == cfSummary, c.sumCur, inputStyle, activeInputStyle, inputPlaceholder, activeBorder)
	cy++
	drawSep(screen, lay.ox+2, cy, lay.contentW, sepStyle)
	cy++
	return cy
}

func drawCreateDescSection(screen tcell.Screen, c *createIssueState, lay createLayout, cy int, labelStyle, activeLabelStyle, inputStyle, activeInputStyle, inputPlaceholder, activeBorder tcell.Style) int {
	lStyle := labelStyle
	if c.field == cfDescription {
		lStyle = activeLabelStyle
	}
	drawText(screen, lay.ox+2, cy, " Description", lStyle, lay.contentW)
	cy++

	descBoxY := cy
	descBoxH := descVisH
	for row := 0; row < descBoxH; row++ {
		screen.SetContent(lay.ox+2, cy, '│', nil, tcell.StyleDefault.Foreground(T().Muted).Background(T().Bg))
		fillRow(screen, lay.ox+3, cy, lay.contentW-1, inputStyle)
		cy++
	}

	lines := descLines(c.desc)
	maxScroll := max(0, len(lines)-descBoxH)
	c.descScroll = clamp(c.descScroll, maxScroll)

	for i := 0; i < descBoxH; i++ {
		lineIdx := c.descScroll + i
		row := descBoxY + i
		screen.SetContent(lay.ox+2, row, '│', nil, tcell.StyleDefault.Foreground(T().Muted).Background(T().Bg))
		fillRow(screen, lay.ox+3, row, lay.contentW-1, inputStyle)
		if c.field == cfDescription {
			screen.SetContent(lay.ox+2, row, '│', nil, activeBorder)
			fillRow(screen, lay.ox+3, row, lay.contentW-1, activeInputStyle)
		}

		if lineIdx < len(lines) {
			st := inputStyle
			if c.field == cfDescription {
				st = activeInputStyle
			}
			drawText(screen, lay.ox+3, row, lines[lineIdx], st, lay.contentW-2)
		} else if lineIdx == len(lines) && c.field == cfDescription && c.desc == "" {
			drawText(screen, lay.ox+3, row, "Optional, Enter for newline…", inputPlaceholder, lay.contentW-2)
		}
	}

	if c.field == cfDescription && c.desc != "" {
		drawDescCursor(screen, c, lay, descBoxY, descBoxH, lines, activeInputStyle)
	}

	if c.field == cfDescription && len(lines) > descBoxH {
		drawDescScrollbar(screen, lay, descBoxY, descBoxH, c, lines)
	}

	return descBoxY + descBoxH
}

func drawDescCursor(screen tcell.Screen, c *createIssueState, lay createLayout, descBoxY, descBoxH int, lines []string, activeInputStyle tcell.Style) {
	cursorSt := tcell.StyleDefault.Foreground(T().BadgeFg).Background(T().Cyan).Bold(true)
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
		col := lay.ox + 3 + curCol
		if curCol < lineRunes {
			ch := []rune(lines[curLine])[curCol]
			screen.SetContent(col, curRow, ch, nil, cursorSt)
		} else {
			screen.SetContent(col, curRow, '▏', nil, activeInputStyle)
		}
	}
}

func drawDescScrollbar(screen tcell.Screen, lay createLayout, descBoxY, descBoxH int, c *createIssueState, lines []string) {
	totalScroll := max(1, len(lines)-descBoxH)
	barH := max(1, descBoxH*descBoxH/len(lines))
	barTop := descBoxY + c.descScroll*(descBoxH-barH)/totalScroll
	for i := 0; i < barH && barTop+i < descBoxY+descBoxH; i++ {
		screen.SetContent(lay.ox+2+lay.contentW-1, barTop+i, '▐', nil, tcell.StyleDefault.Foreground(T().Muted).Background(T().Bg))
	}
}

func drawCreateEpicSection(screen tcell.Screen, c *createIssueState, lay createLayout, cy int, labelStyle, activeLabelStyle, inputStyle, activeInputStyle, inputPlaceholder, activeBorder, sepStyle, bgStyle tcell.Style) int {
	if c.isSubtask() {
		return cy
	}
	lStyle := labelStyle
	if c.field == cfEpic {
		lStyle = activeLabelStyle
	}
	epicLabel := " Epic"
	if c.epicKey != "" && c.field != cfEpic {
		epicLabel = " Epic  ✓ " + c.epicKey + " — " + truncStr(c.epicName, lay.contentW-12)
	}
	drawText(screen, lay.ox+2, cy, epicLabel, lStyle, lay.contentW)
	cy++

	if c.field == cfEpic && c.epicKey == "" {
		drawInputField(screen, lay.ox+2, cy, lay.contentW, c.epicQuery, "Search epic…", true, c.epicCur, inputStyle, activeInputStyle, inputPlaceholder, activeBorder)
		cy++
		drawSep(screen, lay.ox+2, cy, lay.contentW, sepStyle)
		cy++

		items := c.filteredEpics()
		epicBtnY := lay.oy + lay.boxH - 2
		for i := range items {
			style := tcell.StyleDefault.Foreground(T().Fg).Background(T().Panel)
			prefix := "  "
			if i == c.epicSel {
				style = tcell.StyleDefault.Foreground(T().Fg).Background(T().CardSel).Bold(true)
				prefix = "▸ "
			}
			text := items[i].Key + "  " + truncStr(items[i].Summary, lay.contentW-10)
			fillRow(screen, lay.ox+1, cy, lay.boxW-2, style)
			drawText(screen, lay.ox+2, cy, prefix+text, style, lay.contentW)
			cy++
		}
		for cy < epicBtnY {
			fillRow(screen, lay.ox+1, cy, lay.boxW-2, bgStyle)
			cy++
		}
		if len(items) == 0 && c.epicQuery != "" {
			if !c.epicLoaded {
				drawText(screen, lay.ox+2, epicBtnY-2, "  Loading epics…", tcell.StyleDefault.Foreground(T().Cyan).Background(T().Panel), lay.contentW)
			} else {
				drawText(screen, lay.ox+2, epicBtnY-2, "  No epics found", inputPlaceholder, lay.contentW)
			}
		}
	} else if c.epicKey != "" {
		epicIcon, epicBg := typeStyle("Epic")
		epicBadgeStyle := tcell.StyleDefault.Foreground(T().BadgeFg).Background(epicBg).Bold(true)
		badge := " " + epicIcon + " " + truncStr(c.epicKey+" "+c.epicName, lay.contentW-4) + " "
		screen.SetContent(lay.ox+2, cy, '│', nil, tcell.StyleDefault.Foreground(T().Muted).Background(T().Bg))
		fillRow(screen, lay.ox+3, cy, lay.contentW-2, tcell.StyleDefault.Foreground(T().Fg).Background(T().Bg))
		if c.field == cfEpic {
			screen.SetContent(lay.ox+2, cy, '│', nil, activeBorder)
			fillRow(screen, lay.ox+3, cy, lay.contentW-2, activeInputStyle)
			screen.SetContent(lay.ox+2+lay.contentW-1, cy, '│', nil, activeBorder)
		}
		drawText(screen, lay.ox+3, cy, badge, epicBadgeStyle, lay.contentW-2)
		if c.field == cfEpic {
			drawText(screen, lay.ox+3+len([]rune(badge))+1, cy, "Ctrl+U clear", tcell.StyleDefault.Foreground(T().Muted).Background(T().CardSel), lay.contentW-2)
		}
		cy++
	} else {
		fillRow(screen, lay.ox+2, cy, lay.contentW, tcell.StyleDefault.Foreground(T().Muted).Background(T().Bg))
		drawText(screen, lay.ox+2, cy, "  None", inputPlaceholder, lay.contentW)
		cy++
	}
	return cy
}

func drawCreateButtons(screen tcell.Screen, c *createIssueState, lay createLayout, bgStyle, sepStyle, borderStyle tcell.Style) {
	btnY := lay.oy + lay.boxH - 2
	drawSep(screen, lay.ox+2, btnY-1, lay.contentW, sepStyle)
	fillRow(screen, lay.ox+1, btnY, lay.boxW-2, bgStyle)
	createStyle := tcell.StyleDefault.Foreground(T().BadgeFg).Background(T().Green).Bold(true)
	cancelStyle := tcell.StyleDefault.Foreground(T().Fg).Background(T().Muted)
	createText := " Create "
	cancelText := " Cancel "
	gap := 3
	totalBtnW := len([]rune(createText)) + gap + len([]rune(cancelText))
	btnX := lay.ox + (lay.boxW-totalBtnW)/2
	if c.field == cfButtons {
		if c.btnIdx == 0 {
			createStyle = tcell.StyleDefault.Foreground(T().BadgeFg).Background(T().Cyan).Bold(true)
		} else {
			cancelStyle = tcell.StyleDefault.Foreground(T().BadgeFg).Background(T().Cyan).Bold(true)
		}
	}
	drawText(screen, btnX, btnY, createText, createStyle, lay.contentW)
	btnX += len([]rune(createText)) + gap
	drawText(screen, btnX, btnY, cancelText, cancelStyle, lay.contentW)

	if c.creating {
		for row := lay.oy; row < lay.oy+lay.boxH; row++ {
			fillRow(screen, lay.ox, row, lay.boxW, tcell.StyleDefault.Foreground(T().Fg).Background(T().Panel).Dim(true))
		}
		drawBorder(screen, lay.ox, lay.oy, lay.boxW, lay.boxH, borderStyle.Dim(true))
		creatingStyle := tcell.StyleDefault.Foreground(T().Cyan).Background(T().Panel).Bold(true)
		drawText(screen, lay.ox+(lay.boxW-len([]rune(" Creating…")))/2, lay.oy+lay.boxH/2, " Creating…", creatingStyle, lay.boxW)
	}
}

// drawInputField draws a labeled input row with a left border indicator.
// curPos is the rune offset of the cursor within value (-1 means end of text).
// When cursor is within text, the character at cursor position is highlighted.
// When cursor is at end, a ▏ bar is drawn.
func drawInputField(screen tcell.Screen, x, y, w int, value, placeholder string, active bool, curPos int, style, activeStyle, placeholderStyle, activeBorder tcell.Style) {
	borderCh := '│'
	borderSt := tcell.StyleDefault.Foreground(T().Muted).Background(T().Bg)
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
		cursorSt := tcell.StyleDefault.Foreground(T().BadgeFg).Background(T().Cyan).Bold(true)
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