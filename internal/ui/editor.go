package ui

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	"github.com/raulvc/jira-kanban/internal/jira"
)

// sanitizeEditorText cleans text read from an external editor file:
// strips BOM, normalizes \r\n to \n, and trims trailing newlines.
func sanitizeEditorText(s string) string {
	s = strings.TrimPrefix(s, "\uFEFF")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimRight(s, "\n")
}

// openCreateDescEditor opens $EDITOR with the current description text from
// the create-issue form. The edited text is loaded back into the form state.
func openCreateDescEditor(ctx *appContext, c *createIssueState) {
	origDesc := c.desc
	ok := ctx.app.Suspend(func() {
		edited, changed := editInEditor(origDesc)
		if !changed {
			return
		}
		// Update form state directly — we're still on the main goroutine
		// inside Suspend, so QueueUpdateDraw would deadlock.
		if ctx.state.createIssue == c {
			c.desc = edited
			c.descCur = len([]rune(edited))
			c.descAutoScroll()
		}
	})
	if !ok {
		slog.Error("failed to suspend TUI for editor")
	}
}

// editInEditor opens $EDITOR on a temp file pre-filled with text, returns
// the edited text and whether it changed.
func editInEditor(text string) (string, bool) {
	tmp, err := os.CreateTemp("", "jira-desc-*.md")
	if err != nil {
		slog.Error("failed to create temp file for editor", "error", err)
		return text, false
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.WriteString(text); err != nil {
		_ = tmp.Close()
		slog.Error("failed to write to temp file", "error", err)
		return text, false
	}
	_ = tmp.Close()

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	cmd := exec.Command(editor, tmpPath) //nolint:gosec // user's own $EDITOR
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		slog.Error("editor exited with error", "editor", editor, "error", err)
		return text, false
	}

	edited, err := os.ReadFile(tmpPath)
	if err != nil {
		slog.Error("failed to read edited file", "error", err)
		return text, false
	}

	newDesc := sanitizeEditorText(string(edited))
	return newDesc, newDesc != text
}

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
		newDesc, changed := editInEditor(origDesc)
		if !changed {
			return
		}
		go func() {
			desc := newDesc
			edit := jira.IssueEdit{Description: &desc}
			err := ctx.client.EditIssue(d.card.Key, edit)
			ctx.app.QueueUpdateDraw(func() {
				if err != nil {
					ctx.state.statusMsg = fmt.Sprintf(" Edit error: %s", err)
					return
				}
				ctx.client.UpdateCachedIssue(ctx.boardID, d.card.Key, nil, &desc, nil, nil)
				ctx.state.updateIssue(d.card.Key, nil, &desc, nil, nil)
				if ctx.state.detail != nil && ctx.state.detail.card.Key == d.card.Key {
					ctx.state.detail.card.Description = desc
					ctx.state.detail.card.RichDesc = nil
				}
				ctx.state.statusMsg = fmt.Sprintf(" Updated %s description", d.card.Key)
			})
			if err == nil {
				ctx.app.QueueUpdateDraw(func() {
					go func() {
						full, fetchErr := ctx.client.GetIssue(d.card.Key)
						ctx.app.QueueUpdateDraw(func() {
							if fetchErr != nil {
								return
							}
							ctx.state.updateIssue(d.card.Key, nil, &full.Description, nil, nil)
							if ctx.state.detail != nil && ctx.state.detail.card.Key == d.card.Key {
								ctx.state.detail.card.Description = full.Description
								ctx.state.detail.card.RichDesc = full.RichDesc
							}
						})
					}()
				})
			}
		}()
	})
	if !ok {
		slog.Error("failed to suspend TUI for external editor")
	}
}
