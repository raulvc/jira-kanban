package ui

import (
	"fmt"
	"log/slog"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/raulvc/jira-kanban/internal/cache"
	"github.com/raulvc/jira-kanban/internal/jira"
)

// appContext bundles the dependencies needed by input handlers.
type appContext struct {
	app        *tview.Application
	state      *boardState
	client     *jira.Client
	boardID    int
	baseURL    string
	syncTicker *time.Ticker
}

// Run starts the interactive TUI for the given board data.
// When needsSync is true the board was loaded from cache and a
// background refresh is triggered immediately after the first render.
func Run(client *jira.Client, boardID int, data jira.Board, baseURL string, needsSync bool) error {
	if err := loadThemePrefs(); err != nil {
		// Non-fatal: just use the default theme.
		_ = err
	}

	app := tview.NewApplication()
	state := newBoardState(data)
	ctx := &appContext{
		app:     app,
		state:   state,
		client:  client,
		boardID: boardID,
		baseURL: baseURL,
	}

	go func() {
		if me, err := client.GetCurrentUser(); err == nil && me.DisplayName != "" {
			ctx.app.QueueUpdateDraw(func() {
				state.currentUser = me.DisplayName
				state.accountID = me.AccountID
			})
		}
	}()

	box := tview.NewBox().SetBackgroundColor(T().Bg)
	box.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		drawBoard(screen, state, boardID, x, y, width, height)
		return x, y, width, height
	})

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if state.filter != nil {
			return handleFilterInput(ctx, event)
		}
		if state.epicFilter != nil {
			return handleEpicFilterInput(ctx, event)
		}
		if state.assigneePicker != nil {
			return handleAssigneePickerInput(ctx, event)
		}
		if state.createIssue != nil {
			return handleCreateIssueInput(ctx, event)
		}
		if state.history != nil {
			return handleHistoryInput(ctx, event)
		}
		if state.modal != nil {
			return handleModalInput(ctx, event)
		}
		if state.detail != nil {
			return handleDetailInput(ctx, event)
		}
		return handleBoardInput(ctx, event)
	})

	if needsSync {
		startSync(ctx)
	}

	return app.SetRoot(box, true).EnableMouse(false).Run()
}

// startSync begins a background board refresh with progress reporting
// and an animated spinner in the status bar.
func startSync(ctx *appContext) {
	ctx.state.syncing = true
	ctx.state.syncPhase = ""
	ctx.state.syncFetched = 0
	ctx.state.syncTotal = 0

	ctx.syncTicker = time.NewTicker(100 * time.Millisecond)
	go func() {
		for range ctx.syncTicker.C {
			if !ctx.state.syncing {
				ctx.syncTicker.Stop()
				return
			}
			ctx.state.spinnerFrame++
			ctx.app.QueueUpdateDraw(func() {})
		}
	}()

	go func() {
		newData, err := ctx.client.RefreshBoard(ctx.boardID, func(p jira.SyncProgress) {
			ctx.app.QueueUpdateDraw(func() {
				ctx.state.syncPhase = p.Phase
				ctx.state.syncFetched = p.Fetched
				ctx.state.syncTotal = p.Total
			})
		})
		ctx.app.QueueUpdateDraw(func() {
			if ctx.syncTicker != nil {
				ctx.syncTicker.Stop()
			}
			if err != nil {
				ctx.state.syncing = false
				ctx.state.statusMsg = fmt.Sprintf(" Sync error: %s", err.Error())
				return
			}
			ctx.state.reload(newData)
		})
	}()
}

// ── board input ─────────────────────────────────────────────────────────────

func handleBoardInput(ctx *appContext, event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyCtrlC:
		ctx.app.Stop()
		return nil
	case tcell.KeyCtrlY:
		copyIssueURLToClipboard(ctx)
		return nil
	case tcell.KeyEscape:
		if ctx.state.memberFilter != "" {
			ctx.state.memberFilter = ""
			ctx.state.clampSelection()
		}
		if ctx.state.epicFilterVal != "" {
			ctx.state.epicFilterVal = ""
			ctx.state.clampSelection()
		}
		return nil
	case tcell.KeyLeft:
		ctx.state.moveColumn(-1)
		return nil
	case tcell.KeyRight:
		ctx.state.moveColumn(1)
		return nil
	case tcell.KeyUp:
		ctx.state.moveCard(-1)
		return nil
	case tcell.KeyDown:
		ctx.state.moveCard(1)
		return nil
	case tcell.KeyHome:
		ctx.state.jumpCard(false)
		return nil
	case tcell.KeyEnd:
		ctx.state.jumpCard(true)
		return nil
	case tcell.KeyEnter:
		openIssueDetail(ctx)
		return nil
	case tcell.KeyRune:
		return handleBoardRune(ctx, event)
	}
	return event
}

func handleBoardRune(ctx *appContext, event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'q':
		ctx.app.Stop()
		return nil
	case 'o':
		openIssueBrowser(ctx)
		return nil
	case 'r':
		if !ctx.state.syncing {
			refreshBoard(ctx)
		}
		return nil
	case 't':
		if !ctx.state.syncing {
			openTransitionModal(ctx)
		}
		return nil
	case 'f':
		if ctx.state.filter == nil {
			ctx.state.filter = newFilterState(ctx.state.data)
		}
		return nil
	case 'e':
		if ctx.state.epicFilter == nil {
			ctx.state.epicFilter = newEpicFilterState(ctx.state.data)
		}
		return nil
	case 'a':
		openAssigneePicker(ctx)
		return nil
	case 'c':
		if ctx.state.createIssue == nil && ctx.state.projectKey != "" {
			openCreateIssue(ctx)
		}
		return nil
	case 'h':
		if ctx.state.history == nil {
			openHistory(ctx)
		}
		return nil
	case 'y':
		copyKeyToClipboard(ctx)
		return nil
	case '+':
		name := cycleTheme()
		ctx.state.statusMsg = fmt.Sprintf(" Theme: %s", name)
		saveThemePrefs()
		return nil
	}
	return event
}

// ── modal input ─────────────────────────────────────────────────────────────

func handleModalInput(ctx *appContext, event *tcell.EventKey) *tcell.EventKey {
	m := ctx.state.modal
	switch event.Key() {
	case tcell.KeyEscape:
		ctx.state.modal = nil
		return nil
	case tcell.KeyUp:
		m.moveSelection(-1)
		return nil
	case tcell.KeyDown:
		m.moveSelection(1)
		return nil
	case tcell.KeyEnter:
		executeTransition(ctx)
		return nil
	case tcell.KeyCtrlC:
		ctx.app.Stop()
		return nil
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		m.backspace()
		return nil
	case tcell.KeyRune:
		m.typeRune(event.Rune())
		return nil
	}
	return nil
}

// ── filter input ──────────────────────────────────────────────────────────

func handleFilterInput(ctx *appContext, event *tcell.EventKey) *tcell.EventKey {
	f := ctx.state.filter
	switch event.Key() {
	case tcell.KeyEscape:
		ctx.state.memberFilter = ""
		ctx.state.filter = nil
		ctx.state.clampSelection()
		return nil
	case tcell.KeyUp:
		f.moveSelection(-1)
		return nil
	case tcell.KeyDown:
		f.moveSelection(1)
		return nil
	case tcell.KeyEnter:
		items := f.filtered()
		if f.selected >= 0 && f.selected < len(items) {
			ctx.state.memberFilter = items[f.selected]
		}
		ctx.state.filter = nil
		ctx.state.clampSelection()
		return nil
	case tcell.KeyCtrlU:
		ctx.state.memberFilter = ""
		ctx.state.filter = nil
		ctx.state.clampSelection()
		return nil
	case tcell.KeyCtrlC:
		ctx.app.Stop()
		return nil
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		f.backspace()
		return nil
	case tcell.KeyRune:
		f.typeRune(event.Rune())
		return nil
	}
	return nil
}

func handleEpicFilterInput(ctx *appContext, event *tcell.EventKey) *tcell.EventKey {
	f := ctx.state.epicFilter
	switch event.Key() {
	case tcell.KeyEscape:
		ctx.state.epicFilterVal = ""
		ctx.state.epicFilter = nil
		ctx.state.clampSelection()
		return nil
	case tcell.KeyUp:
		f.moveSelection(-1)
		return nil
	case tcell.KeyDown:
		f.moveSelection(1)
		return nil
	case tcell.KeyEnter:
		items := f.filtered()
		if f.selected >= 0 && f.selected < len(items) {
			ctx.state.epicFilterVal = items[f.selected]
		}
		ctx.state.epicFilter = nil
		ctx.state.clampSelection()
		return nil
	case tcell.KeyCtrlU:
		ctx.state.epicFilterVal = ""
		ctx.state.epicFilter = nil
		ctx.state.clampSelection()
		return nil
	case tcell.KeyCtrlC:
		ctx.app.Stop()
		return nil
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		f.backspace()
		return nil
	case tcell.KeyRune:
		f.typeRune(event.Rune())
		return nil
	}
	return nil
}

// ── actions ─────────────────────────────────────────────────────────────────

func openTransitionModal(ctx *appContext) {
	var card *jira.Card
	if d := ctx.state.detail; d != nil {
		card = &d.card
	} else {
		card = ctx.state.selectedCard()
	}
	if card == nil {
		return
	}
	ctx.state.statusMsg = " Loading transitions…"
	ctx.app.ForceDraw()
	go func() {
		transitions, err := ctx.client.GetTransitions(card.Key)
		ctx.app.QueueUpdateDraw(func() {
			ctx.state.statusMsg = ""
			if err != nil {
				ctx.state.statusMsg = fmt.Sprintf(" Error: %s", err.Error())
				return
			}
			if len(transitions) == 0 {
				ctx.state.statusMsg = " No transitions available"
				return
			}
			ctx.state.modal = &modalState{
				issueKey:       card.Key,
				allTransitions: transitions,
			}
		})
	}()
}

func executeTransition(ctx *appContext) {
	m := ctx.state.modal
	t := m.selectedTransition()
	if t == nil {
		return
	}
	issueKey := m.issueKey
	transitionID := t.ID
	transitionName := t.Name
	toStatus := t.ToStatus
	toStatusID := t.ToStatusID

	ctx.state.modal = nil
	ctx.state.moveIssueToStatus(issueKey, toStatus)
	if d := ctx.state.detail; d != nil && d.card.Key == issueKey {
		d.card.Status = toStatus
	}
	ctx.state.statusMsg = fmt.Sprintf(" Transitioning %s → %s…", issueKey, transitionName)
	ctx.app.ForceDraw()

	go func() {
		err := ctx.client.DoTransition(issueKey, transitionID)
		if err != nil {
			ctx.app.QueueUpdateDraw(func() {
				ctx.state.statusMsg = fmt.Sprintf(" Error: %s", err.Error())
			})
			startSync(ctx)
			return
		}
		// Persist the status change to cache so the next sync doesn't revert it.
		ctx.client.UpdateCachedStatus(ctx.boardID, issueKey, toStatusID, toStatus)
		ctx.app.QueueUpdateDraw(func() {
			ctx.state.statusMsg = fmt.Sprintf(" %s → %s", issueKey, transitionName)
		})
		startSync(ctx)
	}()
}

func refreshBoard(ctx *appContext) {
	startSync(ctx)
}

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

// ── detail input ───────────────────────────────────────────────────────────

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
			openCreateSubtask(ctx, d.card.Key)
		}
		return nil
	case 'y':
		copyKeyToClipboard(ctx)
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

func openIssueBrowser(ctx *appContext) {
	card := ctx.state.selectedCard()
	if card == nil {
		return
	}
	issueURL := fmt.Sprintf("%s/browse/%s", ctx.baseURL, card.Key)
	c := exec.Command("xdg-open", issueURL)
	_ = c.Start()
}

func copyToClipboard(text string) bool {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "windows":
		cmd = exec.Command("clip.exe")
	default:
		cmd = exec.Command("xclip", "-selection", "clipboard")
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return false
	}
	if err := cmd.Start(); err != nil {
		return false
	}
	_, _ = fmt.Fprint(stdin, text)
	_ = stdin.Close()
	_ = cmd.Wait()
	return true
}

func selectedCard(ctx *appContext) *jira.Card {
	if ctx.state.detail != nil {
		return &ctx.state.detail.card
	}
	return ctx.state.selectedCard()
}

func copyKeyToClipboard(ctx *appContext) {
	card := selectedCard(ctx)
	if card == nil {
		return
	}
	if copyToClipboard(card.Key) {
		ctx.state.statusMsg = fmt.Sprintf(" Copied %s", card.Key)
	}
}

func copyIssueURLToClipboard(ctx *appContext) {
	card := selectedCard(ctx)
	if card == nil {
		return
	}
	url := fmt.Sprintf("%s/browse/%s", ctx.baseURL, card.Key)
	if copyToClipboard(url) {
		ctx.state.statusMsg = fmt.Sprintf(" Copied %s", url)
	}
}

// ── assignee picker ────────────────────────────────────────────────────────

func openAssigneePicker(ctx *appContext) {
	var card *jira.Card
	if ctx.state.detail != nil {
		card = &ctx.state.detail.card
	} else {
		card = ctx.state.selectedCard()
	}
	if card == nil {
		return
	}
	issueKey := card.Key

	picker := newAssigneePickerState(issueKey, card.Assignee, ctx.state.currentUser, ctx.state.data)
	ctx.state.assigneePicker = picker
}

func startAssigneeSearch(ctx *appContext, a *assigneePickerState) {
	if a.debounce != nil {
		a.debounce.Stop()
	}
	query := a.query
	issueKey := a.issueKey
	a.debounce = time.AfterFunc(100*time.Millisecond, func() {
		users, err := ctx.client.SearchAssignableUsers(issueKey, query)
		ctx.app.QueueUpdateDraw(func() {
			p := ctx.state.assigneePicker
			if p == nil || p != a {
				return
			}
			if err != nil {
				p.errMsg = err.Error(); slog.Error("assignee picker failed", "error", err)
				return
			}
			p.mergeAPIResults(users)
		})
	})
}

func handleAssigneePickerInput(ctx *appContext, event *tcell.EventKey) *tcell.EventKey {
	a := ctx.state.assigneePicker
	switch event.Key() {
	case tcell.KeyEscape:
		a.stopDebounce()
		ctx.state.assigneePicker = nil
		return nil
	case tcell.KeyUp:
		a.moveSelection(-1)
		return nil
	case tcell.KeyDown:
		a.moveSelection(1)
		return nil
	case tcell.KeyEnter:
		items := a.filtered()
		if a.selected >= 0 && a.selected < len(items) {
			selectedUser := items[a.selected]
			a.stopDebounce()
			ctx.state.assigneePicker = nil
			executeAssign(ctx, selectedUser.AccountID, selectedUser.DisplayName, a.issueKey)
		}
		return nil
	case tcell.KeyCtrlU:
		a.stopDebounce()
		ctx.state.assigneePicker = nil
		executeAssign(ctx, "", "Unassigned", a.issueKey)
		return nil
	case tcell.KeyCtrlC:
		ctx.app.Stop()
		return nil
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		a.backspace()
		if len(a.query) < 3 {
			a.resetToBoardUsers()
		}
		return nil
	case tcell.KeyRune:
		a.typeRune(event.Rune())
		if len(a.query) >= 3 {
			startAssigneeSearch(ctx, a)
		} else {
			a.resetToBoardUsers()
		}
		return nil
	}
	return nil
}

func executeAssign(ctx *appContext, accountID, displayName, issueKey string) {
	ctx.state.updateAssignee(issueKey, displayName)
	ctx.state.statusMsg = fmt.Sprintf(" Assigning %s → %s…", issueKey, displayName)

	go func() {
		aid := accountID
		if aid == "" && displayName == "Unassigned" {
			aid = "-1"
		} else if aid == "" && displayName != "Unassigned" {
			users, err := ctx.client.SearchAssignableUsers(issueKey, displayName)
			if err != nil {
				ctx.app.QueueUpdateDraw(func() {
					ctx.state.statusMsg = fmt.Sprintf(" Error: %s", err.Error())
					startSync(ctx)
				})
				return
			}
			for _, u := range users {
				if u.DisplayName == displayName {
					aid = u.AccountID
					break
				}
			}
			if aid == "" {
				ctx.app.QueueUpdateDraw(func() {
					ctx.state.statusMsg = fmt.Sprintf(" Error: user %q not found", displayName)
					startSync(ctx)
				})
				return
			}
		}
		err := ctx.client.AssignIssue(issueKey, aid)
		if err != nil {
			ctx.app.QueueUpdateDraw(func() {
				ctx.state.statusMsg = fmt.Sprintf(" Error: %s", err.Error())
				startSync(ctx)
			})
			return
		}
		ctx.client.UpdateCachedAssignee(ctx.boardID, issueKey, displayName)
		ctx.app.QueueUpdateDraw(func() {
			ctx.state.statusMsg = fmt.Sprintf(" %s → %s", issueKey, displayName)
		})
	}()
}

// ── create issue ───────────────────────────────────────────────────────────

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

func openCreateSubtask(ctx *appContext, parentKey string) {
	c := newCreateIssueState(ctx.state.projectKey)
	c.parentKey = parentKey
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
		if c.field == cfEpic {
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
		if c.field == cfButtons {
			return nil
		}
		c.typeRune(event.Rune())
		if c.field == cfEpic {
			startEpicSearch(ctx, c)
		}
		return nil
	}
	return nil
}

func handleCreateUp(c *createIssueState) {
	if c.field == cfButtons {
		if c.isSubtask() {
			c.field = cfDescription
		} else {
			c.field = cfEpic
		}
		c.clampCur()
		return
	}
	if c.field == cfEpic && c.epicKey == "" {
		if c.epicSel > 0 {
			c.epicSel--
		} else {
			c.field = cfDescription
			c.clampCur()
		}
	} else if c.field == cfDescription {
		lines := descLines(c.desc)
		curLine := c.descCurLine(lines)
		if curLine > 0 {
			c.descMoveUp(lines)
			c.descAutoScroll()
		} else if c.descScroll > 0 {
			c.descScroll--
		} else {
			c.field = cfSummary
			c.clampCur()
		}
	} else {
		c.prevField()
		c.clampCur()
	}
}

func handleCreateDown(c *createIssueState) {
	if c.field == cfButtons {
		c.field = cfType
		c.clampCur()
		return
	}
	if c.field == cfEpic && c.epicKey == "" {
		items := c.filteredEpics()
		if c.epicSel < len(items)-1 {
			c.epicSel++
		} else {
			c.field = cfButtons
			c.clampCur()
		}
	} else if c.field == cfDescription {
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
	if c.field == cfButtons {
		c.btnIdx = 0
		return
	}
	if c.field == cfType {
		c.cycleType(-1)
		return
	}
	switch c.field {
	case cfSummary:
		if c.sumCur > 0 {
			c.sumCur--
		}
	case cfEpic:
		if c.epicCur > 0 {
			c.epicCur--
		}
	case cfDescription:
		if c.descCur > 0 {
			c.descCur--
			c.descAutoScroll()
		}
	}
}

func handleCreateRight(c *createIssueState) {
	if c.field == cfButtons {
		c.btnIdx = 1
		return
	}
	if c.field == cfType {
		c.cycleType(1)
		return
	}
	switch c.field {
	case cfSummary:
		if c.sumCur < len([]rune(c.summary)) {
			c.sumCur++
		}
	case cfEpic:
		if c.epicCur < len([]rune(c.epicQuery)) {
			c.epicCur++
		}
	case cfDescription:
		if c.descCur < len([]rune(c.desc)) {
			c.descCur++
			c.descAutoScroll()
		}
	}
}

func handleCreateEnter(ctx *appContext, c *createIssueState) *tcell.EventKey {
	if c.field == cfButtons {
		if c.btnIdx == 0 {
			executeCreateIssue(ctx)
		} else {
			c.stopDebounce()
			ctx.state.createIssue = nil
		}
		return nil
	}
	if c.field == cfEpic {
		c.handleEnter()
		return nil
	}
	if c.field == cfDescription {
		c.handleNewline()
		return nil
	}
	executeCreateIssue(ctx)
	return nil
}

func handleCreateHome(c *createIssueState) {
	switch c.field {
	case cfSummary:
		c.sumCur = 0
	case cfDescription:
		c.descCur = 0
		c.descAutoScroll()
	case cfEpic:
		c.epicCur = 0
	}
}

func handleCreateEnd(c *createIssueState) {
	switch c.field {
	case cfSummary:
		c.sumCur = len([]rune(c.summary))
	case cfDescription:
		c.descCur = len([]rune(c.desc))
		c.descAutoScroll()
	case cfEpic:
		c.epicCur = len([]rune(c.epicQuery))
	}
}

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
		if c.isSubtask() {
			result, err = ctx.client.CreateSubtask(pk, typeID, summary, desc, c.parentKey)
		} else {
			result, err = ctx.client.CreateIssue(pk, typeID, summary, desc)
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

// ── history ────────────────────────────────────────────────────────────────

func openHistory(ctx *appContext) {
	h := &historyState{loading: true}
	ctx.state.history = h

	go func() {
		items, err := ctx.client.SearchRecentActivity(ctx.state.accountID, historyDays)
		ctx.app.QueueUpdateDraw(func() {
			if ctx.state.history != h {
				return
			}
			h.loading = false
			if err != nil {
				h.err = err.Error(); slog.Error("history load failed", "error", err)
				return
			}
			h.items = items
		})
	}()
}

func handleHistoryInput(ctx *appContext, event *tcell.EventKey) *tcell.EventKey {
	h := ctx.state.history
	switch event.Key() {
	case tcell.KeyEscape:
		ctx.state.history = nil
		return nil
	case tcell.KeyRune:
		if event.Rune() == 'q' {
			ctx.state.history = nil
			return nil
		}
	case tcell.KeyUp:
		if h.selected > 0 {
			h.selected--
		}
		return nil
	case tcell.KeyDown:
		if h.selected < len(h.items)-1 {
			h.selected++
		}
		return nil
	case tcell.KeyPgUp:
		h.selected = max(0, h.selected-10)
		return nil
	case tcell.KeyPgDn:
		h.selected = min(len(h.items)-1, h.selected+10)
		return nil
	case tcell.KeyHome:
		h.selected = 0
		return nil
	case tcell.KeyEnd:
		h.selected = max(0, len(h.items)-1)
		return nil
	case tcell.KeyEnter:
		if h.selected >= 0 && h.selected < len(h.items) {
			key := h.items[h.selected].Key
			summary := h.items[h.selected].Summary
			ctx.state.history = nil
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
				})
			}()
		}
		return nil
	case tcell.KeyCtrlC:
		ctx.app.Stop()
		return nil
	}
	return nil
}

// injectCreatedIssue adds a newly created issue to the in-memory board state
// and persists it to the cache file.  The entry's AddedAt timestamp ensures
// the next incremental sync protects it from deletion if Jira's search index
// hasn't caught up yet.
func injectCreatedIssue(ctx *appContext, card jira.Card) {
	for i := range ctx.state.data.Columns {
		if strings.EqualFold(ctx.state.data.Columns[i].Name, card.Status) {
			ctx.state.data.Columns[i].Issues = append(ctx.state.data.Columns[i].Issues, card)
			if ctx.state.cardIdx != nil && i < len(ctx.state.cardIdx) {
				ctx.state.cardIdx[i] = 0
			}
			ctx.state.selectCardByKey(card.Key)
			break
		}
	}
	store, loadErr := cache.Load(ctx.boardID)
	if loadErr != nil {
		slog.Warn("failed to load cache for inject", "error", loadErr)
		return
	}
	store.UpsertEntry(cache.Entry{
		Key:      card.Key,
		Summary:  card.Summary,
		StatusID: card.StatusID,
		Status:   card.Status,
		Assignee: card.Assignee,
		Labels:   card.Labels,
		Epic:     card.Epic,
		Rank:     card.Rank,
	})
	if saveErr := store.Save(); saveErr != nil {
		slog.Warn("failed to save cache after inject", "error", saveErr)
	}
}
