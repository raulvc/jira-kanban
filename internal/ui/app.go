package ui

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/raulvc/jira-kanban/internal/jira"
)

// appContext bundles the dependencies needed by input handlers.
type appContext struct {
	app         *tview.Application
	state       *boardState
	client      *jira.Client
	boardID     int
	baseURL     string
	syncTicker  *time.Ticker
}

// Run starts the interactive TUI for the given board data.
// When needsSync is true the board was loaded from cache and a
// background refresh is triggered immediately after the first render.
func Run(client *jira.Client, boardID int, data jira.Board, baseURL string, needsSync bool) error {
	app := tview.NewApplication()
	state := newBoardState(data)
	ctx := &appContext{
		app:     app,
		state:   state,
		client:  client,
		boardID: boardID,
		baseURL: baseURL,
	}

	box := tview.NewBox().SetBackgroundColor(colBg)
	box.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		drawBoard(screen, state, boardID, x, y, width, height)
		return x, y, width, height
	})

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if state.modal != nil {
			return handleModalInput(ctx, event)
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
		openIssueView(ctx)
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

// ── actions ─────────────────────────────────────────────────────────────────

func openTransitionModal(ctx *appContext) {
	card := ctx.state.selectedCard()
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

func openIssueView(ctx *appContext) {
	card := ctx.state.selectedCard()
	if card == nil {
		return
	}
	ctx.app.Suspend(func() {
		c := exec.Command("jira", "issue", "view", card.Key) //nolint:gosec // key comes from the Jira API, not user input
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		_ = c.Run()
	})
}

func openIssueBrowser(ctx *appContext) {
	card := ctx.state.selectedCard()
	if card == nil {
		return
	}
	issueURL := fmt.Sprintf("%s/browse/%s", ctx.baseURL, card.Key)
	c := exec.Command("xdg-open", issueURL) //nolint:gosec // URL is constructed from config base URL + Jira key
	_ = c.Start()
}
