package ui

import (
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/raulvc/jira-kanban/internal/fuzzy"
	"github.com/raulvc/jira-kanban/internal/jira"
)

// searchMode determines whether search is local (fuzzy on visible cards)
// or global (JQL query against Jira).
type searchMode int

const (
	searchLocal  searchMode = iota // fuzzy search on visible board cards
	searchGlobal                   // JQL search against Jira API
)

// searchState tracks the issue search overlay.
type searchState struct {
	mode     searchMode
	query    string
	results  []fuzzy.Result
	gresults []jira.SearchResult
	selected int
	loading  bool
	err      string
}

func newSearchState(mode searchMode) *searchState {
	return &searchState{mode: mode, selected: -1}
}

func (s *searchState) moveSelection(delta int) {
	n := s.resultCount()
	if n == 0 {
		return
	}
	if s.selected < 0 {
		s.selected = 0
	}
	s.selected += delta
	if s.selected < 0 {
		s.selected = 0
	}
	if s.selected >= n {
		s.selected = n - 1
	}
}

func (s *searchState) resultCount() int {
	if s.mode == searchLocal {
		return len(s.results)
	}
	return len(s.gresults)
}

func (s *searchState) typeRune(r rune) {
	s.query += string(r)
	s.selected = 0
}

func (s *searchState) backspace() {
	runes := []rune(s.query)
	if len(runes) > 0 {
		s.query = string(runes[:len(runes)-1])
	}
	s.selected = 0
}

// searchItemCount calculates how many result rows to reserve in the modal.
func searchItemCount(s *searchState, maxItems int) int {
	items := min(s.resultCount(), maxItems)
	if items == 0 && (s.loading || s.err != "" || s.query != "") {
		items = 1
	}
	return items
}

// drawSearchModal renders the issue search overlay.
func drawSearchModal(screen tcell.Screen, s *searchState, screenW, screenH int) {
	const padding = 2
	contentW := min(80, screenW-8)

	maxItems := 8
	items := searchItemCount(s, maxItems)
	previewH := 0
	if s.mode == searchLocal && s.selected >= 0 && s.selected < len(s.results) {
		previewH = 10 // separator + status/assignee + epic + labels + desc label + 5 desc lines
	}

	contentH := 4 + items + previewH
	boxW := contentW + padding*2
	boxH := min(contentH+padding, screenH-4)

	ox := (screenW - boxW) / 2
	oy := 0 // top-aligned so board is visible below

	bgStyle := tcell.StyleDefault.Foreground(T().Fg).Background(T().Panel)
	borderStyle := tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel)
	titleStyle := tcell.StyleDefault.Foreground(T().Blue).Background(T().Panel).Bold(true)
	searchStyle := tcell.StyleDefault.Foreground(T().Fg).Background(T().Bg)
	searchPlaceholder := tcell.StyleDefault.Foreground(T().Muted).Background(T().Bg)
	itemStyle := tcell.StyleDefault.Foreground(T().Fg).Background(T().Panel)
	selStyle := tcell.StyleDefault.Foreground(T().Fg).Background(T().CardSel).Bold(true)
	keyStyle := tcell.StyleDefault.Foreground(T().Cyan).Background(T().Panel)
	selKeyStyle := tcell.StyleDefault.Foreground(T().Cyan).Background(T().CardSel).Bold(true)
	statusStyle := tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel)
	hlStyle := tcell.StyleDefault.Foreground(T().Orange).Background(T().Panel).Bold(true)
	selHlStyle := tcell.StyleDefault.Foreground(T().Orange).Background(T().CardSel).Bold(true)

	for row := oy; row < oy+boxH; row++ {
		fillRow(screen, ox, row, boxW, bgStyle)
	}
	drawBorder(screen, ox, oy, boxW, boxH, borderStyle)

	cy := oy + 1
	title := " Search board (local) "
	if s.mode == searchGlobal {
		title = " Search Jira (global JQL) "
	}
	drawText(screen, ox+padding, cy, title, titleStyle, contentW)
	cy++

	fillRow(screen, ox+padding, cy, contentW, searchStyle)
	if s.query == "" {
		placeholder := " Type to search…"
		if s.mode == searchLocal {
			placeholder = " Fuzzy search visible cards… (Tab for global)"
		}
		drawText(screen, ox+padding, cy, placeholder, searchPlaceholder, contentW)
	} else {
		drawText(screen, ox+padding, cy, " "+s.query+"▏", searchStyle, contentW)
	}
	cy++
	cy++

	maxVisible := max(items, 1)
	scrollStart := 0
	if s.selected > 0 {
		if s.selected >= scrollStart+maxVisible {
			scrollStart = s.selected - maxVisible + 1
		}
		if s.selected < scrollStart {
			scrollStart = s.selected
		}
	}

	drawSearchEmptyStates(screen, s, ox, cy, padding, contentW, searchPlaceholder)
	if s.mode == searchLocal {
		drawLocalResults(screen, s, ox, cy, padding, contentW, boxW, maxVisible, scrollStart,
			itemStyle, selStyle, keyStyle, selKeyStyle, statusStyle, hlStyle, selHlStyle)
	} else {
		drawGlobalResults(screen, s, ox, cy, padding, contentW, boxW, maxVisible, scrollStart,
			itemStyle, selStyle, keyStyle, selKeyStyle, statusStyle)
	}
	cy += min(s.resultCount()-scrollStart, maxVisible)
	if s.resultCount() == 0 {
		cy++
	}

	if previewH > 0 {
		drawSearchPreview(screen, s, ox, cy, padding, contentW, searchPlaceholder, hlStyle)
	}

	btnY := oy + boxH - 2
	fillRow(screen, ox+1, btnY, boxW-2, bgStyle)
	hintStyle := tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel)
	hint := " ↑/↓ navigate • Enter open • Esc close "
	if s.mode == searchLocal {
		hint = " ↑/↓ navigate • Enter open • Tab global • Esc close "
	}
	drawText(screen, ox+(boxW-len([]rune(hint)))/2, btnY, hint, hintStyle, contentW)
}

func drawSearchEmptyStates(screen tcell.Screen, s *searchState, ox, cy, padding, contentW int, placeholder tcell.Style) {
	if s.loading && s.resultCount() == 0 {
		drawText(screen, ox+padding, cy, "  Searching…", placeholder, contentW)
	} else if s.err != "" && s.resultCount() == 0 {
		drawText(screen, ox+padding, cy, "  Error: "+truncStr(s.err, contentW-4), placeholder, contentW)
	} else if s.resultCount() == 0 && s.query != "" {
		drawText(screen, ox+padding, cy, "  No results", placeholder, contentW)
	}
}

func drawLocalResults(screen tcell.Screen, s *searchState, ox, cy, padding, contentW, boxW, maxVisible, scrollStart int,
	itemStyle, selStyle, keyStyle, selKeyStyle, statusStyle, hlStyle, selHlStyle tcell.Style) {
	for i := scrollStart; i < len(s.results) && i < scrollStart+maxVisible; i++ {
		r := s.results[i]
		style := itemStyle
		kStyle := keyStyle
		hStyle := hlStyle
		if i == s.selected {
			style = selStyle
			kStyle = selKeyStyle
			hStyle = selHlStyle
			fillRow(screen, ox+1, cy, boxW-2, style)
		}
		keyText := r.Key
		summaryText := truncStr(r.Summary, contentW-24)
		posSet := make(map[int]bool)
		offset := 1 // leading space
		for _, p := range r.Match.Positions {
			posSet[p+offset] = true
		}
		drawHighlightedText(screen, ox+padding, cy, " "+keyText, posSet, kStyle, hStyle)
		sumOffset := offset + len([]rune(keyText)) + 1 // space after key
		sumPosSet := make(map[int]bool)
		for _, p := range r.Match.Positions {
			adjusted := p - len([]rune(keyText)) - 1
			if adjusted >= 0 && adjusted < len([]rune(summaryText)) {
				sumPosSet[adjusted] = true
			}
		}
		_ = sumOffset
		drawHighlightedText(screen, ox+padding+14, cy, summaryText, sumPosSet, style, hStyle)
		drawText(screen, ox+padding+contentW-10, cy, truncStr(r.Status, 10), statusStyle, 10)
		cy++
	}
}

func drawGlobalResults(screen tcell.Screen, s *searchState, ox, cy, padding, contentW, boxW, maxVisible, scrollStart int,
	itemStyle, selStyle, keyStyle, selKeyStyle, statusStyle tcell.Style) {
	for i := scrollStart; i < len(s.gresults) && i < scrollStart+maxVisible; i++ {
		r := s.gresults[i]
		style := itemStyle
		kStyle := keyStyle
		if i == s.selected {
			style = selStyle
			kStyle = selKeyStyle
			fillRow(screen, ox+1, cy, boxW-2, style)
		}
		drawText(screen, ox+padding, cy, " "+r.Key, kStyle, 14)
		drawText(screen, ox+padding+14, cy, truncStr(r.Summary, contentW-24), style, contentW-24)
		drawText(screen, ox+padding+contentW-10, cy, truncStr(r.Status, 10), statusStyle, 10)
		cy++
	}
}

// drawSearchPreview renders a preview pane showing the selected card's fields.
func drawSearchPreview(screen tcell.Screen, s *searchState, ox, cy, padding, contentW int, mutedStyle, hlStyle tcell.Style) {
	if s.selected < 0 || s.selected >= len(s.results) {
		return
	}
	r := s.results[s.selected]
	sepStyle := tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel)
	labelStyle := tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel)
	valueStyle := tcell.StyleDefault.Foreground(T().Fg).Background(T().Panel)

	for x := ox + 1; x < ox+contentW+padding; x++ {
		screen.SetContent(x, cy, '─', nil, sepStyle)
	}
	cy++

	// Status + Assignee
	drawText(screen, ox+padding, cy, " Status: ", labelStyle, contentW)
	drawHighlightedText(screen, ox+padding+9, cy, truncStr(r.Status, 20), posSet(r.StatusMatch), valueStyle, hlStyle)
	drawText(screen, ox+padding+32, cy, " Assignee: ", labelStyle, contentW)
	assignee := r.Assignee
	if assignee == "" {
		assignee = "Unassigned"
	}
	drawHighlightedText(screen, ox+padding+43, cy, truncStr(assignee, contentW-43), posSet(r.AssigneeMatch), valueStyle, hlStyle)
	cy++

	// Epic
	if r.Epic != "" {
		drawText(screen, ox+padding, cy, " Epic: ", labelStyle, contentW)
		drawText(screen, ox+padding+8, cy, truncStr(r.Epic, contentW-8), valueStyle, contentW-8)
	}
	cy++

	// Labels
	var labels strings.Builder
	for i, l := range r.Labels {
		if i > 0 {
			labels.WriteString(", ")
		}
		labels.WriteString(l)
	}
	drawText(screen, ox+padding, cy, " Labels: ", labelStyle, contentW)
	if labels.Len() == 0 {
		drawText(screen, ox+padding+9, cy, "none", mutedStyle, contentW-9)
	} else {
		drawText(screen, ox+padding+9, cy, truncStr(labels.String(), contentW-9), valueStyle, contentW-9)
	}
	cy++

	// Description — show context around first match, with highlighting
	if r.Description == "" {
		drawText(screen, ox+padding, cy, " Description: (empty)", mutedStyle, contentW)
		return
	}

	drawText(screen, ox+padding, cy, " Description:", labelStyle, contentW)
	cy++

	descLines := renderDescPreview(r.Description, r.DescriptionMatch, contentW-2, 5)
	for _, line := range descLines {
		runes := []rune(line.text)
		for i, ch := range runes {
			st := valueStyle
			if i < len(line.hl) && line.hl[i] {
				st = hlStyle
			}
			screen.SetContent(ox+padding+1+i, cy, ch, nil, st)
		}
		cy++
	}
}

// posSet converts a []int of positions to a set for quick lookup.
func posSet(positions []int) map[int]bool {
	m := make(map[int]bool, len(positions))
	for _, p := range positions {
		m[p] = true
	}
	return m
}

// descLine is a rendered line of description text with highlight flags.
type descLine struct {
	text string
	hl   []bool // parallel to text runes
}

// renderDescPreview produces display lines from a description, highlighting
// matched positions. If the match is deep in the text, it trims to show
// context around the first match rather than always starting from the beginning.
func renderDescPreview(desc string, matchPositions []int, width, maxLines int) []descLine {
	if len(matchPositions) == 0 {
		// No match in description — show first lines
		lines := splitLines(desc)
		var result []descLine
		for _, line := range lines {
			if len(result) >= maxLines {
				break
			}
			trunc := truncStr(line, width)
			result = append(result, descLine{text: trunc, hl: make([]bool, len([]rune(trunc)))})
		}
		return result
	}

	// Find first match position
	firstMatch := matchPositions[0]

	// Determine which line the first match is on
	lines := splitLines(desc)
	lineStart := 0
	matchLineIdx := 0
	for i, line := range lines {
		lineEnd := lineStart + len([]rune(line))
		if firstMatch < lineEnd {
			matchLineIdx = i
			break
		}
		lineStart = lineEnd + 1 // +1 for the newline
	}

	// Start a few lines before the match line for context
	startLine := matchLineIdx
	if startLine > 1 {
		startLine = matchLineIdx - 1
	}

	// Adjust match positions relative to startLine
	charOffset := 0
	for i := range startLine {
		charOffset += len([]rune(lines[i])) + 1 // +1 for newline
	}

	var result []descLine
	for i := startLine; i < len(lines) && len(result) < maxLines; i++ {
		line := lines[i]
		lineRunes := []rune(line)
		// Trim long lines around the match
		lineStartIdx := 0
		if i == matchLineIdx && len(lineRunes) > width {
			// Find first match within this line
			localMatch := max(0, firstMatch-charOffset)
			// Show context before the match
			start := max(0, localMatch-5)
			if start+width > len(lineRunes) {
				start = max(0, len(lineRunes)-width)
			}
			lineStartIdx = start
			lineRunes = lineRunes[start:]
		}

		trunc := truncStr(string(lineRunes), width)
		truncRunes := []rune(trunc)
		hl := make([]bool, len(truncRunes))

		// Map global match positions to this line (positions are rune indices)
		for _, pos := range matchPositions {
			localPos := pos - charOffset - lineStartIdx
			if localPos >= 0 && localPos < len(truncRunes) {
				hl[localPos] = true
			}
		}

		result = append(result, descLine{text: trunc, hl: hl})

		// Update charOffset for next line
		charOffset += len([]rune(lines[i])) + 1
	}

	return result
}

// drawHighlightedText draws text with specific character positions highlighted.
// Positions are rune indices (not byte offsets).
func drawHighlightedText(screen tcell.Screen, x, y int, text string, hlPos map[int]bool, baseStyle, hlStyle tcell.Style) {
	col := 0
	for i, r := range text {
		_ = i
		st := baseStyle
		if hlPos[col] {
			st = hlStyle
		}
		screen.SetContent(x+col, y, r, nil, st)
		col++
	}
}

// handleSearchInput processes key events for the search modal.
func handleSearchInput(ctx *appContext, event *tcell.EventKey) *tcell.EventKey {
	s := ctx.state.search
	switch event.Key() {
	case tcell.KeyEscape:
		ctx.state.highlightKey = ""
		ctx.state.search = nil
		return nil
	case tcell.KeyUp:
		s.moveSelection(-1)
		updateHighlightKey(ctx)
		return nil
	case tcell.KeyDown:
		s.moveSelection(1)
		updateHighlightKey(ctx)
		return nil
	case tcell.KeyEnter:
		if s.selected >= 0 && s.selected < s.resultCount() {
			var key string
			if s.mode == searchLocal {
				key = s.results[s.selected].Key
			} else {
				key = s.gresults[s.selected].Key
			}
			ctx.state.highlightKey = ""
			ctx.state.search = nil
			ctx.state.selectCardByKey(key)
			openIssueDetailByKey(ctx, key)
		}
		return nil
	case tcell.KeyCtrlC:
		ctx.app.Stop()
		return nil
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		s.backspace()
		triggerSearch(ctx)
		return nil
	case tcell.KeyTab:
		if s.mode == searchLocal {
			ctx.state.highlightKey = ""
			switchToGlobalSearch(ctx)
		} else {
			s.mode = searchLocal
			s.query = ""
			s.gresults = nil
			s.results = nil
			s.selected = -1
			s.loading = false
			s.err = ""
			ctx.state.highlightKey = ""
		}
		return nil
	case tcell.KeyRune:
		s.typeRune(event.Rune())
		triggerSearch(ctx)
		return nil
	}
	return nil
}

// updateHighlightKey sets the board highlight to the selected search result
// and scrolls the board to show that card.
func updateHighlightKey(ctx *appContext) {
	s := ctx.state.search
	if s == nil || s.mode != searchLocal || s.selected < 0 || s.selected >= len(s.results) {
		ctx.state.highlightKey = ""
		return
	}
	key := s.results[s.selected].Key
	ctx.state.highlightKey = key
	ctx.state.selectCardByKey(key)
}

// switchToGlobalSearch transitions from local to global search mode.
func switchToGlobalSearch(ctx *appContext) {
	s := ctx.state.search
	if s == nil {
		return
	}
	s.mode = searchGlobal
	s.query = ""
	s.results = nil
	s.gresults = nil
	s.selected = -1
	s.loading = false
	s.err = ""
}

// openIssueDetailByKey opens the detail modal for a specific issue key.
func openIssueDetailByKey(ctx *appContext, key string) {
	ctx.state.detail = &detailState{
		card:    jira.Card{Key: key},
		loading: true,
	}
	go func() {
		full, err := ctx.client.GetIssue(key)
		if err == nil {
			ctx.client.UpdateCachedCard(ctx.boardID, full)
		}
		ctx.app.QueueUpdateDraw(func() {
			d := ctx.state.detail
			if d == nil || d.card.Key != key {
				return
			}
			if err != nil {
				d.loading = false
				d.err = err.Error()
				slog.Error("detail load failed", "key", key, "error", err)
				return
			}
			d.card = full
			d.loading = false
			ctx.state.refreshCard(full)
		})
	}()
}

// triggerSearch dispatches to the appropriate search based on mode.
func triggerSearch(ctx *appContext) {
	s := ctx.state.search
	if s == nil {
		return
	}
	if s.query == "" {
		s.results = nil
		s.gresults = nil
		s.loading = false
		s.err = ""
		return
	}
	if s.mode == searchLocal {
		executeLocalSearch(ctx)
	} else {
		triggerGlobalSearch(ctx)
	}
}

func executeLocalSearch(ctx *appContext) {
	s := ctx.state.search
	if s == nil || s.query == "" {
		return
	}
	fd := ctx.state.filteredData()
	var results []fuzzy.Result
	for _, col := range fd.Columns {
		for _, card := range col.Issues {
			m, matched := fuzzy.MatchCard(s.query, card.Key, card.Summary, card.Status, card.Assignee, card.Description, card.Labels)
			if !matched {
				continue
			}
			km, sm, stm, am, dm, lm := fuzzy.MatchFields(s.query, card.Key, card.Summary, card.Status, card.Assignee, card.Description, card.Labels)
			results = append(results, fuzzy.Result{
				Card:             card.Key + " " + card.Summary,
				Key:              card.Key,
				Summary:          card.Summary,
				Status:           card.Status,
				Assignee:         card.Assignee,
				Description:      card.Description,
				Labels:           card.Labels,
				Epic:             card.Epic,
				Match:            m,
				KeyMatch:         km,
				SummaryMatch:     sm,
				StatusMatch:      stm,
				AssigneeMatch:    am,
				DescriptionMatch: dm,
				LabelMatches:     lm,
			})
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Match.Score > results[j].Match.Score
	})
	s.results = results
	if len(results) > 0 && s.selected < 0 {
		s.selected = 0
	}
	if s.selected >= len(results) {
		s.selected = max(0, len(results)-1)
	}
	updateHighlightKey(ctx)
}

var searchDebounceTimer *time.Timer

func triggerGlobalSearch(ctx *appContext) {
	s := ctx.state.search
	if s == nil {
		return
	}
	if searchDebounceTimer != nil {
		searchDebounceTimer.Stop()
	}
	searchDebounceTimer = time.AfterFunc(300*time.Millisecond, func() {
		executeGlobalSearch(ctx)
	})
}

func executeGlobalSearch(ctx *appContext) {
	s := ctx.state.search
	if s == nil || s.query == "" {
		return
	}
	ctx.app.QueueUpdateDraw(func() {
		if ctx.state.search != nil {
			ctx.state.search.loading = true
		}
	})
	go func() {
		results, err := ctx.client.SearchIssues(s.query)
		ctx.app.QueueUpdateDraw(func() {
			if ctx.state.search == nil {
				return
			}
			ctx.state.search.loading = false
			if err != nil {
				ctx.state.search.err = err.Error()
				return
			}
			ctx.state.search.err = ""
			ctx.state.search.gresults = results
			if len(results) > 0 && ctx.state.search.selected < 0 {
				ctx.state.search.selected = 0
			}
			if ctx.state.search.selected >= len(results) {
				ctx.state.search.selected = max(0, len(results)-1)
			}
		})
	}()
}
