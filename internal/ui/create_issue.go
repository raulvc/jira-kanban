package ui

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"

	"github.com/raulvc/jira-kanban/internal/jira"
)

type createIssueState struct {
	formState
	projectKey string
	types      []jira.IssueType
	typeIdx    int
	cloneSrc   string
	creating   bool
}

var typeOrder = map[string]int{"Task": 0, "Bug": 1, "Epic": 2}

func typeStyle(name string) (icon string, bg tcell.Color) {
	switch name {
	case "Task":
		return "\u25B8", T().Cyan
	case "Bug":
		return "\u25CF", T().Red
	case "Epic":
		return "\u2696", T().Violet
	default:
		return "\u25CB", T().Blue
	}
}

func newCreateIssueState(projectKey string) *createIssueState {
	c := &createIssueState{
		projectKey: projectKey,
		types: []jira.IssueType{
			{ID: "", Name: "Task"},
			{ID: "", Name: "Bug"},
			{ID: "", Name: "Epic"},
		},
		typeIdx: 0,
	}
	c.field = ifType
	return c
}

func (c *createIssueState) isSubtask() bool { return c.parentKey != "" }

func (c *createIssueState) skipField(f issueField) bool {
	if f == ifLabels {
		return true
	}
	if c.isSubtask() && (f == ifEpic || f == ifType) {
		return true
	}
	return false
}

func (c *createIssueState) nextField() {
	for {
		c.field = (c.field + 1) % ifFieldCount
		if c.skipField(c.field) {
			continue
		}
		break
	}
	c.clampCur()
}

func (c *createIssueState) prevField() {
	for {
		c.field = (c.field - 1 + ifFieldCount) % ifFieldCount
		if c.skipField(c.field) {
			continue
		}
		break
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

// ── utility functions ──────────────────────────────────────────────────────

const epicMaxVis = 5

func clamp(v, hi int) int { return max(0, min(v, hi)) }

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

func sortEpicsByFreq(epics []jira.EpicItem, freq map[string]int) {
	sort.SliceStable(epics, func(i, j int) bool {
		return freq[epics[i].Summary] > freq[epics[j].Summary]
	})
}

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

func containsFold(s, q string) bool {
	return strings.Contains(foldLower(s), q)
}

// ── drawing ────────────────────────────────────────────────────────────────

const descVisH = 3
const descWrapW = 52

type modalLayout struct {
	ox, oy, boxW, boxH, contentW int
}

func drawCreateIssue(screen tcell.Screen, c *createIssueState, screenW, screenH int, _ string) {
	const padding = 2
	contentW := 54

	descExtra := descVisH - 1

	epicListH := 0
	epicSepRow := 0
	if !c.isSubtask() && c.field == ifEpic && c.epicKey == "" {
		epicListH = epicMaxVis
		epicSepRow = 1
	}
	labelsH := 0
	if len(c.labels) > 0 {
		labelsH = 2
	}
	const maxErrLines = 3
	errLines := 0
	if c.errMsg != "" {
		errLines = min(wrappedLineCount(c.errMsg, contentW-3), maxErrLines)
	}
	parentH := 0
	if c.isSubtask() {
		parentH = 3
	}
	contentH := 15 + descExtra + epicListH + epicSepRow + labelsH + errLines + parentH

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
	lay := modalLayout{ox, oy, boxW, boxH, contentW}

	styles := formStyles{
		bg:             tcell.StyleDefault.Foreground(T().Fg).Background(T().Panel),
		border:         tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel),
		title:          tcell.StyleDefault.Foreground(T().Blue).Background(T().Panel).Bold(true),
		label:          tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel),
		input:          tcell.StyleDefault.Foreground(T().Fg).Background(T().Bg),
		placeholder:    tcell.StyleDefault.Foreground(T().Muted).Background(T().Bg),
		activeLabel:    tcell.StyleDefault.Foreground(T().Cyan).Background(T().Panel).Bold(true),
		activeInput:    tcell.StyleDefault.Foreground(T().Fg).Background(T().CardSel),
		activeBorder:   tcell.StyleDefault.Foreground(T().Cyan).Background(T().Bg),
		err:            tcell.StyleDefault.Foreground(T().Red).Background(T().Panel).Bold(true),
		sep:            tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel).Dim(true),
	}

	for row := oy; row < oy+boxH; row++ {
		fillRow(screen, ox, row, boxW, styles.bg)
	}
	drawBorder(screen, ox, oy, boxW, boxH, styles.border)

	cy := oy + 1
	if c.cloneSrc != "" {
		if c.isSubtask() {
			drawText(screen, ox+padding, cy, " Clone Subtask "+c.cloneSrc+" → "+c.parentKey+": "+c.parentSummary+" ", styles.title, contentW)
		} else {
			drawText(screen, ox+padding, cy, " Clone "+c.cloneSrc+" — "+c.projectKey+" ", styles.title, contentW)
		}
	} else if c.isSubtask() {
		drawText(screen, ox+padding, cy, " Create Subtask — "+c.parentKey+": "+c.parentSummary+" ", styles.title, contentW)
	} else {
		drawText(screen, ox+padding, cy, " Create Issue — "+c.projectKey+" ", styles.title, contentW)
	}
	cy++
	drawSep(screen, ox+padding, cy, contentW, styles.sep)
	cy++

	if c.isSubtask() {
		cy = drawParentSection(screen, &c.formState, lay, cy, styles.label, styles.sep)
	}

	cy = drawTypeSection(screen, c, lay, cy, styles)
	cy = drawSummarySection(screen, &c.formState, lay, cy, styles)
	cy = drawDescSection(screen, &c.formState, lay, cy, styles)
	cy = drawEpicSection(screen, &c.formState, lay, cy, styles, styles.bg)
	cy = drawLabelsBadgeSection(screen, &c.formState, lay, cy, styles.label, styles.sep)

	if c.errMsg != "" {
		lines := wrapText(c.errMsg, contentW-3)
		for i := range min(len(lines), maxErrLines) {
			fillRow(screen, ox+1, cy, boxW-2, styles.bg)
			prefix := " ✗ "
			if i > 0 {
				prefix = "   "
			}
			drawText(screen, ox+padding, cy, prefix+lines[i], styles.err, contentW)
			cy++
		}
	}

	drawCreateButtons(screen, c, lay, styles)
}

func drawTypeSection(screen tcell.Screen, c *createIssueState, lay modalLayout, cy int, s formStyles) int {
	locked := c.isSubtask()
	lStyle := s.label
	if c.field == ifType && !locked {
		lStyle = s.activeLabel
	}
	drawText(screen, lay.ox+2, cy, " Type", lStyle, lay.contentW)
	cy++
	icon, typeBg := typeStyle(c.currentTypeName())
	bStyle := tcell.StyleDefault.Foreground(T().BadgeFg).Background(typeBg).Bold(true)
	badge := " " + icon + " " + c.currentTypeName() + " "
	badgeRowBg := tcell.StyleDefault.Foreground(T().Fg).Background(T().Bg)
	badgeRowBorderSt := tcell.StyleDefault.Foreground(T().Muted).Background(T().Bg)
	if c.field == ifType && !locked {
		badgeRowBg = tcell.StyleDefault.Foreground(T().Fg).Background(T().CardSel)
		badgeRowBorderSt = s.activeBorder
	}
	screen.SetContent(lay.ox+2, cy, '│', nil, badgeRowBorderSt)
	fillRow(screen, lay.ox+3, cy, lay.contentW-2, badgeRowBg)
	drawText(screen, lay.ox+3, cy, badge, bStyle, lay.contentW-2)
	if c.field == ifType && !locked {
		drawText(screen, lay.ox+3+len([]rune(badge))+1, cy, "\u2190/\u2192", tcell.StyleDefault.Foreground(T().Muted).Background(T().CardSel), lay.contentW-2)
		screen.SetContent(lay.ox+2+lay.contentW-1, cy, '│', nil, badgeRowBorderSt)
	}
	cy++
	drawSep(screen, lay.ox+2, cy, lay.contentW, s.sep)
	cy++
	return cy
}

func drawCreateButtons(screen tcell.Screen, c *createIssueState, lay modalLayout, s formStyles) {
	btnY := lay.oy + lay.boxH - 2
	drawSep(screen, lay.ox+2, btnY-1, lay.contentW, s.sep)
	fillRow(screen, lay.ox+1, btnY, lay.boxW-2, s.bg)
	createStyle := tcell.StyleDefault.Foreground(T().BadgeFg).Background(T().Green).Bold(true)
	cancelStyle := tcell.StyleDefault.Foreground(T().Fg).Background(T().Muted)
	createText := " Create "
	cancelText := " Cancel "
	gap := 3
	totalBtnW := len([]rune(createText)) + gap + len([]rune(cancelText))
	btnX := lay.ox + (lay.boxW-totalBtnW)/2
	if c.field == ifButtons {
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
		drawBorder(screen, lay.ox, lay.oy, lay.boxW, lay.boxH, s.border.Dim(true))
		creatingStyle := tcell.StyleDefault.Foreground(T().Cyan).Background(T().Panel).Bold(true)
		drawText(screen, lay.ox+(lay.boxW-len([]rune(" Creating…")))/2, lay.oy+lay.boxH/2, " Creating…", creatingStyle, lay.boxW)
	}
}

// ── shared drawing helpers ────────────────────────────────────────────────

type formStyles struct {
	bg           tcell.Style
	border       tcell.Style
	title        tcell.Style
	label        tcell.Style
	input        tcell.Style
	placeholder  tcell.Style
	activeLabel  tcell.Style
	activeInput  tcell.Style
	activeBorder tcell.Style
	err          tcell.Style
	sep          tcell.Style
}

func drawParentSection(screen tcell.Screen, f *formState, lay modalLayout, cy int, labelStyle, sepStyle tcell.Style) int {
	drawText(screen, lay.ox+2, cy, " Parent", labelStyle, lay.contentW)
	cy++
	pBadge := " " + f.parentKey + ": " + truncStr(f.parentSummary, lay.contentW-6-len([]rune(f.parentKey))) + " "
	pStyle := tcell.StyleDefault.Foreground(T().BadgeFg).Background(assigneeColor(f.parentKey)).Bold(true)
	screen.SetContent(lay.ox+2, cy, '│', nil, tcell.StyleDefault.Foreground(T().Muted).Background(T().Bg))
	fillRow(screen, lay.ox+3, cy, lay.contentW-2, tcell.StyleDefault.Foreground(T().Fg).Background(T().Bg))
	drawText(screen, lay.ox+3, cy, pBadge, pStyle, lay.contentW-2)
	cy++
	drawSep(screen, lay.ox+2, cy, lay.contentW, sepStyle)
	cy++
	return cy
}

func drawSummarySection(screen tcell.Screen, f *formState, lay modalLayout, cy int, s formStyles) int {
	lStyle := s.label
	if f.field == ifSummary {
		lStyle = s.activeLabel
	}
	drawText(screen, lay.ox+2, cy, " Summary", lStyle, lay.contentW)
	cy++
	drawInputField(screen, lay.ox+2, cy, lay.contentW, f.summary, "Issue title…", f.field == ifSummary, f.sumCur, s.input, s.activeInput, s.placeholder, s.activeBorder)
	cy++
	drawSep(screen, lay.ox+2, cy, lay.contentW, s.sep)
	cy++
	return cy
}

func drawDescSection(screen tcell.Screen, f *formState, lay modalLayout, cy int, s formStyles) int {
	lStyle := s.label
	if f.field == ifDescription {
		lStyle = s.activeLabel
	}
	drawText(screen, lay.ox+2, cy, " Description", lStyle, lay.contentW)
	cy++

	descBoxY := cy
	descBoxH := descVisH
	for range descBoxH {
		screen.SetContent(lay.ox+2, cy, '│', nil, tcell.StyleDefault.Foreground(T().Muted).Background(T().Bg))
		fillRow(screen, lay.ox+3, cy, lay.contentW-1, s.input)
		cy++
	}

	lines := descLines(f.desc)
	maxScroll := max(0, len(lines)-descBoxH)
	f.descScroll = clamp(f.descScroll, maxScroll)

	for i := range descBoxH {
		lineIdx := f.descScroll + i
		row := descBoxY + i
		screen.SetContent(lay.ox+2, row, '│', nil, tcell.StyleDefault.Foreground(T().Muted).Background(T().Bg))
		fillRow(screen, lay.ox+3, row, lay.contentW-1, s.input)
		if f.field == ifDescription {
			screen.SetContent(lay.ox+2, row, '│', nil, s.activeBorder)
			fillRow(screen, lay.ox+3, row, lay.contentW-1, s.activeInput)
		}

		if lineIdx < len(lines) {
			st := s.input
			if f.field == ifDescription {
				st = s.activeInput
			}
			drawText(screen, lay.ox+3, row, lines[lineIdx], st, lay.contentW-2)
		} else if lineIdx == len(lines) && f.field == ifDescription && f.desc == "" {
			drawText(screen, lay.ox+3, row, "Optional, Enter for newline…", s.placeholder, lay.contentW-2)
		}
	}

	if f.field == ifDescription && f.desc != "" {
		drawFormDescCursor(screen, f, lay, descBoxY, descBoxH, lines, s.activeInput)
	}

	if f.field == ifDescription && len(lines) > descBoxH {
		drawFormDescScrollbar(screen, lay, descBoxY, descBoxH, f, lines)
	}

	return descBoxY + descBoxH
}

func drawEpicSection(screen tcell.Screen, f *formState, lay modalLayout, cy int, s formStyles, bgStyle tcell.Style) int {
	if f.parentKey != "" && !f.parentEpic {
		return cy
	}
	lStyle := s.label
	if f.field == ifEpic {
		lStyle = s.activeLabel
	}
	epicLabel := " Epic"
	if f.epicKey != "" && f.field != ifEpic {
		epicLabel = " Epic  ✓ " + f.epicKey + " — " + truncStr(f.epicName, lay.contentW-12)
	}
	drawText(screen, lay.ox+2, cy, epicLabel, lStyle, lay.contentW)
	cy++

	if f.field == ifEpic && f.epicKey == "" {
		drawInputField(screen, lay.ox+2, cy, lay.contentW, f.epicQuery, "Search epic…", true, f.epicCur, s.input, s.activeInput, s.placeholder, s.activeBorder)
		cy++
		drawSep(screen, lay.ox+2, cy, lay.contentW, s.sep)
		cy++

		items := f.filteredEpics()
		epicBtnY := lay.oy + lay.boxH - 2
		for i := range items {
			style := tcell.StyleDefault.Foreground(T().Fg).Background(T().Panel)
			prefix := "  "
			if i == f.epicSel {
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
		if len(items) == 0 && f.epicQuery != "" {
			if !f.epicLoaded {
				drawText(screen, lay.ox+2, epicBtnY-2, "  Loading epics…", tcell.StyleDefault.Foreground(T().Cyan).Background(T().Panel), lay.contentW)
			} else {
				drawText(screen, lay.ox+2, epicBtnY-2, "  No epics found", s.placeholder, lay.contentW)
			}
		}
	} else if f.epicKey != "" {
		epicIcon, epicBg := typeStyle("Epic")
		epicBadgeStyle := tcell.StyleDefault.Foreground(T().BadgeFg).Background(epicBg).Bold(true)
		badge := " " + epicIcon + " " + truncStr(f.epicKey+" "+f.epicName, lay.contentW-4) + " "
		screen.SetContent(lay.ox+2, cy, '│', nil, tcell.StyleDefault.Foreground(T().Muted).Background(T().Bg))
		fillRow(screen, lay.ox+3, cy, lay.contentW-2, tcell.StyleDefault.Foreground(T().Fg).Background(T().Bg))
		if f.field == ifEpic {
			screen.SetContent(lay.ox+2, cy, '│', nil, s.activeBorder)
			fillRow(screen, lay.ox+3, cy, lay.contentW-2, s.activeInput)
			screen.SetContent(lay.ox+2+lay.contentW-1, cy, '│', nil, s.activeBorder)
		}
		drawText(screen, lay.ox+3, cy, badge, epicBadgeStyle, lay.contentW-2)
		if f.field == ifEpic {
			drawText(screen, lay.ox+3+len([]rune(badge))+1, cy, "Ctrl+U clear", tcell.StyleDefault.Foreground(T().Muted).Background(T().CardSel), lay.contentW-2)
		}
		cy++
	} else {
		fillRow(screen, lay.ox+2, cy, lay.contentW, tcell.StyleDefault.Foreground(T().Muted).Background(T().Bg))
		drawText(screen, lay.ox+2, cy, "  None", s.placeholder, lay.contentW)
		cy++
	}
	drawSep(screen, lay.ox+2, cy, lay.contentW, s.sep)
	cy++
	return cy
}

func drawLabelsBadgeSection(screen tcell.Screen, f *formState, lay modalLayout, cy int, labelStyle, sepStyle tcell.Style) int {
	if len(f.labels) == 0 {
		return cy
	}
	drawText(screen, lay.ox+2, cy, " Labels", labelStyle, lay.contentW)
	cy++
	lx := lay.ox + 2
	for _, label := range f.labels {
		ls := tcell.StyleDefault.Foreground(labelColor(label)).Background(T().Panel)
		text := " " + label + " "
		drawn := drawText(screen, lx, cy, text, ls, lay.contentW-(lx-lay.ox-2))
		lx += drawn + 1
		if lx-lay.ox-2 >= lay.contentW {
			break
		}
	}
	cy++
	drawSep(screen, lay.ox+2, cy, lay.contentW, sepStyle)
	cy++
	return cy
}

func drawLabelsEditSection(screen tcell.Screen, f *formState, lay modalLayout, cy int, s formStyles) int {
	lStyle := s.label
	if f.field == ifLabels {
		lStyle = s.activeLabel
	}
	drawText(screen, lay.ox+2, cy, " Labels", lStyle, lay.contentW)
	cy++
	lx := lay.ox + 2
	for _, label := range f.labels {
		ls := tcell.StyleDefault.Foreground(labelColor(label)).Background(T().Panel)
		text := " " + label + " "
		drawn := drawText(screen, lx, cy, text, ls, lay.contentW-(lx-lay.ox-2))
		lx += drawn + 1
		if lx-lay.ox-2 >= lay.contentW {
			break
		}
	}
	if f.field == ifLabels {
		drawInputFieldInline(screen, lx, cy, f.labelInput, "add…", f.labelCur, true, s.input, s.activeInput, s.placeholder, lay.contentW-(lx-lay.ox-2))
	}
	cy++
	drawSep(screen, lay.ox+2, cy, lay.contentW, s.sep)
	cy++
	return cy
}

func drawFormDescCursor(screen tcell.Screen, f *formState, lay modalLayout, descBoxY, descBoxH int, lines []string, activeInputStyle tcell.Style) {
	cursorSt := tcell.StyleDefault.Foreground(T().BadgeFg).Background(T().Cyan).Bold(true)
	curLine := f.descCurLine(lines)
	offsets := descLineOffsets(f.desc, lines)
	curCol := f.descCur - offsets[curLine]
	lineRunes := len([]rune(lines[curLine]))
	if curCol > lineRunes {
		curCol = lineRunes
	}
	if curCol < 0 {
		curCol = 0
	}
	curRow := descBoxY + curLine - f.descScroll
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

func drawFormDescScrollbar(screen tcell.Screen, lay modalLayout, descBoxY, descBoxH int, f *formState, lines []string) {
	totalScroll := max(1, len(lines)-descBoxH)
	barH := max(1, descBoxH*descBoxH/len(lines))
	barTop := descBoxY + f.descScroll*(descBoxH-barH)/totalScroll
	for i := 0; i < barH && barTop+i < descBoxY+descBoxH; i++ {
		screen.SetContent(lay.ox+2+lay.contentW-1, barTop+i, '▐', nil, tcell.StyleDefault.Foreground(T().Muted).Background(T().Bg))
	}
}

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
	scrollStart := 0
	if curPos >= visW {
		scrollStart = curPos - visW + 1
	}
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
		ch := runes[curPos]
		cursorSt := tcell.StyleDefault.Foreground(T().BadgeFg).Background(T().Cyan).Bold(true)
		screen.SetContent(x+1+cursorCol, y, ch, nil, cursorSt)
	} else {
		screen.SetContent(x+1+cursorCol, y, '▏', nil, inputSt)
	}
}

func drawInputFieldInline(screen tcell.Screen, x, y int, text, placeholder string, cur int, active bool, style, activeStyle, placeholderStyle tcell.Style, maxW int) {
	if active {
		fillRow(screen, x, y, maxW, activeStyle)
		runes := []rune(text)
		visible := min(len(runes), maxW-2)
		offset := 0
		if cur > visible {
			offset = cur - visible
		}
		drawText(screen, x+1, y, string(runes[offset:offset+visible]), activeStyle, maxW-1)
		cursorX := x + 1 + cur - offset
		if cur < len(runes) {
			screen.SetContent(cursorX, y, runes[cur], nil, tcell.StyleDefault.Foreground(T().BadgeFg).Background(T().Cyan).Bold(true))
		} else {
			screen.SetContent(cursorX, y, '▏', nil, activeStyle)
		}
	} else {
		if text == "" {
			drawText(screen, x, y, placeholder, placeholderStyle, maxW)
		} else {
			drawText(screen, x, y, text, style, maxW)
		}
	}
}

func drawSep(screen tcell.Screen, x, y, w int, style tcell.Style) {
	line := strings.Repeat("─", w)
	drawText(screen, x, y, line, style, w)
}

// ── handlers ────────────────────────────────────────────────────────────────

func startCreateOrClone(ctx *appContext, clone bool) {
	if ctx.state.createIssue != nil || ctx.state.projectKey == "" {
		return
	}
	if clone {
		if card := ctx.state.selectedCard(); card != nil {
			openCloneIssue(ctx, card.Key)
		}
		return
	}
	openCreateIssue(ctx)
}

func openCreateIssue(ctx *appContext) {
	epicFreq := make(map[string]int)
	for _, col := range ctx.state.data.Columns {
		for _, card := range col.Issues {
			if card.Epic != "" {
				epicFreq[card.Epic]++
			}
		}
	}

	c := newCreateIssueState(ctx.state.projectKey)
	c.epicFreq = epicFreq
	ctx.state.createIssue = c

	go func() {
		types, err := ctx.client.GetIssueTypes(c.projectKey)
		ctx.app.QueueUpdateDraw(func() {
			if ctx.state.createIssue != c {
				return
			}
			if err != nil {
				c.errMsg = err.Error(); slog.Error("create issue failed", "error", err)
				return
			}
			if len(types) > 0 {
				sortTypesByPriority(types)
				c.types = types
				c.typeIdx = 0
			}
		})
	}()

	go func() {
		epics, err := ctx.client.SearchEpics(c.projectKey, "")
		ctx.app.QueueUpdateDraw(func() {
			if ctx.state.createIssue != c {
				return
			}
			if err != nil {
				return
			}
			sortEpicsByFreq(epics, epicFreq)
			c.epics = epics
			c.epicLoaded = true
		})
	}()
}

func openCreateSubtask(ctx *appContext, parentKey string, parentSummary string) {
	c := newCreateIssueState(ctx.state.projectKey)
	c.parentKey = parentKey
	c.parentSummary = parentSummary
	c.field = ifSummary
	ctx.state.createIssue = c

	go func() {
		types, err := ctx.client.GetSubtaskTypes(c.projectKey)
		ctx.app.QueueUpdateDraw(func() {
			if ctx.state.createIssue != c {
				return
			}
			if err != nil {
				c.errMsg = err.Error(); slog.Error("create issue failed", "error", err)
				return
			}
			if len(types) > 0 {
				sortTypesByPriority(types)
				c.types = types
				c.typeIdx = 0
			}
		})
	}()
}

func openCloneIssue(ctx *appContext, key string) {
	epicFreq := make(map[string]int)
	for _, col := range ctx.state.data.Columns {
		for _, c := range col.Issues {
			if c.Epic != "" {
				epicFreq[c.Epic]++
			}
		}
	}

	c := newCreateIssueState(ctx.state.projectKey)
	c.cloneSrc = key
	c.epicFreq = epicFreq
	ctx.state.createIssue = c

	go func() {
		full, err := ctx.client.GetIssue(key)
		ctx.app.QueueUpdateDraw(func() {
			if ctx.state.createIssue != c {
				return
			}
			if err != nil {
				c.errMsg = err.Error(); slog.Error("clone issue failed", "error", err)
				return
			}
			c.desc = full.Description
			c.descCur = len([]rune(c.desc))
			c.labels = full.Labels
			if full.ParentKey != "" {
				c.parentKey = full.ParentKey
				c.parentSummary = full.ParentSummary
			}
			if full.Epic != "" {
				c.epicName = full.Epic
			}
		})

		isSub := full.ParentKey != "" && !full.ParentIsEpic
		typeFetcher := ctx.client.GetIssueTypes
		if isSub {
			typeFetcher = ctx.client.GetSubtaskTypes
		}
		types, tErr := typeFetcher(c.projectKey)
		ctx.app.QueueUpdateDraw(func() {
			if ctx.state.createIssue != c {
				return
			}
			if tErr != nil {
				c.errMsg = tErr.Error(); slog.Error("clone issue failed", "error", tErr)
				return
			}
			if len(types) > 0 {
				sortTypesByPriority(types)
				c.types = types
				c.typeIdx = 0
			}
		})

		if !isSub {
			epics, eErr := ctx.client.SearchEpics(c.projectKey, "")
			ctx.app.QueueUpdateDraw(func() {
				if ctx.state.createIssue != c {
					return
				}
				if eErr != nil {
					return
				}
				sortEpicsByFreq(epics, epicFreq)
				c.epics = epics
				c.epicLoaded = true
				if c.epicName != "" {
					for _, e := range epics {
						if e.Summary == c.epicName || e.Key == c.epicName {
							c.epicKey = e.Key
							c.epicName = e.Summary
							break
						}
					}
				}
			})
		}
	}()
}

func handleCreateIssueInput(ctx *appContext, event *tcell.EventKey) *tcell.EventKey {
	c := ctx.state.createIssue
	if c.creating {
		return nil
	}
	switch event.Key() {
	case tcell.KeyEscape:
		c.stopDebounce()
		ctx.state.createIssue = nil
		return nil
	case tcell.KeyTab:
		c.nextField()
		c.clampCur()
		return nil
	case tcell.KeyBacktab:
		c.prevField()
		c.clampCur()
		return nil
	case tcell.KeyUp:
		handleCreateUp(c)
		return nil
	case tcell.KeyDown:
		handleCreateDown(c)
		return nil
	case tcell.KeyLeft:
		handleCreateLeft(c)
		return nil
	case tcell.KeyRight:
		handleCreateRight(c)
		return nil
	case tcell.KeyEnter:
		return handleCreateEnter(ctx, c)
	case tcell.KeyCtrlU:
		if c.field == ifEpic {
			c.epicKey = ""
			c.epicName = ""
			c.epicQuery = ""
			c.epicCur = 0
			c.epicSel = 0
		}
		return nil
	case tcell.KeyHome:
		handleCreateHome(c)
		return nil
	case tcell.KeyEnd:
		handleCreateEnd(c)
		return nil
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		c.backspace()
		return nil
	case tcell.KeyDelete:
		c.deleteForward()
		return nil
	case tcell.KeyRune:
		if c.field == ifButtons {
			return nil
		}
		c.typeRune(event.Rune())
		if c.field == ifEpic {
			startEpicSearch(ctx, c)
		}
		return nil
	}
	return nil
}

func handleCreateUp(c *createIssueState) {
	if c.field == ifButtons {
		if c.isSubtask() {
			c.field = ifDescription
		} else {
			c.field = ifEpic
		}
		c.clampCur()
		return
	}
	if c.field == ifEpic && c.epicKey == "" {
		if c.epicSel > 0 {
			c.epicSel--
		} else {
			c.field = ifDescription
			c.clampCur()
		}
	} else if c.field == ifDescription {
		lines := descLines(c.desc)
		curLine := c.descCurLine(lines)
		if curLine > 0 {
			c.descMoveUp(lines)
			c.descAutoScroll()
		} else if c.descScroll > 0 {
			c.descScroll--
		} else {
			c.field = ifSummary
			c.clampCur()
		}
	} else {
		c.prevField()
		c.clampCur()
	}
}

func handleCreateDown(c *createIssueState) {
	if c.field == ifButtons {
		if c.isSubtask() {
			c.field = ifSummary
		} else {
			c.field = ifType
		}
		c.clampCur()
		return
	}
	if c.field == ifEpic && c.epicKey == "" {
		items := c.filteredEpics()
		if c.epicSel < len(items)-1 {
			c.epicSel++
		} else {
			c.field = ifButtons
			c.clampCur()
		}
	} else if c.field == ifDescription {
		lines := descLines(c.desc)
		curLine := c.descCurLine(lines)
		if curLine < len(lines)-1 {
			c.descMoveDown(lines)
			c.descAutoScroll()
		} else if c.descScroll < max(0, len(lines)-descVisH) {
			c.descScroll++
		} else {
			c.nextField()
			c.clampCur()
		}
	} else {
		c.nextField()
		c.clampCur()
	}
}

func handleCreateLeft(c *createIssueState) {
	if c.field == ifButtons {
		c.btnIdx = 0
		return
	}
	if c.field == ifType && !c.isSubtask() {
		c.cycleType(-1)
		return
	}
	c.moveLeft()
}

func handleCreateRight(c *createIssueState) {
	if c.field == ifButtons {
		c.btnIdx = 1
		return
	}
	if c.field == ifType && !c.isSubtask() {
		c.cycleType(1)
		return
	}
	c.moveRight()
}

func handleCreateEnter(ctx *appContext, c *createIssueState) *tcell.EventKey {
	if c.field == ifButtons {
		if c.btnIdx == 0 {
			executeCreateIssue(ctx)
		} else {
			c.stopDebounce()
			ctx.state.createIssue = nil
		}
		return nil
	}
	if c.field == ifEpic {
		c.handleEpicEnter()
		return nil
	}
	if c.field == ifDescription {
		c.handleNewline()
		return nil
	}
	executeCreateIssue(ctx)
	return nil
}

func handleCreateHome(c *createIssueState) { c.moveHome() }

func handleCreateEnd(c *createIssueState)   { c.moveEnd() }

func startEpicSearch(ctx *appContext, c *createIssueState) {
	if c.debounce != nil {
		c.debounce.Stop()
	}

	if c.epicLoaded {
		return
	}

	projectKey := c.projectKey
	epicFreq := c.epicFreq
	c.debounce = time.AfterFunc(150*time.Millisecond, func() {
		epics, err := ctx.client.SearchEpics(projectKey, "")
		ctx.app.QueueUpdateDraw(func() {
			if ctx.state.createIssue != c {
				return
			}
			if err != nil {
				c.errMsg = err.Error(); slog.Error("create issue failed", "error", err)
				return
			}
			sortEpicsByFreq(epics, epicFreq)
			c.epics = epics
			c.epicLoaded = true
			if c.epicSel >= len(c.filteredEpics()) {
				c.epicSel = 0
			}
		})
	})
}

func executeCreateIssue(ctx *appContext) {
	c := ctx.state.createIssue
	if c.summary == "" {
		c.errMsg = "Summary is required"
		return
	}
	typeID := c.currentTypeID()
	if typeID == "" {
		c.errMsg = "Issue type not loaded yet"
		return
	}
	c.creating = true
	c.errMsg = ""
	summary := c.summary
	desc := c.desc
	pk := c.projectKey

	go func() {
		var result jira.CreateIssueResult
		var err error
		epicKey := c.epicKey
		labels := c.labels
		if c.isSubtask() {
			result, err = ctx.client.CreateSubtask(pk, typeID, summary, desc, c.parentKey, labels)
		} else {
			result, err = ctx.client.CreateIssue(pk, typeID, summary, desc, labels)
		}
		if err == nil && epicKey != "" {
			if linkErr := ctx.client.LinkEpic(result.Key, epicKey); linkErr != nil {
				slog.Warn("failed to link epic after create", "key", result.Key, "epic", epicKey, "error", linkErr)
			}
		}
		ctx.app.QueueUpdateDraw(func() {
			if ctx.state.createIssue != c {
				return
			}
			if err != nil {
				c.creating = false
				c.errMsg = err.Error(); slog.Error("create issue failed", "error", err)
				return
			}
			ctx.state.createIssue = nil
			ctx.state.statusMsg = fmt.Sprintf(" Created %s", result.Key)
			key := result.Key
			ctx.state.pendingSelect = key
			ctx.state.detail = &detailState{
				card: jira.Card{
					Key:     key,
					Summary: summary,
				},
				loading: true,
			}
			go func() {
				full, err := ctx.client.GetIssue(key)
				ctx.app.QueueUpdateDraw(func() {
					d := ctx.state.detail
					if d == nil || d.card.Key != key {
						return
					}
					if err != nil {
						d.loading = false
						d.err = err.Error(); slog.Error("detail load failed", "key", d.card.Key, "error", err)
						return
					}
					d.card = full
					d.loading = false

					injectCreatedIssue(ctx, full)
				})
			}()
			startSync(ctx)
		})
	}()
}