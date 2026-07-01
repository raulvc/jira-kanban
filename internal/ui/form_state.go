package ui

import (
	"time"

	"github.com/raulvc/jira-kanban/internal/jira"
)

type issueField int

const (
	ifType issueField = iota
	ifSummary
	ifDescription
	ifEpic
	ifLabels
	ifButtons
	ifFieldCount
)

type formState struct {
	summary       string
	sumCur        int
	desc          string
	descScroll    int
	descCur       int
	epicQuery     string
	epicCur       int
	epicKey       string
	epicName      string
	epics         []jira.EpicItem
	epicSel       int
	labels        []string
	labelInput    string
	labelCur      int
	parentKey     string
	parentSummary string
	parentEpic     bool
	field         issueField
	btnIdx        int
	errMsg        string
	debounce      *time.Timer
	epicLoaded    bool
	epicFreq      map[string]int
}

func (f *formState) typeRune(r rune) {
	switch f.field {
	case ifSummary:
		runes := []rune(f.summary)
		f.summary = string(runes[:f.sumCur]) + string(r) + string(runes[f.sumCur:])
		f.sumCur++
	case ifDescription:
		runes := []rune(f.desc)
		f.desc = string(runes[:f.descCur]) + string(r) + string(runes[f.descCur:])
		f.descCur++
		f.descAutoScroll()
	case ifEpic:
		runes := []rune(f.epicQuery)
		f.epicQuery = string(runes[:f.epicCur]) + string(r) + string(runes[f.epicCur:])
		f.epicCur++
		f.epicSel = 0
	case ifLabels:
		runes := []rune(f.labelInput)
		f.labelInput = string(runes[:f.labelCur]) + string(r) + string(runes[f.labelCur:])
		f.labelCur++
	}
}

func (f *formState) backspace() {
	switch f.field {
	case ifSummary:
		if f.sumCur > 0 {
			runes := []rune(f.summary)
			f.summary = string(runes[:f.sumCur-1]) + string(runes[f.sumCur:])
			f.sumCur--
		}
	case ifDescription:
		if f.descCur > 0 {
			runes := []rune(f.desc)
			f.desc = string(runes[:f.descCur-1]) + string(runes[f.descCur:])
			f.descCur--
			f.descAutoScroll()
		}
	case ifEpic:
		if f.epicCur > 0 {
			runes := []rune(f.epicQuery)
			f.epicQuery = string(runes[:f.epicCur-1]) + string(runes[f.epicCur:])
			f.epicCur--
			items := f.filteredEpics()
			if f.epicSel >= len(items) && len(items) > 0 {
				f.epicSel = len(items) - 1
			}
		}
	case ifLabels:
		if f.labelCur > 0 {
			runes := []rune(f.labelInput)
			f.labelInput = string(runes[:f.labelCur-1]) + string(runes[f.labelCur:])
			f.labelCur--
		}
	}
}

func (f *formState) deleteForward() {
	switch f.field {
	case ifSummary:
		runes := []rune(f.summary)
		if f.sumCur < len(runes) {
			f.summary = string(runes[:f.sumCur]) + string(runes[f.sumCur+1:])
		}
	case ifDescription:
		runes := []rune(f.desc)
		if f.descCur < len(runes) {
			f.desc = string(runes[:f.descCur]) + string(runes[f.descCur+1:])
			f.descAutoScroll()
		}
	case ifEpic:
		runes := []rune(f.epicQuery)
		if f.epicCur < len(runes) {
			f.epicQuery = string(runes[:f.epicCur]) + string(runes[f.epicCur+1:])
			items := f.filteredEpics()
			if f.epicSel >= len(items) && len(items) > 0 {
				f.epicSel = len(items) - 1
			}
		}
	case ifLabels:
		runes := []rune(f.labelInput)
		if f.labelCur < len(runes) {
			f.labelInput = string(runes[:f.labelCur]) + string(runes[f.labelCur+1:])
		}
	}
}

func (f *formState) handleNewline() {
	if f.field == ifDescription {
		runes := []rune(f.desc)
		f.desc = string(runes[:f.descCur]) + "\n" + string(runes[f.descCur:])
		f.descCur++
		f.descAutoScroll()
	}
	if f.field == ifEpic {
		f.handleEpicEnter()
	}
	if f.field == ifLabels && f.labelInput != "" {
		f.labels = append(f.labels, f.labelInput)
		f.labelInput = ""
		f.labelCur = 0
	}
}

func (f *formState) handleEpicEnter() {
	items := f.filteredEpics()
	if f.epicSel >= 0 && f.epicSel < len(items) {
		f.epicKey = items[f.epicSel].Key
		f.epicName = items[f.epicSel].Summary
		f.clampCur()
	}
}

func (f *formState) stopDebounce() {
	if f.debounce != nil {
		f.debounce.Stop()
		f.debounce = nil
	}
}

func (f *formState) clampCur() {
	switch f.field {
	case ifSummary:
		f.sumCur = clamp(f.sumCur, len([]rune(f.summary)))
	case ifDescription:
		f.descCur = clamp(f.descCur, len([]rune(f.desc)))
		f.descAutoScroll()
	case ifEpic:
		f.epicCur = clamp(f.epicCur, len([]rune(f.epicQuery)))
		items := f.filteredEpics()
		if f.epicSel >= len(items) {
			f.epicSel = max(0, len(items)-1)
		}
	case ifLabels:
		f.labelCur = clamp(f.labelCur, len([]rune(f.labelInput)))
	}
	f.btnIdx = clamp(f.btnIdx, 1)
}

func (f *formState) descAutoScroll() {
	lines := descLines(f.desc)
	curLine := f.descCurLine(lines)
	maxScroll := max(0, len(lines)-descVisH)
	f.descScroll = clamp(f.descScroll, maxScroll)
	if curLine < f.descScroll {
		f.descScroll = curLine
	} else if curLine >= f.descScroll+descVisH {
		f.descScroll = curLine - descVisH + 1
	}
}

func (f *formState) descCurLine(lines []string) int {
	if len(lines) == 0 {
		return 0
	}
	for i, off := range descLineOffsets(f.desc, lines) {
		if f.descCur <= off+len([]rune(lines[i])) {
			return i
		}
	}
	return len(lines) - 1
}

func (f *formState) descMoveUp(lines []string) {
	curLine := f.descCurLine(lines)
	if curLine == 0 {
		f.descCur = 0
		return
	}
	offsets := descLineOffsets(f.desc, lines)
	curCol := f.descCur - offsets[curLine]
	prevLineRunes := len([]rune(lines[curLine-1]))
	newCol := min(curCol, prevLineRunes)
	f.descCur = offsets[curLine-1] + newCol
}

func (f *formState) descMoveDown(lines []string) {
	curLine := f.descCurLine(lines)
	if curLine >= len(lines)-1 {
		f.descCur = len([]rune(f.desc))
		return
	}
	offsets := descLineOffsets(f.desc, lines)
	curCol := f.descCur - offsets[curLine]
	nextLineRunes := len([]rune(lines[curLine+1]))
	newCol := min(curCol, nextLineRunes)
	f.descCur = offsets[curLine+1] + newCol
}

func (f *formState) filteredEpics() []jira.EpicItem {
	var out []jira.EpicItem
	if f.epicQuery == "" {
		n := min(epicMaxVis, len(f.epics))
		out = make([]jira.EpicItem, n)
		copy(out, f.epics[:n])
	} else {
		q := foldLower(f.epicQuery)
		for _, e := range f.epics {
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

func (f *formState) moveLeft() {
	switch f.field {
	case ifSummary:
		if f.sumCur > 0 {
			f.sumCur--
		}
	case ifEpic:
		if f.epicCur > 0 {
			f.epicCur--
		}
	case ifDescription:
		if f.descCur > 0 {
			f.descCur--
			f.descAutoScroll()
		}
	case ifLabels:
		if f.labelCur > 0 {
			f.labelCur--
		}
	}
}

func (f *formState) moveRight() {
	switch f.field {
	case ifSummary:
		if f.sumCur < len([]rune(f.summary)) {
			f.sumCur++
		}
	case ifEpic:
		if f.epicCur < len([]rune(f.epicQuery)) {
			f.epicCur++
		}
	case ifDescription:
		if f.descCur < len([]rune(f.desc)) {
			f.descCur++
			f.descAutoScroll()
		}
	case ifLabels:
		if f.labelCur < len([]rune(f.labelInput)) {
			f.labelCur++
		}
	}
}

func (f *formState) moveHome() {
	switch f.field {
	case ifSummary:
		f.sumCur = 0
	case ifDescription:
		f.descCur = 0
		f.descAutoScroll()
	case ifEpic:
		f.epicCur = 0
	case ifLabels:
		f.labelCur = 0
	}
}

func (f *formState) moveEnd() {
	switch f.field {
	case ifSummary:
		f.sumCur = len([]rune(f.summary))
	case ifDescription:
		f.descCur = len([]rune(f.desc))
		f.descAutoScroll()
	case ifEpic:
		f.epicCur = len([]rune(f.epicQuery))
	case ifLabels:
		f.labelCur = len([]rune(f.labelInput))
	}
}