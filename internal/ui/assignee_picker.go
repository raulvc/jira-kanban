package ui

import (
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/raulvc/jira-kanban/internal/jira"
)

// assigneePickerState tracks the assignee selection modal.
// A nil assigneePickerState means the modal is closed.
type assigneePickerState struct {
	issueKey      string
	boardUsers   []jira.AssignableUser
	users        []jira.AssignableUser
	boardNames   map[string]bool
	query        string
	selected     int
	errMsg       string
	debounce     *time.Timer
}

func newAssigneePickerState(issueKey, currentAssignee, currentUser string, boardData jira.Board) *assigneePickerState {
	boardNames := make(map[string]bool)
	for _, col := range boardData.Columns {
		for _, c := range col.Issues {
			if c.Assignee != "" && c.Assignee != "Unassigned" {
				boardNames[c.Assignee] = true
			}
		}
	}

	users := make([]jira.AssignableUser, 0, len(boardNames))
	for name := range boardNames {
		users = append(users, jira.AssignableUser{DisplayName: name})
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].DisplayName < users[j].DisplayName
	})

	selected := 0
	if currentAssignee != "" && currentAssignee != "Unassigned" {
		for i, u := range users {
			if u.DisplayName == currentAssignee {
				selected = i
				break
			}
		}
	} else if currentUser != "" {
		for i, u := range users {
			if u.DisplayName == currentUser {
				selected = i
				break
			}
		}
	}

	return &assigneePickerState{
		issueKey:    issueKey,
		boardUsers:  users,
		users:       users,
		boardNames:  boardNames,
		selected:    selected,
	}
}

func (a *assigneePickerState) stopDebounce() {
	if a.debounce != nil {
		a.debounce.Stop()
		a.debounce = nil
	}
}

func (a *assigneePickerState) filtered() []jira.AssignableUser {
	if a.query == "" {
		return a.users
	}
	q := toLower(a.query)
	var out []jira.AssignableUser
	for _, u := range a.users {
		if containsLower(u.DisplayName, q) {
			out = append(out, u)
		}
	}
	return out
}

func (a *assigneePickerState) moveSelection(delta int) {
	items := a.filtered()
	if len(items) == 0 {
		return
	}
	a.selected += delta
	a.selected = max(0, min(a.selected, len(items)-1))
}

func (a *assigneePickerState) typeRune(r rune) {
	a.query += string(r)
	a.selected = 0
}

func (a *assigneePickerState) backspace() {
	if len(a.query) > 0 {
		runes := []rune(a.query)
		a.query = string(runes[:len(runes)-1])
		items := a.filtered()
		if a.selected >= len(items) && len(items) > 0 {
			a.selected = len(items) - 1
		}
	}
}

// resetToBoardUsers discards any API-fetched users, keeping only board members.
func (a *assigneePickerState) resetToBoardUsers() {
	a.users = a.boardUsers
}

// mergeAPIResults merges fetched API users into the picker:
// Board members get their accountID filled in; non-board users are appended.
func (a *assigneePickerState) mergeAPIResults(apiUsers []jira.AssignableUser) {
	apiByName := make(map[string]*jira.AssignableUser, len(apiUsers))
	for i := range apiUsers {
		apiByName[apiUsers[i].DisplayName] = &apiUsers[i]
	}

	// Fill accountIDs for board-member entries.
	for i := range a.boardUsers {
		if u, ok := apiByName[a.boardUsers[i].DisplayName]; ok {
			a.boardUsers[i].AccountID = u.AccountID
		}
	}

	// Also fill in current users list (may be same slice).
	for i := range a.users {
		if u, ok := apiByName[a.users[i].DisplayName]; ok {
			a.users[i].AccountID = u.AccountID
		}
	}

	// Append API users not already in the list (non-board members).
	seen := make(map[string]bool, len(a.users))
	for _, u := range a.users {
		seen[u.DisplayName] = true
	}
	var extras []jira.AssignableUser
	for _, u := range apiUsers {
		if !seen[u.DisplayName] {
			extras = append(extras, u)
		}
	}
	sort.Slice(extras, func(i, j int) bool {
		return extras[i].DisplayName < extras[j].DisplayName
	})
	a.users = append(a.users, extras...)
}

// drawAssigneePicker renders the assignee picker modal.
func drawAssigneePicker(screen tcell.Screen, a *assigneePickerState, screenW, screenH int, currentUser string) {
	const padding = 2
	contentW := 44
	items := a.filtered()
	contentH := 5 + len(items)
	if a.errMsg != "" {
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

	bgStyle := tcell.StyleDefault.Foreground(T().Fg).Background(T().Panel)
	borderStyle := tcell.StyleDefault.Foreground(T().Muted).Background(T().Panel)
	titleStyle := tcell.StyleDefault.Foreground(T().Blue).Background(T().Panel).Bold(true)
	searchStyle := tcell.StyleDefault.Foreground(T().Fg).Background(T().Bg)
	searchPlaceholder := tcell.StyleDefault.Foreground(T().Muted).Background(T().Bg)
	itemStyle := tcell.StyleDefault.Foreground(T().Fg).Background(T().Panel)
	selStyle := tcell.StyleDefault.Foreground(T().Fg).Background(T().CardSel).Bold(true)
	meStyle := tcell.StyleDefault.Foreground(T().Gold).Background(T().Panel).Bold(true)
	errStyle := tcell.StyleDefault.Foreground(T().Red).Background(T().Panel).Bold(true)

	for row := oy; row < oy+boxH; row++ {
		fillRow(screen, ox, row, boxW, bgStyle)
	}
	drawBorder(screen, ox, oy, boxW, boxH, borderStyle)

	cy := oy + 1
	drawText(screen, ox+padding, cy, " Assign to: "+a.issueKey+" ", titleStyle, contentW)
	cy++

	fillRow(screen, ox+padding, cy, contentW, searchStyle)
	placeholder := " Search…"
	if a.query == "" {
		drawText(screen, ox+padding, cy, placeholder, searchPlaceholder, contentW)
	} else {
		drawText(screen, ox+padding, cy, " "+a.query+"▏", searchStyle, contentW)
	}
	cy++
	cy++

	if a.errMsg != "" {
		drawText(screen, ox+padding, cy, " "+truncStr(a.errMsg, contentW-2), errStyle, contentW)
	} else {
		maxVisible := max(boxH-6, 1)

		scrollStart := 0
		if a.selected >= scrollStart+maxVisible {
			scrollStart = a.selected - maxVisible + 1
		}
		if a.selected < scrollStart {
			scrollStart = a.selected
		}

		for i := scrollStart; i < len(items) && i < scrollStart+maxVisible; i++ {
			style := itemStyle
			prefix := "  "
			if items[i].DisplayName == currentUser {
				style = meStyle
				prefix = "★ "
			}
			if i == a.selected {
				style = selStyle
				prefix = "▸ "
				if items[i].DisplayName == currentUser {
					prefix = "▸★"
				}
			}
			fillRow(screen, ox+1, cy, boxW-2, style)
			name := items[i].DisplayName
			drawText(screen, ox+padding, cy, prefix+truncStr(name, contentW-3), style, contentW)
			cy++
		}

		if len(items) == 0 {
			drawText(screen, ox+padding, cy, "  No matches", searchPlaceholder, contentW)
		}
	}

	btnY := oy + boxH - 2
	fillRow(screen, ox+1, btnY, boxW-2, bgStyle)
	okStyle := tcell.StyleDefault.Foreground(T().BadgeFg).Background(T().Green).Bold(true)
	unassignStyle := tcell.StyleDefault.Foreground(T().BadgeFg).Background(T().Orange).Bold(true)
	cancelStyle := tcell.StyleDefault.Foreground(T().Fg).Background(T().Muted)
	okText := " Enter "
	unassignText := " Ctrl+U unassign "
	cancelText := " Esc "
	gap := 1
	totalBtnW := len([]rune(okText)) + gap + len([]rune(unassignText)) + gap + len([]rune(cancelText))
	btnX := ox + (boxW-totalBtnW)/2
	drawText(screen, btnX, btnY, okText, okStyle, contentW)
	btnX += len([]rune(okText)) + gap
	drawText(screen, btnX, btnY, unassignText, unassignStyle, contentW)
	btnX += len([]rune(unassignText)) + gap
	drawText(screen, btnX, btnY, cancelText, cancelStyle, contentW)
}// ── assignee picker ────────────────────────────────────────────────────────

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
