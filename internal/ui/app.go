package ui

import (
	"fmt"
	"log/slog"
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
		_ = err
	}

	app := tview.NewApplication()
	state := newBoardState(data)
	loadUIPrefs(&state.hideEmpty)
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
		if state.editIssue != nil {
			return handleEditIssueInput(ctx, event)
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
