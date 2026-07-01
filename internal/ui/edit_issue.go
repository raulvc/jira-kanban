package ui

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/raulvc/jira-kanban/internal/jira"
)

type editIssueState struct {
	formState
	key         string
	saving     bool
	origSum    string
	origDesc   string
	origLabels []string
	origEpic   string
	origEpicName string
}

func (e *editIssueState) isSubtask() bool { return e.parentKey != "" && !e.parentEpic }

func (e *editIssueState) skipField(f issueField) bool {
	if f == ifLabels {
		return true
	}
	if f == ifType {
		return true
	}
	if e.isSubtask() && f == ifEpic {
		return true
	}
	return false
}

func (e *editIssueState) nextField() {
	for {
		e.field = (e.field + 1) % ifFieldCount
		if e.skipField(e.field) {
			continue
		}
		break
	}
	e.clampCur()
}

func (e *editIssueState) prevField() {
	for {
		e.field = (e.field - 1 + ifFieldCount) % ifFieldCount
		if e.skipField(e.field) {
			continue
		}
		break
	}
	e.clampCur()
}

// ── drawing ────────────────────────────────────────────────────────────────

func drawEditIssue(screen tcell.Screen, e *editIssueState, screenW, screenH int) {
	const padding = 2
	contentW := 54

	descExtra := descVisH - 1

	epicListH := 0
	epicSepRow := 0
	if !e.isSubtask() && e.field == ifEpic && e.epicKey == "" {
		epicListH = epicMaxVis
		epicSepRow = 1
	}
	parentH := 0
	if e.isSubtask() {
		parentH = 3
	}
	labelsH := 0
	const maxErrLines = 3
	errLines := 0
	if e.errMsg != "" {
		errLines = min(wrappedLineCount(e.errMsg, contentW-3), maxErrLines)
	}
	contentH := 12 + descExtra + epicListH + epicSepRow + parentH + labelsH + errLines

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
		bg:            tcell.StyleDefault.Foreground(T().Fg).Background(T().Panel),
		border:        tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel),
		title:         tcell.StyleDefault.Foreground(T().Blue).Background(T().Panel).Bold(true),
		label:         tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel),
		input:         tcell.StyleDefault.Foreground(T().Fg).Background(T().Bg),
		placeholder:   tcell.StyleDefault.Foreground(T().Muted).Background(T().Bg),
		activeLabel:   tcell.StyleDefault.Foreground(T().Cyan).Background(T().Panel).Bold(true),
		activeInput:   tcell.StyleDefault.Foreground(T().Fg).Background(T().CardSel),
		activeBorder:  tcell.StyleDefault.Foreground(T().Cyan).Background(T().Bg),
		err:           tcell.StyleDefault.Foreground(T().Red).Background(T().Panel).Bold(true),
		sep:           tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel).Dim(true),
	}

	for row := oy; row < oy+boxH; row++ {
		fillRow(screen, ox, row, boxW, styles.bg)
	}
	drawBorder(screen, ox, oy, boxW, boxH, styles.border)

	cy := oy + 1
	drawText(screen, ox+padding, cy, " Edit "+e.key+" ", styles.title, contentW)
	cy++
	drawSep(screen, ox+padding, cy, contentW, styles.sep)
	cy++

	if e.isSubtask() {
		cy = drawParentSection(screen, &e.formState, lay, cy, styles.label, styles.sep)
	}

	cy = drawSummarySection(screen, &e.formState, lay, cy, styles)
	cy = drawDescSection(screen, &e.formState, lay, cy, styles)
	cy = drawEpicSection(screen, &e.formState, lay, cy, styles, styles.bg)


	if e.errMsg != "" {
		lines := wrapText(e.errMsg, contentW-3)
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

	drawEditButtons(screen, e, lay, styles)
}

func drawEditButtons(screen tcell.Screen, e *editIssueState, lay modalLayout, s formStyles) {
	btnY := lay.oy + lay.boxH - 2
	drawSep(screen, lay.ox+2, btnY-1, lay.contentW, s.sep)
	fillRow(screen, lay.ox+1, btnY, lay.boxW-2, s.bg)
	saveStyle := tcell.StyleDefault.Foreground(T().BadgeFg).Background(T().Green).Bold(true)
	cancelStyle := tcell.StyleDefault.Foreground(T().Fg).Background(T().Muted)
	saveText := " Save "
	cancelText := " Cancel "
	gap := 3
	totalBtnW := len([]rune(saveText)) + gap + len([]rune(cancelText))
	btnX := lay.ox + (lay.boxW-totalBtnW) / 2
	if e.field == ifButtons {
		if e.btnIdx == 0 {
			saveStyle = tcell.StyleDefault.Foreground(T().BadgeFg).Background(T().Cyan).Bold(true)
		} else {
			cancelStyle = tcell.StyleDefault.Foreground(T().BadgeFg).Background(T().Cyan).Bold(true)
		}
	}
	drawText(screen, btnX, btnY, saveText, saveStyle, lay.contentW)
	btnX += len([]rune(saveText)) + gap
	drawText(screen, btnX, btnY, cancelText, cancelStyle, lay.contentW)

	if e.saving {
		for row := lay.oy; row < lay.oy+lay.boxH; row++ {
			fillRow(screen, lay.ox, row, lay.boxW, tcell.StyleDefault.Foreground(T().Fg).Background(T().Panel).Dim(true))
		}
		drawBorder(screen, lay.ox, lay.oy, lay.boxW, lay.boxH, s.border.Dim(true))
		savingStyle := tcell.StyleDefault.Foreground(T().Cyan).Background(T().Panel).Bold(true)
		drawText(screen, lay.ox+(lay.boxW-len([]rune(" Saving…")))/2, lay.oy+lay.boxH/2, " Saving…", savingStyle, lay.boxW)
	}
}

// ── handlers ────────────────────────────────────────────────────────────────

func openEditIssue(ctx *appContext, card jira.Card) {
	epicFreq := make(map[string]int)
	for _, col := range ctx.state.data.Columns {
		for _, c := range col.Issues {
			if c.Epic != "" {
				epicFreq[c.Epic]++
			}
		}
	}

	e := &editIssueState{
		key: card.Key,
		formState: formState{
			summary:       card.Summary,
			sumCur:        len([]rune(card.Summary)),
			desc:          card.Description,
			descCur:       len([]rune(card.Description)),
			labels:        card.Labels,
			parentKey:     card.ParentKey,
			parentSummary: card.ParentSummary,
			parentEpic:    card.ParentIsEpic,
			epicFreq:      epicFreq,
			field:         ifSummary,
		},
	}
	e.origSum = e.summary
	e.origDesc = e.desc
	e.origLabels = make([]string, len(e.labels))
	copy(e.origLabels, e.labels)
	if card.Epic != "" {
		e.epicName = card.Epic
	}
	e.origEpicName = e.epicName
	ctx.state.editIssue = e

	go func() {
		full, err := ctx.client.GetIssue(card.Key)
		if err != nil {
			ctx.app.QueueUpdateDraw(func() {
				if ctx.state.editIssue != e {
					return
				}
				e.errMsg = err.Error()
				slog.Error("edit issue load failed", "error", err)
			})
			return
		}
		isSub := full.ParentKey != "" && !full.ParentIsEpic
		ctx.app.QueueUpdateDraw(func() {
			if ctx.state.editIssue != e {
				return
			}
			e.summary = full.Summary
			e.sumCur = len([]rune(full.Summary))
			e.desc = full.Description
			e.descCur = len([]rune(full.Description))
			e.labels = full.Labels
			e.parentKey = full.ParentKey
			e.parentSummary = full.ParentSummary
			e.parentEpic = full.ParentIsEpic
			if full.ParentIsEpic {
				e.epicKey = full.ParentKey
				e.epicName = full.ParentSummary
			} else if full.Epic != "" {
				e.epicName = full.Epic
			}
			e.origEpic = e.epicKey
			e.origEpicName = e.epicName
		})

		if !isSub {
			epics, err := ctx.client.SearchEpics(ctx.state.projectKey, "")
			ctx.app.QueueUpdateDraw(func() {
				if ctx.state.editIssue != e {
					return
				}
				if err != nil {
					return
				}
				sortEpicsByFreq(epics, epicFreq)
				e.epics = epics
				e.epicLoaded = true
				if e.epicName != "" {
					for _, ep := range epics {
						if ep.Summary == e.epicName || ep.Key == e.epicName {
							e.epicKey = ep.Key
							e.epicName = ep.Summary
							break
						}
					}
				}
				e.origEpic = e.epicKey
				e.origEpicName = e.epicName
			})
		}
	}()
}

func handleEditIssueInput(ctx *appContext, event *tcell.EventKey) *tcell.EventKey {
	e := ctx.state.editIssue
	if e.saving {
		return nil
	}
	switch event.Key() {
	case tcell.KeyEscape:
		e.stopDebounce()
		ctx.state.editIssue = nil
		return nil
	case tcell.KeyTab:
		e.nextField()
		e.clampCur()
		return nil
	case tcell.KeyBacktab:
		e.prevField()
		e.clampCur()
		return nil
	case tcell.KeyUp:
		handleEditUp(e)
		return nil
	case tcell.KeyDown:
		handleEditDown(e)
		return nil
	case tcell.KeyLeft:
		handleEditLeft(e)
		return nil
	case tcell.KeyRight:
		handleEditRight(e)
		return nil
	case tcell.KeyEnter:
		return handleEditEnter(ctx, e)
	case tcell.KeyCtrlU:
		if e.field == ifEpic {
			e.epicKey = ""
			e.epicName = ""
			e.epicQuery = ""
			e.epicCur = 0
			e.epicSel = 0
		}
		return nil
	case tcell.KeyHome:
		handleEditHome(e)
		return nil
	case tcell.KeyEnd:
		handleEditEnd(e)
		return nil
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		e.backspace()
		return nil
	case tcell.KeyDelete:
		e.deleteForward()
		return nil
	case tcell.KeyRune:
		if e.field == ifButtons {
			return nil
		}
		e.typeRune(event.Rune())
		if e.field == ifEpic {
			startEditEpicSearch(ctx, e)
		}
		return nil
	}
	return nil
}

func handleEditUp(e *editIssueState) {
	if e.field == ifButtons {
		if e.isSubtask() {
			e.field = ifDescription
		} else {
			e.field = ifEpic
		}
		e.clampCur()
		return
	}
	if e.field == ifEpic && e.epicKey == "" {
		if e.epicSel > 0 {
			e.epicSel--
		} else {
			e.field = ifDescription
			e.clampCur()
		}
	} else if e.field == ifDescription {
		lines := descLines(e.desc)
		curLine := e.descCurLine(lines)
		if curLine > 0 {
			e.descMoveUp(lines)
			e.descAutoScroll()
		} else if e.descScroll > 0 {
			e.descScroll--
		} else {
			e.field = ifSummary
			e.clampCur()
		}
	} else {
		e.prevField()
		e.clampCur()
	}
}

func handleEditDown(e *editIssueState) {
	if e.field == ifButtons {
		e.field = ifSummary
		e.clampCur()
		return
	}
	if e.field == ifEpic && e.epicKey == "" {
		items := e.filteredEpics()
		if e.epicSel < len(items)-1 {
			e.epicSel++
		} else {
			e.field = ifButtons
			e.clampCur()
		}
	} else if e.field == ifDescription {
		lines := descLines(e.desc)
		curLine := e.descCurLine(lines)
		if curLine < len(lines)-1 {
			e.descMoveDown(lines)
			e.descAutoScroll()
		} else if e.descScroll < max(0, len(lines)-descVisH) {
			e.descScroll++
		} else {
			e.nextField()
			e.clampCur()
		}
	} else {
		e.nextField()
		e.clampCur()
	}
}

func handleEditLeft(e *editIssueState) {
	if e.field == ifButtons {
		e.btnIdx = 0
		return
	}
	e.moveLeft()
}

func handleEditRight(e *editIssueState) {
	if e.field == ifButtons {
		e.btnIdx = 1
		return
	}
	e.moveRight()
}

func handleEditEnter(ctx *appContext, e *editIssueState) *tcell.EventKey {
	if e.field == ifButtons {
		if e.btnIdx == 0 {
			executeEditIssue(ctx)
		} else {
			e.stopDebounce()
			ctx.state.editIssue = nil
		}
		return nil
	}
	if e.field == ifEpic {
		e.handleEpicEnter()
		return nil
	}
	if e.field == ifDescription {
		e.handleNewline()
		return nil
	}
	if e.field == ifLabels && e.labelInput != "" {
		e.labels = append(e.labels, e.labelInput)
		e.labelInput = ""
		e.labelCur = 0
		return nil
	}
	executeEditIssue(ctx)
	return nil
}

func handleEditHome(e *editIssueState) { e.moveHome() }

func handleEditEnd(e *editIssueState)   { e.moveEnd() }

func startEditEpicSearch(ctx *appContext, e *editIssueState) {
	if e.debounce != nil {
		e.debounce.Stop()
	}
	if e.epicLoaded {
		return
	}
	projectKey := ctx.state.projectKey
	epicFreq := e.epicFreq
	e.debounce = time.AfterFunc(150*time.Millisecond, func() {
		epics, err := ctx.client.SearchEpics(projectKey, "")
		ctx.app.QueueUpdateDraw(func() {
			if ctx.state.editIssue != e {
				return
			}
			if err != nil {
				return
			}
			sortEpicsByFreq(epics, epicFreq)
			e.epics = epics
			e.epicLoaded = true
		})
	})
}

func labelsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func executeEditIssue(ctx *appContext) {
	e := ctx.state.editIssue
	if e.summary == "" {
		e.errMsg = "Summary is required"
		return
	}

	var edit jira.IssueEdit
	if e.summary != e.origSum {
		edit.Summary = &e.summary
	}
	if e.desc != e.origDesc {
		edit.Description = &e.desc
	}
	if !labelsEqual(e.labels, e.origLabels) {
		edit.Labels = &e.labels
	}

	epicChanged := e.epicKey != e.origEpic || e.epicName != e.origEpicName
	epicKey := e.epicKey

	var cacheEpic *string
	if epicChanged {
		epicVal := e.epicName
		cacheEpic = &epicVal
	}

	e.saving = true
	e.errMsg = ""

	go func() {
		err := ctx.client.EditIssue(e.key, edit)
		if err == nil && epicChanged && !e.isSubtask() {
			if epicKey != "" {
				if linkErr := ctx.client.LinkEpic(e.key, epicKey); linkErr != nil {
					slog.Warn("failed to link epic after edit", "key", e.key, "epic", epicKey, "error", linkErr)
				}
			} else {
				if clearErr := ctx.client.ClearEpic(e.key); clearErr != nil {
					slog.Warn("failed to clear epic after edit", "key", e.key, "error", clearErr)
				}
			}
		}
		if err == nil {
			ctx.client.UpdateCachedIssue(ctx.boardID, e.key, edit.Summary, edit.Description, edit.Labels, cacheEpic)
		}
		ctx.app.QueueUpdateDraw(func() {
			if ctx.state.editIssue != e {
				return
			}
			if err != nil {
				e.saving = false
				e.errMsg = err.Error()
				slog.Error("edit issue failed", "error", err)
				return
			}
			ctx.state.updateIssue(e.key, edit.Summary, edit.Description, edit.Labels, cacheEpic)
			ctx.state.editIssue = nil
			ctx.state.statusMsg = fmt.Sprintf(" Updated %s", e.key)
			startSync(ctx)
		})
	}()
}