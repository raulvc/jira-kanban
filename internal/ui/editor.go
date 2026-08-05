package ui

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/raulvc/jira-kanban/internal/jira"
)

// openDescriptionInEditor suspends the TUI, launches $EDITOR (or vi) on a
// temp file pre-filled with the issue's plain-text description, and saves
// the edited text back to Jira if it changed.
func openDescriptionInEditor(ctx *appContext) {
	d := ctx.state.detail
	if d == nil || d.loading {
		return
	}

	origDesc := d.card.Description

	ok := ctx.app.Suspend(func() {
		editDescriptionExternal(ctx, d.card.Key, origDesc)
	})
	if !ok {
		slog.Error("failed to suspend TUI for external editor")
	}
}

func editDescriptionExternal(ctx *appContext, key, origDesc string) {
	tmp, err := os.CreateTemp("", "jira-desc-*.md")
	if err != nil {
		slog.Error("failed to create temp file for editor", "error", err)
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.WriteString(origDesc); err != nil {
		_ = tmp.Close()
		slog.Error("failed to write description to temp file", "error", err)
		return
	}
	_ = tmp.Close()

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	cmd := exec.Command(editor, tmpPath) //nolint:gosec // editor path comes from user's own $EDITOR env var
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		slog.Error("editor exited with error", "editor", editor, "error", err)
		return
	}

	edited, err := os.ReadFile(tmpPath)
	if err != nil {
		slog.Error("failed to read edited description", "error", err)
		return
	}

	newDesc := strings.TrimRight(string(edited), "\n")
	if newDesc == origDesc {
		return
	}

	go func() {
		desc := newDesc
		edit := jira.IssueEdit{Description: &desc}
		err := ctx.client.EditIssue(key, edit)
		ctx.app.QueueUpdateDraw(func() {
			if err != nil {
				ctx.state.statusMsg = fmt.Sprintf(" Edit error: %s", err)
				return
			}
			ctx.client.UpdateCachedIssue(ctx.boardID, key, nil, &desc, nil, nil)
			ctx.state.updateIssue(key, nil, &desc, nil, nil)
			if ctx.state.detail != nil && ctx.state.detail.card.Key == key {
				ctx.state.detail.card.Description = desc
				ctx.state.detail.card.RichDesc = nil
			}
			ctx.state.statusMsg = fmt.Sprintf(" Updated %s description", key)
		})
		if err == nil {
			ctx.app.QueueUpdateDraw(func() {
				go func() {
					full, fetchErr := ctx.client.GetIssue(key)
					ctx.app.QueueUpdateDraw(func() {
						if fetchErr != nil {
							return
						}
						ctx.state.updateIssue(key, nil, &full.Description, nil, nil)
						if ctx.state.detail != nil && ctx.state.detail.card.Key == key {
							ctx.state.detail.card.Description = full.Description
							ctx.state.detail.card.RichDesc = full.RichDesc
						}
					})
				}()
			})
		}
	}()
}
