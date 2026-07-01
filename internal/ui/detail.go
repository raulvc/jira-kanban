package ui

import (
	"log/slog"

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

	subDetail        *detailState
	selectedSubtask   int
	subtaskScrollY    int // y-coordinate of first subtask row (for highlight)
	isSubDetail       bool
}

type detailLayout struct {
	ox, oy, boxW, boxH int
	contentW, padding   int
	maxCW              int
	bgStyle             tcell.Style
	borderStyle         tcell.Style
	titleStyle          tcell.Style
	keyStyle            tcell.Style
	valueStyle          tcell.Style
	descStyle           tcell.Style
	loadingStyle        tcell.Style
	errStyle            tcell.Style
}

// drawDetailModal renders a centered issue detail overlay over the board.
func drawDetailModal(screen tcell.Screen, d *detailState, screenW, screenH int) {
	const padding = 2
	contentW := min(screenW-6, 120)
	contentH := detailContentHeight(d, contentW)

	boxW := contentW + padding*2
	boxH := contentH + padding + 2
	if boxW > screenW-4 {
		boxW = screenW - 4
		contentW = boxW - padding*2
	}
	if boxH > screenH-4 {
		boxH = screenH - 4
	}

	ox := (screenW - boxW) / 2
	oy := (screenH - boxH) / 2
	maxCW := contentW

	l := detailLayout{
		ox: ox, oy: oy, boxW: boxW, boxH: boxH,
		contentW: contentW, padding: padding, maxCW: maxCW,
		bgStyle:      tcell.StyleDefault.Foreground(T().Fg).Background(T().Panel),
		borderStyle:  tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel),
		titleStyle:   tcell.StyleDefault.Foreground(T().Blue).Background(T().Panel).Bold(true),
		keyStyle:     tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel),
		valueStyle:   tcell.StyleDefault.Foreground(T().Fg).Background(T().Panel),
		descStyle:    tcell.StyleDefault.Foreground(T().Fg).Background(T().Panel),
		loadingStyle: tcell.StyleDefault.Foreground(T().Cyan).Background(T().Panel),
		errStyle:     tcell.StyleDefault.Foreground(T().Red).Background(T().Panel).Bold(true),
	}

	for row := oy; row < oy+boxH; row++ {
		fillRow(screen, ox, row, boxW, l.bgStyle)
	}

	drawBorder(screen, ox, oy, boxW, boxH, l.borderStyle)

	closeY := oy + boxH - 2
	closeStyle := tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel)
	closeText := " Esc/q close • e edit • a assign • t transition • c subtask • C clone • y copy key • ^Y copy url "
	if d.isSubDetail {
		closeText = " Esc/q back "
	}
	drawText(screen, ox+(boxW-len([]rune(closeText)))/2, closeY, closeText, closeStyle, boxW)

	cy := oy + 1
	cy = drawDetailHeader(screen, d, &l, cy)

	hasSubtasks := len(d.card.Subtasks) > 0
	if hasSubtasks {
		cy = drawDetailSubtasks(screen, d, &l, cy)
	}

	cy++
	drawDetailDescription(screen, d, &l, cy)

	if d.subDetail != nil {
		drawDetailModal(screen, d.subDetail, screenW, screenH)
	}
}

func detailContentHeight(d *detailState, contentW int) int {
	contentH := 0
	contentH++ // key + summary line
	contentH++ // blank
	contentH++ // status
	contentH++ // assignee
	if d.card.ParentKey != "" && !d.card.ParentIsEpic {
		contentH++
	}
	if d.card.Epic != "" {
		contentH++
	}
	if len(d.card.Labels) > 0 {
		contentH++
	}
	if len(d.card.Subtasks) > 0 {
		contentH += 2 + len(d.card.Subtasks)
	}
	contentH++
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
	return max(contentH, 6)
}

func drawDetailHeader(screen tcell.Screen, d *detailState, l *detailLayout, cy int) int {
	titleText := d.card.Key + "  "
	drawText(screen, l.ox+l.padding, cy, titleText, l.titleStyle, l.maxCW)
	remaining := l.maxCW - len([]rune(titleText))
	if remaining > 0 {
		drawText(screen, l.ox+l.padding+len([]rune(titleText)), cy, truncStr(d.card.Summary, remaining), l.valueStyle, remaining)
	}
	cy++
	cy++

	drawText(screen, l.ox+l.padding, cy, "Status: ", l.keyStyle, l.maxCW)
	drawText(screen, l.ox+l.padding+8, cy, d.card.Status, statusColor(d.card.Status), l.maxCW-8)
	cy++

	assigneeText := d.card.Assignee
	if assigneeText == "" {
		assigneeText = "Unassigned"
	}
	drawText(screen, l.ox+l.padding, cy, "Assignee: ", l.keyStyle, l.maxCW)
	assigneeStyle := l.valueStyle
	if d.card.Assignee != "" {
		assigneeStyle = tcell.StyleDefault.Foreground(assigneeColor(d.card.Assignee)).Background(T().Panel)
	}
	drawText(screen, l.ox+l.padding+10, cy, assigneeText, assigneeStyle, l.maxCW-10)
	cy++

	if d.card.ParentKey != "" && !d.card.ParentIsEpic {
		parentText := truncStr(d.card.ParentKey+": "+d.card.ParentSummary, l.maxCW-8)
		drawText(screen, l.ox+l.padding, cy, "Parent: ", l.keyStyle, l.maxCW)
		pStyle := tcell.StyleDefault.Foreground(T().BadgeFg).Background(assigneeColor(d.card.ParentKey)).Bold(true)
		drawText(screen, l.ox+l.padding+8, cy, " "+parentText+" ", pStyle, l.maxCW-8)
		cy++
	}

	if d.card.Epic != "" {
		drawText(screen, l.ox+l.padding, cy, "Epic: ", l.keyStyle, l.maxCW)
		epBadge := " " + truncStr(d.card.Epic, l.maxCW-8) + " "
		epStyle := tcell.StyleDefault.Foreground(T().BadgeFg).Background(epicColor(d.card.Epic)).Bold(true)
		drawText(screen, l.ox+l.padding+6, cy, epBadge, epStyle, l.maxCW-6)
		cy++
	}

	if len(d.card.Labels) > 0 {
		drawText(screen, l.ox+l.padding, cy, "Labels: ", l.keyStyle, l.maxCW)
		lx := l.ox + l.padding + 8
		for _, label := range d.card.Labels {
			ls := tcell.StyleDefault.Foreground(labelColor(label)).Background(T().Panel)
			text := " " + label + " "
			drawn := drawText(screen, lx, cy, text, ls, l.maxCW-(lx-l.ox-l.padding))
			lx += drawn + 1
			if lx-l.ox-l.padding >= l.maxCW {
				break
			}
		}
		cy++
	}

	return cy
}

func drawDetailSubtasks(screen tcell.Screen, d *detailState, l *detailLayout, cy int) int {
	drawText(screen, l.ox+l.padding, cy, "Subtasks", l.titleStyle, l.maxCW)
	cy += 2

	d.subtaskScrollY = cy
	for i, st := range d.card.Subtasks {
		sx := l.ox + l.padding

		rowBg := T().Panel
		if i == d.selectedSubtask {
			rowBg = T().CardSel
		}
		rowStyle := tcell.StyleDefault.Background(rowBg)

		for dx := 0; dx < l.contentW; dx++ {
			screen.SetContent(l.ox+l.padding+dx, cy, ' ', nil, rowStyle)
		}

		icon, iconColor := subtaskIcon(st.Status)
		iconStyle := tcell.StyleDefault.Foreground(iconColor).Background(rowBg)
		drawText(screen, sx, cy, icon, iconStyle, 1)

		sx += 2
		keyStyle := tcell.StyleDefault.Foreground(T().Blue).Background(rowBg)
		drawText(screen, sx, cy, st.Key, keyStyle, l.maxCW-2)

		sx += len([]rune(st.Key)) + 1
		avail := l.maxCW - (sx - l.ox - l.padding)
		valStyle := tcell.StyleDefault.Foreground(T().Fg).Background(rowBg)
		if avail > 0 {
			drawText(screen, sx, cy, truncStr(st.Summary, avail), valStyle, avail)
		}

		if st.Assignee != "" {
			aText := st.Assignee
			aLen := len([]rune(aText))
			aStyle := tcell.StyleDefault.Foreground(assigneeColor(st.Assignee)).Background(rowBg)
			aStartX := l.ox + l.padding + l.maxCW - aLen
			if aStartX > sx {
				drawText(screen, aStartX, cy, aText, aStyle, aLen)
			}
		}

		cy++
	}
	return cy
}

func subtaskIcon(status string) (string, tcell.Color) {
	switch status {
	case "Done":
		return "✓", T().Green
	case "In Progress":
		return "●", T().Yellow
	case "In Review":
		return "◆", T().Cyan
	default:
		return "○", T().Muted
	}
}

func drawDetailDescription(screen tcell.Screen, d *detailState, l *detailLayout, cy int) {
	descAreaTop := cy
	descAreaBot := l.oy + l.boxH - 3
	descAreaH := descAreaBot - descAreaTop
	d.viewH = descAreaH

	if d.loading {
		drawText(screen, l.ox+l.padding, cy, "Loading description…", l.loadingStyle, l.maxCW)
		d.maxScroll = 0
		return
	}
	if d.err != "" {
		drawText(screen, l.ox+l.padding, cy, truncStr(d.err, l.maxCW), l.errStyle, l.maxCW)
		d.maxScroll = 0
		return
	}
	if d.card.Description == "" {
		drawText(screen, l.ox+l.padding, cy, "No description", tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel), l.maxCW)
		d.maxScroll = 0
		return
	}

	descW := l.contentW - 2
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
	drawRichWrappedText(screen, segs, l.ox+l.padding, descAreaTop, descW, descAreaBot, d.scroll, l.descStyle)
	if needsScrollbar {
		trackStyle := tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel)
		thumbStyle := tcell.StyleDefault.Foreground(T().Blue).Background(T().Panel)
		drawScrollbar(screen, l.ox+l.padding+descW+1, descAreaTop, descAreaH, d.scroll, totalDescLines, trackStyle, thumbStyle)
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
		return tcell.StyleDefault.Foreground(T().Green).Background(T().Panel).Bold(true)
	case "In Progress", "In Review":
		return tcell.StyleDefault.Foreground(T().Cyan).Background(T().Panel).Bold(true)
	case "To Do", "Open":
		return tcell.StyleDefault.Foreground(T().Yellow).Background(T().Panel).Bold(true)
	default:
		return tcell.StyleDefault.Foreground(T().Fg).Background(T().Panel).Bold(true)
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
		return tcell.StyleDefault.Foreground(T().Cyan).Background(T().Panel).Underline(true)
	case jira.DsCode:
		return tcell.StyleDefault.Foreground(T().Muted).Background(T().Bg).Dim(true)
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
// ── handlers ────────────────────────────────────────────────────────────────

func openIssueDetail(ctx *appContext) {
	card := ctx.state.selectedCard()
	if card == nil {
		return
	}
	key := card.Key
	ctx.state.detail = &detailState{
		card:    *card,
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
		})
	}()
}

func handleDetailInput(ctx *appContext, event *tcell.EventKey) *tcell.EventKey {
	d := ctx.state.detail

	if d.subDetail != nil {
		return handleSubDetailInput(ctx, d.subDetail, event)
	}

	hasSubtasks := len(d.card.Subtasks) > 0

	switch event.Key() {
	case tcell.KeyEscape:
		ctx.state.detail = nil
		return nil
	case tcell.KeyRune:
		return handleDetailRune(ctx, d, event)
	case tcell.KeyUp:
		handleDetailUp(d, hasSubtasks)
		return nil
	case tcell.KeyDown:
		handleDetailDown(d, hasSubtasks)
		return nil
	case tcell.KeyEnter:
		if hasSubtasks && d.selectedSubtask < len(d.card.Subtasks) {
			st := d.card.Subtasks[d.selectedSubtask]
			openSubDetail(ctx, d, st.Key)
			return nil
		}
	case tcell.KeyPgUp:
		d.scroll = max(0, d.scroll-max(1, d.viewH-1))
		return nil
	case tcell.KeyPgDn:
		d.scroll = min(d.maxScroll, d.scroll+max(1, d.viewH-1))
		return nil
	case tcell.KeyHome:
		d.scroll = 0
		return nil
	case tcell.KeyEnd:
		d.scroll = d.maxScroll
		return nil
	case tcell.KeyCtrlC:
		ctx.app.Stop()
		return nil
	case tcell.KeyCtrlY:
		copyIssueURLToClipboard(ctx)
		return nil
	}
	return nil
}

func handleDetailRune(ctx *appContext, d *detailState, event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'q':
		ctx.state.detail = nil
		return nil
	case 'a':
		openAssigneePicker(ctx)
		return nil
	case 't':
		openTransitionModal(ctx)
		return nil
	case 'c':
		if ctx.state.projectKey != "" {
			openCreateSubtask(ctx, d.card.Key, d.card.Summary)
		}
		return nil
	case 'C':
		if ctx.state.projectKey != "" {
			key := d.card.Key
			if len(d.card.Subtasks) > 0 && d.selectedSubtask >= 0 && d.selectedSubtask < len(d.card.Subtasks) {
				key = d.card.Subtasks[d.selectedSubtask].Key
			}
			openCloneIssue(ctx, key)
		}
		return nil
	case 'y':
		copyKeyToClipboard(ctx)
		return nil
	case 'e':
		openEditIssue(ctx, d.card)
		return nil
	}
	return nil
}

func handleDetailUp(d *detailState, hasSubtasks bool) {
	if hasSubtasks && d.selectedSubtask > 0 {
		d.selectedSubtask--
		return
	}
	if d.scroll > 0 {
		d.scroll--
	}
}

func handleDetailDown(d *detailState, hasSubtasks bool) {
	if hasSubtasks && d.selectedSubtask < len(d.card.Subtasks)-1 {
		d.selectedSubtask++
		return
	}
	if d.scroll < d.maxScroll {
		d.scroll++
	}
}

func handleSubDetailInput(ctx *appContext, d *detailState, event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEscape:
		ctx.state.detail.subDetail = nil
		return nil
	case tcell.KeyRune:
		if event.Rune() == 'q' {
			ctx.state.detail.subDetail = nil
			return nil
		}
		if event.Rune() == 'e' {
			openEditIssue(ctx, d.card)
			return nil
		}
	case tcell.KeyUp:
		if d.scroll > 0 {
			d.scroll--
		}
		return nil
	case tcell.KeyDown:
		if d.scroll < d.maxScroll {
			d.scroll++
		}
		return nil
	case tcell.KeyPgUp:
		d.scroll = max(0, d.scroll-max(1, d.viewH-1))
		return nil
	case tcell.KeyPgDn:
		d.scroll = min(d.maxScroll, d.scroll+max(1, d.viewH-1))
		return nil
	case tcell.KeyHome:
		d.scroll = 0
		return nil
	case tcell.KeyEnd:
		d.scroll = d.maxScroll
		return nil
	case tcell.KeyCtrlC:
		ctx.app.Stop()
		return nil
	case tcell.KeyCtrlY:
		copyIssueURLToClipboard(ctx)
		return nil
	}
	return nil
}

func openSubDetail(ctx *appContext, parent *detailState, key string) {
	sub := &detailState{
		card:        jira.Card{Key: key},
		loading:     true,
		isSubDetail: true,
	}
	parent.subDetail = sub
	go func() {
		full, err := ctx.client.GetIssue(key)
		ctx.app.QueueUpdateDraw(func() {
			if parent.subDetail != sub {
				return
			}
			if err != nil {
				sub.loading = false
				sub.err = err.Error(); slog.Error("sub-detail load failed", "key", sub.card.Key, "error", err)
				return
			}
			sub.card = full
			sub.loading = false
		})
	}()
}
