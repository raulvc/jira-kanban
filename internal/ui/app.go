package ui

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/raulvc/jira-kanban/internal/jira"
)

// Run starts the interactive TUI for the given board data.
// It blocks until the user quits.
func Run(client *jira.Client, boardID int, data jira.Board, baseURL string) error {
	app := tview.NewApplication()
	state := newBoardState(data)

	box := tview.NewBox().SetBackgroundColor(colBg)
	box.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		drawBoard(screen, state, boardID, x, y, width, height)
		return x, y, width, height
	})

	refresh := func() {
		state.statusMsg = " Reloading…"
		app.ForceDraw()
		go func() {
			newData, err := client.FetchBoard(boardID)
			app.QueueUpdateDraw(func() {
				if err != nil {
					state.statusMsg = fmt.Sprintf(" Error: %s", err.Error())
					return
				}
				state.reload(newData)
			})
		}()
	}

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		return handleInput(app, state, refresh, baseURL, event)
	})

	return app.SetRoot(box, true).EnableMouse(false).Run()
}

func handleInput(app *tview.Application, s *boardState, refresh func(), baseURL string, event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyCtrlC:
		app.Stop()
		return nil
	case tcell.KeyLeft:
		s.moveColumn(-1)
		return nil
	case tcell.KeyRight:
		s.moveColumn(1)
		return nil
	case tcell.KeyUp:
		s.moveCard(-1)
		return nil
	case tcell.KeyDown:
		s.moveCard(1)
		return nil
	case tcell.KeyHome:
		s.jumpCard(false)
		return nil
	case tcell.KeyEnd:
		s.jumpCard(true)
		return nil
	case tcell.KeyEnter:
		openIssueView(app, s)
		return nil
	case tcell.KeyRune:
		return handleRune(app, s, refresh, baseURL, event)
	}
	return event
}

func handleRune(app *tview.Application, s *boardState, refresh func(), baseURL string, event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'q':
		app.Stop()
		return nil
	case 'o':
		openIssueBrowser(s, baseURL)
		return nil
	case 'r':
		refresh()
		return nil
	}
	return event
}

func openIssueView(app *tview.Application, s *boardState) {
	card := s.selectedCard()
	if card == nil {
		return
	}
	app.Suspend(func() {
		c := exec.Command("jira", "issue", "view", card.Key) //nolint:gosec // key comes from the Jira API, not user input
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		_ = c.Run()
	})
}

func openIssueBrowser(s *boardState, baseURL string) {
	card := s.selectedCard()
	if card == nil {
		return
	}
	issueURL := fmt.Sprintf("%s/browse/%s", baseURL, card.Key)
	c := exec.Command("xdg-open", issueURL) //nolint:gosec // URL is constructed from config base URL + Jira key
	_ = c.Start()
}
