package ui

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/raulvc/jira-kanban/internal/jira"
)

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
